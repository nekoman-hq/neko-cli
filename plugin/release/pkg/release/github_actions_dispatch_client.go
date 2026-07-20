package release

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/githubdispatch"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
)

// GitHubActionsWorkflowDispatchClient sends exactly one workflow_dispatch HTTP
// request for an already verified immutable dispatch request.
type GitHubActionsWorkflowDispatchClient interface {
	Dispatch(ctx context.Context, target GitHubRepositoryTarget, request *ReleaseDispatchRequest, token GitHubActionsDispatchToken) (GitHubActionsDispatchResponse, error)
}

// GitHubActionsDispatchClient preserves the supported root API while
// delegating the bounded POST to the focused transport package.
type GitHubActionsDispatchClient struct {
	client   *githubdispatch.Client
	protocol githubdispatch.Protocol
	options  []githubdispatch.ClientOption
}

// GitHubActionsDispatchClientOption configures a GitHub Actions dispatch client
// without exposing those options as repository or CLI configuration.
type GitHubActionsDispatchClientOption func(*GitHubActionsDispatchClient) error

// GitHubActionsDispatchResponse is the client-level classification of a single
// workflow_dispatch HTTP request.
//
//nolint:govet // Fields follow result reporting order.
type GitHubActionsDispatchResponse struct {
	State             DispatchJournalState
	HTTPStatus        int
	WorkflowRunID     string
	RunURL            string
	HTMLURL           string
	ResponseTimestamp string
	Error             string
	RecoveryGuidance  string
}

// NewGitHubActionsDispatchClient creates a production-safe dispatch client.
func NewGitHubActionsDispatchClient(options ...GitHubActionsDispatchClientOption) (*GitHubActionsDispatchClient, error) {
	client := &GitHubActionsDispatchClient{
		protocol: githubdispatch.Protocol{APIVersion: githubAPIVersion, UserAgent: githubActionsDispatchUserAgent()},
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(client); err != nil {
			return nil, err
		}
	}
	transport, err := githubdispatch.NewClient(client.protocol, client.options...)
	if err != nil {
		return nil, err
	}
	client.client = transport
	return client, nil
}

// WithGitHubActionsDispatchAPIBaseURL overrides the API base URL for tests.
func WithGitHubActionsDispatchAPIBaseURL(baseURL string) GitHubActionsDispatchClientOption {
	return func(client *GitHubActionsDispatchClient) error {
		return client.applyTransportOption(githubdispatch.WithAPIBaseURL(baseURL))
	}
}

// WithGitHubActionsDispatchTransport injects an HTTP transport for tests.
func WithGitHubActionsDispatchTransport(transport http.RoundTripper) GitHubActionsDispatchClientOption {
	return func(client *GitHubActionsDispatchClient) error {
		return client.applyTransportOption(githubdispatch.WithTransport(transport))
	}
}

// WithGitHubActionsDispatchTimeout overrides the bounded request timeout.
func WithGitHubActionsDispatchTimeout(timeout time.Duration) GitHubActionsDispatchClientOption {
	return func(client *GitHubActionsDispatchClient) error {
		return client.applyTransportOption(githubdispatch.WithTimeout(timeout))
	}
}

func (client *GitHubActionsDispatchClient) applyTransportOption(option githubdispatch.ClientOption) error {
	options := append(append([]githubdispatch.ClientOption(nil), client.options...), option)
	transport, err := githubdispatch.NewClient(client.protocol, options...)
	if err != nil {
		return err
	}
	client.options = options
	client.client = transport
	return nil
}

// Dispatch maps the supported root request and result contracts around the
// neutral transport outcome. Journal policy remains in the root package.
func (client *GitHubActionsDispatchClient) Dispatch(
	ctx context.Context,
	target GitHubRepositoryTarget,
	request *ReleaseDispatchRequest,
	token GitHubActionsDispatchToken,
) (GitHubActionsDispatchResponse, error) {
	if request == nil {
		return GitHubActionsDispatchResponse{}, errDispatchRequestMissing()
	}
	secret := token.secretValue()
	if secret == "" {
		return GitHubActionsDispatchResponse{}, missingGitHubActionsDispatchTokenError()
	}
	bearerToken, err := githubdispatch.NewBearerToken(secret)
	if err != nil {
		return GitHubActionsDispatchResponse{}, missingGitHubActionsDispatchTokenError()
	}
	response, err := client.client.Post(ctx, target, githubdispatch.Request{
		WorkflowFilename: request.WorkflowFileName,
		Ref:              request.Tag,
		Inputs:           releaseworkflow.CanonicalDispatchInputs(request.Inputs),
	}, bearerToken)
	if err != nil {
		return GitHubActionsDispatchResponse{}, err
	}
	state := dispatchJournalStateForOutcome(response.Outcome)
	return GitHubActionsDispatchResponse{
		State:             state,
		HTTPStatus:        response.HTTPStatus,
		WorkflowRunID:     response.WorkflowRunID,
		RunURL:            response.RunURL,
		HTMLURL:           response.HTMLURL,
		ResponseTimestamp: response.ResponseTimestamp,
		Error:             response.Error,
		RecoveryGuidance:  dispatchJournalRecoveryGuidance(state),
	}, nil
}

func dispatchJournalStateForOutcome(outcome githubdispatch.Outcome) DispatchJournalState {
	switch outcome {
	case githubdispatch.OutcomeAccepted:
		return DispatchJournalAccepted
	case githubdispatch.OutcomeRejected:
		return DispatchJournalRejected
	case githubdispatch.OutcomeUnknown:
		return DispatchJournalUnknown
	default:
		return DispatchJournalUnknown
	}
}

func sanitizeDispatchText(value, token string) string {
	bearerToken, err := githubdispatch.NewBearerToken(token)
	if err != nil {
		return githubdispatch.CapText(value)
	}
	return githubdispatch.SanitizeText(value, bearerToken)
}

func capDispatchText(value string) string {
	return githubdispatch.CapText(value)
}

func dispatchHTTPStatus(status int) string {
	if status == 0 {
		return ""
	}
	return strconv.Itoa(status)
}
