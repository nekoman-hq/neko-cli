package pluginindex

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestGenerateBuildsDeterministicIndexFromV2PluginUnits(t *testing.T) {
	root := newIndexTestRepo(t)

	index, err := Generate(context.Background(), GenerateOptions{Root: root})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if index.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d, want 1", index.SchemaVersion)
	}
	if index.Repository != DefaultRepository {
		t.Fatalf("repository = %q, want %q", index.Repository, DefaultRepository)
	}
	if len(index.Plugins) != 2 {
		t.Fatalf("plugins = %d, want 2: %#v", len(index.Plugins), index.Plugins)
	}
	if index.Plugins[0].Name != "release" || index.Plugins[1].Name != "ui" {
		t.Fatalf("plugins not sorted by name: %#v", index.Plugins)
	}

	release := index.Plugins[0]
	if release.Unit != "plugin-release" ||
		release.Version != "4.0.2" ||
		release.Tag != "plugin-release/v4.0.2" ||
		release.TagPrefix != "plugin-release/v" ||
		release.Manifest != "plugin/release/manifest.json" ||
		release.AssetPrefix != "plugin-release" ||
		release.BinaryName != "plugin-release" ||
		release.Description != "Release management plugin" {
		t.Fatalf("unexpected release plugin entry: %#v", release)
	}

	var first bytes.Buffer
	if err := Write(index, &first); err != nil {
		t.Fatalf("Write first: %v", err)
	}
	var second bytes.Buffer
	if err := Write(index, &second); err != nil {
		t.Fatalf("Write second: %v", err)
	}
	if first.String() != second.String() {
		t.Fatalf("index output is not deterministic\nfirst:\n%s\nsecond:\n%s", first.String(), second.String())
	}

	expected := `{
  "schemaVersion": 1,
  "repository": "nekoman-hq/neko-cli",
  "plugins": [
    {
      "name": "release",
      "unit": "plugin-release",
      "version": "4.0.2",
      "tag": "plugin-release/v4.0.2",
      "tagPrefix": "plugin-release/v",
      "manifest": "plugin/release/manifest.json",
      "assetPrefix": "plugin-release",
      "binaryName": "plugin-release",
      "description": "Release management plugin"
    },
    {
      "name": "ui",
      "unit": "plugin-ui",
      "version": "1.0.0",
      "tag": "plugin-ui/v1.0.0",
      "tagPrefix": "plugin-ui/v",
      "manifest": "plugin/ui/manifest.json",
      "assetPrefix": "plugin-ui",
      "binaryName": "plugin-ui",
      "description": "UI component helper plugin"
    }
  ]
}
`
	if first.String() != expected {
		t.Fatalf("unexpected index JSON\nwant:\n%s\ngot:\n%s", expected, first.String())
	}
}

