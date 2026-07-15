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
)

type migrationJournalStage string

const (
	journalStagePrepared      migrationJournalStage = "prepared"
	journalStageConfigWritten migrationJournalStage = "config-written"
	journalStageStateWritten  migrationJournalStage = "state-written"
	journalStageV1Archived    migrationJournalStage = "v1-archived"
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
	SchemaVersion       int                   `json:"schemaVersion"`
	SourcePath          string                `json:"sourcePath"`
	SourceContentSHA256 string                `json:"sourceContentSHA256"`
	ConfigContentSHA256 string                `json:"configContentSHA256"`
	StateContentSHA256  string                `json:"stateContentSHA256"`
	BackupPath          string                `json:"backupPath"`
	Stage               migrationJournalStage `json:"stage"`
}

// ResolvePlan returns the current migration plan without writing files.
func ResolvePlan(startDir string) (*Plan, error) {
	root, err := (gitMigrationRootResolver{}).Resolve(startDir)
	if err != nil {
		return nil, err
	}
	plan, err := (filesystemMigrationPlanResolver{}).Resolve(root)
	if err != nil {
		return nil, err
	}
	return plan.compatibilityPlan(), nil
}

// Run executes or previews the V1-to-V2 migration.
func Run(startDir string, dryRun bool) (*Plan, error) {
	result, failure := newMigrationUseCase().Migrate(migrationCommandRequest{
		startDirectory: startDir,
		preview:        dryRun,
	})
	if failure != nil {
		return nil, failure
	}
	plan := result.plan.compatibilityPlan()
	if result.outcome == migrationCompleted {
		plan.Actions = append(plan.Actions, "migration completed")
	}
	return plan, nil
}

type gitMigrationRootResolver struct{}

func (gitMigrationRootResolver) Resolve(startDirectory string) (string, error) {
	return gitRoot(startDirectory)
}

type filesystemMigrationPlanResolver struct{}

func (filesystemMigrationPlanResolver) Resolve(root string) (migrationPlan, error) {
	paths := migrationPaths(root)
	evidence := migrationRepositoryEvidence{
		journalExists: exists(paths.journal),
		sourceExists:  exists(paths.source),
		configExists:  exists(paths.config),
		stateExists:   exists(paths.state),
	}
	operation, err := selectMigrationPlanningOperation(classifyMigrationEvidence(evidence))
	if err != nil {
		return migrationPlan{}, err
	}

	switch operation {
	case planInterruptedMigration:
		return resolveRecoveryPlan(root, paths)
	case returnCompletedMigration:
		if _, err := releaseconfig.LoadV2Repository(root); err != nil {
			return migrationPlan{}, err
		}
		return completedMigrationPlan(root, paths), nil
	case refuseIncompleteMigrationTarget:
		return migrationPlan{}, fmt.Errorf("incomplete V2 configuration: both %s and %s are required", paths.config, paths.state)
	case refuseMigrationSourceTargetConflict:
		return migrationPlan{}, fmt.Errorf("migration conflict: active V1 config and V2 files exist without migration journal")
	case planNewMigration:
		return resolveNewMigrationPlan(root, paths)
	case inspectUnsupportedMigrationSource:
		if nested, ok, err := findNestedV1(root, paths.source); err != nil {
			return migrationPlan{}, err
		} else if ok {
			return migrationPlan{}, fmt.Errorf("nested V1 release configuration cannot be migrated as a single-unit repository; create a V2 multi-unit configuration explicitly instead: %s", nested)
		}
		return migrationPlan{}, fmt.Errorf("no release configuration found to migrate in %s", root)
	default:
		return migrationPlan{}, fmt.Errorf("migration recovery failed: unsupported planning operation %d", operation)
	}
}

func resolveRecoveryPlan(root string, paths migrationPathSet) (migrationPlan, error) {
	j, err := loadJournal(paths.journal)
	if err != nil {
		return migrationPlan{}, err
	}
	if j.SchemaVersion != 1 ||
		j.SourcePath != paths.source ||
		j.BackupPath != paths.backup {
		return migrationPlan{}, fmt.Errorf("migration recovery failed: journal %s does not match repository paths", paths.journal)
	}

	plan, err := resolvePlanFromJournal(root, paths, j)
	if err != nil {
		return migrationPlan{}, err
	}
	plan.kind = recoveryMigrationPlan
	plan.actions = recoveryActions(paths, j)
	return plan, nil
}

