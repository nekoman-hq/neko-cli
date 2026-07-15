//nolint:staticcheck // Migration contracts intentionally exercise deprecated V1 inputs.
package migrate

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

func TestHandleMigrateDryRunCommandContract(t *testing.T) {
	root := withGitRepo(t)
	sourcePath := filepath.Join(root, releaseconfig.V1FileName)
	writeFile(t, sourcePath, v1Fixture)
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	unrelatedPath := filepath.Join(root, "unrelated.txt")
	writeFile(t, unrelatedPath, "keep me")

	resp, err := HandleMigrate(plugin.Request{
		Flags:   map[string]any{"dry-run": true},
		Context: plugin.Context{WorkingDir: root},
	})
	if err != nil {
		t.Fatalf("HandleMigrate: %v", err)
	}
	if resp.Status != "success" || resp.Error != nil || resp.RendererHint != "table" {
		t.Fatalf("unexpected dry-run response envelope: %#v", resp)
	}
	if resp.Metadata.Plugin != metadata.PluginName || resp.Metadata.Version != metadata.Version || resp.Metadata.Command != "migrate" || resp.Metadata.Timestamp.IsZero() {
		t.Fatalf("unexpected dry-run metadata: %#v", resp.Metadata)
	}

	wantKeys := []string{"actions", "config_json", "items", "state_json"}
	gotKeys := make([]string, 0, len(resp.Data))
	for key := range resp.Data {
		gotKeys = append(gotKeys, key)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("response data keys = %v, want %v", gotKeys, wantKeys)
	}

	items, ok := resp.Data["items"].([]map[string]any)
	if !ok {
		t.Fatalf("items type = %T, want []map[string]any", resp.Data["items"])
	}
	wantItems := []map[string]any{
		{"property": "Status", "value": "dry-run"},
		{"property": "Source Type", "value": "v1"},
		{"property": "Source Path", "value": filepath.Join(resolvedRoot, releaseconfig.V1FileName)},
		{"property": "Config Path", "value": releaseconfig.V2ConfigPath(resolvedRoot)},
		{"property": "State Path", "value": releaseconfig.V2StatePath(resolvedRoot)},
		{"property": "Backup Path", "value": filepath.Join(resolvedRoot, backupFileName)},
		{"property": "Journal Path", "value": filepath.Join(resolvedRoot, releaseconfig.V2Directory, journalFileName)},
		{"property": "Unit ID", "value": "default"},
		{"property": "Version", "value": "1.2.3"},
		{"property": "Tag Prefix", "value": "v"},
		{"property": "Executor", "value": "jreleaser"},
		{"property": "Delivery", "value": "local"},
	}
	if !reflect.DeepEqual(items, wantItems) {
		t.Fatalf("ordered response items changed:\ngot  %#v\nwant %#v", items, wantItems)
	}
	if resp.Data["config_json"] != validConfigJSON() || resp.Data["state_json"] != validStateJSON() {
		t.Fatalf("planned JSON changed: config=%q state=%q", resp.Data["config_json"], resp.Data["state_json"])
	}
	if !exists(sourcePath) || exists(releaseconfig.V2ConfigPath(root)) || exists(releaseconfig.V2StatePath(root)) || exists(filepath.Join(root, backupFileName)) {
		t.Fatalf("dry-run changed migration files")
	}
	assertFileContent(t, unrelatedPath, "keep me")
}

func TestHandleMigrateFailureContractReturnsNilGoError(t *testing.T) {
	root := withGitRepo(t)
	writeFile(t, filepath.Join(root, releaseconfig.V1FileName), "{")

	resp, err := HandleMigrate(plugin.Request{Context: plugin.Context{WorkingDir: root}})
	if err != nil {
		t.Fatalf("handler Go error = %v, want nil compatibility contract", err)
	}
	if resp.Status != "error" || resp.Error == nil || resp.Error.Code != "MIGRATION_FAILED" {
		t.Fatalf("unexpected failure response: %#v", resp)
	}
	if !strings.Contains(resp.Error.Message, "parse V1 config") || resp.Error.Details != nil {
		t.Fatalf("unexpected failure payload: %#v", resp.Error)
	}
	if resp.Metadata.Plugin != metadata.PluginName || resp.Metadata.Version != metadata.Version || resp.Metadata.Command != "migrate" || resp.Metadata.Timestamp.IsZero() {
		t.Fatalf("unexpected failure metadata: %#v", resp.Metadata)
	}
	if resp.Data != nil || resp.RendererHint != "" {
		t.Fatalf("failure response gained success presentation: %#v", resp)
	}
}

func TestHandleMigrateWrongDryRunTypeRetainsFalseDefault(t *testing.T) {
	root := withGitRepo(t)
	writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)

	resp, err := HandleMigrate(plugin.Request{
		Flags:   map[string]any{"dry-run": "true"},
		Context: plugin.Context{WorkingDir: root},
	})
	if err != nil {
		t.Fatalf("HandleMigrate: %v", err)
	}
	items, ok := resp.Data["items"].([]map[string]any)
	if !ok {
		t.Fatalf("items type = %T, want []map[string]any", resp.Data["items"])
	}
	if got := items[0]["value"]; got != "migrated" {
		t.Fatalf("wrongly typed dry-run status = %v, want migrated", got)
	}
	if exists(filepath.Join(root, releaseconfig.V1FileName)) || !exists(filepath.Join(root, backupFileName)) {
		t.Fatal("wrongly typed dry-run flag no longer uses the false default")
	}
}

