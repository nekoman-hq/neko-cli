package release

import (
	"encoding/json"
	"errors"
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

func TestDispatchJournalStorePersistsExactCanonicalBytesAndMode(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	timestamp := time.Date(2026, 7, 14, 10, 11, 12, 0, time.UTC)
	store := NewDispatchJournalStore(root)
	store.clock = fixedReleaseClock{timestamp: timestamp}
	request := &ReleaseDispatchRequest{
		RepositoryRemoteName: "origin",
		UnitID:               "api",
		Version:              "1.2.4",
		Tag:                  "api/v1.2.4",
		ReleaseCommitSHA:     strings.Repeat("c", 40),
		WorkflowPath:         ".github/workflows/release-api.yml",
		WorkflowFileName:     "release-api.yml",
		Delivery:             "github-actions",
		Executor:             "goreleaser",
		Inputs: map[string]string{
			"unit":        "api",
			"version":     "1.2.4",
			"tag":         "api/v1.2.4",
			"release_sha": strings.Repeat("c", 40),
		},
		Identity: ReleaseDispatchIdentity{
			RepositoryRemoteName: "origin",
			RepositoryRemote:     "https://github.com/nekoman/repo.git",
			UnitID:               "api",
			Version:              "1.2.4",
			Tag:                  "api/v1.2.4",
			ReleaseCommitSHA:     strings.Repeat("c", 40),
			WorkflowPath:         ".github/workflows/release-api.yml",
			Executor:             "goreleaser",
			Delivery:             "github-actions",
			SHA256:               strings.Repeat("d", 64),
		},
	}

	resolution, err := store.Prepare(request)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	want := `{
  "schemaVersion": 1,
  "identity": {
    "repositoryRemoteName": "origin",
    "repositoryRemote": "https://github.com/nekoman/repo.git",
    "unit": "api",
    "version": "1.2.4",
    "tag": "api/v1.2.4",
    "releaseCommitSHA": "cccccccccccccccccccccccccccccccccccccccc",
    "workflowPath": ".github/workflows/release-api.yml",
    "executor": "goreleaser",
    "delivery": "github-actions",
    "sha256": "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
  },
  "repositoryRemoteName": "origin",
  "repositoryRemote": "https://github.com/nekoman/repo.git",
  "unit": "api",
  "version": "1.2.4",
  "tag": "api/v1.2.4",
  "releaseCommitSHA": "cccccccccccccccccccccccccccccccccccccccc",
  "workflowPath": ".github/workflows/release-api.yml",
  "workflowFileName": "release-api.yml",
  "executor": "goreleaser",
  "delivery": "github-actions",
  "inputs": {
    "release_sha": "cccccccccccccccccccccccccccccccccccccccc",
    "tag": "api/v1.2.4",
    "unit": "api",
    "version": "1.2.4"
  },
  "state": "prepared",
  "createdAt": "2026-07-14T10:11:12Z",
  "updatedAt": "2026-07-14T10:11:12Z",
  "dispatchMetadata": {
    "requestStartedAt": "0001-01-01T00:00:00Z",
    "requestFinishedAt": "0001-01-01T00:00:00Z"
  },
  "recoveryGuidance": "Dispatch request is prepared locally. No HTTP request has been attempted."
}
`
	if got := mustReadString(t, resolution.Path); got != want {
		t.Fatalf("dispatch journal bytes changed:\n%s", got)
	}
	info, err := os.Stat(resolution.Path)
	if err != nil {
		t.Fatalf("stat dispatch journal: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("dispatch journal mode = %04o, want 0600", got)
	}
	directory, err := os.Stat(filepath.Dir(resolution.Path))
	if err != nil {
		t.Fatalf("stat dispatch journal directory: %v", err)
	}
	if got := directory.Mode().Perm(); got != 0700 {
		t.Fatalf("dispatch journal directory mode = %04o, want 0700", got)
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

func TestDispatchJournalStoreReportsPrivatePersistenceFailure(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx, result := prepareDispatchRequestContext(t, root, Patch)
	request := mustBuildDispatchRequest(t, ctx, result)
	store := NewDispatchJournalStore(root)
	store.files.replaceFile = func(string, []byte, os.FileMode) error {
		return errors.New("injected atomic dispatch journal write failure")
	}

	_, err := store.Prepare(request)
	if err == nil || !strings.Contains(err.Error(), "write dispatch journal") || !strings.Contains(err.Error(), "injected atomic dispatch journal write failure") {
		t.Fatalf("persistence failure contract changed: %v", err)
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
		t.Fatal("prepared -> accepted must be invalid without request-started evidence")
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
	worktreeRoot := filepath.Join(t.TempDir(), "linked")
	gitCmd(t, root, "worktree", "add", worktreeRoot)
	store := NewDispatchJournalStore(worktreeRoot)
	path, err := store.JournalPath(request.Identity)
	if err != nil {
		t.Fatalf("JournalPath: %v", err)
	}
	if strings.Contains(path, filepath.Join(worktreeRoot, ".neko")) {
		t.Fatalf("journal should not be under worktree .neko: %s", path)
	}
	commonDir := strings.TrimSpace(gitOutput(t, worktreeRoot, "rev-parse", "--git-common-dir"))
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(worktreeRoot, commonDir)
	}
	if !strings.HasPrefix(path, filepath.Join(commonDir, "neko", "release", "dispatches")) {
		t.Fatalf("journal path %s not under linked worktree common dir %s", path, commonDir)
	}
}

func mustBuildDispatchRequest(t *testing.T, ctx *ReleaseExecutionContext, result *GitReleaseResult) *ReleaseDispatchRequest {
	t.Helper()
	request, err := buildReleaseDispatchRequestForTest(ctx, result)
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
