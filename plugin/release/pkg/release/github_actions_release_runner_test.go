package release

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestGitHubActionsReleaseRunnerCompletesJournaledRelease(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	bare := filepath.Join(t.TempDir(), "origin.git")
	gitCmd(t, root, "init", "--bare", bare)
	gitCmd(t, root, "remote", "set-url", "--push", "origin", bare)
	repository, err := releaseconfig.LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository: %v", err)
	}
	ctx, err := BuildReleaseExecutionContext(repository, repository.Units[0], Patch, false)
	if err != nil {
		t.Fatalf("BuildReleaseExecutionContext: %v", err)
	}
	client := &recordingWorkflowDispatchClient{response: GitHubActionsDispatchResponse{
		State:      DispatchJournalAccepted,
		HTTPStatus: 204,
	}}
	runner := NewGitHubActionsReleaseRunner(
		WithGitHubActionsReleaseTokenResolver(staticDispatchTokenResolver{token: "secret-token"}),
		WithGitHubActionsReleaseDispatchClient(client),
	)
	result, err := runner.Run(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExecutionState != ReleaseExecutionHandoffReady || result.DispatchState != DispatchJournalAccepted {
		t.Fatalf("unexpected result: %#v", result)
	}
	if client.calls != 1 {
		t.Fatalf("expected one dispatch call, got %d", client.calls)
	}
	if client.request.Tag != ctx.Tag || client.request.Inputs["release_sha"] != result.CommitSHA {
		t.Fatalf("unexpected dispatch request: %#v", client.request)
	}
	branch := strings.TrimSpace(gitOutput(t, root, "symbolic-ref", "--short", "HEAD"))
	if got := strings.TrimSpace(gitDirOutput(t, bare, "rev-parse", "refs/heads/"+branch)); got != result.CommitSHA {
		t.Fatalf("bare branch not pushed to release commit: got %s want %s", got, result.CommitSHA)
	}
	if got := strings.TrimSpace(gitDirOutput(t, bare, "rev-parse", "refs/tags/"+ctx.Tag+"^{}")); got != result.CommitSHA {
		t.Fatalf("bare tag not pushed to release commit: got %s want %s", got, result.CommitSHA)
	}
	journal := loadReleaseExecutionJournalForTest(t, result.ExecutionJournalPath)
	if journal.State != ReleaseExecutionHandoffReady || strings.Contains(mustReadString(t, result.ExecutionJournalPath), "secret-token") {
		t.Fatalf("unexpected execution journal: %#v", journal)
	}
	dispatchJournal := loadDispatchJournalForReleaseTest(t, result.DispatchJournalPath)
	if dispatchJournal.State != DispatchJournalAccepted {
		t.Fatalf("unexpected dispatch journal: %#v", dispatchJournal)
	}
}

func TestGitHubActionsReleaseRunnerCommitsPluginReleaseMaterializedFiles(t *testing.T) {
	root := newPluginReleaseMaterializationRepository(t)
	gitCmd(t, root, "init")
	gitCmd(t, root, "config", "user.email", "test@example.com")
	gitCmd(t, root, "config", "user.name", "Test User")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "initial")
	gitCmd(t, root, "remote", "add", "origin", "https://github.com/nekoman/repo.git")
	branch := strings.TrimSpace(gitOutput(t, root, "symbolic-ref", "--short", "HEAD"))
	gitCmd(t, root, "config", "branch."+branch+".remote", "origin")
	gitCmd(t, root, "config", "branch."+branch+".merge", "refs/heads/"+branch)
	bare := filepath.Join(t.TempDir(), "origin.git")
	gitCmd(t, root, "init", "--bare", bare)
	gitCmd(t, root, "remote", "set-url", "--push", "origin", bare)

	repository, err := releaseconfig.LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository: %v", err)
	}
	ctx, err := BuildReleaseExecutionContext(repository, repository.Units[0], Patch, false)
	if err != nil {
		t.Fatalf("BuildReleaseExecutionContext: %v", err)
	}
	client := &recordingWorkflowDispatchClient{response: GitHubActionsDispatchResponse{
		State:      DispatchJournalAccepted,
		HTTPStatus: 204,
	}}
	runner := NewGitHubActionsReleaseRunner(
		WithGitHubActionsReleaseTokenResolver(staticDispatchTokenResolver{token: "secret-token"}),
		WithGitHubActionsReleaseDispatchClient(client),
	)

	result, err := runner.Run(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	changed := sortedNonEmptyLines(gitDirOutput(t, bare, "diff-tree", "--no-commit-id", "--name-only", "-r", result.CommitSHA))
	want := []string{".neko/release.state.json", "plugin/release/manifest.json"}
	if !sameStringSet(changed, want) {
		t.Fatalf("unexpected release commit files: got %#v want %#v", changed, want)
	}
	if client.calls != 1 {
		t.Fatalf("expected one dispatch call, got %d", client.calls)
	}
	if status := strings.TrimSpace(gitOutput(t, root, "status", "--porcelain")); status != "" {
		t.Fatalf("expected clean repository after runner, got %q", status)
	}
}

