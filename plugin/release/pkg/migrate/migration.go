// Package migrate implements the conservative V1-to-V2 migration command.
//
//nolint:staticcheck // Migration is the intentional bridge from deprecated V1 APIs to V2 files.
package migrate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const (
	backupFileName  = ".release.neko.json.v1.bak"
	journalFileName = "release.migration.json"

	journalStagePrepared      = "prepared"
	journalStageConfigWritten = "config-written"
	journalStageStateWritten  = "state-written"
	journalStageV1Archived    = "v1-archived"
)

// Plan describes the exact migration actions and content.
type Plan struct {
	RepositoryRoot string
	SourceType     string
	SourcePath     string
	ConfigPath     string
	StatePath      string
	BackupPath     string
	JournalPath    string
	UnitID         string
	Version        string
	TagPrefix      string
	Executor       string
	Delivery       string
	ConfigJSON     string
	StateJSON      string
	Actions        []string
	AlreadyDone    bool
	Recovery       bool
}

//nolint:govet // Journal field order mirrors the documented recovery file.
type journal struct {
	SchemaVersion       int    `json:"schemaVersion"`
	SourcePath          string `json:"sourcePath"`
	SourceContentSHA256 string `json:"sourceContentSHA256"`
	ConfigContentSHA256 string `json:"configContentSHA256"`
	StateContentSHA256  string `json:"stateContentSHA256"`
	BackupPath          string `json:"backupPath"`
	Stage               string `json:"stage"`
}

// ResolvePlan returns the current migration plan without writing files.
func ResolvePlan(startDir string) (*Plan, error) {
	root, err := gitRoot(startDir)
	if err != nil {
		return nil, err
	}
	return resolvePlanAtRoot(root)
}

// Run executes or previews the V1-to-V2 migration.
func Run(startDir string, dryRun bool) (*Plan, error) {
	plan, err := ResolvePlan(startDir)
	if err != nil {
		return nil, err
	}
	if dryRun || plan.AlreadyDone {
		if dryRun && plan.Recovery {
			plan.Actions = append([]string{"preview recovery of interrupted migration"}, plan.Actions...)
		}
		return plan, nil
	}
	if err := executePlan(plan); err != nil {
		return nil, err
	}
	plan.Actions = append(plan.Actions, "migration completed")
	return plan, nil
}

func resolvePlanAtRoot(root string) (*Plan, error) {
	paths := migrationPaths(root)
	journalExists := exists(paths.journal)
	if journalExists {
		return recoveryPlan(root, paths)
	}

	rootV1 := exists(paths.source)
	configExists := exists(paths.config)
	stateExists := exists(paths.state)

	switch {
	case configExists && stateExists && !rootV1:
		if _, err := releaseconfig.LoadV2Repository(root); err != nil {
			return nil, err
		}
		return &Plan{
			RepositoryRoot: root,
			SourceType:     "v2",
			ConfigPath:     paths.config,
			StatePath:      paths.state,
			BackupPath:     paths.backup,
			JournalPath:    paths.journal,
			AlreadyDone:    true,
			Actions:        []string{"already migrated; no changes required"},
		}, nil
	case configExists != stateExists:
		return nil, fmt.Errorf("incomplete V2 configuration: both %s and %s are required", paths.config, paths.state)
	case rootV1 && (configExists || stateExists):
		return nil, fmt.Errorf("migration conflict: active V1 config and V2 files exist without migration journal")
	case rootV1:
		return planFromV1(root, paths, false)
	}

	if nested, ok, err := findNestedV1(root, paths.source); err != nil {
		return nil, err
	} else if ok {
		return nil, fmt.Errorf("nested V1 release configuration cannot be migrated as a single-unit repository; create a V2 multi-unit configuration explicitly instead: %s", nested)
	}

	return nil, fmt.Errorf("no release configuration found to migrate in %s", root)
}

func recoveryPlan(root string, paths migrationPathSet) (*Plan, error) {
	j, err := loadJournal(paths.journal)
	if err != nil {
		return nil, err
	}
	if j.SchemaVersion != 1 ||
		j.SourcePath != paths.source ||
		j.BackupPath != paths.backup {
		return nil, fmt.Errorf("migration recovery failed: journal %s does not match repository paths", paths.journal)
	}

	plan, err := planFromJournal(root, paths, j)
	if err != nil {
		return nil, err
	}
	plan.Recovery = true
	plan.Actions = recoveryActions(paths, j)
	return plan, nil
}

