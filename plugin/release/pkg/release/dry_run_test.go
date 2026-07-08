//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package release

//lint:file-ignore SA1019 V1 compatibility release paths intentionally use deprecated V1 APIs during migration

import (
	"os"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestDryRunDoesNotFetchOrWriteConfigAndShowsNextVersion(t *testing.T) {
	withWorkingDirectory(t)

	originalFetch := refreshVersionTags
	originalLatest := latestVersionTag
	t.Cleanup(func() {
		refreshVersionTags = originalFetch
		latestVersionTag = originalLatest
	})

	fetched := false
	refreshVersionTags = func() {
		fetched = true
	}
	latestVersionTag = func() string {
		return "1.2.3"
	}

	configContent := `{
  "project-name": "neko-cli",
  "project-owner": "nekoman-hq",
  "project-type": "backend",
  "release-system": "goreleaser",
  "version": "1.2.3"
}`
	if err := os.WriteFile(config.V1FileName, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resp, err := HandleRelease(plugin.Request{
		Command: "patch",
		Flags: map[string]any{
			"dry-run": true,
		},
	}, Patch)
	if err != nil {
		t.Fatalf("HandleRelease: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %s: %#v", resp.Status, resp.Error)
	}
	if fetched {
		t.Fatal("dry-run called git fetch")
	}

	after, err := os.ReadFile(config.V1FileName)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(after) != configContent {
		t.Fatalf("dry-run rewrote config:\n%s", string(after))
	}

	if !responseContains(resp.Data["items"], "1.2.4") {
		t.Fatalf("expected dry-run response to show next version 1.2.4, got %#v", resp.Data["items"])
	}
}

func TestRevertGitReleaseWithoutMutatingStepIsNoop(t *testing.T) {
	withWorkingDirectory(t)

	var tb ToolBase
	if err := tb.RevertGitRelease(GitReleaseState{}); err != nil {
		t.Fatalf("RevertGitRelease with empty state: %v", err)
	}
}

func TestHandleReleaseV2GitHubActionsRequiresTokenBeforeMutation(t *testing.T) {
	withWorkingDirectory(t)

	if err := os.MkdirAll(".neko", 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	if err := os.WriteFile(".neko/release.state.json", []byte(`{"schemaVersion":2,"units":{"default":{"version":"1.0.0"}}}`), 0644); err != nil {
		t.Fatalf("write v2 state: %v", err)
	}
	if err := os.MkdirAll(".github/workflows", 0755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.WriteFile(".github/workflows/release-default.yml", []byte("name: release\n"), 0644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	if err := os.WriteFile(".neko/release.config.json", []byte(`{"schemaVersion":2,"units":[{"id":"default","paths":["**"],"tagPrefix":"v","executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-default.yml"}}]}`), 0644); err != nil {
		t.Fatalf("write v2 config: %v", err)
	}

	resp, err := HandleRelease(plugin.Request{Command: "patch"}, Patch)
	if err != nil {
		t.Fatalf("HandleRelease: %v", err)
	}
	if resp.Status != "error" || resp.Error.Code != "V2_GITHUB_ACTIONS_RELEASE_FAILED" {
		t.Fatalf("expected V2 github-actions token blocker, got %#v", resp)
	}
	if !strings.Contains(resp.Error.Message, "GITHUB_TOKEN") {
		t.Fatalf("expected token guidance, got %q", resp.Error.Message)
	}
}

func TestHandleReleaseV2DryRunPlansWithoutFetchOrStateWrite(t *testing.T) {
	withWorkingDirectory(t)

	originalFetch := refreshVersionTags
	t.Cleanup(func() {
		refreshVersionTags = originalFetch
	})
	fetched := false
	refreshVersionTags = func() {
		fetched = true
	}

	if err := os.MkdirAll(".neko", 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	if err := os.WriteFile(".goreleaser.yml", []byte("{}"), 0644); err != nil {
		t.Fatalf("write goreleaser config: %v", err)
	}
	t.Setenv("GITHUB_TOKEN", "test-token")
	configContent := `{"schemaVersion":2,"units":[{"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"goreleaser","delivery":"local"}}]}`
	stateContent := `{"schemaVersion":2,"units":{"api":{"version":"0.1.0"}}}`
	if err := os.WriteFile(".neko/release.config.json", []byte(configContent), 0644); err != nil {
		t.Fatalf("write v2 config: %v", err)
	}
	if err := os.WriteFile(".neko/release.state.json", []byte(stateContent), 0644); err != nil {
		t.Fatalf("write v2 state: %v", err)
	}

	resp, err := HandleRelease(plugin.Request{
		Command: "patch",
		Flags: map[string]any{
			"dry-run": true,
			"unit":    "api",
		},
	}, Patch)
	if err != nil {
		t.Fatalf("HandleRelease: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	if fetched {
		t.Fatal("V2 dry-run called git fetch")
	}
	after, err := os.ReadFile(".neko/release.state.json")
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if string(after) != stateContent {
		t.Fatalf("V2 dry-run rewrote state: %s", string(after))
	}
	if !responseContains(resp.Data["items"], "api/v0.1.1") {
		t.Fatalf("expected planned tag api/v0.1.1, got %#v", resp.Data["items"])
	}
}

func TestHandleReleaseV2DryRunAllowsGitHubActionsDelivery(t *testing.T) {
	withWorkingDirectory(t)

	if err := os.MkdirAll(".neko", 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	if err := os.WriteFile(".goreleaser.yml", []byte("{}"), 0644); err != nil {
		t.Fatalf("write goreleaser config: %v", err)
	}
	if err := os.MkdirAll(".github/workflows", 0755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.WriteFile(".github/workflows/release-api.yml", []byte("name: release api\n"), 0644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	t.Setenv("GITHUB_TOKEN", "")
	configContent := `{"schemaVersion":2,"units":[{"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-api.yml"}}]}`
	stateContent := `{"schemaVersion":2,"units":{"api":{"version":"0.1.0"}}}`
	if err := os.WriteFile(".neko/release.config.json", []byte(configContent), 0644); err != nil {
		t.Fatalf("write v2 config: %v", err)
	}
	if err := os.WriteFile(".neko/release.state.json", []byte(stateContent), 0644); err != nil {
		t.Fatalf("write v2 state: %v", err)
	}

	resp, err := HandleRelease(plugin.Request{
		Command: "patch",
		Flags: map[string]any{
			"dry-run": true,
			"unit":    "api",
		},
	}, Patch)
	if err != nil {
		t.Fatalf("HandleRelease: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	if !responseContains(resp.Data["items"], "github-actions") {
		t.Fatalf("expected dry-run response to include github-actions delivery, got %#v", resp.Data["items"])
	}
	if !responseContains(resp.Data["items"], ".github/workflows/release-api.yml") {
		t.Fatalf("expected dry-run response to include workflow, got %#v", resp.Data["items"])
	}
	if !responseContains(resp.Data["items"], "planned after release commit and tag push") {
		t.Fatalf("expected dry-run response to show planned dispatch, got %#v", resp.Data["items"])
	}
	if !responseContains(resp.Data["items"], "api/v0.1.1") {
		t.Fatalf("expected planned tag api/v0.1.1, got %#v", resp.Data["items"])
	}
	if got := responseValueForProperty(t, resp.Data["items"], "Materialized Files"); got != "none" {
		t.Fatalf("expected api dry-run to have no materialized files, got %q", got)
	}
	if got := responseValueForProperty(t, resp.Data["items"], "Known Release Files"); got != ".neko/release.state.json" {
		t.Fatalf("expected api dry-run known files to stay state-only, got %q", got)
	}
}

func TestHandleReleasePluginReleaseDryRunMaterializesManifestsWithoutToken(t *testing.T) {
	root := newPluginReleaseMaterializationRepository(t)
	gitCmd(t, root, "init")
	gitCmd(t, root, "config", "user.email", "test@example.com")
	gitCmd(t, root, "config", "user.name", "Test User")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "initial")
	withWorkingDirectoryRoot(t, root)
	t.Setenv("GITHUB_TOKEN", "")

	stateBefore := mustReadString(t, ".neko/release.state.json")
	manifestBefore := mustReadString(t, pluginReleaseManifestPath)
	statusBefore := strings.TrimSpace(gitOutput(t, root, "status", "--porcelain"))

	resp, err := HandleRelease(plugin.Request{
		Command: "patch",
		Flags: map[string]any{
			"dry-run": true,
			"unit":    "plugin-release",
		},
	}, Patch)
	if err != nil {
		t.Fatalf("HandleRelease: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	if !responseContains(resp.Data["items"], "3.0.1") {
		t.Fatalf("expected dry-run response to show next version 3.0.1, got %#v", resp.Data["items"])
	}
	if !responseContains(resp.Data["items"], "plugin-release/v3.0.1") {
		t.Fatalf("expected dry-run response to show planned tag, got %#v", resp.Data["items"])
	}

	materialized := responseValueForProperty(t, resp.Data["items"], "Materialized Files")
	if materialized != pluginReleaseManifestPath {
		t.Fatalf("expected plugin release materialized files, got %q", materialized)
	}
	knownFiles := responseValueForProperty(t, resp.Data["items"], "Known Release Files")
	for _, path := range []string{".neko/release.state.json", pluginReleaseManifestPath} {
		if !strings.Contains(knownFiles, path) {
			t.Fatalf("expected known release files to include %s, got %q", path, knownFiles)
		}
	}
	if got := mustReadString(t, ".neko/release.state.json"); got != stateBefore {
		t.Fatalf("dry-run rewrote state:\n%s", got)
	}
	if got := mustReadString(t, pluginReleaseManifestPath); got != manifestBefore {
		t.Fatalf("dry-run rewrote plugin manifest:\n%s", got)
	}
	if statusAfter := strings.TrimSpace(gitOutput(t, root, "status", "--porcelain")); statusAfter != statusBefore {
		t.Fatalf("dry-run changed git status: before %q after %q", statusBefore, statusAfter)
	}
}

func TestHandleReleasePluginUIDryRunMaterializesOnlyUIManifestWithoutToken(t *testing.T) {
	root := newPluginUIReleaseMaterializationRepository(t)
	gitCmd(t, root, "init")
	gitCmd(t, root, "config", "user.email", "test@example.com")
	gitCmd(t, root, "config", "user.name", "Test User")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "initial")
	withWorkingDirectoryRoot(t, root)
	t.Setenv("GITHUB_TOKEN", "")

	stateBefore := mustReadString(t, ".neko/release.state.json")
	releaseManifestBefore := mustReadString(t, pluginReleaseManifestPath)
	uiManifestBefore := mustReadString(t, pluginUIManifestPath)

	resp, err := HandleRelease(plugin.Request{
		Command: "patch",
		Flags: map[string]any{
			"dry-run": true,
			"unit":    "plugin-ui",
		},
	}, Patch)
	if err != nil {
		t.Fatalf("HandleRelease: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	if !responseContains(resp.Data["items"], "1.0.1") {
		t.Fatalf("expected dry-run response to show next version 1.0.1, got %#v", resp.Data["items"])
	}
	if !responseContains(resp.Data["items"], "plugin-ui/v1.0.1") {
		t.Fatalf("expected dry-run response to show planned tag, got %#v", resp.Data["items"])
	}
	if materialized := responseValueForProperty(t, resp.Data["items"], "Materialized Files"); materialized != pluginUIManifestPath {
		t.Fatalf("expected plugin-ui materialized file, got %q", materialized)
	}
	knownFiles := responseValueForProperty(t, resp.Data["items"], "Known Release Files")
	for _, path := range []string{".neko/release.state.json", pluginUIManifestPath} {
		if !strings.Contains(knownFiles, path) {
			t.Fatalf("expected known release files to include %s, got %q", path, knownFiles)
		}
	}
	if strings.Contains(knownFiles, pluginReleaseManifestPath) {
		t.Fatalf("plugin-ui dry-run must not include release manifest, got %q", knownFiles)
	}
	if got := mustReadString(t, ".neko/release.state.json"); got != stateBefore {
		t.Fatalf("dry-run rewrote state:\n%s", got)
	}
	if got := mustReadString(t, pluginReleaseManifestPath); got != releaseManifestBefore {
		t.Fatalf("dry-run rewrote release plugin manifest:\n%s", got)
	}
	if got := mustReadString(t, pluginUIManifestPath); got != uiManifestBefore {
		t.Fatalf("dry-run rewrote ui plugin manifest:\n%s", got)
	}
}

func responseContains(items any, expected string) bool {
	rows, ok := items.([]map[string]any)
	if !ok {
		return false
	}
	for _, row := range rows {
		for _, value := range row {
			if strings.Contains(valueAsString(value), expected) {
				return true
			}
		}
	}
	return false
}

func responseValueForProperty(t *testing.T, items any, property string) string {
	t.Helper()
	rows, ok := items.([]map[string]any)
	if !ok {
		t.Fatalf("unexpected response items: %#v", items)
	}
	for _, row := range rows {
		if row["property"] == property {
			return valueAsString(row["value"])
		}
	}
	t.Fatalf("response property %q not found in %#v", property, items)
	return ""
}

func valueAsString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
