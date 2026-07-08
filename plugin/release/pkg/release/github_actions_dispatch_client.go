package release

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

const (
	githubAPIVersion              = "2026-03-10"
	defaultDispatchRequestTimeout = 15 * time.Second
	maxDispatchResponseBytes      = 4096
	maxStoredDispatchErrorLength  = 1024
)

// GitHubActionsWorkflowDispatchClient sends exactly one workflow_dispatch HTTP
// request for an already verified immutable dispatch request.
type GitHubActionsWorkflowDispatchClient interface {
	Dispatch(ctx context.Context, target GitHubRepositoryTarget, request *ReleaseDispatchRequest, token string) (GitHubActionsDispatchResponse, error)
}

// GitHubActionsDispatchClient is the production HTTP implementation of the
// GitHub Actions workflow_dispatch adapter.
//
//nolint:govet // Fields are grouped by construction concern, not memory layout.
type GitHubActionsDispatchClient struct {
	baseURLOverride string
	httpClient      *http.Client
	timeout         time.Duration
	userAgent       string
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
		timeout:   defaultDispatchRequestTimeout,
		userAgent: githubActionsDispatchUserAgent(),
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
		client.httpClient = &http.Client{
			// Redirects are intentionally disabled so Authorization can never be
			// forwarded to a different host.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return client, nil
}

// WithGitHubActionsDispatchAPIBaseURL overrides the API base URL for tests.
func WithGitHubActionsDispatchAPIBaseURL(baseURL string) GitHubActionsDispatchClientOption {
	return func(client *GitHubActionsDispatchClient) error {
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

// WithGitHubActionsDispatchTransport injects an HTTP transport for tests.
func WithGitHubActionsDispatchTransport(transport http.RoundTripper) GitHubActionsDispatchClientOption {
	return func(client *GitHubActionsDispatchClient) error {
		if transport == nil {
			return fmt.Errorf("github actions dispatch transport is missing")
		}
		client.httpClient = &http.Client{
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		return nil
	}
}

// WithGitHubActionsDispatchTimeout overrides the bounded request timeout.
func WithGitHubActionsDispatchTimeout(timeout time.Duration) GitHubActionsDispatchClientOption {
	return func(client *GitHubActionsDispatchClient) error {
		if timeout <= 0 {
			return fmt.Errorf("github actions dispatch timeout must be positive")
		}
		client.timeout = timeout
		return nil
	}
}

func (client *GitHubActionsDispatchClient) Dispatch(ctx context.Context, target GitHubRepositoryTarget, request *ReleaseDispatchRequest, token string) (GitHubActionsDispatchResponse, error) {
	if request == nil {
		return GitHubActionsDispatchResponse{}, errDispatchRequestMissing()
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return GitHubActionsDispatchResponse{}, missingGitHubActionsDispatchTokenError()
	}
	endpoint, err := client.dispatchEndpoint(target, request.WorkflowFileName)
	if err != nil {
		return GitHubActionsDispatchResponse{}, err
	}
	payload := workflowDispatchPayload{
		Ref:    request.Tag,
		Inputs: canonicalWorkflowDispatchInputs(request),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return GitHubActionsDispatchResponse{}, fmt.Errorf("marshal github actions dispatch body: %w", err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return GitHubActionsDispatchResponse{}, fmt.Errorf("build github actions dispatch request: %w", err)
	}
	httpRequest.Header.Set("Accept", "application/vnd.github+json")
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	httpRequest.Header.Set("User-Agent", client.userAgent)

	response, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return GitHubActionsDispatchResponse{
			State:            DispatchJournalUnknown,
			Error:            sanitizeDispatchText(classifyTransportError(err), token),
			RecoveryGuidance: dispatchJournalRecoveryGuidance(DispatchJournalUnknown),
		}, nil
	}
	defer func() { _ = response.Body.Close() }()
	return classifyGitHubActionsDispatchResponse(response, token), nil
}

func (client *GitHubActionsDispatchClient) dispatchEndpoint(target GitHubRepositoryTarget, workflowFilename string) (string, error) {
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
type workflowDispatchPayload struct {
	Ref    string            `json:"ref"`
	Inputs map[string]string `json:"inputs"`
}

func canonicalWorkflowDispatchInputs(request *ReleaseDispatchRequest) map[string]string {
	// Workflow inputs are intentionally minimal: the workflow must derive
	// executor, delivery, paths and configuration from the checked-out tag.
	return map[string]string{
		"unit":        request.Inputs["unit"],
		"version":     request.Inputs["version"],
		"tag":         request.Inputs["tag"],
		"release_sha": request.Inputs["release_sha"],
	}
}

func classifyGitHubActionsDispatchResponse(response *http.Response, token string) GitHubActionsDispatchResponse {
	status := response.StatusCode
	body := readBoundedDispatchBody(response.Body)
	result := GitHubActionsDispatchResponse{
		HTTPStatus:        status,
		ResponseTimestamp: response.Header.Get("Date"),
	}
	switch {
	case status >= 200 && status <= 299:
		result.State = DispatchJournalAccepted
		metadata := parseOptionalWorkflowRunMetadata(body)
		result.WorkflowRunID = metadata.RunID
		result.RunURL = metadata.RunURL
		result.HTMLURL = metadata.HTMLURL
		if metadata.ResponseTimestamp != "" {
			result.ResponseTimestamp = metadata.ResponseTimestamp
		}
		result.RecoveryGuidance = dispatchJournalRecoveryGuidance(DispatchJournalAccepted)
	case isDefinitiveDispatchRejectionStatus(status):
		result.State = DispatchJournalRejected
		result.Error = sanitizeDispatchText(parseGitHubDispatchError(body, status), token)
		result.RecoveryGuidance = dispatchJournalRecoveryGuidance(DispatchJournalRejected)
	default:
		result.State = DispatchJournalUnknown
		result.Error = sanitizeDispatchText(fmt.Sprintf("GitHub Actions dispatch returned HTTP %d; outcome is uncertain", status), token)
		result.RecoveryGuidance = dispatchJournalRecoveryGuidance(DispatchJournalUnknown)
	}
	return result
}

func isDefinitiveDispatchRejectionStatus(status int) bool {
	switch status {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusTooManyRequests:
		return true
	default:
		return false
	}
}

func parseOptionalWorkflowRunMetadata(body []byte) DispatchJournalMetadata {
	if len(bytes.TrimSpace(body)) == 0 {
		return DispatchJournalMetadata{}
	}
	var payload struct {
		ID        json.Number `json:"id"`
		RunID     json.Number `json:"run_id"`
		RunURL    string      `json:"url"`
		HTMLURL   string      `json:"html_url"`
		CreatedAt string      `json:"created_at"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return DispatchJournalMetadata{}
	}
	runID := payload.RunID.String()
	if runID == "" {
		runID = payload.ID.String()
	}
	return DispatchJournalMetadata{
		RunID:             runID,
		RunURL:            payload.RunURL,
		HTMLURL:           payload.HTMLURL,
		ResponseTimestamp: payload.CreatedAt,
	}
}

func parseGitHubDispatchError(body []byte, status int) string {
	var payload struct {
		Message          string `json:"message"`
		DocumentationURL string `json:"documentation_url"`
	}
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &payload); err == nil && payload.Message != "" {
			message := payload.Message
			if payload.DocumentationURL != "" {
				message += " (" + payload.DocumentationURL + ")"
			}
			if status == http.StatusTooManyRequests {
				message += "; request was not accepted and requires a later explicit retry or resume decision"
			}
			return capDispatchText(message)
		}
	}
	if status == http.StatusTooManyRequests {
		return "GitHub Actions dispatch was rejected with HTTP 429; request was not accepted and requires a later explicit retry or resume decision"
	}
	return fmt.Sprintf("GitHub Actions dispatch was rejected with HTTP %d", status)
}

func classifyTransportError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "GitHub Actions dispatch context was canceled after request start; outcome is uncertain"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "GitHub Actions dispatch timed out after request start; outcome is uncertain"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "GitHub Actions dispatch timed out after request start; outcome is uncertain"
	}
	return "GitHub Actions dispatch transport failed after request start; outcome is uncertain: " + err.Error()
}

func readBoundedDispatchBody(body io.Reader) []byte {
	if body == nil {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(body, maxDispatchResponseBytes))
	return data
}

func sanitizeDispatchText(value, token string) string {
	value = capDispatchText(value)
	token = strings.TrimSpace(token)
	if token != "" {
		value = strings.ReplaceAll(value, token, "[redacted]")
	}
	value = strings.ReplaceAll(value, "Bearer [redacted]", "Bearer [redacted]")
	return value
}

func capDispatchText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxStoredDispatchErrorLength {
		return value
	}
	return value[:maxStoredDispatchErrorLength] + "...[truncated]"
}

func githubActionsDispatchUserAgent() string {
	version := strings.TrimSpace(metadata.Version)
	if version == "" {
		version = "dev"
	}
	return "neko-cli/" + version
}

// GitHubActionsDispatchTokenResolver resolves the production dispatch token.
type GitHubActionsDispatchTokenResolver interface {
	ResolveGitHubActionsDispatchToken(ctx context.Context) (string, error)
}

// EnvironmentGitHubActionsDispatchTokenResolver resolves GITHUB_TOKEN for real
// internal dispatch attempts.
type EnvironmentGitHubActionsDispatchTokenResolver struct{}

func (EnvironmentGitHubActionsDispatchTokenResolver) ResolveGitHubActionsDispatchToken(_ context.Context) (string, error) {
	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if token == "" {
		return "", missingGitHubActionsDispatchTokenError()
	}
	return token, nil
}

func missingGitHubActionsDispatchTokenError() error {
	return fmt.Errorf("GitHub Actions dispatch requires GITHUB_TOKEN with the appropriate repository Actions write permission")
}

func dispatchHTTPStatus(status int) string {
	if status == 0 {
		return ""
	}
	return strconv.Itoa(status)
}
