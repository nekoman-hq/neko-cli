package release

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestUnitOverviewIsTokenFreeAndDoesNotMutateInspectedFiles(t *testing.T) {
	root := newUnitOverviewRepository(t)
	writeValidUnitOverviewPair(t, root.Path())
	workflowPath := filepath.Join(root.Path(), ".github", "workflows", "release-api.yml")
	writeUnitOverviewBytes(t, workflowPath, []byte("name: must-not-be-inspected\n"))
	paths := []string{releaseconfig.V2ConfigPath(root.Path()), releaseconfig.V2StatePath(root.Path()), workflowPath}
	preserved := time.Unix(1_711_000_000, 0)
	before := make(map[string]unitOverviewFileSnapshot, len(paths))
	for _, path := range paths {
		if err := os.Chmod(path, 0600); err != nil {
			t.Fatalf("set preserved mode for %s: %v", path, err)
		}
		if err := os.Chtimes(path, preserved, preserved); err != nil {
			t.Fatalf("set preserved time for %s: %v", path, err)
		}
		before[path] = snapshotUnitOverviewFile(t, path)
	}
	const token = "unit-overview-token-sentinel"
	t.Setenv("GITHUB_TOKEN", token)

	response := runUnitOverview(t, root)
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if bytes.Contains(encoded, []byte(token)) {
		t.Fatal("unit overview leaked the ambient token")
	}
	if got := os.Getenv("GITHUB_TOKEN"); got != token {
		t.Fatalf("unit overview mutated ambient token: %q", got)
	}
	for _, path := range paths {
		if got := snapshotUnitOverviewFile(t, path); !reflect.DeepEqual(got, before[path]) {
			t.Fatalf("unit overview mutated %s: got=%#v want=%#v", path, got, before[path])
		}
	}
}

func TestHandleUnitsResolvesRepositoryAndNestedDirectoriesWithoutChangingCWD(t *testing.T) {
	root := newUnitOverviewRepository(t)
	writeValidUnitOverviewPair(t, root.Path())
	nested := filepath.Join(root.Path(), "services", "api")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	before, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd before inspection: %v", err)
	}

	for name, workingDirectory := range map[string]string{"repository root": root.Path(), "nested directory": nested} {
		t.Run(name, func(t *testing.T) {
			response, handleErr := HandleUnits(plugin.Request{
				Command: unitOverviewCommandName,
				Context: plugin.Context{WorkingDir: workingDirectory},
			})
			if handleErr != nil {
				t.Fatalf("HandleUnits: %v", handleErr)
			}
			if got := unitOverviewVersionFromResponse(t, response, "api"); got != "1.2.3" {
				t.Fatalf("version = %q, want 1.2.3", got)
			}
		})
	}
	after, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd after inspection: %v", err)
	}
	if filepath.Clean(after) != filepath.Clean(before) {
		t.Fatalf("unit overview changed cwd: before=%q after=%q", before, after)
	}
}

func TestUnitOverviewExplicitRootsRemainIsolatedAcrossRepositories(t *testing.T) {
	first := newUnitOverviewRepository(t)
	second := newUnitOverviewRepository(t)
	writeValidUnitOverviewPair(t, first.Path())
	writeUnitOverviewPair(t, second.Path(),
		releaseconfig.V2ReleaseConfig{SchemaVersion: 2, Units: []releaseconfig.V2Unit{unitOverviewConfigUnit("worker", "Worker", "worker/v", ".github/workflows/release-worker.yml")}},
		releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{"worker": {Version: "9.8.7"}}},
	)

	firstResponse := runUnitOverview(t, first)
	secondResponse := runUnitOverview(t, second)
	firstAgain := runUnitOverview(t, first)
	if got := unitOverviewVersionFromResponse(t, firstResponse, "api"); got != "1.2.3" {
		t.Fatalf("first repository version = %q", got)
	}
	if got := unitOverviewVersionFromResponse(t, secondResponse, "worker"); got != "9.8.7" {
		t.Fatalf("second repository version = %q", got)
	}
	if got := unitOverviewVersionFromResponse(t, firstAgain, "api"); got != "1.2.3" {
		t.Fatalf("first repository cached cross-repository state: %q", got)
	}
}

//nolint:govet // Logical metadata order keeps mutation assertions readable.
type unitOverviewFileSnapshot struct {
	Content []byte
	Mode    os.FileMode
	ModTime time.Time
}

func snapshotUnitOverviewFile(t *testing.T, path string) unitOverviewFileSnapshot {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return unitOverviewFileSnapshot{Content: content, Mode: info.Mode(), ModTime: info.ModTime()}
}

func unitOverviewVersionFromResponse(t *testing.T, response *plugin.Response, unitID string) string {
	t.Helper()
	result := unitOverviewResponseResult(t, response)
	for _, row := range result.units {
		if row["id"] == unitID {
			version, ok := row["version"].(string)
			if !ok {
				t.Fatalf("unit %s version type = %T", unitID, row["version"])
			}
			return version
		}
	}
	t.Fatalf("unit %s missing from %#v", unitID, result.units)
	return ""
}
