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

func TestHandleReleaseRejectsV2GitHubActionsExecution(t *testing.T) {
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
	if resp.Status != "error" || resp.Error.Code != "V2_PUBLICATION_ADAPTERS_UNAVAILABLE" {
		t.Fatalf("expected V2 publication adapter blocker, got %#v", resp)
	}
	if !strings.Contains(resp.Error.Message, "V2 Git release coordination is prepared") {
		t.Fatalf("expected M5A blocker message, got %q", resp.Error.Message)
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
	t.Setenv("GITHUB_TOKEN", "test-token")
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
	if !responseContains(resp.Data["items"], "not implemented") {
		t.Fatalf("expected dry-run response to show dispatch not implemented, got %#v", resp.Data["items"])
	}
	if !responseContains(resp.Data["items"], "api/v0.1.1") {
		t.Fatalf("expected planned tag api/v0.1.1, got %#v", resp.Data["items"])
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

func valueAsString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
