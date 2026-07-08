package release

import (
	"context"
	"fmt"
	"time"
)

// GitHubActionsDispatchResult summarizes an internal dispatch attempt without
// exposing credentials or arbitrary response bodies.
//
//nolint:govet // Fields follow user-facing dispatch outcome order.
type GitHubActionsDispatchResult struct {
	JournalPath      string
	Identity         ReleaseDispatchIdentity
	State            DispatchJournalState
	Attempted        bool
	Accepted         bool
	HTTPStatus       int
	WorkflowRunID    string
	RunURL           string
	HTMLURL          string
	RecoveryGuidance string
	Error            string
}

// GitHubActionsDispatcher coordinates target resolution, token resolution, the
// durable journal and one workflow_dispatch HTTP request.
type GitHubActionsDispatcher struct {
	store         *DispatchJournalStore
	client        GitHubActionsWorkflowDispatchClient
	tokenResolver GitHubActionsDispatchTokenResolver
	now           func() time.Time
}

// GitHubActionsDispatcherOption configures internal dispatch dependencies.
type GitHubActionsDispatcherOption func(*GitHubActionsDispatcher) error

// NewGitHubActionsDispatcher creates the internal GitHub Actions dispatcher.
func NewGitHubActionsDispatcher(repositoryRoot string, options ...GitHubActionsDispatcherOption) (*GitHubActionsDispatcher, error) {
	client, err := NewGitHubActionsDispatchClient()
	if err != nil {
		return nil, err
	}
	dispatcher := &GitHubActionsDispatcher{
		store:         NewDispatchJournalStore(repositoryRoot),
		client:        client,
		tokenResolver: EnvironmentGitHubActionsDispatchTokenResolver{},
		now:           func() time.Time { return time.Now().UTC() },
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(dispatcher); err != nil {
			return nil, err
		}
	}
	return dispatcher, nil
}

// WithGitHubActionsDispatcherClient injects a dispatch client for tests.
func WithGitHubActionsDispatcherClient(client GitHubActionsWorkflowDispatchClient) GitHubActionsDispatcherOption {
	return func(dispatcher *GitHubActionsDispatcher) error {
		if client == nil {
			return fmt.Errorf("github actions dispatch client is missing")
		}
		dispatcher.client = client
		return nil
	}
}

// WithGitHubActionsDispatcherTokenResolver injects a token resolver for tests.
func WithGitHubActionsDispatcherTokenResolver(resolver GitHubActionsDispatchTokenResolver) GitHubActionsDispatcherOption {
	return func(dispatcher *GitHubActionsDispatcher) error {
		if resolver == nil {
			return fmt.Errorf("github actions dispatch token resolver is missing")
		}
		dispatcher.tokenResolver = resolver
		return nil
	}
}

// WithGitHubActionsDispatcherStore injects a journal store for tests.
func WithGitHubActionsDispatcherStore(store *DispatchJournalStore) GitHubActionsDispatcherOption {
	return func(dispatcher *GitHubActionsDispatcher) error {
		if store == nil {
			return fmt.Errorf("github actions dispatch journal store is missing")
		}
		dispatcher.store = store
		return nil
	}
}

// Dispatch performs one internal workflow_dispatch attempt if the durable
// journal is still in prepared state.
func (dispatcher *GitHubActionsDispatcher) Dispatch(ctx context.Context, request *ReleaseDispatchRequest) (*GitHubActionsDispatchResult, error) {
	if request == nil {
		return nil, errDispatchRequestMissing()
	}
	// The repository target is derived from the same verified Git remote that V2
	// Git coordination used, never from legacy config or workflow paths.
	target, err := ResolveGitHubRepositoryTarget(request.RepositoryRemoteName, request.Identity.RepositoryRemote)
	if err != nil {
		return &GitHubActionsDispatchResult{Identity: request.Identity, Error: err.Error()}, err
	}
	resolution, err := dispatcher.store.Prepare(request)
	if err != nil {
		return nil, err
	}
	if resolution.Blocked {
		return dispatchResultFromJournal(resolution.Path, resolution.Journal, false), nil
	}
	token, err := dispatcher.tokenResolver.ResolveGitHubActionsDispatchToken(ctx)
	if err != nil {
		return dispatchResultFromJournal(resolution.Path, resolution.Journal, false), err
	}
	startedAt := dispatcher.now()
	// request-started is persisted before the HTTP request because after this
	// point a transport failure may hide whether GitHub accepted the dispatch.
	started, err := dispatcher.store.Transition(request, DispatchJournalRequestStarted, DispatchJournalMetadata{
		RequestStartedAt: startedAt,
	}, "")
	if err != nil {
		return nil, err
	}
	response, clientErr := dispatcher.client.Dispatch(ctx, target, request, token)
	if clientErr != nil {
		response = GitHubActionsDispatchResponse{
			State:            DispatchJournalUnknown,
			Error:            sanitizeDispatchText(clientErr.Error(), token),
			RecoveryGuidance: dispatchJournalRecoveryGuidance(DispatchJournalUnknown),
		}
	}
	if response.State == "" {
		response.State = DispatchJournalUnknown
	}
	finishedAt := dispatcher.now()
	metadata := DispatchJournalMetadata{
		RunID:             response.WorkflowRunID,
		RunURL:            response.RunURL,
		HTMLURL:           response.HTMLURL,
		ResponseStatus:    dispatchHTTPStatus(response.HTTPStatus),
		ResponseTimestamp: response.ResponseTimestamp,
		RequestFinishedAt: finishedAt,
	}
	// Unknown outcomes never auto-retry. A future resume command must inspect
	// GitHub and this journal before deciding whether another request is safe.
	final, err := dispatcher.store.Transition(request, response.State, metadata, response.Error)
	if err != nil {
		return dispatchResultFromJournal(started.Path, started.Journal, true), err
	}
	result := dispatchResultFromJournal(final.Path, final.Journal, true)
	result.HTTPStatus = response.HTTPStatus
	result.WorkflowRunID = response.WorkflowRunID
	result.RunURL = response.RunURL
	result.HTMLURL = response.HTMLURL
	if response.RecoveryGuidance != "" {
		result.RecoveryGuidance = response.RecoveryGuidance
	}
	return result, nil
}

func dispatchResultFromJournal(path string, journal *DispatchJournal, attempted bool) *GitHubActionsDispatchResult {
	result := &GitHubActionsDispatchResult{
		JournalPath: path,
		Attempted:   attempted,
	}
	if journal == nil {
		return result
	}
	result.Identity = journal.Identity
	result.State = journal.State
	result.Accepted = journal.State == DispatchJournalAccepted
	result.WorkflowRunID = journal.DispatchMetadata.RunID
	result.RunURL = journal.DispatchMetadata.RunURL
	result.HTMLURL = journal.DispatchMetadata.HTMLURL
	result.HTTPStatus = 0
	result.RecoveryGuidance = journal.RecoveryGuidance
	result.Error = journal.LastError
	return result
}