func TestRecoveryPlanActionsDescribeDurableEvidence(t *testing.T) {
	tests := []struct { //nolint:govet // Contract cases read most clearly with evidence before expected actions.
		name        string
		stage       string
		writeConfig bool
		writeState  bool
		archive     bool
		wantActions []string
	}{
		{
			name:  "prepared",
			stage: journalStagePrepared,
			wantActions: []string{
				"resume from journal stage prepared",
				"validate migration journal",
				"write missing .neko/release.config.json",
				"write missing .neko/release.state.json",
				"archive active .release.neko.json",
				"validate migrated V2 configuration",
				"remove migration journal",
			},
		},
		{
			name:        "config written",
			stage:       journalStageConfigWritten,
			writeConfig: true,
			wantActions: []string{
				"resume from journal stage config-written",
				"validate migration journal",
				"write missing .neko/release.state.json",
				"archive active .release.neko.json",
				"validate migrated V2 configuration",
				"remove migration journal",
			},
		},
		{
			name:        "state written",
			stage:       journalStageStateWritten,
			writeConfig: true,
			writeState:  true,
			wantActions: []string{
				"resume from journal stage state-written",
				"validate migration journal",
				"archive active .release.neko.json",
				"validate migrated V2 configuration",
				"remove migration journal",
			},
		},
		{
			name:        "v1 archived",
			stage:       journalStageV1Archived,
			writeConfig: true,
			writeState:  true,
			archive:     true,
			wantActions: []string{
				"resume from journal stage v1-archived",
				"validate migration journal",
				"validate migrated V2 configuration",
				"remove migration journal",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := withGitRepo(t)
			writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)
			plan, err := ResolvePlan(root)
			if err != nil {
				t.Fatalf("ResolvePlan initial: %v", err)
			}
			j := journalForPlan(t, plan, test.stage)
			writeJournalForTest(t, root, j)
			if test.writeConfig {
				writeFile(t, plan.ConfigPath, plan.ConfigJSON)
			}
			if test.writeState {
				writeFile(t, plan.StatePath, plan.StateJSON)
			}
			if test.archive {
				if archiveErr := os.Rename(plan.SourcePath, plan.BackupPath); archiveErr != nil {
					t.Fatalf("archive source: %v", archiveErr)
				}
			}

			recovery, err := ResolvePlan(root)
			if err != nil {
				t.Fatalf("ResolvePlan recovery: %v", err)
			}
			if !recovery.Recovery || !reflect.DeepEqual(recovery.Actions, test.wantActions) {
				t.Fatalf("recovery actions changed:\ngot  %#v\nwant %#v", recovery.Actions, test.wantActions)
			}
		})
	}
}

func TestRecoveryPlanningCurrentlyAcceptsUnvalidatedJournalStages(t *testing.T) {
	for _, stage := range []string{"", "unknown-future-stage"} {
		t.Run(stage, func(t *testing.T) {
			root := withGitRepo(t)
			writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)
			plan, err := ResolvePlan(root)
			if err != nil {
				t.Fatalf("ResolvePlan initial: %v", err)
			}
			writeJournalForTest(t, root, journalForPlan(t, plan, stage))

			recovery, err := ResolvePlan(root)
			if err != nil {
				t.Fatalf("current unvalidated stage %q was rejected: %v", stage, err)
			}
			if !recovery.Recovery {
				t.Fatalf("stage %q did not produce recovery plan: %#v", stage, recovery)
			}
		})
	}
}

func TestSuccessfulMigrationPreservesSourceBytesAndModeInBackup(t *testing.T) {
	root := withGitRepo(t)
	sourcePath := filepath.Join(root, releaseconfig.V1FileName)
	writeFile(t, sourcePath, v1Fixture)
	if err := os.Chmod(sourcePath, 0600); err != nil {
		t.Fatalf("chmod source: %v", err)
	}
	unrelatedPath := filepath.Join(root, "notes", "keep.txt")
	writeFile(t, unrelatedPath, "unchanged")

	if _, err := Run(root, false); err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertFileBytesAndMode(t, filepath.Join(root, backupFileName), []byte(v1Fixture), 0600)
	assertFileBytesAndMode(t, releaseconfig.V2ConfigPath(root), []byte(validConfigJSON()), 0644)
	assertFileBytesAndMode(t, releaseconfig.V2StatePath(root), []byte(validStateJSON()), 0644)
	assertFileContent(t, unrelatedPath, "unchanged")
}

func assertFileBytesAndMode(t *testing.T, path string, want []byte, wantMode os.FileMode) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s bytes = %q, want %q", path, got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if gotMode := info.Mode().Perm(); gotMode != wantMode {
		t.Fatalf("%s mode = %04o, want %04o", path, gotMode, wantMode)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}