func planFromV1(root string, paths migrationPathSet, recovery bool) (*Plan, error) {
	sourceBytes, err := os.ReadFile(paths.source)
	if err != nil {
		return nil, fmt.Errorf("read V1 config %s: %w", paths.source, err)
	}
	return planFromSourceBytes(root, paths, sourceBytes, recovery)
}

func planFromJournal(root string, paths migrationPathSet, j *journal) (*Plan, error) {
	var sourceBytes []byte
	if exists(paths.source) {
		data, err := os.ReadFile(paths.source)
		if err != nil {
			return nil, fmt.Errorf("read V1 config %s: %w", paths.source, err)
		}
		if sha256Hex(data) != j.SourceContentSHA256 {
			return nil, fmt.Errorf("migration recovery failed: active V1 config %s does not match journal hash", paths.source)
		}
		sourceBytes = data
	} else if exists(paths.backup) {
		data, err := os.ReadFile(paths.backup)
		if err != nil {
			return nil, fmt.Errorf("read V1 backup %s: %w", paths.backup, err)
		}
		if sha256Hex(data) != j.SourceContentSHA256 {
			return nil, fmt.Errorf("migration recovery failed: V1 backup %s does not match journal hash", paths.backup)
		}
		sourceBytes = data
	} else {
		return nil, fmt.Errorf("migration recovery failed: neither active V1 config nor backup exists")
	}

	plan, err := planFromSourceBytes(root, paths, sourceBytes, true)
	if err != nil {
		return nil, err
	}
	if sha256Hex([]byte(plan.ConfigJSON)) != j.ConfigContentSHA256 ||
		sha256Hex([]byte(plan.StateJSON)) != j.StateContentSHA256 {
		return nil, fmt.Errorf("migration recovery failed: planned V2 content does not match journal hashes")
	}
	if err := verifyExistingIfPresent(paths.config, []byte(plan.ConfigJSON), "config"); err != nil {
		return nil, err
	}
	if err := verifyExistingIfPresent(paths.state, []byte(plan.StateJSON), "state"); err != nil {
		return nil, err
	}
	return plan, nil
}

func planFromSourceBytes(root string, paths migrationPathSet, sourceBytes []byte, recovery bool) (*Plan, error) {
	if exists(paths.backup) {
		backupBytes, err := os.ReadFile(paths.backup)
		if err != nil {
			return nil, fmt.Errorf("read V1 backup %s: %w", paths.backup, err)
		}
		if !bytes.Equal(backupBytes, sourceBytes) {
			return nil, fmt.Errorf("migration conflict: existing backup %s differs from active V1 config", paths.backup)
		}
	}

	var v1 releaseconfig.V1ReleaseConfig
	if err := json.Unmarshal(sourceBytes, &v1); err != nil {
		return nil, fmt.Errorf("parse V1 config %s: %w", paths.source, err)
	}
	if err := releaseconfig.V1Validate(&v1); err != nil {
		return nil, err
	}

	v2Config := releaseconfig.V2ReleaseConfig{
		SchemaVersion: 2,
		Units: []releaseconfig.V2Unit{
			{
				ID:               "default",
				DisplayName:      v1.ProjectName,
				Paths:            []string{"**"},
				WorkingDirectory: ".",
				TagPrefix:        "v",
				Executor: releaseconfig.V2Executor{
					Type:     releaseconfig.ExecutorType(v1.ReleaseSystem),
					Delivery: releaseconfig.DeliveryLocal,
				},
			},
		},
	}
	v2State := releaseconfig.V2ReleaseState{
		SchemaVersion: 2,
		Units: map[string]releaseconfig.V2UnitState{
			"default": {Version: v1.Version},
		},
	}
	configBytes, err := releaseconfig.CanonicalV2Config(v2Config)
	if err != nil {
		return nil, err
	}
	stateBytes, err := releaseconfig.CanonicalV2State(v2State)
	if err != nil {
		return nil, err
	}

	plan := &Plan{
		RepositoryRoot: root,
		SourceType:     "v1",
		SourcePath:     paths.source,
		ConfigPath:     paths.config,
		StatePath:      paths.state,
		BackupPath:     paths.backup,
		JournalPath:    paths.journal,
		UnitID:         "default",
		Version:        v1.Version,
		TagPrefix:      "v",
		Executor:       string(v1.ReleaseSystem),
		Delivery:       string(releaseconfig.DeliveryLocal),
		ConfigJSON:     string(configBytes),
		StateJSON:      string(stateBytes),
		Recovery:       recovery,
		Actions: []string{
			"create .neko directory",
			"write migration journal",
			"write .neko/release.config.json",
			"write .neko/release.state.json",
			"archive .release.neko.json to .release.neko.json.v1.bak",
			"validate migrated V2 configuration",
			"remove migration journal",
		},
	}
	return plan, nil
}

