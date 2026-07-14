package release

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestGitHubActionsDispatcherCreatesJournalAndPersistsAccepted(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx, gitResult := prepareDispatchRequestContext(t, root, Patch)
	request := mustBuildDispatchRequest(t, ctx, gitResult)
	client := &fakeWorkflowDispatchClient{response: GitHubActionsDispatchResponse{
		State:         DispatchJournalAccepted,
		HTTPStatus:    204,
		WorkflowRunID: "123",
		RunURL:        "https://api.github.com/runs/123",
		HTMLURL:       "https://github.com/nekoman/repo/actions/runs/123",
	}}
	dispatcher, err := NewGitHubActionsDispatcher(root,
		WithGitHubActionsDispatcherClient(client),
		WithGitHubActionsDispatcherTokenResolver(staticDispatchTokenResolver{token: "secret-token"}),
	)
	if err != nil {
		t.Fatalf("NewGitHubActionsDispatcher: %v", err)
	}

	result, err := dispatcher.Dispatch(context.Background(), request)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !result.Attempted || !result.Accepted || result.State != DispatchJournalAccepted || result.WorkflowRunID != "123" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if client.calls != 1 {
		t.Fatalf("expected one client call, got %d", client.calls)
	}
	journal := loadDispatchJournalForTest(t, result.JournalPath)
	if journal.State != DispatchJournalAccepted || journal.DispatchMetadata.RunID != "123" {
		t.Fatalf("unexpected journal: %#v", journal)
	}
	data := mustReadString(t, result.JournalPath)
	if strings.Contains(data, "secret-token") || strings.Contains(data, "Bearer") {
		t.Fatalf("token leaked into journal:\n%s", data)
	}
}

func TestGitHubActionsDispatcherPersistsRequestStartedBeforeOutboundCall(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx, gitResult := prepareDispatchRequestContext(t, root, Patch)
	request := mustBuildDispatchRequest(t, ctx, gitResult)
	store := NewDispatchJournalStore(root)
	client := &journalInspectingDispatchClient{store: store}
	dispatcher, err := NewGitHubActionsDispatcher(root,
		WithGitHubActionsDispatcherStore(store),
		WithGitHubActionsDispatcherClient(client),
		WithGitHubActionsDispatcherTokenResolver(staticDispatchTokenResolver{token: "test-token"}),
	)
	if err != nil {
		t.Fatalf("NewGitHubActionsDispatcher: %v", err)
	}

	result, err := dispatcher.Dispatch(context.Background(), request)

	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if client.observedState != DispatchJournalRequestStarted {
		t.Fatalf("outbound call observed journal state %s, expected %s", client.observedState, DispatchJournalRequestStarted)
	}
	if result.State != DispatchJournalAccepted {
		t.Fatalf("unexpected final dispatch result: %#v", result)
	}
}

func TestGitHubActionsDispatcherMissingTokenDoesNotStartRequest(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx, gitResult := prepareDispatchRequestContext(t, root, Patch)
	request := mustBuildDispatchRequest(t, ctx, gitResult)
	client := &fakeWorkflowDispatchClient{response: GitHubActionsDispatchResponse{State: DispatchJournalAccepted}}
	dispatcher, err := NewGitHubActionsDispatcher(root,
		WithGitHubActionsDispatcherClient(client),
		WithGitHubActionsDispatcherTokenResolver(staticDispatchTokenResolver{err: missingGitHubActionsDispatchTokenError()}),
	)
	if err != nil {
		t.Fatalf("NewGitHubActionsDispatcher: %v", err)
	}

	result, err := dispatcher.Dispatch(context.Background(), request)
	if err == nil {
		t.Fatal("expected missing token error")
	}
	if result == nil || result.Attempted {
		t.Fatalf("missing token should be a pre-request failure, got %#v", result)
	}
	if client.calls != 0 {
		t.Fatalf("client must not be called, got %d calls", client.calls)
	}
	journal := loadDispatchJournalForTest(t, result.JournalPath)
	if journal.State != DispatchJournalPrepared {
		t.Fatalf("missing token must leave prepared journal unchanged, got %s", journal.State)
	}
}

func TestGitHubActionsDispatcherExistingTerminalJournalsPreventRedispatch(t *testing.T) {
	states := []DispatchJournalState{
		DispatchJournalRequestStarted,
		DispatchJournalAccepted,
		DispatchJournalRejected,
		DispatchJournalUnknown,
	}
	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			root := newGitHubActionsDispatchRepository(t)
			ctx, gitResult := prepareDispatchRequestContext(t, root, Patch)
			request := mustBuildDispatchRequest(t, ctx, gitResult)
			store := NewDispatchJournalStore(root)
			prepared, err := store.Prepare(request)
			if err != nil {
				t.Fatalf("Prepare: %v", err)
			}
			prepared.Journal.State = state
			prepared.Journal.RecoveryGuidance = dispatchJournalRecoveryGuidance(state)
			writeDispatchJournalForTest(t, prepared.Path, prepared.Journal)
			client := &fakeWorkflowDispatchClient{response: GitHubActionsDispatchResponse{State: DispatchJournalAccepted}}
			dispatcher, err := NewGitHubActionsDispatcher(root,
				WithGitHubActionsDispatcherStore(store),
				WithGitHubActionsDispatcherClient(client),
				WithGitHubActionsDispatcherTokenResolver(staticDispatchTokenResolver{token: "secret-token"}),
			)
			if err != nil {
				t.Fatalf("NewGitHubActionsDispatcher: %v", err)
			}
			result, err := dispatcher.Dispatch(context.Background(), request)
			if err != nil {
				t.Fatalf("Dispatch: %v", err)
			}
			if result.Attempted || client.calls != 0 || result.State != state {
				t.Fatalf("expected no redispatch for %s, calls=%d result=%#v", state, client.calls, result)
			}
		})
	}
}

