package release

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositorySelfMigrationUsesV2StateForPluginUnits(t *testing.T) {
	root := repositoryRootForSelfMigrationTest()
	stateContent, err := os.ReadFile(filepath.Join(root, ".neko", "release.state.json"))
	if err != nil {
		t.Fatalf("read V2 state: %v", err)
	}
	var state struct {
		Units map[string]struct {
			Version string `json:"version"`
		} `json:"units"`
		SchemaVersion int `json:"schemaVersion"`
	}
	if unmarshalErr := json.Unmarshal(stateContent, &state); unmarshalErr != nil {
		t.Fatalf("decode V2 state: %v", unmarshalErr)
	}
	if state.SchemaVersion != 2 {
		t.Fatalf("expected schemaVersion 2, got %d", state.SchemaVersion)
	}
	want := map[string]string{
		"cli":            "2.2.4",
		"plugin-release": "3.0.0",
		"plugin-ui":      "1.0.0",
	}
	for unitID, version := range want {
		if state.Units[unitID].Version != version {
			t.Fatalf("expected %s version %s, got %s", unitID, version, state.Units[unitID].Version)
		}
	}

	configContent, err := os.ReadFile(filepath.Join(root, ".neko", "release.config.json"))
	if err != nil {
		t.Fatalf("read V2 config: %v", err)
	}
	var config struct {
		Units []struct {
			ID        string `json:"id"`
			TagPrefix string `json:"tagPrefix"`
			Executor  struct {
				Type     string `json:"type"`
				Delivery string `json:"delivery"`
				Workflow string `json:"workflow"`
			} `json:"executor"`
		} `json:"units"`
	}
	if err := json.Unmarshal(configContent, &config); err != nil {
		t.Fatalf("decode V2 config: %v", err)
	}
	if len(config.Units) != 3 {
		t.Fatalf("expected exactly three V2 units, got %d", len(config.Units))
	}
	units := map[string]struct {
		tagPrefix string
		workflow  string
	}{}
	for _, unit := range config.Units {
		if unit.Executor.Type != "goreleaser" || unit.Executor.Delivery != "github-actions" {
			t.Fatalf("unexpected executor for %s: %#v", unit.ID, unit.Executor)
		}
		units[unit.ID] = struct {
			tagPrefix string
			workflow  string
		}{tagPrefix: unit.TagPrefix, workflow: unit.Executor.Workflow}
	}
	if units["plugin-ui"].tagPrefix != "plugin-ui/v" || units["plugin-ui"].workflow != ".github/workflows/release-plugin-ui.yml" {
		t.Fatalf("unexpected plugin-ui config: %#v", units["plugin-ui"])
	}
}

func TestRepositorySelfMigrationRemovedLegacyPluginVersionMapReferences(t *testing.T) {
	root := repositoryRootForSelfMigrationTest()
	legacyReleaseMapName := strings.Join([]string{".plugin", "release", "neko", "json"}, ".")
	legacyUIMapName := strings.Join([]string{".plugin", "ui", "neko", "json"}, ".")
	legacyReleaseStatePath := strings.Join([]string{"plugins", "release"}, ".")
	legacyUIStatePath := strings.Join([]string{"plugins", "ui"}, ".")

	if _, err := os.Stat(filepath.Join(root, legacyReleaseMapName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy plugin version map must be deleted, stat error: %v", err)
	}

	forbidden := []string{
		legacyReleaseMapName,
		legacyUIMapName,
		legacyReleaseStatePath,
		legacyUIStatePath,
	}
	files := []string{
		"Makefile",
		"plugin/release/Makefile",
		"plugin/ui/Makefile",
		"plugin/release/pkg/release/tool/goreleaser/goreleaser.go",
		"plugin/release/pkg/release/version_materializer.go",
		"plugin/release/pkg/release/plugin_manifest_materializer.go",
	}
	for _, relPath := range files {
		content, err := os.ReadFile(filepath.Join(root, relPath))
		if err != nil {
			t.Fatalf("read %s: %v", relPath, err)
		}
		for _, token := range forbidden {
			if strings.Contains(string(content), token) {
				t.Fatalf("%s must not reference legacy plugin version token %q", relPath, token)
			}
		}
	}
}

func repositoryRootForSelfMigrationTest() string {
	return filepath.Clean(filepath.Join("..", "..", "..", ".."))
}