func executePlan(plan *Plan) error {
	paths := migrationPaths(plan.RepositoryRoot)
	if err := os.MkdirAll(filepath.Dir(paths.config), 0755); err != nil {
		return fmt.Errorf("create V2 directory %s: %w", filepath.Dir(paths.config), err)
	}

	j := &journal{
		SchemaVersion:       1,
		SourcePath:          paths.source,
		SourceContentSHA256: sourceHashForPlan(plan),
		ConfigContentSHA256: sha256Hex([]byte(plan.ConfigJSON)),
		StateContentSHA256:  sha256Hex([]byte(plan.StateJSON)),
		BackupPath:          paths.backup,
		Stage:               journalStagePrepared,
	}
	if err := writeJournal(paths.journal, j); err != nil {
		return err
	}

	if err := writeExpected(paths.config, []byte(plan.ConfigJSON)); err != nil {
		return err
	}
	j.Stage = journalStageConfigWritten
	if err := writeJournal(paths.journal, j); err != nil {
		return err
	}

	if err := writeExpected(paths.state, []byte(plan.StateJSON)); err != nil {
		return err
	}
	j.Stage = journalStageStateWritten
	if err := writeJournal(paths.journal, j); err != nil {
		return err
	}

	if err := archiveV1(paths, j.SourceContentSHA256); err != nil {
		return err
	}
	j.Stage = journalStageV1Archived
	if err := writeJournal(paths.journal, j); err != nil {
		return err
	}

	if err := validateFinal(plan, j); err != nil {
		return err
	}
	if err := os.Remove(paths.journal); err != nil {
		return fmt.Errorf("remove migration journal %s: %w", paths.journal, err)
	}
	return nil
}

func validateFinal(plan *Plan, j *journal) error {
	repo, err := releaseconfig.LoadV2Repository(plan.RepositoryRoot)
	if err != nil {
		return err
	}
	if len(repo.Units) != 1 {
		return fmt.Errorf("migration validation failed: expected exactly one unit, got %d", len(repo.Units))
	}
	unit := repo.Units[0]
	if unit.ID != "default" ||
		unit.Version != plan.Version ||
		unit.TagPrefix != "v" ||
		unit.ExecutorType != plan.Executor ||
		unit.Delivery != string(releaseconfig.DeliveryLocal) {
		return fmt.Errorf("migration validation failed: unexpected migrated unit: %#v", unit)
	}
	paths := migrationPaths(plan.RepositoryRoot)
	if exists(paths.source) {
		return fmt.Errorf("migration validation failed: active V1 config still exists at %s", paths.source)
	}
	backupBytes, err := os.ReadFile(paths.backup)
	if err != nil {
		return fmt.Errorf("migration validation failed: read backup %s: %w", paths.backup, err)
	}
	if sha256Hex(backupBytes) != j.SourceContentSHA256 {
		return fmt.Errorf("migration validation failed: backup hash mismatch at %s", paths.backup)
	}
	return nil
}

func writeExpected(path string, expected []byte) error {
	if err := verifyExistingIfPresent(path, expected, path); err != nil {
		return err
	}
	if exists(path) {
		return nil
	}
	if err := releaseconfig.AtomicWriteFile(path, expected, 0644); err != nil {
		return err
	}
	return nil
}

