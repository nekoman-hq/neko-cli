package githubdispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
)

const defaultRequestTimeout = 15 * time.Second

// Protocol supplies the GitHub API and caller identity headers owned by the
// composing command boundary.
type Protocol struct {
	APIVersion string
	UserAgent  string
}

// Request contains only the immutable facts needed for one workflow-dispatch
// POST.
type Request struct {
	Inputs           map[string]string
	WorkflowFilename string
	Ref              string
}

// Client sends one bounded authenticated workflow-dispatch POST.
//
//nolint:govet // Fields are grouped by construction concern, not memory layout.
type Client struct {
	baseURLOverride string
	httpClient      *http.Client
	timeout         time.Duration
	protocol        Protocol
}

// ClientOption configures transport details without making them release or
// repository configuration.
type ClientOption func(*Client) error

// NewClient creates a bounded client that refuses redirects.
func NewClient(protocol Protocol, options ...ClientOption) (*Client, error) {
	protocol.APIVersion = strings.TrimSpace(protocol.APIVersion)
	protocol.UserAgent = strings.TrimSpace(protocol.UserAgent)
	if protocol.APIVersion == "" {
		return nil, fmt.Errorf("github actions dispatch API version is missing")
	}
	if protocol.UserAgent == "" {
		return nil, fmt.Errorf("github actions dispatch user agent is missing")
	}
	client := &Client{
		timeout:  defaultRequestTimeout,
		protocol: protocol,
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(client); err != nil {
			return nil, err
		}
	}
	if client.httpClient == nil {
		client.httpClient = redirectRefusingHTTPClient(nil)
	}
	return client, nil
}

// WithAPIBaseURL overrides the target API base URL for an injected endpoint.
func WithAPIBaseURL(baseURL string) ClientOption {
	return func(client *Client) error {
		baseURL = strings.TrimSpace(baseURL)
		if baseURL == "" {
			return fmt.Errorf("github actions dispatch API base URL is empty")
		}
		parsed, err := url.Parse(baseURL)
		if err != nil {
			return fmt.Errorf("parse github actions dispatch API base URL: %w", err)
		}
		if parsed.Scheme != "https" && parsed.Scheme != "http" {
			return fmt.Errorf("github actions dispatch API base URL must be http or https")
		}
		client.baseURLOverride = strings.TrimRight(baseURL, "/")
		return nil
	}
}

// WithTransport injects the single-request HTTP transport.
func WithTransport(transport http.RoundTripper) ClientOption {
	return func(client *Client) error {
		if transport == nil {
			return fmt.Errorf("github actions dispatch transport is missing")
		}
		client.httpClient = redirectRefusingHTTPClient(transport)
		return nil
	}
}

// WithTimeout overrides the positive per-request timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(client *Client) error {
		if timeout <= 0 {
			return fmt.Errorf("github actions dispatch timeout must be positive")
		}
		client.timeout = timeout
		return nil
	}
}

func redirectRefusingHTTPClient(transport http.RoundTripper) *http.Client {
	return &http.Client{
		Transport: transport,
		// Authorization must never be forwarded to another location.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// Post sends exactly one authenticated workflow-dispatch request. Transport
// failures are uncertain outcomes rather than proof that GitHub rejected it.
func (client *Client) Post(
	ctx context.Context,
	target releaseworkflow.GitHubRepositoryTarget,
	request Request,
	token BearerToken,
) (Response, error) {
	if token.value() == "" {
		return Response{}, fmt.Errorf("github actions dispatch token is missing")
	}
	endpoint, err := client.dispatchEndpoint(target, request.WorkflowFilename)
	if err != nil {
		return Response{}, err
	}
	body, err := json.Marshal(dispatchPayload{Ref: request.Ref, Inputs: request.Inputs})
	if err != nil {
		return Response{}, fmt.Errorf("marshal github actions dispatch body: %w", err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("build github actions dispatch request: %w", err)
	}
	client.setRequestHeaders(httpRequest, token)

	response, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return Response{
			Outcome: OutcomeUnknown,
			Error:   SanitizeText(classifyTransportError(err), token),
		}, nil
	}
	defer func() { _ = response.Body.Close() }()
	return classifyResponse(response, token), nil
}

func (client *Client) setRequestHeaders(request *http.Request, token BearerToken) {
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+token.value())
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-GitHub-Api-Version", client.protocol.APIVersion)
	request.Header.Set("User-Agent", client.protocol.UserAgent)
}

func (client *Client) dispatchEndpoint(target releaseworkflow.GitHubRepositoryTarget, workflowFilename string) (string, error) {
	baseURL := strings.TrimRight(target.APIBaseURL, "/")
	if client.baseURLOverride != "" {
		baseURL = client.baseURLOverride
	}
	if baseURL == "" {
		return "", fmt.Errorf("github actions dispatch API base URL is missing")
	}
	workflowFilename = strings.TrimSpace(workflowFilename)
	if workflowFilename == "" || strings.Contains(workflowFilename, "/") || strings.Contains(workflowFilename, "\\") {
		return "", fmt.Errorf("github actions dispatch workflow filename is invalid")
	}
	path := "/repos/" + url.PathEscape(target.Owner) + "/" + url.PathEscape(target.Repository) +
		"/actions/workflows/" + url.PathEscape(workflowFilename) + "/dispatches"
	return baseURL + path, nil
}

//nolint:govet // JSON field order must stay ref then inputs.
type dispatchPayload struct {
	Ref    string            `json:"ref"`
	Inputs map[string]string `json:"inputs"`
}
