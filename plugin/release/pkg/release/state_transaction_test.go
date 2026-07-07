package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestStateTransactionUpdatesOnlySelectedUnit(t *testing.T) {
	root := newV2StateTestRepository(t)
	statePath := releaseconfig.V2StatePath(root)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state before: %v", err)
	}

	tx := NewStateTransaction(root)
	if captureErr := tx.CaptureSnapshot(); captureErr != nil {
		t.Fatalf("CaptureSnapshot: %v", captureErr)
	}
	if writeErr := tx.WriteUnitVersion("api", "0.2.1"); writeErr != nil {
		t.Fatalf("WriteUnitVersion: %v", writeErr)
	}

	afterState, err := releaseconfig.LoadV2State(statePath)
	if err != nil {
		t.Fatalf("LoadV2State: %v", err)
	}
	if afterState.Units["api"].Version != "0.2.1" {
		t.Fatalf("api version was not updated: %#v", afterState.Units["api"])
	}
	if afterState.Units["web"].Version != "1.4.0" {
		t.Fatalf("web version changed unexpectedly: %#v", afterState.Units["web"])
	}
	if string(before) == mustReadString(t, statePath) {
		t.Fatal("expected state bytes to change")
	}
	assertNoAtomicTemps(t, filepath.Join(root, ".neko"))
}

func TestStateTransactionSupportsPatchMinorMajor(t *testing.T) {
	tests := []struct {
		releaseType Type
		expected    string
	}{
		{Patch, "0.2.1"},
		{Minor, "0.3.0"},
		{Major, "1.0.0"},
	}

	for _, tt := range tests {
		t.Run(string(tt.releaseType), func(t *testing.T) {
			root := newV2StateTestRepository(t)
			repository, err := releaseconfig.LoadV2Repository(root)
			if err != nil {
				t.Fatalf("LoadV2Repository: %v", err)
			}
			ctx, err := BuildReleaseExecutionContext(repository, repository.Units[0], tt.releaseType, false)
			if err != nil {
				t.Fatalf("BuildReleaseExecutionContext: %v", err)
			}
			tx := NewStateTransaction(root)
			if captureErr := tx.CaptureSnapshot(); captureErr != nil {
				t.Fatalf("CaptureSnapshot: %v", captureErr)
			}
			if writeErr := tx.WriteUnitVersion(ctx.Unit.ID, ctx.NextVersion); writeErr != nil {
				t.Fatalf("WriteUnitVersion: %v", writeErr)
			}
			state, err := releaseconfig.LoadV2State(releaseconfig.V2StatePath(root))
			if err != nil {
				t.Fatalf("LoadV2State: %v", err)
			}
			if state.Units["api"].Version != tt.expected {
				t.Fatalf("expected %s, got %#v", tt.expected, state.Units["api"])
			}
		})
	}
}

func TestStateTransactionRejectsInvalidTargetStateBeforeWrite(t *testing.T) {
	root := newV2StateTestRepository(t)
	statePath := releaseconfig.V2StatePath(root)
	before := mustReadString(t, statePath)
	tx := NewStateTransaction(root)
	if err := tx.CaptureSnapshot(); err != nil {
		t.Fatalf("CaptureSnapshot: %v", err)
	}

	if err := tx.WriteUnitVersion("api", "not-semver"); err == nil {
		t.Fatal("expected invalid semver error")
	}
	if got := mustReadString(t, statePath); got != before {
		t.Fatalf("state changed after invalid target:\n%s", got)
	}
}

func TestStateSnapshotRestoresOriginalBytes(t *testing.T) {
	root := newV2StateTestRepository(t)
	statePath := releaseconfig.V2StatePath(root)
	before := mustReadString(t, statePath)
	tx := NewStateTransaction(root)
	if err := tx.CaptureSnapshot(); err != nil {
		t.Fatalf("CaptureSnapshot: %v", err)
	}
	if err := tx.WriteUnitVersion("api", "0.2.1"); err != nil {
		t.Fatalf("WriteUnitVersion: %v", err)
	}
	if err := tx.RestoreSnapshot(); err != nil {
		t.Fatalf("RestoreSnapshot: %v", err)
	}
	if got := mustReadString(t, statePath); got != before {
		t.Fatalf("snapshot restore changed bytes:\n%s", got)
	}
}

func newV2StateTestRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".neko"), 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".goreleaser.yml"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write goreleaser config: %v", err)
	}
	cfg := `{
  "schemaVersion": 2,
  "units": [
    {"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"goreleaser","delivery":"local"}},
    {"id":"web","paths":["web/**"],"workingDirectory":".","tagPrefix":"web/v","executor":{"type":"jreleaser","delivery":"local"}}
  ]
}`
	state := `{
  "schemaVersion": 2,
  "units": {
    "api": {
      "version": "0.2.0"
    },
    "web": {
      "version": "1.4.0"
    }
  }
}
`
	if !json.Valid([]byte(cfg)) || !json.Valid([]byte(state)) {
		t.Fatal("test fixture JSON is invalid")
	}
	if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(cfg), 0644); err != nil {
		t.Fatalf("write v2 config: %v", err)
	}
	if err := os.WriteFile(releaseconfig.V2StatePath(root), []byte(state), 0644); err != nil {
		t.Fatalf("write v2 state: %v", err)
	}
	return root
}

func mustReadString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func assertNoAtomicTemps(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temporary file was not cleaned up: %s", entry.Name())
		}
	}
}
