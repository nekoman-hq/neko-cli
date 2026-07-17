package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadV2RepositoryValidSingleUnit(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "app"))
	writeV2Files(t, root, validV2Config(`{
  "id": "default",
  "displayName": "My Project",
  "paths": ["**"],
  "workingDirectory": "app",
  "tagPrefix": "v",
  "executor": {
    "type": "goreleaser",
    "delivery": "local"
  }
}`), validV2State(`"default": {"version": "2.2.4"}`))

	repo, err := LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository: %v", err)
	}
	if repo.SourceFormat != SourceFormatV2 || len(repo.Units) != 1 {
		t.Fatalf("unexpected repository: %#v", repo)
	}
	unit := repo.Units[0]
	if unit.ID != "default" || unit.Version != "2.2.4" || unit.WorkingDirectory != "app" || unit.Delivery != "local" {
		t.Fatalf("unexpected normalized unit: %#v", unit)
	}
	if unit.IsPlugin || unit.Kind != "" || unit.PluginName != "" {
		t.Fatalf("non-plugin unit must not expose plugin metadata: %#v", unit)
	}
}

func TestLoadV2RepositoryValidMultiUnitWithDefaults(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "api"))
	mustMkdir(t, filepath.Join(root, "web"))
	writeV2Files(t, root, validV2Config(`{
  "id": "api",
  "paths": ["api/**"],
  "workingDirectory": "api",
  "tagPrefix": "api/v",
  "executor": {"type": "goreleaser"}
}, {
  "id": "web",
  "paths": ["web/**"],
  "workingDirectory": "web",
  "tagPrefix": "web/v",
  "executor": {"type": "release-it", "delivery": "github-actions", "workflow": ".github/workflows/release-web.yml"}
}`), validV2State(`"api": {"version": "1.0.0"}, "web": {"version": "2.0.0"}`))
	mustWrite(t, filepath.Join(root, ".github", "workflows", "release-web.yml"), "name: release web\n")

	repo, err := LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository: %v", err)
	}
	if len(repo.Units) != 2 {
		t.Fatalf("expected 2 units, got %#v", repo.Units)
	}
	if repo.Units[0].Delivery != "local" {
		t.Fatalf("expected default local delivery, got %s", repo.Units[0].Delivery)
	}
	if repo.Units[1].Delivery != "github-actions" {
		t.Fatalf("expected github-actions delivery, got %s", repo.Units[1].Delivery)
	}
	if repo.Units[1].Workflow != ".github/workflows/release-web.yml" {
		t.Fatalf("expected normalized workflow, got %#v", repo.Units[1])
	}
}

func TestLoadV2RepositoryValidPluginUnitMetadata(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "plugin", "release", "manifest.json"), `{"name":"release","version":"4.0.2"}`)
	writeV2Files(t, root, validV2Config(`{
  "id": "plugin-release",
  "displayName": "neko-cli release plugin",
  "paths": ["plugin/release/**"],
  "workingDirectory": ".",
  "tagPrefix": "plugin-release/v",
  "kind": "plugin",
  "plugin": {
    "name": "release",
    "manifest": "plugin/release/manifest.json",
    "assetPrefix": "plugin-release",
    "binaryName": "plugin-release"
  },
  "executor": {"type": "goreleaser"}
}`), validV2State(`"plugin-release": {"version": "4.0.2"}`))

	repo, err := LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository: %v", err)
	}
	unit := repo.Units[0]
	if !unit.IsPlugin || unit.Kind != "plugin" {
		t.Fatalf("expected normalized plugin unit, got %#v", unit)
	}
	if unit.PluginName != "release" ||
		unit.PluginManifestPath != "plugin/release/manifest.json" ||
		unit.PluginAssetPrefix != "plugin-release" ||
		unit.PluginBinaryName != "plugin-release" {
		t.Fatalf("unexpected normalized plugin metadata: %#v", unit)
	}
}

func TestValidateV2ReleaseConfigStructureAllowsPluginManifestWithoutRepositoryFile(t *testing.T) {
	cfg := &V2ReleaseConfig{
		SchemaVersion: 2,
		Units: []V2Unit{
			{
				ID:        "plugin-release",
				Paths:     []string{"plugin/release/**"},
				TagPrefix: "plugin-release/v",
				Kind:      UnitKindPlugin,
				Plugin: &V2Plugin{
					Name:        "release",
					Manifest:    "plugin/release/manifest.json",
					AssetPrefix: "plugin-release",
					BinaryName:  "plugin-release",
				},
				Executor: V2Executor{Type: ExecutorGoReleaser},
			},
		},
	}
	if err := ValidateV2ReleaseConfigStructure(cfg); err != nil {
		t.Fatalf("ValidateV2ReleaseConfigStructure: %v", err)
	}
}

