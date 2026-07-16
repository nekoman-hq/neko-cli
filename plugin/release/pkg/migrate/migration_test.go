//nolint:staticcheck // Migration tests intentionally exercise deprecated V1 inputs.
package migrate

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/validate"
)

const v1Fixture = `{
  "project-name": "example",
  "project-owner": "example-owner",
  "project-type": "backend",
  "release-system": "jreleaser",
  "version": "1.2.3"
}`

func TestRunMigratesRootV1ToV2(t *testing.T) {
	root := withGitRepo(t)
	writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)
	originalBytes, err := os.ReadFile(filepath.Join(root, releaseconfig.V1FileName))
	if err != nil {
		t.Fatalf("read original: %v", err)
	}

	plan, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run migrate: %v", err)
	}
	if plan.UnitID != "default" || plan.Version != "1.2.3" || plan.TagPrefix != "v" || plan.Executor != "jreleaser" {
		t.Fatalf("unexpected plan: %#v", plan)
	}

	if exists(filepath.Join(root, releaseconfig.V1FileName)) {
		t.Fatal("active V1 config still exists")
	}
	backupBytes, err := os.ReadFile(filepath.Join(root, backupFileName))
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backupBytes) != string(originalBytes) {
		t.Fatalf("backup is not byte-identical to original")
	}
	if exists(filepath.Join(root, releaseconfig.V2Directory, journalFileName)) {
		t.Fatal("journal still exists after successful migration")
	}

	assertV2Config(t, releaseconfig.V2ConfigPath(root))
	assertV2State(t, releaseconfig.V2StatePath(root))

	withChdir(t, root)
	resp, err := validate.HandleValidate(plugin.Request{Flags: map[string]any{"show": true}, Context: plugin.Context{WorkingDir: root}})
	if err != nil {
		t.Fatalf("validate after migration: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected validate success, got %#v", resp.Error)
	}
}

func TestRunDryRunIsReadOnlyAndShowsContent(t *testing.T) {
	root := withGitRepo(t)
	writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)

	plan, err := Run(root, true)
	if err != nil {
		t.Fatalf("Run dry-run: %v", err)
	}
	if plan.ConfigJSON == "" || plan.StateJSON == "" {
		t.Fatalf("dry-run did not include planned JSON: %#v", plan)
	}
	if exists(filepath.Join(root, releaseconfig.V2Directory)) {
		t.Fatal("dry-run created .neko directory")
	}
	if !exists(filepath.Join(root, releaseconfig.V1FileName)) {
		t.Fatal("dry-run removed active V1 config")
	}
	if exists(filepath.Join(root, backupFileName)) {
		t.Fatal("dry-run created backup")
	}
}

func TestRunIsNoopWhenAlreadyMigrated(t *testing.T) {
	root := withGitRepo(t)
	writeMigratedV2(t, root)

	plan, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run already migrated: %v", err)
	}
	if !plan.AlreadyDone {
		t.Fatalf("expected already-done plan, got %#v", plan)
	}
}