func TestGenerateValidationFailures(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(t *testing.T, root string)
		wantErr string
	}{
		{
			name: "missing state entry",
			mutate: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".neko", "release.state.json"), `{"schemaVersion":2,"units":{"cli":{"version":"3.0.2"},"plugin-ui":{"version":"1.0.0"}}}`)
			},
			wantErr: `plugin unit "plugin-release" has no matching state entry`,
		},
		{
			name: "invalid semver",
			mutate: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".neko", "release.state.json"), `{"schemaVersion":2,"units":{"cli":{"version":"3.0.2"},"plugin-release":{"version":"v4.0.2"},"plugin-ui":{"version":"1.0.0"}}}`)
			},
			wantErr: `state version "v4.0.2" is not valid semver`,
		},
		{
			name: "missing manifest",
			mutate: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "plugin", "release", "manifest.json")); err != nil {
					t.Fatalf("remove manifest: %v", err)
				}
			},
			wantErr: `manifest file is missing: plugin/release/manifest.json`,
		},
		{
			name: "malformed manifest",
			mutate: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, "plugin", "release", "manifest.json"), `{"name":`)
			},
			wantErr: `manifest plugin/release/manifest.json is malformed`,
		},
		{
			name: "manifest name mismatch",
			mutate: func(t *testing.T, root string) {
				writeManifest(t, root, "plugin/release/manifest.json", "not-release", "4.0.2", "Release management plugin")
			},
			wantErr: `manifest name "not-release" does not match plugin.name "release"`,
		},
		{
			name: "manifest version mismatch",
			mutate: func(t *testing.T, root string) {
				writeManifest(t, root, "plugin/release/manifest.json", "release", "4.0.1", "Release management plugin")
			},
			wantErr: `manifest version "4.0.1" does not match state version "4.0.2"`,
		},
		{
			name: "duplicate plugin names",
			mutate: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".neko", "release.config.json"), configJSON(
					pluginUnitJSON("plugin-release", "plugin-release/v", "release", "plugin/release/manifest.json", "plugin-release", "plugin-release"),
					pluginUnitJSON("plugin-ui", "plugin-ui/v", "release", "plugin/ui/manifest.json", "plugin-ui", "plugin-ui"),
				))
				writeManifest(t, root, "plugin/ui/manifest.json", "release", "1.0.0", "UI component helper plugin")
			},
			wantErr: `duplicate plugin name "release"`,
		},
		{
			name: "duplicate unit ids",
			mutate: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".neko", "release.config.json"), configJSON(
					pluginUnitJSON("plugin-release", "plugin-release/v", "release", "plugin/release/manifest.json", "plugin-release", "plugin-release"),
					pluginUnitJSON("plugin-release", "plugin-release/v", "ui", "plugin/ui/manifest.json", "plugin-ui", "plugin-ui"),
				))
			},
			wantErr: `duplicate unit id "plugin-release"`,
		},
		{
			name: "duplicate tags",
			mutate: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".neko", "release.config.json"), configJSON(
					pluginUnitJSON("plugin-release", "shared/v", "release", "plugin/release/manifest.json", "plugin-release", "plugin-release"),
					pluginUnitJSON("plugin-ui", "shared/v", "ui", "plugin/ui/manifest.json", "plugin-ui", "plugin-ui"),
				))
				writeFile(t, filepath.Join(root, ".neko", "release.state.json"), `{"schemaVersion":2,"units":{"cli":{"version":"3.0.2"},"plugin-release":{"version":"1.0.0"},"plugin-ui":{"version":"1.0.0"}}}`)
				writeManifest(t, root, "plugin/release/manifest.json", "release", "1.0.0", "Release management plugin")
			},
			wantErr: `duplicate tag "shared/v1.0.0"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newIndexTestRepo(t)
			tt.mutate(t, root)
			_, err := Generate(context.Background(), GenerateOptions{Root: root})
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want contains %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestHandlePluginIndexCheck(t *testing.T) {
	root := newIndexTestRepo(t)
	t.Chdir(root)

	resp, err := HandlePluginIndex(pluginRequest(map[string]any{"check": true}))
	if err != nil {
		t.Fatalf("HandlePluginIndex: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.RendererHint != "" {
		t.Fatalf("check response must not use raw renderer hint: %q", resp.RendererHint)
	}
	items, ok := resp.Data["items"].([]map[string]any)
	if !ok || len(items) != 3 {
		t.Fatalf("unexpected check items: %#v", resp.Data["items"])
	}
}

func TestHandlePluginIndexDefaultReturnsRawJSONIndex(t *testing.T) {
	root := newIndexTestRepo(t)
	t.Chdir(root)

	resp, err := HandlePluginIndex(pluginRequest(nil))
	if err != nil {
		t.Fatalf("HandlePluginIndex: %v", err)
	}
	if resp.RendererHint != "raw-json" {
		t.Fatalf("renderer hint = %q, want raw-json", resp.RendererHint)
	}
	data, ok := resp.Data["raw"].(string)
	if !ok {
		t.Fatalf("raw index is %T, want string", resp.Data["raw"])
	}
	var index Index
	if err := json.Unmarshal([]byte(data), &index); err != nil {
		t.Fatalf("raw index is not valid JSON: %v", err)
	}
	if len(index.Plugins) != 2 {
		t.Fatalf("plugins = %d, want 2", len(index.Plugins))
	}
}

func TestHandlePluginIndexWritesOutputFile(t *testing.T) {
	root := newIndexTestRepo(t)
	t.Chdir(root)
	output := filepath.Join(newPluginIndexTempDir(t), "plugin-index.json")

	resp, err := HandlePluginIndex(pluginRequest(map[string]any{"output-file": output}))
	if err != nil {
		t.Fatalf("HandlePluginIndex: %v", err)
	}
	if resp.RendererHint != "" {
		t.Fatalf("output response must not use raw renderer hint: %q", resp.RendererHint)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("output is not valid index JSON: %v", err)
	}
	if len(index.Plugins) != 2 {
		t.Fatalf("plugins = %d, want 2", len(index.Plugins))
	}
}

func TestHandlePluginIndexRejectsCheckWithOutput(t *testing.T) {
	root := newIndexTestRepo(t)
	t.Chdir(root)

	_, err := HandlePluginIndex(pluginRequest(map[string]any{"check": true, "output-file": filepath.Join(newPluginIndexTempDir(t), "plugin-index.json")}))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--check cannot be used with --output-file") {
		t.Fatalf("error = %q", err.Error())
	}
}

func pluginRequest(flags map[string]any) plugin.Request {
	if flags == nil {
		flags = map[string]any{}
	}
	return plugin.Request{Command: CommandName, Flags: flags}
}

func newPluginIndexTempDir(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("/private/tmp", "neko-plugin-index-test-*")
	if err != nil {
		t.Fatalf("create plugin-index test directory: %v", err)
	}
	t.Cleanup(func() {
		if removeErr := os.RemoveAll(root); removeErr != nil {
			t.Errorf("remove plugin-index test directory %s: %v", root, removeErr)
		}
	})
	return root
}

func newIndexTestRepo(t *testing.T) string {
	t.Helper()
	root := newPluginIndexTempDir(t)
	writeFile(t, filepath.Join(root, ".neko", "release.config.json"), configJSON(
		`{"id":"cli","paths":["**"],"workingDirectory":".","tagPrefix":"v","executor":{"type":"goreleaser"}}`,
		pluginUnitJSON("plugin-ui", "plugin-ui/v", "ui", "plugin/ui/manifest.json", "plugin-ui", "plugin-ui"),
		pluginUnitJSON("plugin-release", "plugin-release/v", "release", "plugin/release/manifest.json", "plugin-release", "plugin-release"),
	))
	writeFile(t, filepath.Join(root, ".neko", "release.state.json"), `{"schemaVersion":2,"units":{"cli":{"version":"3.0.2"},"plugin-release":{"version":"4.0.2"},"plugin-ui":{"version":"1.0.0"}}}`)
	writeManifest(t, root, "plugin/release/manifest.json", "release", "4.0.2", "Release management plugin")
	writeManifest(t, root, "plugin/ui/manifest.json", "ui", "1.0.0", "UI component helper plugin")
	return root
}

func configJSON(units ...string) string {
	return `{"schemaVersion":2,"units":[` + strings.Join(units, ",") + `]}`
}

func pluginUnitJSON(unitID, tagPrefix, pluginName, manifest, assetPrefix, binaryName string) string {
	return `{"id":"` + unitID + `","paths":["plugin/**"],"workingDirectory":".","tagPrefix":"` + tagPrefix + `","kind":"plugin","plugin":{"name":"` + pluginName + `","manifest":"` + manifest + `","assetPrefix":"` + assetPrefix + `","binaryName":"` + binaryName + `"},"executor":{"type":"goreleaser"}}`
}

func writeManifest(t *testing.T, root, path, name, version, description string) {
	t.Helper()
	writeFile(t, filepath.Join(root, filepath.FromSlash(path)), `{"name":"`+name+`","version":"`+version+`","description":"`+description+`"}`)
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
