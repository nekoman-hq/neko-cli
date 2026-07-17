package init

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestGetAvailableOptionsExposesV2OnlyInitOptions(t *testing.T) {
	resp, err := GetAvailableOptions()
	if err != nil {
		t.Fatalf("GetAvailableOptions: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp)
	}

	options := initOptionNames(t, resp)
	for _, want := range []string{
		"unit",
		"display-name",
		"version",
		"executor",
		"delivery",
		"workflow",
		"tag-prefix",
		"working-directory",
		"paths",
		"kind",
		"plugin-name",
		"plugin-manifest",
		"plugin-asset-prefix",
		"plugin-binary-name",
		"force",
	} {
		if !options[want] {
			t.Fatalf("init-options missing %q: %#v", want, options)
		}
	}
	for _, legacy := range []string{"project-type", "release-system", "metadata"} {
		if options[legacy] {
			t.Fatalf("init-options still exposes legacy option %q", legacy)
		}
	}
}

func TestGetAvailableOptionsClarifiesPluginOnlyOptions(t *testing.T) {
	resp, err := GetAvailableOptions()
	if err != nil {
		t.Fatalf("GetAvailableOptions: %v", err)
	}
	rows := initOptionRows(t, resp)
	for _, row := range rows {
		for _, key := range []string{"option", "values", "required", "description"} {
			if _, ok := row[key]; !ok {
				t.Fatalf("init-options row missing %s: %#v", key, row)
			}
		}
	}

	byName := map[string]map[string]any{}
	for _, row := range rows {
		byName[rowString(t, row, "option")] = row
	}
	kindDescription := rowString(t, byName["kind"], "description")
	assertInitOptionContains(t, "kind", kindDescription, "release is the default", "normal release units", "Neko CLI plugins", "invalid unless kind=plugin")

	for _, option := range []string{"plugin-name", "plugin-manifest", "plugin-asset-prefix", "plugin-binary-name"} {
		row := byName[option]
		if rowString(t, row, "required") != "when kind=plugin" {
			t.Fatalf("%s required value = %#v", option, row["required"])
		}
		description := rowString(t, row, "description")
		assertInitOptionContains(t, option, description, "Only with kind=plugin", "Neko CLI plugin")
		if strings.Contains(strings.ToLower(description), "ignored") {
			t.Fatalf("%s must not say plugin fields are ignored: %q", option, description)
		}
	}
	assertInitOptionContains(t, "plugin-name", rowString(t, byName["plugin-name"], "description"), "Normal repositories do not use plugin fields")
}

func TestV2UnitConstructionRejectsLegacyFlags(t *testing.T) {
	for _, legacy := range []string{"project-type", "release-system", "metadata"} {
		_, err := constructV2Unit(parseV2UnitRequest(map[string]any{
			"executor": "goreleaser",
			"delivery": "local",
			legacy:     "legacy",
		}))
		if err == nil || !strings.Contains(err.Error(), "V2-only") {
			t.Fatalf("expected V2-only error for %s, got %v", legacy, err)
		}
	}
}

func TestHandleInitCreatesV2LocalConfigAndState(t *testing.T) {
	withWorkingDirectory(t)

	resp, err := HandleInit(plugin.Request{Flags: map[string]any{
		"unit":              "api",
		"display-name":      "API",
		"version":           "1.2.3",
		"executor":          "goreleaser",
		"delivery":          "local",
		"workflow":          ".github/workflows/ignored.yml",
		"tag-prefix":        "api/v",
		"working-directory": ".",
		"paths":             "apps/api/**, platform/** ,docs/**",
	}})
	if err != nil {
		t.Fatalf("HandleInit: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	assertNoFile(t, legacyV1ConfigFileName)

	cfg, state := loadGeneratedV2(t)
	if cfg.SchemaVersion != 2 || state.SchemaVersion != 2 {
		t.Fatalf("expected v2 schema, got config=%d state=%d", cfg.SchemaVersion, state.SchemaVersion)
	}
	if len(cfg.Units) != 1 {
		t.Fatalf("expected one unit, got %#v", cfg.Units)
	}
	unit := cfg.Units[0]
	if unit.ID != "api" ||
		unit.DisplayName != "API" ||
		unit.TagPrefix != "api/v" ||
		unit.WorkingDirectory != "." ||
		unit.Executor.Type != releaseconfig.ExecutorGoReleaser ||
		unit.Executor.Delivery != releaseconfig.DeliveryLocal {
		t.Fatalf("unexpected generated unit: %#v", unit)
	}
	if unit.Executor.Workflow != "" {
		t.Fatalf("local delivery must omit workflow, got %#v", unit.Executor)
	}
	if got := strings.Join(unit.Paths, ","); got != "apps/api/**,platform/**,docs/**" {
		t.Fatalf("paths were not normalized: %#v", unit.Paths)
	}
	if state.Units["api"].Version != "1.2.3" {
		t.Fatalf("unexpected state: %#v", state.Units)
	}
	if _, err := releaseconfig.LoadV2Repository("."); err != nil {
		t.Fatalf("generated V2 repository must validate: %v", err)
	}
}

func TestHandleInitCreatesV2GitHubActionsConfig(t *testing.T) {
	withWorkingDirectory(t)
	mustWrite(t, ".github/workflows/release.yml", "name: release\n")

	resp, err := HandleInit(plugin.Request{Flags: map[string]any{
		"unit":              "cli",
		"display-name":      "Neko CLI",
		"version":           "0.1.0",
		"executor":          "goreleaser",
		"delivery":          "github-actions",
		"workflow":          ".github/workflows/release.yml",
		"tag-prefix":        "v",
		"working-directory": ".",
		"paths":             "**",
	}})
	if err != nil {
		t.Fatalf("HandleInit: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}

	cfg, state := loadGeneratedV2(t)
	unit := cfg.Units[0]
	if unit.Executor.Delivery != releaseconfig.DeliveryGitHubActions ||
		unit.Executor.Workflow != ".github/workflows/release.yml" ||
		state.Units["cli"].Version != "0.1.0" {
		t.Fatalf("unexpected generated github-actions config: %#v state=%#v", unit, state)
	}
	if _, err := releaseconfig.LoadV2Repository("."); err != nil {
		t.Fatalf("generated V2 repository must validate: %v", err)
	}
}

func TestHandleInitCreatesV2PluginConfigAndState(t *testing.T) {
	withWorkingDirectory(t)
	writeMinimalPluginManifest(t, "plugin/release/manifest.json", "release", "4.0.0")
	mustWrite(t, ".github/workflows/release-plugin-release.yml", "name: release plugin\n")

	resp, err := HandleInit(plugin.Request{Flags: validPluginInitFlags()})
	if err != nil {
		t.Fatalf("HandleInit: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	assertNoFile(t, legacyV1ConfigFileName)

	cfg, state := loadGeneratedV2(t)
	if len(cfg.Units) != 1 {
		t.Fatalf("expected one unit, got %#v", cfg.Units)
	}
	unit := cfg.Units[0]
	if unit.ID != "plugin-release" ||
		unit.DisplayName != "neko-cli release plugin" ||
		unit.TagPrefix != "plugin-release/v" ||
		unit.Kind != releaseconfig.UnitKindPlugin ||
		unit.Plugin == nil {
		t.Fatalf("unexpected generated plugin unit: %#v", unit)
	}
	if unit.Plugin.Name != "release" ||
		unit.Plugin.Manifest != "plugin/release/manifest.json" ||
		unit.Plugin.AssetPrefix != "plugin-release" ||
		unit.Plugin.BinaryName != "plugin-release" {
		t.Fatalf("unexpected plugin metadata: %#v", unit.Plugin)
	}
	if got := strings.Join(unit.Paths, ","); got != "plugin/release/**,docs/plugins/release.md" {
		t.Fatalf("paths were not normalized: %#v", unit.Paths)
	}
	if state.Units["plugin-release"].Version != "4.0.0" {
		t.Fatalf("unexpected state: %#v", state.Units)
	}
	if _, err := releaseconfig.LoadV2Repository("."); err != nil {
		t.Fatalf("generated plugin V2 repository must validate: %v", err)
	}
}

func TestHandleUnitAddAppendsNormalUnit(t *testing.T) {
	withWorkingDirectory(t)
	writeV2(t, "cli", "0.1.0")
	mustWrite(t, ".github/workflows/release-api.yml", "name: release api\n")

	resp, err := HandleUnitAdd(plugin.Request{Flags: validUnitAddFlags()})
	if err != nil {
		t.Fatalf("HandleUnitAdd: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	assertNoFile(t, legacyV1ConfigFileName)

	cfg, state := loadGeneratedV2(t)
	if len(cfg.Units) != 2 {
		t.Fatalf("expected two units, got %#v", cfg.Units)
	}
	if cfg.Units[0].ID != "cli" || cfg.Units[1].ID != "api" {
		t.Fatalf("unit order not preserved/appended: %#v", cfg.Units)
	}
	if cfg.Units[0].TagPrefix != "v" || state.Units["cli"].Version != "0.1.0" {
		t.Fatalf("existing cli unit changed: %#v state=%#v", cfg.Units[0], state.Units)
	}
	api := cfg.Units[1]
	if api.DisplayName != "API" ||
		api.TagPrefix != "api/v" ||
		api.Executor.Delivery != releaseconfig.DeliveryGitHubActions ||
		api.Executor.Workflow != ".github/workflows/release-api.yml" ||
		api.Kind != "" ||
		api.Plugin != nil {
		t.Fatalf("unexpected api unit: %#v", api)
	}
	if state.Units["api"].Version != "1.2.3" {
		t.Fatalf("api state missing: %#v", state.Units)
	}
	if _, err := releaseconfig.LoadV2Repository("."); err != nil {
		t.Fatalf("appended V2 repository must validate: %v", err)
	}
}

func TestHandleUnitAddAppendsPluginUnit(t *testing.T) {
	withWorkingDirectory(t)
	writeV2(t, "cli", "0.1.0")
	writeMinimalPluginManifest(t, "plugin/release/manifest.json", "release", "4.0.0")
	mustWrite(t, ".github/workflows/release-plugin-release.yml", "name: release plugin\n")

	resp, err := HandleUnitAdd(plugin.Request{Flags: validPluginInitFlags()})
	if err != nil {
		t.Fatalf("HandleUnitAdd: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	assertNoFile(t, legacyV1ConfigFileName)

	cfg, state := loadGeneratedV2(t)
	if len(cfg.Units) != 2 || cfg.Units[0].ID != "cli" || cfg.Units[1].ID != "plugin-release" {
		t.Fatalf("unit order not preserved/appended: %#v", cfg.Units)
	}
	unit := cfg.Units[1]
	if unit.Kind != releaseconfig.UnitKindPlugin || unit.Plugin == nil {
		t.Fatalf("expected plugin unit, got %#v", unit)
	}
	if unit.Plugin.Name != "release" ||
		unit.Plugin.Manifest != "plugin/release/manifest.json" ||
		unit.Plugin.AssetPrefix != "plugin-release" ||
		unit.Plugin.BinaryName != "plugin-release" {
		t.Fatalf("unexpected plugin metadata: %#v", unit.Plugin)
	}
	if state.Units["plugin-release"].Version != "4.0.0" {
		t.Fatalf("plugin-release state missing: %#v", state.Units)
	}
	if _, err := releaseconfig.LoadV2Repository("."); err != nil {
		t.Fatalf("appended plugin V2 repository must validate: %v", err)
	}
}

func TestHandleUnitAddExistingConfigRequirements(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T)
		wantCode string
		wantText string
	}{
		{
			name:     "no V2 files",
			setup:    func(t *testing.T) {},
			wantCode: "V2_CONFIG_MISSING",
			wantText: "release init",
		},
		{
			name: "only config exists",
			setup: func(t *testing.T) {
				mustWrite(t, releaseconfig.V2ConfigPath("."), `{"schemaVersion":2,"units":[]}`)
			},
			wantCode: "PARTIAL_V2_CONFIG",
			wantText: "both .neko/release.config.json and .neko/release.state.json",
		},
		{
			name: "only state exists",
			setup: func(t *testing.T) {
				mustWrite(t, releaseconfig.V2StatePath("."), `{"schemaVersion":2,"units":{}}`)
			},
			wantCode: "PARTIAL_V2_CONFIG",
			wantText: "both .neko/release.config.json and .neko/release.state.json",
		},
		{
			name: "V1 only",
			setup: func(t *testing.T) {
				writeV1(t, "0.1.0")
			},
			wantCode: "V1_CONFIG_EXISTS",
			wantText: "release migrate",
		},
		{
			name: "V1 and V2 conflict",
			setup: func(t *testing.T) {
				writeV1(t, "0.1.0")
				writeV2(t, "cli", "0.1.0")
			},
			wantCode: "CONFIG_CONFLICT",
			wantText: "conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWorkingDirectory(t)
			tt.setup(t)

			resp, err := HandleUnitAdd(plugin.Request{Flags: validUnitAddFlags()})
			if err != nil {
				t.Fatalf("HandleUnitAdd: %v", err)
			}
			if resp.Status != "error" || resp.Error == nil {
				t.Fatalf("expected error, got %#v", resp)
			}
			if resp.Error.Code != tt.wantCode || !strings.Contains(resp.Error.Message, tt.wantText) {
				t.Fatalf("expected %s containing %q, got %#v", tt.wantCode, tt.wantText, resp.Error)
			}
		})
	}
}

func TestHandleUnitAddDuplicateBehavior(t *testing.T) {
	t.Run("duplicate unit id fails", func(t *testing.T) {
		withWorkingDirectory(t)
		writeV2(t, "cli", "0.1.0")

		flags := validUnitAddFlags()
		flags["unit"] = "cli"
		flags["tag-prefix"] = "cli/v"
		resp, err := HandleUnitAdd(plugin.Request{Flags: flags})
		if err != nil {
			t.Fatalf("HandleUnitAdd: %v", err)
		}
		if resp.Status != "error" || resp.Error == nil || !strings.Contains(resp.Error.Message, "already exists") {
			t.Fatalf("expected duplicate unit error, got %#v", resp)
		}
	})

	t.Run("duplicate state entry fails", func(t *testing.T) {
		withWorkingDirectory(t)
		writeV2WithExtraState(t)
		mustWrite(t, ".github/workflows/release-api.yml", "name: release api\n")

		resp, err := HandleUnitAdd(plugin.Request{Flags: validUnitAddFlags()})
		if err != nil {
			t.Fatalf("HandleUnitAdd: %v", err)
		}
		if resp.Status != "error" || resp.Error == nil || !strings.Contains(resp.Error.Message, "already exists in state") {
			t.Fatalf("expected duplicate state error, got %#v", resp)
		}
	})

	t.Run("duplicate plugin name fails", func(t *testing.T) {
		withWorkingDirectory(t)
		writeExistingPluginV2(t)
		writeMinimalPluginManifest(t, "plugin/other/manifest.json", "release", "1.0.0")
		mustWrite(t, ".github/workflows/release-plugin-other.yml", "name: release other\n")

		flags := validPluginInitFlags()
		flags["unit"] = "plugin-other"
		flags["display-name"] = "other plugin"
		flags["version"] = "1.0.0"
		flags["workflow"] = ".github/workflows/release-plugin-other.yml"
		flags["tag-prefix"] = "plugin-other/v"
		flags["paths"] = "plugin/other/**"
		flags["plugin-manifest"] = "plugin/other/manifest.json"
		flags["plugin-asset-prefix"] = "plugin-other"
		flags["plugin-binary-name"] = "plugin-other"
		resp, err := HandleUnitAdd(plugin.Request{Flags: flags})
		if err != nil {
			t.Fatalf("HandleUnitAdd: %v", err)
		}
		if resp.Status != "error" || resp.Error == nil || !strings.Contains(resp.Error.Message, "plugin name") {
			t.Fatalf("expected duplicate plugin name error, got %#v", resp)
		}
	})
}

func TestHandleUnitAddInvalidInputs(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T)
		mutate    func(map[string]any)
		wantError string
	}{
		{
			name: "missing unit",
			mutate: func(flags map[string]any) {
				delete(flags, "unit")
			},
			wantError: "missing required flag: --unit",
		},
		{
			name: "force unsupported",
			mutate: func(flags map[string]any) {
				flags["force"] = true
			},
			wantError: "does not support --force",
		},
		{
			name: "invalid version",
			mutate: func(flags map[string]any) {
				flags["version"] = "v1.2.3"
			},
			wantError: "without leading v",
		},
		{
			name: "invalid unit",
			mutate: func(flags map[string]any) {
				flags["unit"] = "API"
			},
			wantError: "unit id",
		},
		{
			name: "github actions missing workflow",
			mutate: func(flags map[string]any) {
				delete(flags, "workflow")
			},
			wantError: "requires workflow",
		},
		{
			name: "github actions missing workflow file",
			setup: func(t *testing.T) {
				if err := os.Remove(".github/workflows/release-api.yml"); err != nil {
					t.Fatalf("remove workflow: %v", err)
				}
			},
			wantError: "does not exist",
		},
		{
			name: "plugin flags without plugin kind",
			mutate: func(flags map[string]any) {
				flags["plugin-name"] = "release"
			},
			wantError: "plugin flags require --kind plugin",
		},
		{
			name: "kind plugin missing plugin-name",
			mutate: func(flags map[string]any) {
				flags["kind"] = "plugin"
			},
			wantError: "--kind plugin requires --plugin-name",
		},
		{
			name: "kind plugin missing plugin-manifest",
			mutate: func(flags map[string]any) {
				pluginFlags := validPluginInitFlags()
				for key, value := range pluginFlags {
					flags[key] = value
				}
				delete(flags, "plugin-manifest")
			},
			wantError: "--kind plugin requires --plugin-manifest",
		},
		{
			name: "kind plugin missing plugin-asset-prefix",
			mutate: func(flags map[string]any) {
				pluginFlags := validPluginInitFlags()
				for key, value := range pluginFlags {
					flags[key] = value
				}
				delete(flags, "plugin-asset-prefix")
			},
			wantError: "--kind plugin requires --plugin-asset-prefix",
		},
		{
			name: "kind plugin missing plugin-binary-name",
			mutate: func(flags map[string]any) {
				pluginFlags := validPluginInitFlags()
				for key, value := range pluginFlags {
					flags[key] = value
				}
				delete(flags, "plugin-binary-name")
			},
			wantError: "--kind plugin requires --plugin-binary-name",
		},
		{
			name: "plugin manifest missing",
			mutate: func(flags map[string]any) {
				pluginFlags := validPluginInitFlags()
				for key, value := range pluginFlags {
					flags[key] = value
				}
			},
			wantError: "plugin.manifest \"plugin/release/manifest.json\" does not exist",
		},
		{
			name: "plugin unit id not plugin-prefixed",
			setup: func(t *testing.T) {
				writeMinimalPluginManifest(t, "plugin/release/manifest.json", "release", "4.0.0")
				mustWrite(t, ".github/workflows/release-plugin-release.yml", "name: release plugin\n")
			},
			mutate: func(flags map[string]any) {
				pluginFlags := validPluginInitFlags()
				for key, value := range pluginFlags {
					flags[key] = value
				}
				flags["unit"] = "release"
				flags["tag-prefix"] = "release/v"
				flags["plugin-asset-prefix"] = "release"
			},
			wantError: "id must start with plugin-",
		},
		{
			name: "plugin tag prefix mismatch",
			setup: func(t *testing.T) {
				writeMinimalPluginManifest(t, "plugin/release/manifest.json", "release", "4.0.0")
				mustWrite(t, ".github/workflows/release-plugin-release.yml", "name: release plugin\n")
			},
			mutate: func(flags map[string]any) {
				pluginFlags := validPluginInitFlags()
				for key, value := range pluginFlags {
					flags[key] = value
				}
				flags["tag-prefix"] = "release/v"
			},
			wantError: "tagPrefix must be \"plugin-release/v\"",
		},
		{
			name: "plugin asset prefix mismatch",
			setup: func(t *testing.T) {
				writeMinimalPluginManifest(t, "plugin/release/manifest.json", "release", "4.0.0")
				mustWrite(t, ".github/workflows/release-plugin-release.yml", "name: release plugin\n")
			},
			mutate: func(flags map[string]any) {
				pluginFlags := validPluginInitFlags()
				for key, value := range pluginFlags {
					flags[key] = value
				}
				flags["plugin-asset-prefix"] = "release"
			},
			wantError: "assetPrefix must equal unit id",
		},
		{
			name: "invalid paths",
			mutate: func(flags map[string]any) {
				flags["paths"] = "apps/api/**,"
			},
			wantError: "empty entries",
		},
		{
			name: "invalid executor",
			mutate: func(flags map[string]any) {
				flags["executor"] = "custom"
			},
			wantError: "invalid executor",
		},
		{
			name: "invalid delivery",
			mutate: func(flags map[string]any) {
				flags["delivery"] = "ship"
			},
			wantError: "invalid delivery",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWorkingDirectory(t)
			writeV2(t, "cli", "0.1.0")
			mustWrite(t, ".github/workflows/release-api.yml", "name: release api\n")
			if tt.setup != nil {
				tt.setup(t)
			}
			flags := validUnitAddFlags()
			if tt.mutate != nil {
				tt.mutate(flags)
			}
			resp, err := HandleUnitAdd(plugin.Request{Flags: flags})
			if err != nil {
				t.Fatalf("HandleUnitAdd: %v", err)
			}
			if resp.Status != "error" || resp.Error == nil {
				t.Fatalf("expected error, got %#v", resp)
			}
			if !strings.Contains(resp.Error.Message, tt.wantError) {
				t.Fatalf("expected error containing %q, got %#v", tt.wantError, resp.Error)
			}
		})
	}
}

func TestHandleInitExistingConfigHandling(t *testing.T) {
	tests := []struct { //nolint:govet // Test table keeps setup and expected behavior grouped for readability.
		name        string
		setup       func(t *testing.T)
		force       bool
		wantStatus  string
		wantCode    string
		wantVersion string
	}{
		{
			name: "V1 only fails",
			setup: func(t *testing.T) {
				writeV1(t, "0.1.0")
			},
			wantStatus: "error",
			wantCode:   "V1_CONFIG_EXISTS",
		},
		{
			name: "V1 only force still fails",
			setup: func(t *testing.T) {
				writeV1(t, "0.1.0")
			},
			force:      true,
			wantStatus: "error",
			wantCode:   "V1_CONFIG_EXISTS",
		},
		{
			name: "V2 exists fails without force",
			setup: func(t *testing.T) {
				writeV2(t, "old", "1.0.0")
			},
			wantStatus: "error",
			wantCode:   "CONFIG_EXISTS",
		},
		{
			name: "V2 exists force overwrites",
			setup: func(t *testing.T) {
				writeV2(t, "old", "1.0.0")
			},
			force:       true,
			wantStatus:  "success",
			wantVersion: "2.0.0",
		},
		{
			name: "partial V2 fails without force",
			setup: func(t *testing.T) {
				mustWrite(t, releaseconfig.V2ConfigPath("."), `{"schemaVersion":2,"units":[]}`)
			},
			wantStatus: "error",
			wantCode:   "CONFIG_EXISTS",
		},
		{
			name: "partial V2 force recreates both",
			setup: func(t *testing.T) {
				mustWrite(t, releaseconfig.V2ConfigPath("."), `{"schemaVersion":2,"units":[]}`)
			},
			force:       true,
			wantStatus:  "success",
			wantVersion: "2.0.0",
		},
		{
			name: "V1 and V2 conflict with force",
			setup: func(t *testing.T) {
				writeV1(t, "0.1.0")
				writeV2(t, "old", "1.0.0")
			},
			force:      true,
			wantStatus: "error",
			wantCode:   "CONFIG_CONFLICT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWorkingDirectory(t)
			tt.setup(t)

			flags := validInitFlags()
			flags["version"] = "2.0.0"
			if tt.force {
				flags["force"] = true
			}
			resp, err := HandleInit(plugin.Request{Flags: flags})
			if err != nil {
				t.Fatalf("HandleInit: %v", err)
			}
			if resp.Status != tt.wantStatus {
				t.Fatalf("expected status %s, got %#v", tt.wantStatus, resp)
			}
			if tt.wantCode != "" && (resp.Error == nil || resp.Error.Code != tt.wantCode) {
				t.Fatalf("expected code %s, got %#v", tt.wantCode, resp.Error)
			}
			if tt.wantVersion != "" {
				_, state := loadGeneratedV2(t)
				if state.Units["cli"].Version != tt.wantVersion {
					t.Fatalf("expected overwritten version %s, got %#v", tt.wantVersion, state.Units)
				}
			}
		})
	}
}

func TestHandleInitPluginExistingConfigHandling(t *testing.T) {
	tests := []struct { //nolint:govet // Test table keeps setup and expected behavior grouped for readability.
		name       string
		setup      func(t *testing.T)
		force      bool
		wantStatus string
		wantCode   string
	}{
		{
			name: "V1 only fails",
			setup: func(t *testing.T) {
				writeV1(t, "0.1.0")
			},
			wantStatus: "error",
			wantCode:   "V1_CONFIG_EXISTS",
		},
		{
			name: "V2 exists fails without force",
			setup: func(t *testing.T) {
				writeV2(t, "old", "1.0.0")
			},
			wantStatus: "error",
			wantCode:   "CONFIG_EXISTS",
		},
		{
			name: "partial V2 force recreates plugin unit",
			setup: func(t *testing.T) {
				mustWrite(t, releaseconfig.V2StatePath("."), `{"schemaVersion":2,"units":{}}`)
			},
			force:      true,
			wantStatus: "success",
		},
		{
			name: "V1 and V2 conflict with force",
			setup: func(t *testing.T) {
				writeV1(t, "0.1.0")
				writeV2(t, "old", "1.0.0")
			},
			force:      true,
			wantStatus: "error",
			wantCode:   "CONFIG_CONFLICT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWorkingDirectory(t)
			writeMinimalPluginManifest(t, "plugin/release/manifest.json", "release", "4.0.0")
			mustWrite(t, ".github/workflows/release-plugin-release.yml", "name: release plugin\n")
			tt.setup(t)

			flags := validPluginInitFlags()
			if tt.force {
				flags["force"] = true
			}
			resp, err := HandleInit(plugin.Request{Flags: flags})
			if err != nil {
				t.Fatalf("HandleInit: %v", err)
			}
			if resp.Status != tt.wantStatus {
				t.Fatalf("expected status %s, got %#v", tt.wantStatus, resp)
			}
			if tt.wantCode != "" && (resp.Error == nil || resp.Error.Code != tt.wantCode) {
				t.Fatalf("expected code %s, got %#v", tt.wantCode, resp.Error)
			}
			if tt.wantStatus == "success" {
				cfg, state := loadGeneratedV2(t)
				if cfg.Units[0].Kind != releaseconfig.UnitKindPlugin || state.Units["plugin-release"].Version != "4.0.0" {
					t.Fatalf("expected plugin V2 files, got %#v %#v", cfg.Units[0], state.Units)
				}
			}
		})
	}
}

func TestHandleInitInvalidInputs(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(map[string]any)
		wantError string
	}{
		{
			name: "invalid unit",
			mutate: func(flags map[string]any) {
				flags["unit"] = "Bad"
			},
			wantError: "unit id",
		},
		{
			name: "invalid version",
			mutate: func(flags map[string]any) {
				flags["version"] = "v1.2.3"
			},
			wantError: "without leading v",
		},
		{
			name: "invalid executor",
			mutate: func(flags map[string]any) {
				flags["executor"] = "custom"
			},
			wantError: "invalid executor",
		},
		{
			name: "invalid delivery",
			mutate: func(flags map[string]any) {
				flags["delivery"] = "ship"
			},
			wantError: "invalid delivery",
		},
		{
			name: "github actions without workflow",
			mutate: func(flags map[string]any) {
				flags["delivery"] = "github-actions"
				delete(flags, "workflow")
			},
			wantError: "requires workflow",
		},
		{
			name: "github actions workflow outside workflows",
			mutate: func(flags map[string]any) {
				flags["delivery"] = "github-actions"
				flags["workflow"] = "release.yml"
			},
			wantError: ".github/workflows",
		},
		{
			name: "empty paths",
			mutate: func(flags map[string]any) {
				flags["paths"] = "api/**,,docs/**"
			},
			wantError: "empty entries",
		},
		{
			name: "unknown kind",
			mutate: func(flags map[string]any) {
				flags["kind"] = "service"
			},
			wantError: "invalid kind",
		},
		{
			name: "plugin flags without plugin kind",
			mutate: func(flags map[string]any) {
				flags["plugin-name"] = "release"
			},
			wantError: "plugin flags require --kind plugin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWorkingDirectory(t)
			flags := validInitFlags()
			tt.mutate(flags)
			resp, err := HandleInit(plugin.Request{Flags: flags})
			if err != nil {
				t.Fatalf("HandleInit: %v", err)
			}
			if resp.Status != "error" || resp.Error == nil {
				t.Fatalf("expected error, got %#v", resp)
			}
			if !strings.Contains(resp.Error.Message, tt.wantError) {
				t.Fatalf("expected error containing %q, got %#v", tt.wantError, resp.Error)
			}
			assertNoFile(t, legacyV1ConfigFileName)
		})
	}
}

func TestHandleInitInvalidPluginInputs(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T)
		mutate    func(map[string]any)
		wantError string
	}{
		{
			name: "missing plugin-name",
			mutate: func(flags map[string]any) {
				delete(flags, "plugin-name")
			},
			wantError: "--kind plugin requires --plugin-name",
		},
		{
			name: "missing plugin-manifest",
			mutate: func(flags map[string]any) {
				delete(flags, "plugin-manifest")
			},
			wantError: "--kind plugin requires --plugin-manifest",
		},
		{
			name: "missing plugin-asset-prefix",
			mutate: func(flags map[string]any) {
				delete(flags, "plugin-asset-prefix")
			},
			wantError: "--kind plugin requires --plugin-asset-prefix",
		},
		{
			name: "missing plugin-binary-name",
			mutate: func(flags map[string]any) {
				delete(flags, "plugin-binary-name")
			},
			wantError: "--kind plugin requires --plugin-binary-name",
		},
		{
			name: "invalid plugin-name",
			mutate: func(flags map[string]any) {
				flags["plugin-name"] = "plugin-release"
			},
			wantError: "must not start with plugin-",
		},
		{
			name: "invalid plugin-manifest path",
			mutate: func(flags map[string]any) {
				flags["plugin-manifest"] = "../manifest.json"
			},
			wantError: "repository-root-relative",
		},
		{
			name: "missing plugin-manifest file",
			setup: func(t *testing.T) {
				if err := os.Remove("plugin/release/manifest.json"); err != nil {
					t.Fatalf("remove manifest: %v", err)
				}
			},
			wantError: "does not exist",
		},
		{
			name: "invalid plugin-asset-prefix",
			mutate: func(flags map[string]any) {
				flags["plugin-asset-prefix"] = "plugin.release"
			},
			wantError: "must match [a-z0-9][a-z0-9-]*",
		},
		{
			name: "invalid plugin-binary-name",
			mutate: func(flags map[string]any) {
				flags["plugin-binary-name"] = "plugin/release"
			},
			wantError: "must match [a-z0-9][a-z0-9-]*",
		},
		{
			name: "plugin unit id not starting with plugin",
			mutate: func(flags map[string]any) {
				flags["unit"] = "release"
				flags["tag-prefix"] = "release/v"
				flags["plugin-asset-prefix"] = "release"
			},
			wantError: "id must start with plugin-",
		},
		{
			name: "plugin tagPrefix mismatch",
			mutate: func(flags map[string]any) {
				flags["tag-prefix"] = "v"
			},
			wantError: `tagPrefix must be "plugin-release/v"`,
		},
		{
			name: "plugin assetPrefix mismatch",
			mutate: func(flags map[string]any) {
				flags["plugin-asset-prefix"] = "plugin-other"
			},
			wantError: "assetPrefix must equal unit id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWorkingDirectory(t)
			writeMinimalPluginManifest(t, "plugin/release/manifest.json", "release", "4.0.0")
			mustWrite(t, ".github/workflows/release-plugin-release.yml", "name: release plugin\n")

			flags := validPluginInitFlags()
			if tt.setup != nil {
				tt.setup(t)
			}
			if tt.mutate != nil {
				tt.mutate(flags)
			}
			resp, err := HandleInit(plugin.Request{Flags: flags})
			if err != nil {
				t.Fatalf("HandleInit: %v", err)
			}
			if resp.Status != "error" || resp.Error == nil {
				t.Fatalf("expected error, got %#v", resp)
			}
			if !strings.Contains(resp.Error.Message, tt.wantError) {
				t.Fatalf("expected error containing %q, got %#v", tt.wantError, resp.Error)
			}
			assertNoFile(t, legacyV1ConfigFileName)
		})
	}
}