func TestRunMigrationConflicts(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(root string)
		wantErr string
	}{
		{
			name: "only config",
			setup: func(root string) {
				writeFile(t, releaseconfig.V2ConfigPath(root), validConfigJSON())
			},
			wantErr: "incomplete V2",
		},
		{
			name: "only state",
			setup: func(root string) {
				writeFile(t, releaseconfig.V2StatePath(root), validStateJSON())
			},
			wantErr: "incomplete V2",
		},
		{
			name: "active v1 plus v2",
			setup: func(root string) {
				writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)
				writeMigratedV2(t, root)
			},
			wantErr: "conflict",
		},
		{
			name: "different backup",
			setup: func(root string) {
				writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)
				writeFile(t, filepath.Join(root, backupFileName), "{}")
			},
			wantErr: "backup",
		},
		{
			name: "invalid v1",
			setup: func(root string) {
				writeFile(t, filepath.Join(root, releaseconfig.V1FileName), strings.Replace(v1Fixture, "jreleaser", "custom", 1))
			},
			wantErr: "ReleaseSystem",
		},
		{
			name: "nested v1",
			setup: func(root string) {
				writeFile(t, filepath.Join(root, "api", releaseconfig.V1FileName), v1Fixture)
			},
			wantErr: "nested V1",
		},
		{
			name:    "no config",
			setup:   func(root string) {},
			wantErr: "no release configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := withGitRepo(t)
			tt.setup(root)
			_, err := Run(root, false)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestRunRecoversInterruptedMigration(t *testing.T) {
	stages := []struct { //nolint:govet // Test table readability matters more than field packing.
		name  string
		setup func(root string, plan *Plan, j *journal)
	}{
		{
			name: "journal created",
			setup: func(root string, plan *Plan, j *journal) {
				writeJournalForTest(t, root, j)
			},
		},
		{
			name: "config written",
			setup: func(root string, plan *Plan, j *journal) {
				writeJournalForTest(t, root, j)
				writeFile(t, plan.ConfigPath, plan.ConfigJSON)
				j.Stage = journalStageConfigWritten
				writeJournalForTest(t, root, j)
			},
		},
		{
			name: "state written",
			setup: func(root string, plan *Plan, j *journal) {
				writeJournalForTest(t, root, j)
				writeFile(t, plan.ConfigPath, plan.ConfigJSON)
				writeFile(t, plan.StatePath, plan.StateJSON)
				j.Stage = journalStageStateWritten
				writeJournalForTest(t, root, j)
			},
		},
		{
			name: "v1 archived",
			setup: func(root string, plan *Plan, j *journal) {
				writeJournalForTest(t, root, j)
				writeFile(t, plan.ConfigPath, plan.ConfigJSON)
				writeFile(t, plan.StatePath, plan.StateJSON)
				if err := os.Rename(plan.SourcePath, plan.BackupPath); err != nil {
					t.Fatalf("archive v1: %v", err)
				}
				j.Stage = journalStageV1Archived
				writeJournalForTest(t, root, j)
			},
		},
	}

	for _, tt := range stages {
		t.Run(tt.name, func(t *testing.T) {
			root := withGitRepo(t)
			writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)
			plan, err := ResolvePlan(root)
			if err != nil {
				t.Fatalf("ResolvePlan: %v", err)
			}
			j := journalForPlan(t, plan, journalStagePrepared)
			tt.setup(root, plan, j)

			recovered, err := Run(root, false)
			if err != nil {
				t.Fatalf("Run recovery: %v", err)
			}
			if !recovered.Recovery {
				t.Fatalf("expected recovery plan, got %#v", recovered)
			}
			if exists(filepath.Join(root, releaseconfig.V1FileName)) {
				t.Fatal("active V1 still exists after recovery")
			}
			if !exists(filepath.Join(root, backupFileName)) {
				t.Fatal("backup missing after recovery")
			}
			if exists(filepath.Join(root, releaseconfig.V2Directory, journalFileName)) {
				t.Fatal("journal still exists after recovery")
			}
			assertV2Config(t, releaseconfig.V2ConfigPath(root))
			assertV2State(t, releaseconfig.V2StatePath(root))
		})
	}
}

func TestRunRecoversArchiveCrashBeforeSourceConfirmation(t *testing.T) {
	root := withGitRepo(t)
	writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)
	plan, err := ResolvePlan(root)
	if err != nil {
		t.Fatalf("ResolvePlan: %v", err)
	}
	j := journalForPlan(t, plan, journalStageStateWritten)
	writeFile(t, plan.ConfigPath, plan.ConfigJSON)
	writeFile(t, plan.StatePath, plan.StateJSON)
	if err := os.Rename(plan.SourcePath, plan.BackupPath); err != nil {
		t.Fatalf("archive source before simulated crash: %v", err)
	}
	writeJournalForTest(t, root, j)

	recovered, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run recovery: %v", err)
	}
	if !recovered.Recovery {
		t.Fatalf("expected recovery plan, got %#v", recovered)
	}
	if exists(plan.SourcePath) || !exists(plan.BackupPath) || exists(plan.JournalPath) {
		t.Fatalf("recovery did not close archive-before-confirmation evidence")
	}
	assertFileBytesAndMode(t, plan.BackupPath, []byte(v1Fixture), 0644)
	assertV2Config(t, plan.ConfigPath)
	assertV2State(t, plan.StatePath)
}

func TestRunRecoversBackupVerifiedButActiveSourceRemovalInterrupted(t *testing.T) {
	root := withGitRepo(t)
	writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)
	plan, err := ResolvePlan(root)
	if err != nil {
		t.Fatalf("ResolvePlan: %v", err)
	}
	writeFile(t, plan.ConfigPath, plan.ConfigJSON)
	writeFile(t, plan.StatePath, plan.StateJSON)
	writeFile(t, plan.BackupPath, v1Fixture)
	writeJournalForTest(t, root, journalForPlan(t, plan, journalStageStateWritten))

	recovered, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run recovery: %v", err)
	}
	if !recovered.Recovery {
		t.Fatalf("expected recovery plan, got %#v", recovered)
	}
	if exists(plan.SourcePath) || !exists(plan.BackupPath) || exists(plan.JournalPath) {
		t.Fatalf("recovery did not remove the duplicate active source safely")
	}
	assertFileBytesAndMode(t, plan.BackupPath, []byte(v1Fixture), 0644)
	assertV2Config(t, plan.ConfigPath)
	assertV2State(t, plan.StatePath)
}