func TestNormalizeV1Repository(t *testing.T) {
	cfg := &V1ReleaseConfig{
		ProjectName:   "neko-cli",
		ProjectOwner:  "nekoman-hq",
		ProjectType:   V1ProjectTypeBackend,
		ReleaseSystem: V1ReleaseTypeGoReleaser,
		Version:       "1.2.3",
	}

	repo := NormalizeV1Repository("/repo", cfg)
	if repo.SourceFormat != SourceFormatV1 || repo.SchemaVersion != 1 || repo.Legacy != cfg {
		t.Fatalf("unexpected V1 repository: %#v", repo)
	}
	unit := repo.Units[0]
	if unit.ID != "default" || unit.Paths[0] != "**" || unit.TagPrefix != "v" || unit.Version != "1.2.3" {
		t.Fatalf("unexpected V1 normalized unit: %#v", unit)
	}
	if unit.Workflow != "" {
		t.Fatalf("V1 normalized unit must not carry workflow: %#v", unit)
	}
}

func TestLoadReleaseRepositoryRejectsRootV1V2Conflict(t *testing.T) {
	root := t.TempDir()
	writeV1Config(t, root)
	writeV2Files(t, root, validV2Config(`{
  "id": "default",
  "paths": ["**"],
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`), validV2State(`"default": {"version": "1.0.0"}`))

	_, err := LoadReleaseRepository(root)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestLoadV2RepositoryValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		state     string
		wantError string
	}{
		{
			name:      "unknown config field",
			config:    `{"schemaVersion":2,"unknown":true,"units":[]}`,
			state:     validV2State(``),
			wantError: "unknown field",
		},
		{
			name: "unknown plugin field",
			config: validV2Config(`{
  "id": "plugin-release",
  "paths": ["plugin/release/**"],
  "tagPrefix": "plugin-release/v",
  "kind": "plugin",
  "plugin": {
    "name": "release",
    "manifest": "plugin/release/manifest.json",
    "assetPrefix": "plugin-release",
    "binaryName": "plugin-release",
    "extra": true
  },
  "executor": {"type": "goreleaser"}
}`),
			state:     validV2State(`"plugin-release": {"version": "4.0.2"}`),
			wantError: "unknown field",
		},
		{
			name:      "wrong config schema",
			config:    `{"schemaVersion":1,"units":[]}`,
			state:     validV2State(``),
			wantError: "schemaVersion",
		},
		{
			name: "duplicate unit id",
			config: validV2Config(`{
  "id": "api",
  "paths": ["api/**"],
  "tagPrefix": "api/v",
  "executor": {"type": "goreleaser"}
}, {
  "id": "api",
  "paths": ["web/**"],
  "tagPrefix": "web/v",
  "executor": {"type": "release-it"}
}`),
			state:     validV2State(`"api": {"version": "1.0.0"}`),
			wantError: "more than once",
		},
		{
			name: "bad unit id",
			config: validV2Config(`{
  "id": "Bad/ID",
  "paths": ["**"],
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`),
			state:     validV2State(`"Bad/ID": {"version": "1.0.0"}`),
			wantError: "invalid",
		},
		{
			name: "overlapping tag prefix",
			config: validV2Config(`{
  "id": "api",
  "paths": ["api/**"],
  "tagPrefix": "api/v",
  "executor": {"type": "goreleaser"}
}, {
  "id": "web",
  "paths": ["web/**"],
  "tagPrefix": "api/v1",
  "executor": {"type": "release-it"}
}`),
			state:     validV2State(`"api": {"version": "1.0.0"}, "web": {"version": "1.0.0"}`),
			wantError: "overlaps",
		},
		{
			name: "working directory leaves repo",
			config: validV2Config(`{
  "id": "api",
  "paths": ["**"],
  "workingDirectory": "../api",
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`),
			state:     validV2State(`"api": {"version": "1.0.0"}`),
			wantError: "leaves the repository",
		},
		{
			name: "path leaves repo",
			config: validV2Config(`{
  "id": "api",
  "paths": ["../**"],
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`),
			state:     validV2State(`"api": {"version": "1.0.0"}`),
			wantError: "path pattern",
		},
		{
			name: "unknown executor",
			config: validV2Config(`{
  "id": "api",
  "paths": ["**"],
  "tagPrefix": "v",
  "executor": {"type": "custom"}
}`),
			state:     validV2State(`"api": {"version": "1.0.0"}`),
			wantError: "unknown executor",
		},
		{
			name: "unknown delivery",
			config: validV2Config(`{
  "id": "api",
  "paths": ["**"],
  "tagPrefix": "v",
  "executor": {"type": "goreleaser", "delivery": "ftp"}
}`),
			state:     validV2State(`"api": {"version": "1.0.0"}`),
			wantError: "unknown delivery",
		},
		{
			name: "missing state entry",
			config: validV2Config(`{
  "id": "api",
  "paths": ["**"],
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`),
			state:     validV2State(``),
			wantError: "missing unit",
		},
		{
			name: "unknown state unit",
			config: validV2Config(`{
  "id": "api",
  "paths": ["**"],
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`),
			state:     validV2State(`"api": {"version": "1.0.0"}, "web": {"version": "1.0.0"}`),
			wantError: "unknown unit",
		},
		{
			name: "bad semver",
			config: validV2Config(`{
  "id": "api",
  "paths": ["**"],
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`),
			state:     validV2State(`"api": {"version": "not-semver"}`),
			wantError: "SemVer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeV2Files(t, root, tt.config, tt.state)

			_, err := LoadV2Repository(root)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func TestLoadV2RepositoryPluginMetadataValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		unitID    string
		tagPrefix string
		kind      string
		plugin    string
		wantError string
	}{
		{
			name:      "plugin metadata without plugin kind",
			unitID:    "plugin-release",
			tagPrefix: "plugin-release/v",
			plugin:    validPluginMetadata("release", "plugin/release/manifest.json", "plugin-release", "plugin-release"),
			wantError: "requires kind",
		},
		{
			name:      "plugin kind without metadata",
			unitID:    "plugin-release",
			tagPrefix: "plugin-release/v",
			kind:      "plugin",
			wantError: "requires plugin metadata",
		},
		{
			name:      "unknown kind",
			unitID:    "plugin-release",
			tagPrefix: "plugin-release/v",
			kind:      "service",
			wantError: "unknown kind",
		},
		{
			name:      "plugin unit id must start plugin",
			unitID:    "release",
			tagPrefix: "release/v",
			kind:      "plugin",
			plugin:    validPluginMetadata("release", "plugin/release/manifest.json", "release", "plugin-release"),
			wantError: "id must start with plugin-",
		},
		{
			name:      "tag prefix mismatch",
			unitID:    "plugin-release",
			tagPrefix: "release/v",
			kind:      "plugin",
			plugin:    validPluginMetadata("release", "plugin/release/manifest.json", "plugin-release", "plugin-release"),
			wantError: "tagPrefix must be",
		},
		{
			name:      "invalid plugin name",
			unitID:    "plugin-release",
			tagPrefix: "plugin-release/v",
			kind:      "plugin",
			plugin:    validPluginMetadata("plugin-release", "plugin/release/manifest.json", "plugin-release", "plugin-release"),
			wantError: "must not start with plugin-",
		},
		{
			name:      "invalid manifest path",
			unitID:    "plugin-release",
			tagPrefix: "plugin-release/v",
			kind:      "plugin",
			plugin:    validPluginMetadata("release", "../manifest.json", "plugin-release", "plugin-release"),
			wantError: "clean repository-root-relative path",
		},
		{
			name:      "manifest must end with manifest json",
			unitID:    "plugin-release",
			tagPrefix: "plugin-release/v",
			kind:      "plugin",
			plugin:    validPluginMetadata("release", "plugin/release/plugin.json", "plugin-release", "plugin-release"),
			wantError: "must end with manifest.json",
		},
		{
			name:      "missing manifest file",
			unitID:    "plugin-release",
			tagPrefix: "plugin-release/v",
			kind:      "plugin",
			plugin:    validPluginMetadata("release", "plugin/release/manifest.json", "plugin-release", "plugin-release"),
			wantError: "does not exist",
		},
		{
			name:      "invalid asset prefix",
			unitID:    "plugin-release",
			tagPrefix: "plugin-release/v",
			kind:      "plugin",
			plugin:    validPluginMetadata("release", "plugin/release/manifest.json", "plugin.release", "plugin-release"),
			wantError: "assetPrefix",
		},
		{
			name:      "asset prefix must equal unit id",
			unitID:    "plugin-release",
			tagPrefix: "plugin-release/v",
			kind:      "plugin",
			plugin:    validPluginMetadata("release", "plugin/release/manifest.json", "release-plugin", "plugin-release"),
			wantError: "assetPrefix must equal unit id",
		},
		{
			name:      "invalid binary name",
			unitID:    "plugin-release",
			tagPrefix: "plugin-release/v",
			kind:      "plugin",
			plugin:    validPluginMetadata("release", "plugin/release/manifest.json", "plugin-release", "../plugin-release"),
			wantError: "binaryName",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if tt.name != "missing manifest file" {
				mustWrite(t, filepath.Join(root, "plugin", "release", "manifest.json"), `{"name":"release","version":"4.0.2"}`)
			}
			writeV2Files(t, root, validV2Config(pluginUnitJSON(tt.unitID, tt.tagPrefix, tt.kind, tt.plugin)), validV2State(fmt.Sprintf(`%q: {"version": "4.0.2"}`, tt.unitID)))

			_, err := LoadV2Repository(root)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func TestLoadV2RepositoryRejectsDuplicatePluginNames(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "plugin", "release", "manifest.json"), `{"name":"release","version":"4.0.2"}`)
	mustWrite(t, filepath.Join(root, "plugin", "other", "manifest.json"), `{"name":"release","version":"1.0.0"}`)
	writeV2Files(t, root, validV2Config(
		pluginUnitJSON("plugin-release", "plugin-release/v", "plugin", validPluginMetadata("release", "plugin/release/manifest.json", "plugin-release", "plugin-release"))+","+
			pluginUnitJSON("plugin-other", "plugin-other/v", "plugin", validPluginMetadata("release", "plugin/other/manifest.json", "plugin-other", "plugin-other")),
	), validV2State(`"plugin-release": {"version": "4.0.2"}, "plugin-other": {"version": "1.0.0"}`))

	_, err := LoadV2Repository(root)
	if err == nil || !strings.Contains(err.Error(), "plugin name") {
		t.Fatalf("expected duplicate plugin name error, got %v", err)
	}
}

func TestLoadV2RepositoryPluginManifestSymlinkEscapeFails(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "manifest.json"), `{"name":"release","version":"4.0.2"}`)
	mustMkdir(t, filepath.Join(root, "plugin", "release"))
	if err := os.Symlink(filepath.Join(outside, "manifest.json"), filepath.Join(root, "plugin", "release", "manifest.json")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	writeV2Files(t, root, validV2Config(pluginUnitJSON(
		"plugin-release",
		"plugin-release/v",
		"plugin",
		validPluginMetadata("release", "plugin/release/manifest.json", "plugin-release", "plugin-release"),
	)), validV2State(`"plugin-release": {"version": "4.0.2"}`))

	_, err := LoadV2Repository(root)
	if err == nil || !strings.Contains(err.Error(), "outside repository root") {
		t.Fatalf("expected symlink escape error, got %v", err)
	}
}

