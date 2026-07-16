package release

import (
	"context"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestCompletedExecutionAndDispatchJournalsDoNotBlockNextRelease(t *testing.T) {
	root, _, ctx := newActiveGitHubActionsReleaseRepository(t)
	client := &recordingWorkflowDispatchClient{response: GitHubActionsDispatchResponse{
		State:      DispatchJournalAccepted,
		HTTPStatus: 204,
	}}
	runner := NewGitHubActionsReleaseRunner(
		WithGitHubActionsReleaseTokenResolver(staticDispatchTokenResolver{token: "test-token"}),
		WithGitHubActionsReleaseDispatchClient(client),
	)

	first, err := runner.Run(context.Background(), ctx)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if first.ExecutionState != ReleaseExecutionHandoffReady || first.DispatchState != DispatchJournalAccepted {
		t.Fatalf("first release was not completed handoff: %#v", first)
	}
	firstExecutionBytes := mustReadString(t, first.ExecutionJournalPath)
	firstDispatchBytes := mustReadString(t, first.DispatchJournalPath)
	remote := strings.TrimSpace(gitOutput(t, root, "remote", "get-url", "origin"))
	unresolved, err := NewReleaseExecutionJournalStore(root).FindUnresolved(remote, ctx.Unit.ID)
	if err != nil {
		t.Fatalf("FindUnresolved after first release: %v", err)
	}
	if len(unresolved) != 0 {
		t.Fatalf("handoff-ready journal was treated as unresolved: %#v", unresolved)
	}

	repository, err := releaseconfig.LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository after first release: %v", err)
	}
	nextCtx, err := BuildReleaseExecutionContext(repository, repository.Units[0], Patch, false)
	if err != nil {
		t.Fatalf("BuildReleaseExecutionContext after first release: %v", err)
	}
	if nextCtx.Tag == ctx.Tag {
		t.Fatalf("second release reused first tag %q", ctx.Tag)
	}

	second, err := runner.Run(context.Background(), nextCtx)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if second.ExecutionState != ReleaseExecutionHandoffReady || second.DispatchState != DispatchJournalAccepted {
		t.Fatalf("second release was not completed handoff: %#v", second)
	}
	if client.calls != 2 {
		t.Fatalf("dispatch calls = %d, want 2", client.calls)
	}
	if got := mustReadString(t, first.ExecutionJournalPath); got != firstExecutionBytes {
		t.Fatalf("second release rewrote completed execution journal:\n%s", got)
	}
	if got := mustReadString(t, first.DispatchJournalPath); got != firstDispatchBytes {
		t.Fatalf("second release rewrote completed dispatch journal:\n%s", got)
	}
}