func resolveNewMigrationPlan(root string, paths migrationPathSet) (migrationPlan, error) {
	source, err := captureMigrationFile(paths.source)
	if err != nil {
		return migrationPlan{}, fmt.Errorf("read V1 config %s: %w", paths.source, err)
	}
	backup, err := captureMigrationFile(paths.backup)
	if err != nil {
		return migrationPlan{}, fmt.Errorf("read V1 backup %s: %w", paths.backup, err)
	}
	return constructMigrationPlan(root, paths, source, backup, newMigrationPlan)
}

func resolvePlanFromJournal(root string, paths migrationPathSet, j *journal) (migrationPlan, error) {
	activeSourceExists := exists(paths.source)
	var source migrationFileSnapshot
	if activeSourceExists {
		activeSource, err := captureMigrationFile(paths.source)
		if err != nil {
			return migrationPlan{}, fmt.Errorf("read V1 config %s: %w", paths.source, err)
		}
		if sha256Hex(activeSource.data) != j.SourceContentSHA256 {
			return migrationPlan{}, fmt.Errorf("migration recovery failed: active V1 config %s does not match journal hash", paths.source)
		}
		source = activeSource
	} else if exists(paths.backup) {
		archivedSource, err := captureMigrationFile(paths.backup)
		if err != nil {
			return migrationPlan{}, fmt.Errorf("read V1 backup %s: %w", paths.backup, err)
		}
		if sha256Hex(archivedSource.data) != j.SourceContentSHA256 {
			return migrationPlan{}, fmt.Errorf("migration recovery failed: V1 backup %s does not match journal hash", paths.backup)
		}
		source = archivedSource
	} else {
		return migrationPlan{}, fmt.Errorf("migration recovery failed: neither active V1 config nor backup exists")
	}

	backup, err := captureMigrationFile(paths.backup)
	if err != nil {
		return migrationPlan{}, fmt.Errorf("read V1 backup %s: %w", paths.backup, err)
	}
	plan, err := constructMigrationPlan(root, paths, source, backup, recoveryMigrationPlan)
	if err != nil {
		return migrationPlan{}, err
	}
	if sha256Hex(plan.target.configJSON) != j.ConfigContentSHA256 ||
		sha256Hex(plan.target.stateJSON) != j.StateContentSHA256 {
		return migrationPlan{}, fmt.Errorf("migration recovery failed: planned V2 content does not match journal hashes")
	}
	if err := verifyExistingIfPresent(paths.config, plan.target.configJSON, "config"); err != nil {
		return migrationPlan{}, err
	}
	if err := verifyExistingIfPresent(paths.state, plan.target.stateJSON, "state"); err != nil {
		return migrationPlan{}, err
	}
	plan.journal = *j
	plan.targetOperation = selectRecoveryTargetOperation(exists(paths.config), exists(paths.state))
	plan.sourceOperation = selectRecoverySourceOperation(activeSourceExists)
	return plan, nil
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
	if err := validateMigrationJournalStage(j.Stage); err != nil {
		return nil, fmt.Errorf("parse migration journal %s: %w", path, err)
	}
	return &j, nil
}

func validateMigrationJournalStage(stage migrationJournalStage) error {
	switch stage {
	case journalStagePrepared, journalStageConfigWritten, journalStageStateWritten, journalStageV1Archived:
		return nil
	default:
		return fmt.Errorf("unknown migration journal stage %q", stage)
	}
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

func captureMigrationFile(path string) (migrationFileSnapshot, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return migrationFileSnapshot{path: path}, nil
	}
	if err != nil {
		return migrationFileSnapshot{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return migrationFileSnapshot{}, err
	}
	return newMigrationFileSnapshot(path, data, info.Mode().Perm()), nil
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