func TestGitHubActionsReleaseRunnerBlocksUnresolvedExecutionBeforeMutation(t *testing.T) {
	root, _, ctx := newActiveGitHubActionsReleaseRepository(t)
	resolution := prepareUnresolvedExecutionJournal(t, root)
	journalBefore := mustReadString(t, resolution.Path)
	headBefore := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	stateBefore := mustReadString(t, releaseconfig.V2StatePath(root))
	client := &recordingWorkflowDispatchClient{response: GitHubActionsDispatchResponse{State: DispatchJournalAccepted}}
	runner := NewGitHubActionsReleaseRunner(
		WithGitHubActionsReleaseTokenResolver(staticDispatchTokenResolver{token: "test-token"}),
		WithGitHubActionsReleaseDispatchClient(client),
	)

	result, err := runner.Run(context.Background(), ctx)

	if err == nil || result != nil || !strings.Contains(err.Error(), "neko release resume --unit api") {
		t.Fatalf("expected unresolved-journal blocker, result=%#v err=%v", result, err)
	}
	if client.calls != 0 {
		t.Fatalf("unresolved journal reached dispatch: %d calls", client.calls)
	}
	if headAfter := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD")); headAfter != headBefore {
		t.Fatalf("unresolved-journal blocker changed HEAD: before=%s after=%s", headBefore, headAfter)
	}
	if stateAfter := mustReadString(t, releaseconfig.V2StatePath(root)); stateAfter != stateBefore {
		t.Fatalf("unresolved-journal blocker changed release state:\n%s", stateAfter)
	}
	if journalAfter := mustReadString(t, resolution.Path); journalAfter != journalBefore {
		t.Fatalf("unresolved-journal blocker rewrote the existing journal:\n%s", journalAfter)
	}
}

func TestGitHubActionsReleaseRunnerPersistsPendingCommitCreationAfterFailure(t *testing.T) {
	root, _, ctx := newActiveGitHubActionsReleaseRepository(t)
	baseSHA := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	client := &recordingWorkflowDispatchClient{response: GitHubActionsDispatchResponse{State: DispatchJournalAccepted}}
	runner := NewGitHubActionsReleaseRunner(
		WithGitHubActionsReleaseTokenResolver(staticDispatchTokenResolver{token: "test-token"}),
		WithGitHubActionsReleaseDispatchClient(client),
	)
	runner.coordinator = newGitReleaseCoordinatorWithRunner(failingReleaseCommitGitRunner{})

	result, err := runner.Run(context.Background(), ctx)

	if err == nil || result != nil || !strings.Contains(err.Error(), "simulated release commit failure") {
		t.Fatalf("expected injected commit failure, result=%#v err=%v", result, err)
	}
	resolution := soleUnresolvedExecutionJournal(t, root, ctx.Unit.ID)
	journal := resolution.Journal
	if journal.State != ReleaseExecutionReleaseFilesStaged || journal.PendingAction != ReleaseExecutionPendingCreateReleaseCommit {
		t.Fatalf("commit failure lost its recovery boundary: %#v", journal)
	}
	if journal.ReleaseCommitSHA != "" || journal.TagTargetSHA != "" {
		t.Fatalf("commit failure persisted unconfirmed commit metadata: %#v", journal)
	}
	if head := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD")); head != baseSHA {
		t.Fatalf("commit failure changed HEAD: got %s want %s", head, baseSHA)
	}
	if staged := strings.TrimSpace(gitOutput(t, root, "diff", "--cached", "--name-only")); staged != ".neko/release.state.json" {
		t.Fatalf("commit failure did not preserve the staged recovery state: %q", staged)
	}
	if client.calls != 0 {
		t.Fatalf("commit failure reached dispatch: %d calls", client.calls)
	}
}

