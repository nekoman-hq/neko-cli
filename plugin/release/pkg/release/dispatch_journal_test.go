package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDispatchJournalStoreCreatesPreparedJournalUnderGitCommonDir(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx, result := prepareDispatchRequestContext(t, root, Patch)
	request := mustBuildDispatchRequest(t, ctx, result)
	store := NewDispatchJournalStore(root)

	resolution, err := store.Prepare(request)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !resolution.Created || resolution.Journal.State != DispatchJournalPrepared {
		t.Fatalf("unexpected resolution: %#v", resolution)
	}
	commonDir := strings.TrimSpace(gitOutput(t, root, "rev-parse", "--git-common-dir"))
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(root, commonDir)
	}
	if !strings.HasPrefix(resolution.Path, filepath.Join(commonDir, "neko", "release", "dispatches")) {
		t.Fatalf("journal path %s is not below git common dir %s", resolution.Path, commonDir)
	}
	if strings.Contains(resolution.Path, filepath.Join(root, ".neko")) {
		t.Fatalf("journal path must not live in worktree .neko: %s", resolution.Path)
	}
	if filepath.Base(resolution.Path) != request.Identity.SHA256+".json" {
		t.Fatalf("journal filename must be identity hash, got %s", filepath.Base(resolution.Path))
	}
	data := mustReadString(t, resolution.Path)
	for _, forbidden := range []string{"GITHUB_TOKEN", "Authorization", "Bearer", "token"} {
		if strings.Contains(data, forbidden) {
			t.Fatalf("journal contains forbidden token-like value %q:\n%s", forbidden, data)
		}
	}
}

func TestDispatchJournalStoreReusesPreparedJournalAndRejectsUnexpectedContent(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx, result := prepareDispatchRequestContext(t, root, Patch)
	request := mustBuildDispatchRequest(t, ctx, result)
	store := NewDispatchJournalStore(root)

	first, err := store.Prepare(request)
	if err != nil {
		t.Fatalf("Prepare first: %v", err)
	}
	second, err := store.Prepare(request)
	if err != nil {
		t.Fatalf("Prepare second: %v", err)
	}
	if !second.Reused || second.Created {
		t.Fatalf("expected idempotent reuse, got %#v", second)
	}

	first.Journal.Version = "9.9.9"
	writeDispatchJournalForTest(t, first.Path, first.Journal)
	if _, err := store.Prepare(request); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("expected journal conflict, got %v", err)
	}
}

func TestDispatchJournalStoreFailsOnCorruptedJournal(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx, result := prepareDispatchRequestContext(t, root, Patch)
	request := mustBuildDispatchRequest(t, ctx, result)
	store := NewDispatchJournalStore(root)
	path, err := store.JournalPath(request.Identity)
	if err != nil {
		t.Fatalf("JournalPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("mkdir journal dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0600); err != nil {
		t.Fatalf("write corrupt journal: %v", err)
	}
	if _, err := store.Prepare(request); err == nil || !strings.Contains(err.Error(), "parse dispatch journal") {
		t.Fatalf("expected parse failure, got %v", err)
	}
}

func TestDispatchJournalStatesBlockOverwriteWithGuidance(t *testing.T) {
	states := []DispatchJournalState{
		DispatchJournalRequestStarted,
		DispatchJournalAccepted,
		DispatchJournalRejected,
		DispatchJournalUnknown,
	}
	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			root := newGitHubActionsDispatchRepository(t)
			ctx, result := prepareDispatchRequestContext(t, root, Patch)
			request := mustBuildDispatchRequest(t, ctx, result)
			store := NewDispatchJournalStore(root)
			resolution, err := store.Prepare(request)
			if err != nil {
				t.Fatalf("Prepare: %v", err)
			}
			resolution.Journal.State = state
			resolution.Journal.RecoveryGuidance = dispatchJournalRecoveryGuidance(state)
			writeDispatchJournalForTest(t, resolution.Path, resolution.Journal)

			next, err := store.Prepare(request)
			if err != nil {
				t.Fatalf("Prepare existing %s: %v", state, err)
			}
			if !next.Blocked || next.RecoveryGuidance == "" {
				t.Fatalf("expected blocked guidance for %s, got %#v", state, next)
			}
		})
	}
}

func TestDispatchJournalStateTransitions(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx, result := prepareDispatchRequestContext(t, root, Patch)
	request := mustBuildDispatchRequest(t, ctx, result)
	journal, err := NewPreparedDispatchJournal(request, time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewPreparedDispatchJournal: %v", err)
	}
	if journal.State != DispatchJournalPrepared {
		t.Fatalf("expected prepared state, got %s", journal.State)
	}
	if err := journal.Transition(DispatchJournalAccepted, time.Now(), ""); err == nil {
		t.Fatal("prepared -> accepted must be invalid in this milestone")
	}
	if err := journal.Transition(DispatchJournalRequestStarted, time.Now(), ""); err != nil {
		t.Fatalf("prepared -> request-started: %v", err)
	}
	if err := journal.Transition(DispatchJournalPrepared, time.Now(), ""); err == nil {
		t.Fatal("request-started -> prepared must be invalid")
	}
	if err := journal.Transition(DispatchJournalUnknown, time.Now(), "timeout"); err != nil {
		t.Fatalf("request-started -> unknown: %v", err)
	}
	if err := journal.Transition(DispatchJournalPrepared, time.Now(), ""); err == nil {
		t.Fatal("unknown -> prepared must be invalid")
	}
}

func TestDispatchJournalStoreSupportsGitCommonDirFromWorktree(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx, result := prepareDispatchRequestContext(t, root, Patch)
	request := mustBuildDispatchRequest(t, ctx, result)
	store := NewDispatchJournalStore(root)
	path, err := store.JournalPath(request.Identity)
	if err != nil {
		t.Fatalf("JournalPath: %v", err)
	}
	if strings.Contains(path, filepath.Join(root, ".neko")) {
		t.Fatalf("journal should not be under worktree .neko: %s", path)
	}
}

func mustBuildDispatchRequest(t *testing.T, ctx *ReleaseExecutionContext, result *GitReleaseResult) *ReleaseDispatchRequest {
	t.Helper()
	request, err := BuildReleaseDispatchRequest(ctx, result)
	if err != nil {
		t.Fatalf("BuildReleaseDispatchRequest: %v", err)
	}
	return request
}

func writeDispatchJournalForTest(t *testing.T, path string, journal *DispatchJournal) {
	t.Helper()
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		t.Fatalf("marshal journal: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		t.Fatalf("write journal: %v", err)
	}
}