func TestRunTreatsCompletedMigrationWithRetainedJournalAsAlreadyCompleted(t *testing.T) {
	root := withGitRepo(t)
	writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)
	plan, err := ResolvePlan(root)
	if err != nil {
		t.Fatalf("ResolvePlan: %v", err)
	}
	j := journalForPlan(t, plan, journalStageV1Archived)
	writeFile(t, plan.ConfigPath, plan.ConfigJSON)
	writeFile(t, plan.StatePath, plan.StateJSON)
	if err := os.Rename(plan.SourcePath, plan.BackupPath); err != nil {
		t.Fatalf("archive source before simulated cleanup crash: %v", err)
	}
	writeJournalForTest(t, root, j)

	recovered, err := Run(root, false)
	if err != nil {
		t.Fatalf("Run recovery: %v", err)
	}
	if !recovered.Recovery {
		t.Fatalf("expected recovery plan, got %#v", recovered)
	}
	if exists(plan.SourcePath) || !exists(plan.BackupPath) || exists(plan.JournalPath) {
		t.Fatalf("recovery did not recognize completed migration evidence")
	}
	assertV2Config(t, plan.ConfigPath)
	assertV2State(t, plan.StatePath)
}

func TestHandleMigrateDryRun(t *testing.T) {
	root := withGitRepo(t)
	writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)

	resp, err := HandleMigrate(plugin.Request{Flags: map[string]any{"dry-run": true}, Context: plugin.Context{WorkingDir: root}})
	if err != nil {
		t.Fatalf("HandleMigrate: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	if resp.Data["config_json"] == "" || resp.Data["state_json"] == "" {
		t.Fatalf("dry-run response missing JSON: %#v", resp.Data)
	}
}

func assertV2Config(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg releaseconfig.V2ReleaseConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if cfg.SchemaVersion != 2 || len(cfg.Units) != 1 {
		t.Fatalf("unexpected config: %s", string(data))
	}
	unit := cfg.Units[0]
	if unit.ID != "default" || unit.DisplayName != "example" || unit.WorkingDirectory != "." || unit.TagPrefix != "v" || unit.Executor.Type != "jreleaser" || unit.Executor.Delivery != "local" {
		t.Fatalf("unexpected unit: %#v", unit)
	}
}

func assertV2State(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var state releaseconfig.V2ReleaseState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	if state.SchemaVersion != 2 || state.Units["default"].Version != "1.2.3" {
		t.Fatalf("unexpected state: %s", string(data))
	}
}

func writeMigratedV2(t *testing.T, root string) {
	t.Helper()
	writeFile(t, releaseconfig.V2ConfigPath(root), validConfigJSON())
	writeFile(t, releaseconfig.V2StatePath(root), validStateJSON())
	writeFile(t, filepath.Join(root, backupFileName), v1Fixture)
}

func validConfigJSON() string {
	return `{
  "schemaVersion": 2,
  "units": [
    {
      "id": "default",
      "displayName": "example",
      "paths": [
        "**"
      ],
      "workingDirectory": ".",
      "tagPrefix": "v",
      "executor": {
        "type": "jreleaser",
        "delivery": "local"
      }
    }
  ]
}
`
}

func validStateJSON() string {
	return `{
  "schemaVersion": 2,
  "units": {
    "default": {
      "version": "1.2.3"
    }
  }
}
`
}

func journalForPlan(t *testing.T, plan *Plan, stage migrationJournalStage) *journal {
	t.Helper()
	sourceBytes, err := os.ReadFile(plan.SourcePath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	return &journal{
		SchemaVersion:       1,
		SourcePath:          plan.SourcePath,
		SourceContentSHA256: sha256Hex(sourceBytes),
		ConfigContentSHA256: sha256Hex([]byte(plan.ConfigJSON)),
		StateContentSHA256:  sha256Hex([]byte(plan.StateJSON)),
		BackupPath:          plan.BackupPath,
		Stage:               stage,
	}
}

func writeJournalForTest(t *testing.T, root string, j *journal) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, releaseconfig.V2Directory), 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		t.Fatalf("marshal journal: %v", err)
	}
	writeFile(t, filepath.Join(root, releaseconfig.V2Directory, journalFileName), string(append(data, '\n')))
}

func withGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitCmd(t, root, "init")
	return root
}

func withChdir(t *testing.T, dir string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s", args, string(out))
	}
}