func TestManifestExposesV2InitFlagsOnly(t *testing.T) {
	data, err := os.ReadFile("../../manifest.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest struct {
		Commands []struct {
			Name  string `json:"name"`
			Flags []struct {
				Name string `json:"name"`
			} `json:"flags"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	for _, command := range manifest.Commands {
		if command.Name != "init" {
			continue
		}
		flags := map[string]bool{}
		for _, flag := range command.Flags {
			flags[flag.Name] = true
		}
		for _, want := range []string{"unit", "display-name", "version", "executor", "delivery", "workflow", "tag-prefix", "working-directory", "paths", "kind", "plugin-name", "plugin-manifest", "plugin-asset-prefix", "plugin-binary-name", "force"} {
			if !flags[want] {
				t.Fatalf("manifest init flags missing %q: %#v", want, flags)
			}
		}
		for _, legacy := range []string{"project-type", "release-system", "metadata"} {
			if flags[legacy] {
				t.Fatalf("manifest still exposes legacy flag %q", legacy)
			}
		}
		return
	}

	t.Fatal("init command not found")
}

func initOptionRows(t *testing.T, resp *plugin.Response) []map[string]any {
	t.Helper()
	items, ok := resp.Data["items"].([]map[string]any)
	if !ok {
		t.Fatalf("unexpected init-options data: %#v", resp.Data["items"])
	}
	return items
}

func rowString(t *testing.T, row map[string]any, key string) string {
	t.Helper()
	value, ok := row[key].(string)
	if !ok {
		t.Fatalf("row key %s is not a string: %#v", key, row)
	}
	return value
}

func assertInitOptionContains(t *testing.T, option, description string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(description, fragment) {
			t.Fatalf("init-options %s description %q does not contain %q", option, description, fragment)
		}
	}
}

func initOptionNames(t *testing.T, resp *plugin.Response) map[string]bool {
	t.Helper()
	items, ok := resp.Data["items"].([]map[string]any)
	if !ok {
		t.Fatalf("unexpected init-options data: %#v", resp.Data["items"])
	}
	options := map[string]bool{}
	for _, item := range items {
		option, ok := item["option"].(string)
		if !ok {
			t.Fatalf("unexpected option row: %#v", item)
		}
		options[option] = true
	}
	return options
}

func validInitFlags() map[string]any {
	return map[string]any{
		"unit":              "cli",
		"display-name":      "CLI",
		"version":           "0.1.0",
		"executor":          "goreleaser",
		"delivery":          "local",
		"tag-prefix":        "v",
		"working-directory": ".",
		"paths":             "**",
	}
}

func validPluginInitFlags() map[string]any {
	return map[string]any{
		"unit":                "plugin-release",
		"display-name":        "neko-cli release plugin",
		"version":             "4.0.0",
		"executor":            "goreleaser",
		"delivery":            "github-actions",
		"workflow":            ".github/workflows/release-plugin-release.yml",
		"tag-prefix":          "plugin-release/v",
		"working-directory":   ".",
		"paths":               "plugin/release/**,docs/plugins/release.md",
		"kind":                "plugin",
		"plugin-name":         "release",
		"plugin-manifest":     "plugin/release/manifest.json",
		"plugin-asset-prefix": "plugin-release",
		"plugin-binary-name":  "plugin-release",
	}
}

func validUnitAddFlags() map[string]any {
	return map[string]any{
		"unit":              "api",
		"display-name":      "API",
		"version":           "1.2.3",
		"executor":          "goreleaser",
		"delivery":          "github-actions",
		"workflow":          ".github/workflows/release-api.yml",
		"tag-prefix":        "api/v",
		"working-directory": ".",
		"paths":             "apps/api/**",
	}
}

func loadGeneratedV2(t *testing.T) (releaseconfig.V2ReleaseConfig, releaseconfig.V2ReleaseState) {
	t.Helper()
	configData, err := os.ReadFile(releaseconfig.V2ConfigPath("."))
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	stateData, err := os.ReadFile(releaseconfig.V2StatePath("."))
	if err != nil {
		t.Fatalf("read generated state: %v", err)
	}
	var cfg releaseconfig.V2ReleaseConfig
	if err := json.Unmarshal(configData, &cfg); err != nil {
		t.Fatalf("decode generated config: %v\n%s", err, configData)
	}
	var state releaseconfig.V2ReleaseState
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatalf("decode generated state: %v\n%s", err, stateData)
	}
	return cfg, state
}

func writeV1(t *testing.T, version string) {
	t.Helper()
	mustWrite(t, legacyV1ConfigFileName, `{
  "project-name": "old",
  "project-owner": "owner",
  "project-type": "backend",
  "release-system": "goreleaser",
  "version": "`+version+`"
}`)
}

func writeV2(t *testing.T, unitID, version string) {
	t.Helper()
	mustWrite(t, releaseconfig.V2ConfigPath("."), `{
  "schemaVersion": 2,
  "units": [
    {
      "id": "`+unitID+`",
      "paths": ["**"],
      "workingDirectory": ".",
      "tagPrefix": "v",
      "executor": {
        "type": "goreleaser",
        "delivery": "local"
      }
    }
  ]
}`)
	mustWrite(t, releaseconfig.V2StatePath("."), `{
  "schemaVersion": 2,
  "units": {
    "`+unitID+`": {
      "version": "`+version+`"
    }
  }
}`)
}

func writeV2WithExtraState(t *testing.T) {
	t.Helper()
	mustWrite(t, releaseconfig.V2ConfigPath("."), `{
  "schemaVersion": 2,
  "units": [
    {
      "id": "cli",
      "paths": ["**"],
      "workingDirectory": ".",
      "tagPrefix": "v",
      "executor": {
        "type": "goreleaser",
        "delivery": "local"
      }
    }
  ]
}`)
	mustWrite(t, releaseconfig.V2StatePath("."), `{
  "schemaVersion": 2,
  "units": {
    "cli": {
      "version": "0.1.0"
    },
    "api": {
      "version": "1.0.0"
    }
  }
}`)
}

func writeExistingPluginV2(t *testing.T) {
	t.Helper()
	writeMinimalPluginManifest(t, "plugin/release/manifest.json", "release", "4.0.0")
	mustWrite(t, ".github/workflows/release-plugin-release.yml", "name: release plugin\n")
	mustWrite(t, releaseconfig.V2ConfigPath("."), `{
  "schemaVersion": 2,
  "units": [
    {
      "id": "plugin-release",
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
      "executor": {
        "type": "goreleaser",
        "delivery": "github-actions",
        "workflow": ".github/workflows/release-plugin-release.yml"
      }
    }
  ]
}`)
	mustWrite(t, releaseconfig.V2StatePath("."), `{
  "schemaVersion": 2,
  "units": {
    "plugin-release": {
      "version": "4.0.0"
    }
  }
}`)
}

func withWorkingDirectory(t *testing.T) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeMinimalPluginManifest(t *testing.T, path, name, version string) {
	t.Helper()
	mustWrite(t, path, `{
  "name": "`+name+`",
  "version": "`+version+`",
  "description": "Release management plugin",
  "commands": [],
  "renderer_types": ["table", "json", "text"]
}`)
}

func assertNoFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("unexpected file exists: %s", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}