func TestGitHubActionsReleaseRunnerPersistsCommitBeforeTagCreationFailure(t *testing.T) {
	root, _, ctx := newActiveGitHubActionsReleaseRepository(t)
	client := &recordingWorkflowDispatchClient{response: GitHubActionsDispatchResponse{State: DispatchJournalAccepted}}
	runner := NewGitHubActionsReleaseRunner(
		WithGitHubActionsReleaseTokenResolver(staticDispatchTokenResolver{token: "test-token"}),
		WithGitHubActionsReleaseDispatchClient(client),
	)
	runner.coordinator = newGitReleaseCoordinatorWithRunner(failingUnitTagCreationGitRunner{})

	result, err := runner.Run(context.Background(), ctx)

	if err == nil || result != nil || !strings.Contains(err.Error(), "simulated unit tag creation failure") {
		t.Fatalf("expected injected tag failure, result=%#v err=%v", result, err)
	}
	journal := soleUnresolvedExecutionJournal(t, root, ctx.Unit.ID).Journal
	if journal.State != ReleaseExecutionCommitCreated || journal.PendingAction != ReleaseExecutionPendingCreateUnitTag {
		t.Fatalf("tag failure lost its recovery boundary: %#v", journal)
	}
	head := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	if journal.ReleaseCommitSHA != head || journal.TagTargetSHA != "" {
		t.Fatalf("tag failure did not retain only the confirmed commit SHA: %#v", journal)
	}
	if tags := strings.TrimSpace(gitOutput(t, root, "tag", "--list", ctx.Tag)); tags != "" {
		t.Fatalf("tag failure left an unconfirmed tag: %q", tags)
	}
	if client.calls != 0 {
		t.Fatalf("tag failure reached dispatch: %d calls", client.calls)
	}
}

func TestGitHubActionsReleaseRunnerPersistsPendingCommitPushBeforeFailure(t *testing.T) {
	root, bare, ctx := newActiveGitHubActionsReleaseRepository(t)
	client := &recordingWorkflowDispatchClient{response: GitHubActionsDispatchResponse{State: DispatchJournalAccepted}}
	gitRunner := &recordingGitRunner{failCommitPush: true}
	runner := NewGitHubActionsReleaseRunner(
		WithGitHubActionsReleaseTokenResolver(staticDispatchTokenResolver{token: "test-token"}),
		WithGitHubActionsReleaseDispatchClient(client),
	)
	runner.coordinator = newGitReleaseCoordinatorWithRunner(gitRunner)

	result, err := runner.Run(context.Background(), ctx)

	if err == nil || result != nil || !strings.Contains(err.Error(), "simulated commit push failure") {
		t.Fatalf("expected injected commit-push failure, result=%#v err=%v", result, err)
	}
	journal := soleUnresolvedExecutionJournal(t, root, ctx.Unit.ID).Journal
	if journal.State != ReleaseExecutionDispatchJournalPrepared || journal.PendingAction != ReleaseExecutionPendingPushReleaseCommit {
		t.Fatalf("commit-push failure lost its recovery boundary: %#v", journal)
	}
	if journal.ReleaseCommitSHA == "" || journal.TagTargetSHA != journal.ReleaseCommitSHA || journal.DispatchJournalIdentity == "" {
		t.Fatalf("commit-push failure lost confirmed commit, tag, or dispatch metadata: %#v", journal)
	}
	assertPreparedDispatchJournal(t, root, journal.DispatchJournalIdentity)
	if gitRunner.tagPushes != 0 || client.calls != 0 {
		t.Fatalf("commit-push failure reached a later effect: tag pushes=%d dispatches=%d", gitRunner.tagPushes, client.calls)
	}
	if entries, readErr := os.ReadDir(filepath.Join(bare, "refs", "heads")); readErr != nil || len(entries) != 0 {
		t.Fatalf("commit-push failure changed the remote branch refs: entries=%d err=%v", len(entries), readErr)
	}
}

