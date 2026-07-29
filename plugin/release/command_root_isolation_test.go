package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestHandleRequestAtIsolatesValidateAcrossRepositories(t *testing.T) {
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	otherRoot := t.TempDir()
	writeExplicitRootReleaseRepository(t, firstRoot, "api", "1.2.3")
	writeExplicitRootReleaseRepository(t, secondRoot, "web", "2.0.0")
	first := mustValidateRepositoryRoot(t, firstRoot)
	second := mustValidateRepositoryRoot(t, secondRoot)
	withProcessWorkingDirectory(t, otherRoot)

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
	assertProcessWorkingDirectory(t, otherRoot)
}

func TestHandleRequestAtIsolatesPluginIndexAcrossRepositories(t *testing.T) {
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	otherRoot := t.TempDir()
	writeExplicitRootPluginRepository(t, firstRoot, "plugin-release", "release", "4.0.7")
	writeExplicitRootPluginRepository(t, secondRoot, "plugin-audit", "audit", "0.3.0")
	first := mustValidateRepositoryRoot(t, firstRoot)
	second := mustValidateRepositoryRoot(t, secondRoot)
	withProcessWorkingDirectory(t, otherRoot)

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
	assertProcessWorkingDirectory(t, otherRoot)
}

func TestHandleRequestAtIsolatesPluginIndexOutputAcrossRepositories(t *testing.T) {
	firstRoot := newReleasePluginIndexTempDir(t)
	secondRoot := newReleasePluginIndexTempDir(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get current cwd: %v", err)
	}
	writeExplicitRootPluginRepository(t, firstRoot, "plugin-release", "release", "4.0.7")
	writeExplicitRootPluginRepository(t, secondRoot, "plugin-audit", "audit", "0.3.0")
	first := mustValidateRepositoryRoot(t, firstRoot)
	second := mustValidateRepositoryRoot(t, secondRoot)

	firstResp, err := handleRequestAt(first, plugin.Request{Command: "plugin-index", Flags: map[string]any{"output-file": "dist/plugin-index.json"}}, nil)
	if err != nil {
		t.Fatalf("plugin-index output first: %v", err)
	}
	secondResp, err := handleRequestAt(second, plugin.Request{Command: "plugin-index", Flags: map[string]any{"output-file": "dist/plugin-index.json"}}, nil)
	if err != nil {
		t.Fatalf("plugin-index output second: %v", err)
	}

	firstOutput := readExplicitRootFile(t, filepath.Join(firstRoot, "dist", "plugin-index.json"))
	secondOutput := readExplicitRootFile(t, filepath.Join(secondRoot, "dist", "plugin-index.json"))
	if !strings.Contains(firstOutput, `"name": "release"`) || strings.Contains(firstOutput, `"name": "audit"`) {
		t.Fatalf("first output leaked or missed data: %s", firstOutput)
	}
	if !strings.Contains(secondOutput, `"name": "audit"`) || strings.Contains(secondOutput, `"name": "release"`) {
		t.Fatalf("second output leaked or missed data: %s", secondOutput)
	}
	assertRootIsolationOutputItem(t, firstResp, "dist/plugin-index.json")
	assertRootIsolationOutputItem(t, secondResp, "dist/plugin-index.json")
	assertProcessWorkingDirectory(t, cwd)
}

func newReleasePluginIndexTempDir(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("/private/tmp", "neko-plugin-index-root-*")
	if err != nil {
		t.Fatalf("create plugin-index root fixture: %v", err)
	}
	t.Cleanup(func() {
		if removeErr := os.RemoveAll(root); removeErr != nil {
			t.Errorf("remove plugin-index root fixture %s: %v", root, removeErr)
		}
	})
	return root
}

func TestReleaseContextValidationCommandUsesNestedExplicitRootsWithoutMutation(t *testing.T) {
	firstRoot, firstRequest := writeExplicitRootReleaseContextGitRepository(t, "api", "1.2.3")
	secondRoot, secondRequest := writeExplicitRootReleaseContextGitRepository(t, "web", "2.0.0")
	otherRoot := t.TempDir()
	withProcessWorkingDirectory(t, otherRoot)

	firstNested := filepath.Join(firstRoot, "services", "api")
	secondNested := filepath.Join(secondRoot, "services", "web")
	if err := os.MkdirAll(firstNested, 0o755); err != nil {
		t.Fatalf("mkdir first nested path: %v", err)
	}
	if err := os.MkdirAll(secondNested, 0o755); err != nil {
		t.Fatalf("mkdir second nested path: %v", err)
	}
	firstResolved, err := workspace.ResolveRepositoryRoot(firstNested)
	if err != nil {
		t.Fatalf("resolve first nested root: %v", err)
	}
	secondResolved, err := workspace.ResolveRepositoryRoot(secondNested)
	if err != nil {
		t.Fatalf("resolve second nested root: %v", err)
	}

	firstStatus := explicitRootGitOutput(t, firstRoot, "status", "--porcelain", "--untracked-files=all")
	secondStatus := explicitRootGitOutput(t, secondRoot, "status", "--porcelain", "--untracked-files=all")
	firstResponse, err := handleRequestAt(firstResolved, firstRequest, nil)
	if err != nil {
		t.Fatalf("validate first context: %v", err)
	}
	secondResponse, err := handleRequestAt(secondResolved, secondRequest, nil)
	if err != nil {
		t.Fatalf("validate second context: %v", err)
	}
	if firstResponse.Data["unit"] != "api" || firstResponse.Data["version"] != "1.2.3" || secondResponse.Data["unit"] != "web" || secondResponse.Data["version"] != "2.0.0" {
		t.Fatalf("isolated responses: first=%#v second=%#v", firstResponse.Data, secondResponse.Data)
	}
	if got := explicitRootGitOutput(t, firstRoot, "status", "--porcelain", "--untracked-files=all"); got != firstStatus {
		t.Fatalf("first repository changed: before=%q after=%q", firstStatus, got)
	}
	if got := explicitRootGitOutput(t, secondRoot, "status", "--porcelain", "--untracked-files=all"); got != secondStatus {
		t.Fatalf("second repository changed: before=%q after=%q", secondStatus, got)
	}
	assertProcessWorkingDirectory(t, otherRoot)
}

