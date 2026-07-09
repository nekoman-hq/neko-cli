//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package validate

//lint:file-ignore SA1019 V1 validation compatibility intentionally uses deprecated V1 APIs during migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestHandleValidateV2Show(t *testing.T) {
	withWorkingDirectory(t)
	writeV2(t, `{"schemaVersion":2,"units":[{
  "id":"default",
  "paths":["**"],
  "tagPrefix":"v",
  "executor":{"type":"goreleaser"}
}]}`, `{"schemaVersion":2,"units":{"default":{"version":"2.2.4"}}}`)

	resp, err := HandleValidate(plugin.Request{Flags: map[string]any{"show": true}})
	if err != nil {
		t.Fatalf("HandleValidate: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	if !itemsContain(resp.Data["items"], "delivery=local") {
		t.Fatalf("expected normalized delivery in show output, got %#v", resp.Data["items"])
	}
	if !itemsContain(resp.Data["items"], "workflow=not applicable") {
		t.Fatalf("expected local workflow to be not applicable, got %#v", resp.Data["items"])
	}
}

func TestHandleValidateV2ShowDisplaysGitHubActionsWorkflow(t *testing.T) {
	withWorkingDirectory(t)
	mustWrite(t, ".github/workflows/release-api.yml", "name: release api\n")
	writeV2(t, `{"schemaVersion":2,"units":[{
  "id":"api",
  "paths":["api/**"],
  "workingDirectory":".",
  "tagPrefix":"api/v",
  "executor":{"type":"jreleaser","delivery":"github-actions","workflow":".github/workflows/release-api.yml"}
}]}`, `{"schemaVersion":2,"units":{"api":{"version":"0.2.1"}}}`)

	resp, err := HandleValidate(plugin.Request{Flags: map[string]any{"show": true}})
	if err != nil {
		t.Fatalf("HandleValidate: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	if !itemsContain(resp.Data["items"], "delivery=github-actions") {
		t.Fatalf("expected github-actions delivery, got %#v", resp.Data["items"])
	}
	if !itemsContain(resp.Data["items"], "workflow=.github/workflows/release-api.yml") {
		t.Fatalf("expected workflow in show output, got %#v", resp.Data["items"])
	}
}

func TestHandleValidateV2ShowDisplaysPluginMetadata(t *testing.T) {
	withWorkingDirectory(t)
	mustWrite(t, ".github/workflows/release-plugin-release.yml", "name: release plugin\n")
	mustWrite(t, "plugin/release/manifest.json", `{"name":"release","version":"4.0.2"}`)
	writeV2(t, `{"schemaVersion":2,"units":[{
  "id":"plugin-release",
  "paths":["plugin/release/**"],
  "workingDirectory":".",
  "tagPrefix":"plugin-release/v",
  "kind":"plugin",
  "plugin":{
    "name":"release",
    "manifest":"plugin/release/manifest.json",
    "assetPrefix":"plugin-release",
    "binaryName":"plugin-release"
  },
  "executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-plugin-release.yml"}
}]}`, `{"schemaVersion":2,"units":{"plugin-release":{"version":"4.0.2"}}}`)

	resp, err := HandleValidate(plugin.Request{Flags: map[string]any{"show": true}})
	if err != nil {
		t.Fatalf("HandleValidate: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	for _, want := range []string{
		"kind=plugin",
		"plugin=release",
		"pluginManifest=plugin/release/manifest.json",
		"pluginAssetPrefix=plugin-release",
		"pluginBinary=plugin-release",
	} {
		if !itemsContain(resp.Data["items"], want) {
			t.Fatalf("expected show output to contain %q, got %#v", want, resp.Data["items"])
		}
	}
}

func TestHandleValidateV2ShowFocusesRequestedUnit(t *testing.T) {
	withWorkingDirectory(t)
	writeV2(t, `{"schemaVersion":2,"units":[
  {"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"goreleaser"}},
  {"id":"web","paths":["web/**"],"workingDirectory":".","tagPrefix":"web/v","executor":{"type":"release-it"}}
]}`, `{"schemaVersion":2,"units":{"api":{"version":"1.0.0"},"web":{"version":"2.0.0"}}}`)

	resp, err := HandleValidate(plugin.Request{Flags: map[string]any{"show": true, "unit": "api"}})
	if err != nil {
		t.Fatalf("HandleValidate: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	if !itemsContain(resp.Data["items"], "Unit api") {
		t.Fatalf("expected api row, got %#v", resp.Data["items"])
	}
	if itemsContain(resp.Data["items"], "Unit web") {
		t.Fatalf("web row should not be shown when api is focused: %#v", resp.Data["items"])
	}
}

func TestHandleValidateV1StillUsesLegacyConfig(t *testing.T) {
	withWorkingDirectory(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	mustWrite(t, config.V1FileName, `{
  "project-name": "neko-cli",
  "project-owner": "nekoman-hq",
  "project-type": "backend",
  "release-system": "goreleaser",
  "version": "1.2.3"
}`)
	mustWrite(t, ".goreleaser.yml", "{}")

	resp, err := HandleValidate(plugin.Request{})
	if err != nil {
		t.Fatalf("HandleValidate: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected V1 success, got %#v", resp.Error)
	}
}

func writeV2(t *testing.T, cfg, state string) {
	t.Helper()
	mustWrite(t, config.V2ConfigPath("."), cfg)
	mustWrite(t, config.V2StatePath("."), state)
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
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func itemsContain(items any, expected string) bool {
	rows, ok := items.([]map[string]any)
	if !ok {
		return false
	}
	for _, row := range rows {
		for _, value := range row {
			if value == expected {
				return true
			}
			if s, ok := value.(string); ok && strings.Contains(s, expected) {
				return true
			}
		}
	}
	return false
}