func TestGitHubActionsReleaseRunnerPersistsCommitPushBeforeTagPushFailure(t *testing.T) {
	root, bare, ctx := newActiveGitHubActionsReleaseRepository(t)
	client := &recordingWorkflowDispatchClient{response: GitHubActionsDispatchResponse{State: DispatchJournalAccepted}}
	gitRunner := &recordingGitRunner{failTagPush: true}
	runner := NewGitHubActionsReleaseRunner(
		WithGitHubActionsReleaseTokenResolver(staticDispatchTokenResolver{token: "test-token"}),
		WithGitHubActionsReleaseDispatchClient(client),
	)
	runner.coordinator = newGitReleaseCoordinatorWithRunner(gitRunner)

	result, err := runner.Run(context.Background(), ctx)

	if err == nil || result != nil || !strings.Contains(err.Error(), "simulated tag push failure") {
		t.Fatalf("expected injected tag-push failure, result=%#v err=%v", result, err)
	}
	journal := soleUnresolvedExecutionJournal(t, root, ctx.Unit.ID).Journal
	if journal.State != ReleaseExecutionCommitPushed || journal.PendingAction != ReleaseExecutionPendingPushUnitTag {
		t.Fatalf("tag-push failure lost its recovery boundary: %#v", journal)
	}
	if journal.CommitPushStatus != "pushed" || journal.TagPushStatus != "" {
		t.Fatalf("tag-push failure recorded false completion: %#v", journal)
	}
	branch := strings.TrimSpace(gitOutput(t, root, "symbolic-ref", "--short", "HEAD"))
	if remoteCommit := strings.TrimSpace(gitDirOutput(t, bare, "rev-parse", "refs/heads/"+branch)); remoteCommit != journal.ReleaseCommitSHA {
		t.Fatalf("commit was not pushed before the tag attempt: got %s want %s", remoteCommit, journal.ReleaseCommitSHA)
	}
	if _, statErr := os.Stat(filepath.Join(bare, "refs", "tags", filepath.FromSlash(ctx.Tag))); !os.IsNotExist(statErr) {
		t.Fatalf("failed tag push changed the remote tag ref: %v", statErr)
	}
	if gitRunner.tagPushes != 1 || client.calls != 0 {
		t.Fatalf("tag-push failure did not stop before dispatch: tag pushes=%d dispatches=%d", gitRunner.tagPushes, client.calls)
	}
}

func TestGitHubActionsReleaseRunnerPersistsRejectedDispatchWithoutFalseHandoff(t *testing.T) {
	_, _, ctx := newActiveGitHubActionsReleaseRepository(t)
	client := &recordingWorkflowDispatchClient{response: GitHubActionsDispatchResponse{
		State:            DispatchJournalRejected,
		HTTPStatus:       422,
		Error:            "request rejected",
		RecoveryGuidance: dispatchJournalRecoveryGuidance(DispatchJournalRejected),
	}}
	runner := NewGitHubActionsReleaseRunner(
		WithGitHubActionsReleaseTokenResolver(staticDispatchTokenResolver{token: "test-token"}),
		WithGitHubActionsReleaseDispatchClient(client),
	)

	result, err := runner.Run(context.Background(), ctx)

	if err == nil || result == nil || result.DispatchState != DispatchJournalRejected {
		t.Fatalf("expected rejected dispatch result, result=%#v err=%v", result, err)
	}
	journal := loadReleaseExecutionJournalForTest(t, result.ExecutionJournalPath)
	if journal.State != ReleaseExecutionTagPushed || journal.PendingAction != ReleaseExecutionPendingNone {
		t.Fatalf("rejected dispatch falsely completed execution: %#v", journal)
	}
	if dispatch := loadDispatchJournalForReleaseTest(t, result.DispatchJournalPath); dispatch.State != DispatchJournalRejected {
		t.Fatalf("rejected dispatch state was not persisted: %#v", dispatch)
	}
}