func writeExplicitRootReleaseContextGitRepository(t *testing.T, unitID, version string) (string, plugin.Request) {
	t.Helper()
	root := t.TempDir()
	writeExplicitRootReleaseRepository(t, root, unitID, version)
	explicitRootGitCommand(t, root, "init")
	explicitRootGitCommand(t, root, "config", "user.email", "context@example.invalid")
	explicitRootGitCommand(t, root, "config", "user.name", "Context Validation")
	explicitRootGitCommand(t, root, "add", ".neko", ".github")
	explicitRootGitCommand(t, root, "commit", "-m", "release context")
	sha := strings.TrimSpace(explicitRootGitOutput(t, root, "rev-parse", "HEAD"))
	tag := unitID + "/v" + version
	explicitRootGitCommand(t, root, "tag", tag, sha)
	return root, plugin.Request{
		Command: "ci-validate-context",
		Flags: map[string]any{
			"unit": unitID, "version": version, "tag": tag, "release-sha": sha,
		},
	}
}

func explicitRootGitCommand(t *testing.T, root string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", arguments, output, err)
	}
}

func explicitRootGitOutput(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", arguments, output, err)
	}
	return string(output)
}

func writeExplicitRootReleaseRepository(t *testing.T, root, unitID, version string) {
	t.Helper()
	workflowPath := fmt.Sprintf(".github/workflows/release-%s.yml", unitID)
	writeExplicitRootFile(t, filepath.Join(root, workflowPath), fmt.Sprintf("name: release %s\n", unitID))
	configJSON := fmt.Sprintf(`{"schemaVersion":2,"units":[{"id":"%s","paths":["**"],"workingDirectory":".","tagPrefix":"%s/v","executor":{"type":"goreleaser","delivery":"github-actions","workflow":"%s"}}]}`, unitID, unitID, workflowPath)
	stateJSON := fmt.Sprintf(`{"schemaVersion":2,"units":{"%s":{"version":"%s"}}}`, unitID, version)
	writeExplicitRootFile(t, releaseconfig.V2ConfigPath(root), configJSON)
	writeExplicitRootFile(t, releaseconfig.V2StatePath(root), stateJSON)
}

func writeExplicitRootPluginRepository(t *testing.T, root, unitID, pluginName, version string) {
	t.Helper()
	manifestPath := filepath.Join("plugins", pluginName, "manifest.json")
	writeExplicitRootFile(t, filepath.Join(root, manifestPath), fmt.Sprintf(`{"name":"%s","version":"%s","description":"%s plugin"}`, pluginName, version, pluginName))
	workflowPath := fmt.Sprintf(".github/workflows/release-%s.yml", unitID)
	writeExplicitRootFile(t, filepath.Join(root, workflowPath), fmt.Sprintf("name: release %s\n", unitID))
	configJSON := fmt.Sprintf(`{"schemaVersion":2,"units":[{"id":"%s","paths":["plugins/%s/**"],"workingDirectory":".","tagPrefix":"%s/v","kind":"plugin","plugin":{"name":"%s","manifest":"%s","assetPrefix":"%s","binaryName":"%s"},"executor":{"type":"goreleaser","delivery":"github-actions","workflow":"%s"}}]}`, unitID, pluginName, unitID, pluginName, manifestPath, unitID, unitID, workflowPath)
	stateJSON := fmt.Sprintf(`{"schemaVersion":2,"units":{"%s":{"version":"%s"}}}`, unitID, version)
	writeExplicitRootFile(t, releaseconfig.V2ConfigPath(root), configJSON)
	writeExplicitRootFile(t, releaseconfig.V2StatePath(root), stateJSON)
}

func writeExplicitRootFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readExplicitRootFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func assertRootIsolationOutputItem(t *testing.T, resp *plugin.Response, want string) {
	t.Helper()
	items := fmt.Sprint(resp.Data["items"])
	if !strings.Contains(items, "Output") || !strings.Contains(items, want) {
		t.Fatalf("response output item = %s, want %s", items, want)
	}
}

func mustValidateRepositoryRoot(t *testing.T, root string) workspace.RepositoryRoot {
	t.Helper()
	resolved, err := workspace.ValidateRepositoryRoot(root)
	if err != nil {
		t.Fatalf("ValidateRepositoryRoot(%s): %v", root, err)
	}
	return resolved
}

func withProcessWorkingDirectory(t *testing.T, root string) {
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

func assertProcessWorkingDirectory(t *testing.T, want string) {
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
