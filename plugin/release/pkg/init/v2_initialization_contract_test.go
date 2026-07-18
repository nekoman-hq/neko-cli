package init

import (
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestInitAndUnitAddPolicyPrecedesUnitFlagValidation(t *testing.T) {
	t.Run("init existing config", func(t *testing.T) {
		withWorkingDirectory(t)
		writeV2(t, "cli", "0.1.0")

		resp, err := HandleInit(plugin.Request{Flags: map[string]any{}})
		if err != nil {
			t.Fatalf("HandleInit: %v", err)
		}
		if resp.Error == nil || resp.Error.Code != "CONFIG_EXISTS" {
			t.Fatalf("existing-config policy order changed: %#v", resp)
		}
	})

	t.Run("unit-add missing repository", func(t *testing.T) {
		withWorkingDirectory(t)

		resp, err := HandleUnitAdd(plugin.Request{Flags: map[string]any{}})
		if err != nil {
			t.Fatalf("HandleUnitAdd: %v", err)
		}
		if resp.Error == nil || resp.Error.Code != "V2_CONFIG_MISSING" {
			t.Fatalf("missing-repository policy order changed: %#v", resp)
		}
	})
}

func TestUnitAddErrorMetadataRetainsInitCompatibilityCommand(t *testing.T) {
	withWorkingDirectory(t)

	resp, err := HandleUnitAdd(plugin.Request{Flags: validUnitAddFlags()})
	if err != nil {
		t.Fatalf("HandleUnitAdd: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != "V2_CONFIG_MISSING" {
		t.Fatalf("expected missing V2 error, got %#v", resp)
	}
	if resp.Metadata.Command != "init" {
		t.Fatalf("unit-add error metadata command = %q, want characterized compatibility value init", resp.Metadata.Command)
	}
}

func TestInitPersistsCanonicalBytesAndModes(t *testing.T) {
	withWorkingDirectory(t)
	writeValidInitWorkflow(t)

	resp, err := HandleInit(plugin.Request{Flags: validInitFlags()})
	if err != nil {
		t.Fatalf("HandleInit: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp)
	}

	wantConfig := `{
  "schemaVersion": 2,
  "units": [
    {
      "id": "cli",
      "displayName": "CLI",
      "paths": [
        "**"
      ],
      "workingDirectory": ".",
      "tagPrefix": "v",
      "executor": {
        "type": "goreleaser",
        "delivery": "github-actions",
        "workflow": ".github/workflows/release-cli.yml"
      }
    }
  ]
}
`
	wantState := `{
  "schemaVersion": 2,
  "units": {
    "cli": {
      "version": "0.1.0"
    }
  }
}
`
	assertFileBytesAndMode(t, releaseconfig.V2ConfigPath("."), []byte(wantConfig), 0644)
	assertFileBytesAndMode(t, releaseconfig.V2StatePath("."), []byte(wantState), 0644)

	wantKeys := []string{
		"config_file", "delivery", "display_name", "executor", "kind", "next_steps", "paths",
		"plugin", "schema", "state_file", "tag_prefix", "unit", "version", "workflow", "working_directory",
	}
	gotKeys := make([]string, 0, len(resp.Data))
	for key := range resp.Data {
		gotKeys = append(gotKeys, key)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) || resp.RendererHint != "text" || resp.Metadata.Command != "init" {
		t.Fatalf("init response contract changed: keys=%v response=%#v", gotKeys, resp)
	}
}

func assertFileBytesAndMode(t *testing.T, path string, want []byte, mode os.FileMode) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s bytes changed:\n%s", path, got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if gotMode := info.Mode().Perm(); gotMode != mode {
		t.Fatalf("%s mode = %04o, want %04o", path, gotMode, mode)
	}
}