func TestGitHubActionsReleaseRunnerRedactsTokenFromUncertainDispatchSurfaces(t *testing.T) {
	_, _, ctx := newActiveGitHubActionsReleaseRepository(t)
	client := &recordingWorkflowDispatchClient{err: fmt.Errorf("transport failed with %s", releaseSecretSentinel)}
	runner := NewGitHubActionsReleaseRunner(
		WithGitHubActionsReleaseTokenResolver(staticDispatchTokenResolver{token: releaseSecretSentinel}),
		WithGitHubActionsReleaseDispatchClient(client),
	)

	result, err := runner.Run(context.Background(), ctx)

	if err == nil || result == nil || result.DispatchState != DispatchJournalUnknown {
		t.Fatal("expected uncertain dispatch result")
	}
	if strings.Contains(err.Error(), releaseSecretSentinel) || strings.Contains(fmt.Sprint(result), releaseSecretSentinel) {
		t.Fatal("secret sentinel appeared in the runner result or error")
	}
	for _, path := range []string{result.ExecutionJournalPath, result.DispatchJournalPath} {
		if strings.Contains(mustReadString(t, path), releaseSecretSentinel) {
			t.Fatal("secret sentinel appeared in a release journal")
		}
	}
	journal := loadReleaseExecutionJournalForTest(t, result.ExecutionJournalPath)
	if journal.State != ReleaseExecutionTagPushed || journal.PendingAction != ReleaseExecutionPendingNone {
		t.Fatalf("uncertain dispatch falsely completed execution: %#v", journal)
	}
}

//nolint:govet // Test fake fields are ordered by behavior.
type recordingWorkflowDispatchClient struct {
	response GitHubActionsDispatchResponse
	err      error
	request  *ReleaseDispatchRequest
	calls    int
}

func (client *recordingWorkflowDispatchClient) Dispatch(_ context.Context, _ GitHubRepositoryTarget, request *ReleaseDispatchRequest, _ string) (GitHubActionsDispatchResponse, error) {
	client.calls++
	client.request = request
	return client.response, client.err
}

type failingReleaseCommitGitRunner struct{}

func (failingReleaseCommitGitRunner) Run(repositoryRoot string, args ...string) (string, error) {
	if len(args) > 0 && args[0] == "commit" {
		return "", fmt.Errorf("simulated release commit failure")
	}
	return execGitRunner{}.Run(repositoryRoot, args...)
}

type failingUnitTagCreationGitRunner struct{}

func (failingUnitTagCreationGitRunner) Run(repositoryRoot string, args ...string) (string, error) {
	if len(args) == 3 && args[0] == "tag" {
		return "", fmt.Errorf("simulated unit tag creation failure")
	}
	return execGitRunner{}.Run(repositoryRoot, args...)
}

func newActiveGitHubActionsReleaseRepository(t *testing.T) (string, string, *ReleaseExecutionContext) {
	t.Helper()
	root := newGitHubActionsDispatchRepository(t)
	bare := filepath.Join(t.TempDir(), "origin.git")
	gitCmd(t, root, "init", "--bare", bare)
	gitCmd(t, root, "remote", "set-url", "--push", "origin", bare)
	repository, err := releaseconfig.LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository: %v", err)
	}
	ctx, err := BuildReleaseExecutionContext(repository, repository.Units[0], Patch, false)
	if err != nil {
		t.Fatalf("BuildReleaseExecutionContext: %v", err)
	}
	return root, bare, ctx
}

func soleUnresolvedExecutionJournal(t *testing.T, root, unitID string) ReleaseExecutionJournalResolution {
	t.Helper()
	remote := strings.TrimSpace(gitOutput(t, root, "remote", "get-url", "origin"))
	matches, err := NewReleaseExecutionJournalStore(root).FindUnresolved(remote, unitID)
	if err != nil {
		t.Fatalf("FindUnresolved: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one unresolved execution journal, got %d", len(matches))
	}
	return matches[0]
}

func assertPreparedDispatchJournal(t *testing.T, root, identity string) {
	t.Helper()
	path, err := NewDispatchJournalStore(root).JournalPath(ReleaseDispatchIdentity{SHA256: identity})
	if err != nil {
		t.Fatalf("dispatch JournalPath: %v", err)
	}
	if journal := loadDispatchJournalForReleaseTest(t, path); journal.State != DispatchJournalPrepared {
		t.Fatalf("dispatch journal was not prepared before push: %#v", journal)
	}
}

func loadDispatchJournalForReleaseTest(t *testing.T, path string) *DispatchJournal {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dispatch journal: %v", err)
	}
	var journal DispatchJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		t.Fatalf("decode dispatch journal: %v", err)
	}
	return &journal
}
