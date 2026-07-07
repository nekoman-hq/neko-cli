package release

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
	if err := os.WriteFile(config.FileName, []byte(configContent), 0644); err != nil {
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

	after, err := os.ReadFile(config.FileName)
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