func TestGitHubActionsDispatcherPersistsRejectedAndUnknown(t *testing.T) {
	tests := []struct { //nolint:govet // Table order mirrors terminal states.
		state DispatchJournalState
		err   string
	}{
		{state: DispatchJournalRejected, err: "bad request"},
		{state: DispatchJournalUnknown, err: "outcome is uncertain secret-token"},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			root := newGitHubActionsDispatchRepository(t)
			ctx, gitResult := prepareDispatchRequestContext(t, root, Patch)
			request := mustBuildDispatchRequest(t, ctx, gitResult)
			client := &fakeWorkflowDispatchClient{response: GitHubActionsDispatchResponse{
				State:      tt.state,
				HTTPStatus: 500,
				Error:      sanitizeDispatchText(tt.err, "secret-token"),
			}}
			dispatcher, err := NewGitHubActionsDispatcher(root,
				WithGitHubActionsDispatcherClient(client),
				WithGitHubActionsDispatcherTokenResolver(staticDispatchTokenResolver{token: "secret-token"}),
			)
			if err != nil {
				t.Fatalf("NewGitHubActionsDispatcher: %v", err)
			}
			result, err := dispatcher.Dispatch(context.Background(), request)
			if err != nil {
				t.Fatalf("Dispatch: %v", err)
			}
			journal := loadDispatchJournalForTest(t, result.JournalPath)
			if journal.State != tt.state || strings.Contains(journal.LastError, "secret-token") {
				t.Fatalf("unexpected journal: %#v", journal)
			}
		})
	}
}

func TestGitHubActionsDispatcherRejectsUnsupportedRemoteBeforeJournal(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx, gitResult := prepareDispatchRequestContext(t, root, Patch)
	request := mustBuildDispatchRequest(t, ctx, gitResult)
	request.Identity.RepositoryRemote = "https://gitlab.com/owner/repo.git"
	client := &fakeWorkflowDispatchClient{response: GitHubActionsDispatchResponse{State: DispatchJournalAccepted}}
	dispatcher, err := NewGitHubActionsDispatcher(root,
		WithGitHubActionsDispatcherClient(client),
		WithGitHubActionsDispatcherTokenResolver(staticDispatchTokenResolver{token: "secret-token"}),
	)
	if err != nil {
		t.Fatalf("NewGitHubActionsDispatcher: %v", err)
	}
	result, err := dispatcher.Dispatch(context.Background(), request)
	if err == nil || result == nil || result.Attempted {
		t.Fatalf("expected unsupported remote pre-request failure, result=%#v err=%v", result, err)
	}
	if client.calls != 0 {
		t.Fatalf("client must not be called, got %d", client.calls)
	}
	store := NewDispatchJournalStore(root)
	path, pathErr := store.JournalPath(request.Identity)
	if pathErr != nil {
		t.Fatalf("JournalPath: %v", pathErr)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("journal should not exist for unsupported remote, stat=%v", statErr)
	}
}

//nolint:govet // Test fake fields are ordered by behavior.
type fakeWorkflowDispatchClient struct {
	response GitHubActionsDispatchResponse
	err      error
	calls    int
	target   GitHubRepositoryTarget
}

type journalInspectingDispatchClient struct {
	store         *DispatchJournalStore
	observedState DispatchJournalState
}

func (client *journalInspectingDispatchClient) Dispatch(_ context.Context, _ GitHubRepositoryTarget, request *ReleaseDispatchRequest, _ GitHubActionsDispatchToken) (GitHubActionsDispatchResponse, error) {
	resolution, err := client.store.Load(request)
	if err != nil {
		return GitHubActionsDispatchResponse{}, err
	}
	if resolution.Journal != nil {
		client.observedState = resolution.Journal.State
	}
	return GitHubActionsDispatchResponse{State: DispatchJournalAccepted, HTTPStatus: 204}, nil
}

func (client *fakeWorkflowDispatchClient) Dispatch(_ context.Context, target GitHubRepositoryTarget, _ *ReleaseDispatchRequest, token GitHubActionsDispatchToken) (GitHubActionsDispatchResponse, error) {
	client.calls++
	client.target = target
	if strings.Contains(fmt.Sprint(client.response), token.secretValue()) {
		return GitHubActionsDispatchResponse{}, fmt.Errorf("fake response contains token")
	}
	return client.response, client.err
}

//nolint:govet // Test fake fields are ordered by behavior.
type staticDispatchTokenResolver struct {
	token string
	err   error
}

func (resolver staticDispatchTokenResolver) ResolveGitHubActionsDispatchToken(_ context.Context) (GitHubActionsDispatchToken, error) {
	if resolver.err != nil {
		return GitHubActionsDispatchToken{}, resolver.err
	}
	return NewGitHubActionsDispatchToken(resolver.token)
}

func loadDispatchJournalForTest(t *testing.T, path string) *DispatchJournal {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read journal %s: %v", path, err)
	}
	var journal DispatchJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		t.Fatalf("decode journal %s: %v", path, err)
	}
	return &journal
}
