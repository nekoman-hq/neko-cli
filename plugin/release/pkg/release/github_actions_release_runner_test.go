package release

import (
	"context"
	"encoding/json"
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
	want := []string{".neko/release.state.json", ".plugin.release.neko.json", "plugin/release/manifest.json"}
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

//nolint:govet // Test fake fields are ordered by behavior.
type recordingWorkflowDispatchClient struct {
	response GitHubActionsDispatchResponse
	request  *ReleaseDispatchRequest
	calls    int
}

func (client *recordingWorkflowDispatchClient) Dispatch(_ context.Context, _ GitHubRepositoryTarget, request *ReleaseDispatchRequest, _ string) (GitHubActionsDispatchResponse, error) {
	client.calls++
	client.request = request
	return client.response, nil
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
