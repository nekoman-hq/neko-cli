package release

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
)

func TestRepositorySelfMigrationUsesV2StateForPluginUnits(t *testing.T) {
	root := repositoryRootForSelfMigrationTest()
	state := readSelfMigrationState(t, root)
	releaseManifest := readSelfMigrationManifest(t, root, pluginReleaseManifestPath)
	uiManifest := readSelfMigrationManifest(t, root, pluginUIManifestPath)
	assertSelfMigrationVersionInvariants(t, state, releaseManifest, uiManifest)

	configContent, err := os.ReadFile(filepath.Join(root, ".neko", "release.config.json"))
	if err != nil {
		t.Fatalf("read V2 config: %v", err)
	}
	var config struct {
		Units []struct {
			ID        string `json:"id"`
			TagPrefix string `json:"tagPrefix"`
			Kind      string `json:"kind"`
			Plugin    struct {
				Name        string `json:"name"`
				Manifest    string `json:"manifest"`
				AssetPrefix string `json:"assetPrefix"`
				BinaryName  string `json:"binaryName"`
			} `json:"plugin"`
			Executor struct {
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
		kind      string
		plugin    struct {
			Name        string `json:"name"`
			Manifest    string `json:"manifest"`
			AssetPrefix string `json:"assetPrefix"`
			BinaryName  string `json:"binaryName"`
		}
	}{}
	for _, unit := range config.Units {
		if unit.Executor.Type != "goreleaser" || unit.Executor.Delivery != "github-actions" {
			t.Fatalf("unexpected executor for %s: %#v", unit.ID, unit.Executor)
		}
		units[unit.ID] = struct {
			tagPrefix string
			workflow  string
			kind      string
			plugin    struct {
				Name        string `json:"name"`
				Manifest    string `json:"manifest"`
				AssetPrefix string `json:"assetPrefix"`
				BinaryName  string `json:"binaryName"`
			}
		}{tagPrefix: unit.TagPrefix, workflow: unit.Executor.Workflow, kind: unit.Kind, plugin: unit.Plugin}
	}
	if units["plugin-ui"].tagPrefix != "plugin-ui/v" || units["plugin-ui"].workflow != ".github/workflows/release-plugin-ui.yml" {
		t.Fatalf("unexpected plugin-ui config: %#v", units["plugin-ui"])
	}
	if units["plugin-release"].kind != "plugin" ||
		units["plugin-release"].plugin.Name != "release" ||
		units["plugin-release"].plugin.Manifest != pluginReleaseManifestPath ||
		units["plugin-release"].plugin.AssetPrefix != "plugin-release" ||
		units["plugin-release"].plugin.BinaryName != "plugin-release" {
		t.Fatalf("unexpected plugin-release metadata: %#v", units["plugin-release"])
	}
	if units["plugin-ui"].kind != "plugin" ||
		units["plugin-ui"].plugin.Name != "ui" ||
		units["plugin-ui"].plugin.Manifest != pluginUIManifestPath ||
		units["plugin-ui"].plugin.AssetPrefix != "plugin-ui" ||
		units["plugin-ui"].plugin.BinaryName != "plugin-ui" {
		t.Fatalf("unexpected plugin-ui metadata: %#v", units["plugin-ui"])
	}
	if units["cli"].kind != "" || units["cli"].plugin.Name != "" {
		t.Fatalf("cli unit must not carry plugin metadata: %#v", units["cli"])
	}
}

func TestRepositorySelfMigrationVersionInvariantSurvivesFutureBumps(t *testing.T) {
	state := selfMigrationState{
		SchemaVersion: 2,
		Units: map[string]selfMigrationUnitState{
			"cli":            {Version: "8.1.0"},
			"plugin-release": {Version: "9.2.3"},
			"plugin-ui":      {Version: "10.0.1"},
		},
	}
	releaseManifest := selfMigrationManifest{Version: "9.2.3"}
	uiManifest := selfMigrationManifest{Version: "10.0.1"}

	assertSelfMigrationVersionInvariants(t, state, releaseManifest, uiManifest)
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

func TestRepositorySelfMigrationWorkflowsUseDedicatedGoReleaserConfigs(t *testing.T) {
	root := repositoryRootForSelfMigrationTest()
	cases := []struct {
		unit        string
		workflow    string
		config      string
		mustContain string
		forbidden   []string
	}{
		{
			unit:        "cli",
			workflow:    ".github/workflows/release-neko-cli.yml",
			config:      ".goreleaser.cli.yaml",
			mustContain: "id: neko-cli",
			forbidden:   []string{"id: plugin-release", "id: plugin-ui", "plugin/release/manifest.json", "plugin/ui/manifest.json"},
		},
		{
			unit:        "plugin-release",
			workflow:    ".github/workflows/release-plugin-release.yml",
			config:      ".goreleaser.plugin-release.yaml",
			mustContain: "id: plugin-release",
			forbidden:   []string{"id: neko-cli", "id: plugin-ui", "plugin/ui/manifest.json"},
		},
		{
			unit:        "plugin-ui",
			workflow:    ".github/workflows/release-plugin-ui.yml",
			config:      ".goreleaser.plugin-ui.yaml",
			mustContain: "id: plugin-ui",
			forbidden:   []string{"id: neko-cli", "id: plugin-release", "plugin/release/manifest.json"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.unit, func(t *testing.T) {
			workflow := mustReadSelfMigrationFile(t, root, tc.workflow)
			if !strings.Contains(workflow, "workflow_dispatch:") {
				t.Fatalf("%s must use workflow_dispatch", tc.workflow)
			}
			if tc.unit == "cli" && !strings.Contains(workflow, "args: release --config ${{ env.GORELEASER_CONFIG }} --clean") {
				t.Fatalf("%s must publish via its dedicated GoReleaser config", tc.workflow)
			}
			if strings.HasPrefix(tc.unit, "plugin-") && !strings.Contains(workflow, "args: release --config ${{ env.GORELEASER_CONFIG }} --snapshot --clean --skip=publish") {
				t.Fatalf("%s must package plugin artifacts with its dedicated GoReleaser config", tc.workflow)
			}
			if strings.HasPrefix(tc.unit, "plugin-") && !strings.Contains(workflow, "gh release create \"$RELEASE_TAG\"") {
				t.Fatalf("%s must create the GitHub Release for the exact prefixed tag", tc.workflow)
			}
			if !strings.Contains(workflow, "GORELEASER_CONFIG: "+tc.config) {
				t.Fatalf("%s must reference %s", tc.workflow, tc.config)
			}
			if strings.Contains(workflow, "GORELEASER_CONFIG: .goreleaser.yaml") || strings.Contains(workflow, "--config .goreleaser.yaml") {
				t.Fatalf("%s must not use the global mixed-artifact GoReleaser config", tc.workflow)
			}

			config := mustReadSelfMigrationFile(t, root, tc.config)
			if !strings.Contains(config, tc.mustContain) {
				t.Fatalf("%s must contain %s", tc.config, tc.mustContain)
			}
			for _, forbidden := range tc.forbidden {
				if strings.Contains(config, forbidden) {
					t.Fatalf("%s must not contain unrelated artifact token %q", tc.config, forbidden)
				}
			}
		})
	}
}

type selfMigrationState struct {
	Units         map[string]selfMigrationUnitState `json:"units"`
	SchemaVersion int                               `json:"schemaVersion"`
}

type selfMigrationUnitState struct {
	Version string `json:"version"`
}

type selfMigrationManifest struct {
	Version string `json:"version"`
}

func mustReadSelfMigrationFile(t *testing.T, root, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func readSelfMigrationState(t *testing.T, root string) selfMigrationState {
	t.Helper()
	stateContent, err := os.ReadFile(filepath.Join(root, ".neko", "release.state.json"))
	if err != nil {
		t.Fatalf("read V2 state: %v", err)
	}
	var state selfMigrationState
	if err := json.Unmarshal(stateContent, &state); err != nil {
		t.Fatalf("decode V2 state: %v", err)
	}
	return state
}

func readSelfMigrationManifest(t *testing.T, root, path string) selfMigrationManifest {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var manifest selfMigrationManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return manifest
}

func assertSelfMigrationVersionInvariants(t *testing.T, state selfMigrationState, releaseManifest, uiManifest selfMigrationManifest) {
	t.Helper()
	if state.SchemaVersion != 2 {
		t.Fatalf("expected schemaVersion 2, got %d", state.SchemaVersion)
	}
	for _, unitID := range []string{"cli", "plugin-release", "plugin-ui"} {
		unit, ok := state.Units[unitID]
		if !ok {
			t.Fatalf("expected V2 state unit %s to exist", unitID)
		}
		if _, err := semver.NewVersion(unit.Version); err != nil {
			t.Fatalf("expected %s version %q to be valid semver: %v", unitID, unit.Version, err)
		}
	}
	if state.Units["plugin-release"].Version != releaseManifest.Version {
		t.Fatalf("plugin-release state version %s must match manifest version %s", state.Units["plugin-release"].Version, releaseManifest.Version)
	}
	if state.Units["plugin-ui"].Version != uiManifest.Version {
		t.Fatalf("plugin-ui state version %s must match manifest version %s", state.Units["plugin-ui"].Version, uiManifest.Version)
	}
}

func repositoryRootForSelfMigrationTest() string {
	return filepath.Clean(filepath.Join("..", "..", "..", ".."))
}