func verifyExistingIfPresent(path string, expected []byte, label string) error {
	if !exists(path) {
		return nil
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read existing %s %s: %w", label, path, err)
	}
	if !bytes.Equal(current, expected) {
		return fmt.Errorf("migration conflict: existing %s %s differs from planned content", label, path)
	}
	return nil
}

func archiveV1(paths migrationPathSet, sourceHash string) error {
	if exists(paths.backup) {
		backupBytes, err := os.ReadFile(paths.backup)
		if err != nil {
			return fmt.Errorf("read backup %s: %w", paths.backup, err)
		}
		if sha256Hex(backupBytes) != sourceHash {
			return fmt.Errorf("migration conflict: existing backup %s differs from source", paths.backup)
		}
		if exists(paths.source) {
			sourceBytes, err := os.ReadFile(paths.source)
			if err != nil {
				return fmt.Errorf("read active V1 config %s: %w", paths.source, err)
			}
			if sha256Hex(sourceBytes) != sourceHash {
				return fmt.Errorf("migration recovery failed: active V1 config changed since journal creation")
			}
			if err := os.Remove(paths.source); err != nil {
				return fmt.Errorf("remove already-archived active V1 config %s: %w", paths.source, err)
			}
		}
		return nil
	}

	if !exists(paths.source) {
		return fmt.Errorf("migration recovery failed: active V1 config missing before backup was created")
	}
	sourceBytes, err := os.ReadFile(paths.source)
	if err != nil {
		return fmt.Errorf("read active V1 config %s: %w", paths.source, err)
	}
	if sha256Hex(sourceBytes) != sourceHash {
		return fmt.Errorf("migration recovery failed: active V1 config changed since journal creation")
	}
	if err := os.Rename(paths.source, paths.backup); err != nil {
		return fmt.Errorf("archive V1 config %s to %s: %w", paths.source, paths.backup, err)
	}
	return nil
}

func writeJournal(path string, j *journal) error {
	if err := releaseconfig.AtomicWriteJSON(path, j, 0644); err != nil {
		return fmt.Errorf("write migration journal %s: %w", path, err)
	}
	return nil
}

func loadJournal(path string) (*journal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read migration journal %s: %w", path, err)
	}
	var j journal
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&j); err != nil {
		return nil, fmt.Errorf("parse migration journal %s: %w", path, err)
	}
	return &j, nil
}

func recoveryActions(paths migrationPathSet, j *journal) []string {
	actions := []string{"validate migration journal"}
	if !exists(paths.config) {
		actions = append(actions, "write missing .neko/release.config.json")
	}
	if !exists(paths.state) {
		actions = append(actions, "write missing .neko/release.state.json")
	}
	if exists(paths.source) {
		actions = append(actions, "archive active .release.neko.json")
	}
	actions = append(actions, "validate migrated V2 configuration", "remove migration journal")
	if j.Stage != "" {
		actions = append([]string{fmt.Sprintf("resume from journal stage %s", j.Stage)}, actions...)
	}
	return actions
}

func sourceHashForPlan(plan *Plan) string {
	if data, err := os.ReadFile(plan.SourcePath); err == nil {
		return sha256Hex(data)
	}
	if data, err := os.ReadFile(plan.BackupPath); err == nil {
		return sha256Hex(data)
	}
	return ""
}

type migrationPathSet struct {
	source  string
	config  string
	state   string
	backup  string
	journal string
}

func migrationPaths(root string) migrationPathSet {
	return migrationPathSet{
		source:  filepath.Join(root, releaseconfig.V1FileName),
		config:  releaseconfig.V2ConfigPath(root),
		state:   releaseconfig.V2StatePath(root),
		backup:  filepath.Join(root, backupFileName),
		journal: filepath.Join(root, releaseconfig.V2Directory, journalFileName),
	}
}

func gitRoot(startDir string) (string, error) {
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("determine working directory: %w", err)
		}
	}
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = startDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("determine git repository root from %s: %s", startDir, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func findNestedV1(root, rootV1 string) (string, bool, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if entry.IsDir() || entry.Name() != releaseconfig.V1FileName || path == rootV1 {
			return nil
		}
		found = path
		return filepath.SkipAll
	})
	if err != nil {
		return "", false, fmt.Errorf("scan for nested V1 configs: %w", err)
	}
	return found, found != "", nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
