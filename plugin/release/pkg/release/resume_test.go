package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestHandleResumeDryRunReadsExistingJournalWithoutTokenOrMutation(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx := newExecutionJournalContext(t, root)
	journal := newPreparedExecutionJournal(t, ctx)
	store := NewReleaseExecutionJournalStore(root)
	resolution, err := store.Prepare(journal)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	beforeJournal := mustReadString(t, resolution.Path)
	beforeStatus := strings.TrimSpace(gitOutput(t, root, "status", "--porcelain", "--untracked-files=all"))
	t.Setenv("GITHUB_TOKEN", "")
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(plugin.Request{
		Command: "resume",
		Flags: map[string]any{
			"unit":    "api",
			"dry-run": true,
		},
	})
	if err != nil {
		t.Fatalf("HandleResume: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp)
	}
	if !responseContains(resp.Data["items"], "not-started") {
		t.Fatalf("expected recovery assessment, got %#v", resp.Data["items"])
	}
	if after := mustReadString(t, resolution.Path); after != beforeJournal {
		t.Fatalf("resume dry-run rewrote journal:\n%s", after)
	}
	afterStatus := strings.TrimSpace(gitOutput(t, root, "status", "--porcelain", "--untracked-files=all"))
	if afterStatus != beforeStatus {
		t.Fatalf("resume dry-run mutated worktree: before=%q after=%q", beforeStatus, afterStatus)
	}
}

func TestHandleResumeRequiresExistingJournalAndDoesNotCalculateFreshVersion(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	statePath := filepath.Join(root, ".neko", "release.state.json")
	beforeState := mustReadString(t, statePath)
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(plugin.Request{
		Command: "resume",
		Flags: map[string]any{
			"unit":    "api",
			"dry-run": true,
		},
	})
	if err != nil {
		t.Fatalf("HandleResume: %v", err)
	}
	if resp.Status != "error" || resp.Error.Code != "NO_RESUMABLE_JOURNAL" {
		t.Fatalf("expected no journal error, got %#v", resp)
	}
	if after := mustReadString(t, statePath); after != beforeState {
		t.Fatalf("resume without journal changed state:\n%s", after)
	}
}

func TestHandleResumeRequiresExactlyOneUnresolvedJournal(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx := newExecutionJournalContext(t, root)
	store := NewReleaseExecutionJournalStore(root)
	if _, err := store.Prepare(newPreparedExecutionJournal(t, ctx)); err != nil {
		t.Fatalf("Prepare first: %v", err)
	}
	otherCtx := *ctx
	otherCtx.NextVersion = "0.2.2"
	otherCtx.Tag = "api/v0.2.2"
	otherFiles := newExecutionJournalKnownFiles(t, &otherCtx)
	baseSHA := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	remote := strings.TrimSpace(gitOutput(t, root, "remote", "get-url", "origin"))
	if _, err := store.Prepare(mustBuildExecutionJournal(t, &otherCtx, otherFiles, baseSHA, remote)); err != nil {
		t.Fatalf("Prepare second: %v", err)
	}
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(plugin.Request{
		Command: "resume",
		Flags: map[string]any{
			"unit":    "api",
			"dry-run": true,
		},
	})
	if err != nil {
		t.Fatalf("HandleResume: %v", err)
	}
	if resp.Status != "error" || resp.Error.Code != "MULTIPLE_RESUMABLE_JOURNALS" {
		t.Fatalf("expected multiple journal error, got %#v", resp)
	}
}

func TestHandleResumeConservativelyBlocksPreCommitContinuation(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx := newExecutionJournalContext(t, root)
	store := NewReleaseExecutionJournalStore(root)
	if _, err := store.Prepare(newPreparedExecutionJournal(t, ctx)); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	t.Setenv("GITHUB_TOKEN", "secret-token")
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(plugin.Request{
		Command: "resume",
		Flags: map[string]any{
			"unit": "api",
		},
	})
	if err != nil {
		t.Fatalf("HandleResume: %v", err)
	}
	if resp.Status != "error" || resp.Error.Code != "RESUME_FAILED" {
		t.Fatalf("expected conservative resume block, got status=%s error=%#v", resp.Status, resp.Error)
	}
	if !strings.Contains(resp.Error.Message, "resume before release commit is not yet safe") {
		t.Fatalf("expected pre-commit guidance, got %q", resp.Error.Message)
	}
}

func withWorkingDirectoryRoot(t *testing.T, root string) {
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
