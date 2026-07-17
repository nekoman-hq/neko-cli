package release

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestReleaseDryRunCharacterizesPlanningFactsForInspection(t *testing.T) {
	root := newPluginReleaseMaterializationRepository(t)
	gitCmd(t, root, "init")
	gitCmd(t, root, "config", "user.email", "test@example.com")
	gitCmd(t, root, "config", "user.name", "Test User")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "initial")
	withWorkingDirectoryRoot(t, root)
	t.Setenv("GITHUB_TOKEN", releaseSecretSentinel)

	stateBefore := mustReadString(t, releaseconfig.V2StatePath(root))
	manifestBefore := mustReadString(t, filepath.Join(root, pluginReleaseManifestPath))
	headBefore := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	tagsBefore := strings.TrimSpace(gitOutput(t, root, "tag", "--list"))

	resp, err := HandleRelease(plugin.Request{
		Command: "patch",
		Flags: map[string]any{
			"dry-run": true,
			"unit":    "plugin-release",
		},
	}, Patch)

	if err != nil || resp.Status != "success" {
		t.Fatalf("dry-run failed: response=%#v err=%v", resp, err)
	}
	expected := map[string]string{
		"Release Type":           "patch",
		"Unit":                   "plugin-release",
		"Current Version":        "3.0.0",
		"New Version":            "3.0.1",
		"Tag":                    "plugin-release/v3.0.1",
		"Executor":               "goreleaser",
		"Delivery":               "github-actions",
		"Workflow":               ".github/workflows/release-plugin-release.yml",
		"Materialized Files":     pluginReleaseManifestPath,
		"Known Release Files":    ".neko/release.state.json, plugin/release/manifest.json",
		"Executor Start":         "no",
		"Dry Run":                "yes",
		"Dispatch Ref":           "plugin-release/v3.0.1",
		"Dispatch Status":        "planned after release commit and tag push",
	}
	for property, want := range expected {
		if got := responseValueForProperty(t, resp.Data["items"], property); got != want {
			t.Fatalf("%s = %q, want %q", property, got, want)
		}
	}
	if got := responseProperties(t, resp.Data["items"]); !slices.Contains(got, "Dispatch Inputs") {
		t.Fatalf("dry-run response omitted dispatch planning facts: %#v", got)
	}
	if got := mustReadString(t, releaseconfig.V2StatePath(root)); got != stateBefore {
		t.Fatalf("dry-run rewrote state:\n%s", got)
	}
	if got := mustReadString(t, filepath.Join(root, pluginReleaseManifestPath)); got != manifestBefore {
		t.Fatalf("dry-run rewrote manifest:\n%s", got)
	}
	if headAfter := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD")); headAfter != headBefore {
		t.Fatalf("dry-run created a commit: before=%s after=%s", headBefore, headAfter)
	}
	if tagsAfter := strings.TrimSpace(gitOutput(t, root, "tag", "--list")); tagsAfter != tagsBefore {
		t.Fatalf("dry-run changed tags: before=%q after=%q", tagsBefore, tagsAfter)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".git", "neko")); !os.IsNotExist(statErr) {
		t.Fatalf("dry-run created release journals: %v", statErr)
	}
	assertSecretAbsentFromResponse(t, resp)
}

func TestReleaseDryRunCharacterizesExplicitRootPlanning(t *testing.T) {
	rootPath := newGitHubActionsDispatchRepository(t)
	otherRoot := t.TempDir()
	root, err := workspace.ValidateRepositoryRoot(rootPath)
	if err != nil {
		t.Fatalf("ValidateRepositoryRoot: %v", err)
	}
	withWorkingDirectoryRoot(t, otherRoot)
	t.Setenv("GITHUB_TOKEN", releaseSecretSentinel)

	resp, err := HandleReleaseAt(root, plugin.Request{
		Command: "minor",
		Flags: map[string]any{
			"dry-run": true,
			"unit":    "api",
		},
	}, Minor)

	if err != nil || resp.Status != "success" {
		t.Fatalf("explicit-root dry-run failed: response=%#v err=%v", resp, err)
	}
	if got := responseValueForProperty(t, resp.Data["items"], "Current Version"); got != "0.2.0" {
		t.Fatalf("current version = %q, want 0.2.0", got)
	}
	if got := responseValueForProperty(t, resp.Data["items"], "New Version"); got != "0.3.0" {
		t.Fatalf("next version = %q, want 0.3.0", got)
	}
	if got := responseValueForProperty(t, resp.Data["items"], "Tag"); got != "api/v0.3.0" {
		t.Fatalf("tag = %q, want api/v0.3.0", got)
	}
	if _, err := os.Stat(releaseconfig.V2ConfigPath(otherRoot)); !os.IsNotExist(err) {
		t.Fatalf("explicit-root dry-run touched process cwd; stat err=%v", err)
	}
	assertSecretAbsentFromResponse(t, resp)
}
