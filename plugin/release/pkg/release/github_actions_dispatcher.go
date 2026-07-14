package release

import (
	"context"
	"fmt"

	"github.com/nekoman-hq/neko-cli/pkg/log"
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
	clock         ReleaseClock
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
		clock:         systemReleaseClock{},
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

// WithGitHubActionsDispatcherClock injects the timestamp source used for
// request-started, request-finished, and dispatch journal transitions.
func WithGitHubActionsDispatcherClock(clock ReleaseClock) GitHubActionsDispatcherOption {
	return func(dispatcher *GitHubActionsDispatcher) error {
		if clock == nil {
			return fmt.Errorf("github actions dispatch clock is missing")
		}
		dispatcher.clock = clock
		dispatcher.store.clock = clock
		return nil
	}
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
		dispatcher.store.clock = dispatcher.clock
		return nil
	}
}

// Dispatch performs one internal workflow_dispatch attempt if the durable
// journal is still in prepared state.
func (dispatcher *GitHubActionsDispatcher) Dispatch(ctx context.Context, request *ReleaseDispatchRequest) (*GitHubActionsDispatchResult, error) {
	return dispatcher.dispatch(ctx, request, GitHubActionsDispatchToken{})
}

func (dispatcher *GitHubActionsDispatcher) dispatchWithToken(ctx context.Context, request *ReleaseDispatchRequest, token GitHubActionsDispatchToken) (*GitHubActionsDispatchResult, error) {
	if token.secretValue() == "" {
		return nil, missingGitHubActionsDispatchTokenError()
	}
	return dispatcher.dispatch(ctx, request, token)
}

func (dispatcher *GitHubActionsDispatcher) dispatch(ctx context.Context, request *ReleaseDispatchRequest, token GitHubActionsDispatchToken) (*GitHubActionsDispatchResult, error) {
	if request == nil {
		return nil, errDispatchRequestMissing()
	}
	// The repository target is derived from the same verified Git remote that V2
	// Git coordination used, never from legacy config or workflow paths.
	target, err := ResolveGitHubRepositoryTarget(request.RepositoryRemoteName, request.Identity.RepositoryRemote)
	if err != nil {
		return &GitHubActionsDispatchResult{Identity: request.Identity, Error: err.Error()}, err
	}
	log.PluginPrint(log.Exec, "GitHub Actions target resolved: %s/%s workflow=%s ref=%s", target.Owner, target.Repository, request.WorkflowPath, request.Tag)
	log.PluginPrint(log.Exec, "Preparing GitHub Actions dispatch journal")
	resolution, err := dispatcher.store.Prepare(request)
	if err != nil {
		return nil, err
	}
	log.PluginPrint(log.Exec, "Dispatch journal path: %s", resolution.Path)
	if resolution.Blocked {
		log.PluginPrint(log.Exec, "Dispatch blocked by existing journal state: %s", resolution.Journal.State)
		return dispatchResultFromJournal(resolution.Path, resolution.Journal, false), nil
	}
	if token.secretValue() == "" {
		log.PluginPrint(log.Exec, "Resolving GitHub Actions dispatch token")
		token, err = dispatcher.tokenResolver.ResolveGitHubActionsDispatchToken(ctx)
		if err != nil {
			return dispatchResultFromJournal(resolution.Path, resolution.Journal, false), err
		}
	}
	log.PluginPrint(log.Exec, "GitHub Actions dispatch token available")
	startedAt := dispatcher.clock.Now().UTC()
	// request-started is persisted before the HTTP request because after this
	// point a transport failure may hide whether GitHub accepted the dispatch.
	log.PluginPrint(log.Exec, "Recording dispatch request-started before HTTP call")
	started, err := dispatcher.store.Transition(request, DispatchJournalRequestStarted, DispatchJournalMetadata{
		RequestStartedAt: startedAt,
	}, "")
	if err != nil {
		return nil, err
	}
	log.PluginPrint(log.Exec, "Sending workflow_dispatch request")
	response, clientErr := dispatcher.client.Dispatch(ctx, target, request, token)
	if clientErr != nil {
		response = GitHubActionsDispatchResponse{
			State:            DispatchJournalUnknown,
			Error:            sanitizeDispatchText(clientErr.Error(), token.secretValue()),
			RecoveryGuidance: dispatchJournalRecoveryGuidance(DispatchJournalUnknown),
		}
	}
	if response.State == "" {
		response.State = DispatchJournalUnknown
	}
	log.PluginPrint(log.Exec, "workflow_dispatch response state=%s status=%d", response.State, response.HTTPStatus)
	finishedAt := dispatcher.clock.Now().UTC()
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
	log.PluginPrint(log.Exec, "Dispatch journal finalized with state: %s", final.Journal.State)
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
