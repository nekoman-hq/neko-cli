package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestDX2HandleRequestAtIsolatesValidateAcrossRepositories(t *testing.T) {
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	otherRoot := t.TempDir()
	writeDX2ReleaseRepository(t, firstRoot, "api", "1.2.3")
	writeDX2ReleaseRepository(t, secondRoot, "web", "2.0.0")
	first := mustValidateDX2Root(t, firstRoot)
	second := mustValidateDX2Root(t, secondRoot)
	withDX2ProcessWorkingDirectory(t, otherRoot)

	firstResp, err := handleRequestAt(first, plugin.Request{Command: "validate", Flags: map[string]any{"show": true}}, nil)
	if err != nil {
		t.Fatalf("validate first: %v", err)
	}
	secondResp, err := handleRequestAt(second, plugin.Request{Command: "validate", Flags: map[string]any{"show": true}}, nil)
	if err != nil {
		t.Fatalf("validate second: %v", err)
	}

	firstItems := fmt.Sprint(firstResp.Data["items"])
	secondItems := fmt.Sprint(secondResp.Data["items"])
	if !strings.Contains(firstItems, "Unit api") || !strings.Contains(firstItems, "version=1.2.3") || strings.Contains(firstItems, "Unit web") {
		t.Fatalf("first repository validation leaked or missed data: %s", firstItems)
	}
	if !strings.Contains(secondItems, "Unit web") || !strings.Contains(secondItems, "version=2.0.0") || strings.Contains(secondItems, "Unit api") {
		t.Fatalf("second repository validation leaked or missed data: %s", secondItems)
	}
	assertDX2Cwd(t, otherRoot)
}

func TestDX2HandleRequestAtIsolatesPluginIndexAcrossRepositories(t *testing.T) {
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	otherRoot := t.TempDir()
	writeDX2PluginRepository(t, firstRoot, "plugin-release", "release", "4.0.7")
	writeDX2PluginRepository(t, secondRoot, "plugin-audit", "audit", "0.3.0")
	first := mustValidateDX2Root(t, firstRoot)
	second := mustValidateDX2Root(t, secondRoot)
	withDX2ProcessWorkingDirectory(t, otherRoot)

	firstResp, err := handleRequestAt(first, plugin.Request{Command: "plugin-index"}, nil)
	if err != nil {
		t.Fatalf("plugin-index first: %v", err)
	}
	secondResp, err := handleRequestAt(second, plugin.Request{Command: "plugin-index"}, nil)
	if err != nil {
		t.Fatalf("plugin-index second: %v", err)
	}

	firstRaw := fmt.Sprint(firstResp.Data["raw"])
	secondRaw := fmt.Sprint(secondResp.Data["raw"])
	if !strings.Contains(firstRaw, `"name": "release"`) || !strings.Contains(firstRaw, `"tag": "plugin-release/v4.0.7"`) || strings.Contains(firstRaw, `"name": "audit"`) {
		t.Fatalf("first plugin index leaked or missed data: %s", firstRaw)
	}
	if !strings.Contains(secondRaw, `"name": "audit"`) || !strings.Contains(secondRaw, `"tag": "plugin-audit/v0.3.0"`) || strings.Contains(secondRaw, `"name": "release"`) {
		t.Fatalf("second plugin index leaked or missed data: %s", secondRaw)
	}
	assertDX2Cwd(t, otherRoot)
}

func writeDX2ReleaseRepository(t *testing.T, root, unitID, version string) {
	t.Helper()
	configJSON := fmt.Sprintf(`{"schemaVersion":2,"units":[{"id":"%s","paths":["**"],"workingDirectory":".","tagPrefix":"%s/v","executor":{"type":"goreleaser","delivery":"local"}}]}`, unitID, unitID)
	stateJSON := fmt.Sprintf(`{"schemaVersion":2,"units":{"%s":{"version":"%s"}}}`, unitID, version)
	writeDX2File(t, releaseconfig.V2ConfigPath(root), configJSON)
	writeDX2File(t, releaseconfig.V2StatePath(root), stateJSON)
}

func writeDX2PluginRepository(t *testing.T, root, unitID, pluginName, version string) {
	t.Helper()
	manifestPath := filepath.Join("plugins", pluginName, "manifest.json")
	writeDX2File(t, filepath.Join(root, manifestPath), fmt.Sprintf(`{"name":"%s","version":"%s","description":"%s plugin"}`, pluginName, version, pluginName))
	configJSON := fmt.Sprintf(`{"schemaVersion":2,"units":[{"id":"%s","paths":["plugins/%s/**"],"workingDirectory":".","tagPrefix":"%s/v","kind":"plugin","plugin":{"name":"%s","manifest":"%s","assetPrefix":"%s","binaryName":"%s"},"executor":{"type":"goreleaser","delivery":"local"}}]}`, unitID, pluginName, unitID, pluginName, manifestPath, unitID, unitID)
	stateJSON := fmt.Sprintf(`{"schemaVersion":2,"units":{"%s":{"version":"%s"}}}`, unitID, version)
	writeDX2File(t, releaseconfig.V2ConfigPath(root), configJSON)
	writeDX2File(t, releaseconfig.V2StatePath(root), stateJSON)
}

func writeDX2File(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustValidateDX2Root(t *testing.T, root string) workspace.RepositoryRoot {
	t.Helper()
	resolved, err := workspace.ValidateRepositoryRoot(root)
	if err != nil {
		t.Fatalf("ValidateRepositoryRoot(%s): %v", root, err)
	}
	return resolved
}

func withDX2ProcessWorkingDirectory(t *testing.T, root string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir %s: %v", root, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd %s: %v", cwd, err)
		}
	})
}

func assertDX2Cwd(t *testing.T, want string) {
	t.Helper()
	got, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	gotEval, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("eval current cwd: %v", err)
	}
	wantEval, err := filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatalf("eval expected cwd: %v", err)
	}
	if gotEval != wantEval {
		t.Fatalf("cwd changed: got %s want %s", gotEval, wantEval)
	}
}