func TestLoadV2RepositoryWorkingDirectorySymlinkEscapeFails(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "app")); err != nil {
		t.Fatalf("symlink working directory: %v", err)
	}
	writeV2Files(t, root, validV2Config(`{
  "id": "default",
  "paths": ["app/**"],
  "workingDirectory": "app",
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`), validV2State(`"default": {"version": "1.0.0"}`))

	_, err := LoadV2Repository(root)
	if err == nil || !strings.Contains(err.Error(), "resolves outside repository root") {
		t.Fatalf("expected workingDirectory symlink escape error, got %v", err)
	}
}

func validV2Config(units string) string {
	return `{"schemaVersion":2,"units":[` + units + `]}`
}

func validV2State(units string) string {
	return `{"schemaVersion":2,"units":{` + units + `}}`
}

func writeV2Files(t *testing.T, root, cfg, state string) {
	t.Helper()
	mustMkdir(t, filepath.Join(root, V2Directory))
	mustWrite(t, V2ConfigPath(root), cfg)
	mustWrite(t, V2StatePath(root), state)
}

func pluginUnitJSON(unitID, tagPrefix, kind, pluginMetadata string) string {
	kindField := ""
	if kind != "" {
		kindField = fmt.Sprintf(`, "kind": %q`, kind)
	}
	pluginField := ""
	if pluginMetadata != "" {
		pluginField = `, "plugin": ` + pluginMetadata
	}
	return fmt.Sprintf(`{
  "id": %q,
  "paths": ["plugin/release/**"],
  "tagPrefix": %q%s%s,
  "executor": {"type": "goreleaser"}
}`, unitID, tagPrefix, kindField, pluginField)
}

func validPluginMetadata(name, manifest, assetPrefix, binaryName string) string {
	return fmt.Sprintf(`{
    "name": %q,
    "manifest": %q,
    "assetPrefix": %q,
    "binaryName": %q
  }`, name, manifest, assetPrefix, binaryName)
}

func writeV1Config(t *testing.T, root string) {
	t.Helper()
	mustWrite(t, filepath.Join(root, V1FileName), `{
  "project-name": "neko-cli",
  "project-owner": "nekoman-hq",
  "project-type": "backend",
  "release-system": "goreleaser",
  "version": "1.2.3"
}`)
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
