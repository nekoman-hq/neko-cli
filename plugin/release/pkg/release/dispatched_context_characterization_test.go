package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestDispatchedReleaseContextUsesCanonicalInputsAndPolicies(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx, result := prepareDispatchRequestContext(t, root, Patch)

	request, err := buildReleaseDispatchRequestForTest(ctx, result)
	if err != nil {
		t.Fatalf("BuildReleaseDispatchRequest: %v", err)
	}
	if got := strings.Join(sortedDispatchInputKeys(request.Inputs), ","); got != "release_sha,tag,unit,version" {
		t.Fatalf("dispatch inputs = %q, want canonical four-input contract", got)
	}

	repository := &releaseconfig.ReleaseRepository{
		SourceFormat: releaseconfig.SourceFormatV2,
		Units: []releaseconfig.ReleaseUnit{
			{ID: "api", Version: request.Version, TagPrefix: "api/v"},
			{ID: "web", Version: "1.0.0", TagPrefix: "web/v"},
		},
	}
	unit, err := releaseconfig.ResolveReleaseUnit(repository, request.UnitID, releaseconfig.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		t.Fatalf("ResolveReleaseUnit: %v", err)
	}
	if unit.ID != request.UnitID || unit.Version != request.Version {
		t.Fatalf("resolved unit = %#v, want dispatched unit and version", unit)
	}
	tagSpec, err := releaseconfig.NewTagSpec(unit.TagPrefix)
	if err != nil {
		t.Fatalf("NewTagSpec: %v", err)
	}
	if got := tagSpec.Format(unit.Version); got != request.Tag {
		t.Fatalf("canonical tag = %q, want dispatched tag %q", got, request.Tag)
	}
	if _, err := releaseconfig.ResolveReleaseUnit(repository, "", releaseconfig.UnitResolutionOptions{RequireExplicitForMulti: true}); err == nil {
		t.Fatal("multi-unit resolution must keep explicit unit selection")
	}
}

func TestDispatchedReleaseContextGitReadsPeelTagsWithoutMutation(t *testing.T) {
	root := newContextCharacterizationRepository(t)
	head := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	gitCmd(t, root, "tag", "api/v1.0.0", head)
	gitCmd(t, root, "tag", "-a", "api/v1.0.1", "-m", "annotated release", head)

	statusBefore := gitOutput(t, root, "status", "--porcelain", "--untracked-files=all")
	tagsBefore := gitOutput(t, root, "show-ref", "--tags")
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	coordinator := NewGitReleaseCoordinator()
	for _, tag := range []string{"api/v1.0.0", "api/v1.0.1"} {
		commit, tagErr := coordinator.tagCommit(root, tag)
		if tagErr != nil {
			t.Fatalf("tagCommit(%q): %v", tag, tagErr)
		}
		if commit != head {
			t.Fatalf("tagCommit(%q) = %q, want %q", tag, commit, head)
		}
	}

	if got := gitOutput(t, root, "status", "--porcelain", "--untracked-files=all"); got != statusBefore {
		t.Fatalf("Git reads changed worktree or index: before=%q after=%q", statusBefore, got)
	}
	if got := gitOutput(t, root, "show-ref", "--tags"); got != tagsBefore {
		t.Fatalf("Git reads changed refs: before=%q after=%q", tagsBefore, got)
	}
	if got, err := os.Getwd(); err != nil || got != workingDirectory {
		t.Fatalf("Git reads changed process cwd: got=%q err=%v want=%q", got, err, workingDirectory)
	}
}

func TestDispatchedReleaseContextGitReadsKeepRepositoryRootsIsolated(t *testing.T) {
	first := newContextCharacterizationRepository(t)
	second := newContextCharacterizationRepository(t)
	firstHead := strings.TrimSpace(gitOutput(t, first, "rev-parse", "HEAD"))
	secondHead := strings.TrimSpace(gitOutput(t, second, "rev-parse", "HEAD"))
	gitCmd(t, first, "tag", "api/v1.0.0", firstHead)
	gitCmd(t, second, "tag", "api/v1.0.0", secondHead)

	coordinator := NewGitReleaseCoordinator()
	if got, err := coordinator.tagCommit(first, "api/v1.0.0"); err != nil || got != firstHead {
		t.Fatalf("first repository tag = %q err=%v, want %q", got, err, firstHead)
	}
	if got, err := coordinator.tagCommit(second, "api/v1.0.0"); err != nil || got != secondHead {
		t.Fatalf("second repository tag = %q err=%v, want %q", got, err, secondHead)
	}
}

func newContextCharacterizationRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitCmd(t, root, "init")
	gitCmd(t, root, "config", "user.email", "release-context@example.invalid")
	gitCmd(t, root, "config", "user.name", "Release Context")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("release context\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitCmd(t, root, "add", "README.md")
	gitCmd(t, root, "commit", "-m", "initial")
	return root
}
