package release

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	integrationDoctorGitHubReadTimeout   = 12 * time.Second
	integrationDoctorGitHubReadBodyLimit = 1 << 20
)

type integrationDoctorGitHubReadOutcome struct {
	State          integrationDoctorVerificationState
	HTTPStatus     int
	RetryAfter     string
	RateLimitReset string
}

type integrationDoctorGitHubRepository struct {
	Owner         string
	Name          string
	DefaultBranch string
	Visibility    string
	Private       bool
}

type integrationDoctorGitHubWorkflowContent struct {
	Path    string
	Content []byte
}

type integrationDoctorGitHubWorkflowMetadata struct {
	Path  string
	State string
}

type integrationDoctorGitHubVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type integrationDoctorGitHubSecretMetadata struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type integrationDoctorGitHubActionsPolicy struct {
	Enabled        bool
	AllowedActions string
}

type integrationDoctorGitHubRelease struct {
	TagName    string
	Assets     []string
	Draft      bool
	Prerelease bool
}

type integrationDoctorGitHubTagReference struct {
	Reference  string
	ObjectType string
	ObjectSHA  string
}

type integrationDoctorGitHubWorkflowRun struct {
	ID         string
	WorkflowID string
	Status     string
	Conclusion string
}

type integrationDoctorGitHubReader interface {
	Repository(context.Context, integrationDoctorRepositoryIdentity, GitHubActionsDispatchToken) (integrationDoctorGitHubRepository, integrationDoctorGitHubReadOutcome)
	WorkflowContent(context.Context, integrationDoctorRepositoryIdentity, string, string, GitHubActionsDispatchToken) (integrationDoctorGitHubWorkflowContent, integrationDoctorGitHubReadOutcome)
	WorkflowMetadata(context.Context, integrationDoctorRepositoryIdentity, string, GitHubActionsDispatchToken) (integrationDoctorGitHubWorkflowMetadata, integrationDoctorGitHubReadOutcome)
	RepositoryVariable(context.Context, integrationDoctorRepositoryIdentity, string, GitHubActionsDispatchToken) (integrationDoctorGitHubVariable, integrationDoctorGitHubReadOutcome)
	RepositorySecret(context.Context, integrationDoctorRepositoryIdentity, string, GitHubActionsDispatchToken) (integrationDoctorGitHubSecretMetadata, integrationDoctorGitHubReadOutcome)
	ActionsPolicy(context.Context, integrationDoctorRepositoryIdentity, GitHubActionsDispatchToken) (integrationDoctorGitHubActionsPolicy, integrationDoctorGitHubReadOutcome)
	ReleaseByTag(context.Context, integrationDoctorRepositoryIdentity, string, GitHubActionsDispatchToken) (integrationDoctorGitHubRelease, integrationDoctorGitHubReadOutcome)
	TagReference(context.Context, integrationDoctorRepositoryIdentity, string, GitHubActionsDispatchToken) (integrationDoctorGitHubTagReference, integrationDoctorGitHubReadOutcome)
	WorkflowRun(context.Context, integrationDoctorRepositoryIdentity, string, GitHubActionsDispatchToken) (integrationDoctorGitHubWorkflowRun, integrationDoctorGitHubReadOutcome)
}

// integrationDoctorGitHubReadClient contains only the fixed GitHub GET
// operations used by explicit Doctor remote verification.
type integrationDoctorGitHubReadClient struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
	userAgent  string
}

type integrationDoctorGitHubReadClientOption func(*integrationDoctorGitHubReadClient) error

func newIntegrationDoctorGitHubReadClient(
	options ...integrationDoctorGitHubReadClientOption,
) (*integrationDoctorGitHubReadClient, error) {
	client := &integrationDoctorGitHubReadClient{
		baseURL:   githubActionsAPIBaseURL,
		timeout:   integrationDoctorGitHubReadTimeout,
		userAgent: githubActionsDispatchUserAgent(),
	}
	for _, option := range options {
		if option != nil {
			if err := option(client); err != nil {
				return nil, err
			}
		}
	}
	if client.httpClient == nil {
		client.httpClient = integrationDoctorReadOnlyHTTPClient(nil)
	}
	return client, nil
}

func withIntegrationDoctorGitHubReadBaseURL(baseURL string) integrationDoctorGitHubReadClientOption {
	return func(client *integrationDoctorGitHubReadClient) error {
		parsed, err := url.Parse(strings.TrimSpace(baseURL))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("integration doctor GitHub read base URL is invalid")
		}
		if parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("integration doctor GitHub read base URL must not contain a query or fragment")
		}
		client.baseURL = strings.TrimRight(parsed.String(), "/")
		return nil
	}
}

func withIntegrationDoctorGitHubReadTransport(
	transport http.RoundTripper,
) integrationDoctorGitHubReadClientOption {
	return func(client *integrationDoctorGitHubReadClient) error {
		if transport == nil {
			return fmt.Errorf("integration doctor GitHub read transport is missing")
		}
		client.httpClient = integrationDoctorReadOnlyHTTPClient(transport)
		return nil
	}
}

func withIntegrationDoctorGitHubReadTimeout(timeout time.Duration) integrationDoctorGitHubReadClientOption {
	return func(client *integrationDoctorGitHubReadClient) error {
		if timeout <= 0 {
			return fmt.Errorf("integration doctor GitHub read timeout must be positive")
		}
		client.timeout = timeout
		return nil
	}
}

func integrationDoctorReadOnlyHTTPClient(transport http.RoundTripper) *http.Client {
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (client *integrationDoctorGitHubReadClient) Repository(
	ctx context.Context,
	identity integrationDoctorRepositoryIdentity,
	token GitHubActionsDispatchToken,
) (integrationDoctorGitHubRepository, integrationDoctorGitHubReadOutcome) {
	body, outcome := client.get(ctx, client.repositoryEndpoint(identity), token)
	if outcome.State != integrationDoctorVerified {
		return integrationDoctorGitHubRepository{}, outcome
	}
	var payload struct {
		Name          string `json:"name"`
		DefaultBranch string `json:"default_branch"`
		Visibility    string `json:"visibility"`
		Private       bool   `json:"private"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
	}
	if !decodeIntegrationDoctorGitHubResponse(body, &payload) || payload.Owner.Login == "" ||
		payload.Name == "" || payload.DefaultBranch == "" {
		return integrationDoctorGitHubRepository{}, integrationDoctorMalformedGitHubOutcome(outcome.HTTPStatus)
	}
	return integrationDoctorGitHubRepository{
		Owner: payload.Owner.Login, Name: payload.Name, DefaultBranch: payload.DefaultBranch,
		Visibility: payload.Visibility, Private: payload.Private,
	}, outcome
}

func (client *integrationDoctorGitHubReadClient) WorkflowContent(
	ctx context.Context,
	identity integrationDoctorRepositoryIdentity,
	workflowPath string,
	branch string,
	token GitHubActionsDispatchToken,
) (integrationDoctorGitHubWorkflowContent, integrationDoctorGitHubReadOutcome) {
	segments := append(client.repositorySegments(identity), "contents")
	segments = append(segments, strings.Split(workflowPath, "/")...)
	query := url.Values{"ref": []string{branch}}
	body, outcome := client.get(ctx, client.endpoint(segments, query), token)
	if outcome.State != integrationDoctorVerified {
		return integrationDoctorGitHubWorkflowContent{}, outcome
	}
	var payload struct {
		Path     string `json:"path"`
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
		Type     string `json:"type"`
	}
	if !decodeIntegrationDoctorGitHubResponse(body, &payload) || payload.Type != "file" ||
		payload.Path != workflowPath || payload.Encoding != "base64" {
		return integrationDoctorGitHubWorkflowContent{}, integrationDoctorMalformedGitHubOutcome(outcome.HTTPStatus)
	}
	content, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(payload.Content, "\n", ""))
	if err != nil || len(content) > integrationDoctorGitHubReadBodyLimit {
		return integrationDoctorGitHubWorkflowContent{}, integrationDoctorMalformedGitHubOutcome(outcome.HTTPStatus)
	}
	return integrationDoctorGitHubWorkflowContent{Path: payload.Path, Content: content}, outcome
}

func (client *integrationDoctorGitHubReadClient) WorkflowMetadata(
	ctx context.Context,
	identity integrationDoctorRepositoryIdentity,
	workflowPath string,
	token GitHubActionsDispatchToken,
) (integrationDoctorGitHubWorkflowMetadata, integrationDoctorGitHubReadOutcome) {
	filename := workflowPath
	if index := strings.LastIndex(workflowPath, "/"); index >= 0 {
		filename = workflowPath[index+1:]
	}
	segments := append(client.repositorySegments(identity), "actions", "workflows", filename)
	body, outcome := client.get(ctx, client.endpoint(segments, nil), token)
	if outcome.State != integrationDoctorVerified {
		return integrationDoctorGitHubWorkflowMetadata{}, outcome
	}
	var payload struct {
		Path  string `json:"path"`
		State string `json:"state"`
	}
	if !decodeIntegrationDoctorGitHubResponse(body, &payload) || payload.Path != workflowPath || payload.State == "" {
		return integrationDoctorGitHubWorkflowMetadata{}, integrationDoctorMalformedGitHubOutcome(outcome.HTTPStatus)
	}
	return integrationDoctorGitHubWorkflowMetadata{Path: payload.Path, State: payload.State}, outcome
}

func (client *integrationDoctorGitHubReadClient) RepositoryVariable(
	ctx context.Context,
	identity integrationDoctorRepositoryIdentity,
	name string,
	token GitHubActionsDispatchToken,
) (integrationDoctorGitHubVariable, integrationDoctorGitHubReadOutcome) {
	segments := append(client.repositorySegments(identity), "actions", "variables", name)
	body, outcome := client.get(ctx, client.endpoint(segments, nil), token)
	if outcome.State != integrationDoctorVerified {
		return integrationDoctorGitHubVariable{}, outcome
	}
	var payload integrationDoctorGitHubVariable
	if !decodeIntegrationDoctorGitHubResponse(body, &payload) || payload.Name != name {
		return integrationDoctorGitHubVariable{}, integrationDoctorMalformedGitHubOutcome(outcome.HTTPStatus)
	}
	return payload, outcome
}

func (client *integrationDoctorGitHubReadClient) RepositorySecret(
	ctx context.Context,
	identity integrationDoctorRepositoryIdentity,
	name string,
	token GitHubActionsDispatchToken,
) (integrationDoctorGitHubSecretMetadata, integrationDoctorGitHubReadOutcome) {
	segments := append(client.repositorySegments(identity), "actions", "secrets", name)
	body, outcome := client.get(ctx, client.endpoint(segments, nil), token)
	if outcome.State != integrationDoctorVerified {
		return integrationDoctorGitHubSecretMetadata{}, outcome
	}
	var payload integrationDoctorGitHubSecretMetadata
	if !decodeIntegrationDoctorGitHubResponse(body, &payload) || payload.Name != name {
		return integrationDoctorGitHubSecretMetadata{}, integrationDoctorMalformedGitHubOutcome(outcome.HTTPStatus)
	}
	return payload, outcome
}

func (client *integrationDoctorGitHubReadClient) ActionsPolicy(
	ctx context.Context,
	identity integrationDoctorRepositoryIdentity,
	token GitHubActionsDispatchToken,
) (integrationDoctorGitHubActionsPolicy, integrationDoctorGitHubReadOutcome) {
	segments := append(client.repositorySegments(identity), "actions", "permissions")
	body, outcome := client.get(ctx, client.endpoint(segments, nil), token)
	if outcome.State != integrationDoctorVerified {
		return integrationDoctorGitHubActionsPolicy{}, outcome
	}
	var payload struct {
		Enabled        *bool  `json:"enabled"`
		AllowedActions string `json:"allowed_actions"`
	}
	if !decodeIntegrationDoctorGitHubResponse(body, &payload) || payload.Enabled == nil {
		return integrationDoctorGitHubActionsPolicy{}, integrationDoctorMalformedGitHubOutcome(outcome.HTTPStatus)
	}
	return integrationDoctorGitHubActionsPolicy{
		Enabled: *payload.Enabled, AllowedActions: payload.AllowedActions,
	}, outcome
}

func (client *integrationDoctorGitHubReadClient) ReleaseByTag(
	ctx context.Context,
	identity integrationDoctorRepositoryIdentity,
	tag string,
	token GitHubActionsDispatchToken,
) (integrationDoctorGitHubRelease, integrationDoctorGitHubReadOutcome) {
	segments := append(client.repositorySegments(identity), "releases", "tags", tag)
	body, outcome := client.get(ctx, client.endpoint(segments, nil), token)
	if outcome.State != integrationDoctorVerified {
		return integrationDoctorGitHubRelease{}, outcome
	}
	var payload struct {
		TagName    string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if !decodeIntegrationDoctorGitHubResponse(body, &payload) || payload.TagName != tag || payload.Assets == nil {
		return integrationDoctorGitHubRelease{}, integrationDoctorMalformedGitHubOutcome(outcome.HTTPStatus)
	}
	assets := make([]string, 0, len(payload.Assets))
	for _, asset := range payload.Assets {
		if asset.Name == "" {
			return integrationDoctorGitHubRelease{}, integrationDoctorMalformedGitHubOutcome(outcome.HTTPStatus)
		}
		assets = append(assets, asset.Name)
	}
	return integrationDoctorGitHubRelease{
		TagName: payload.TagName, Assets: assets, Draft: payload.Draft, Prerelease: payload.Prerelease,
	}, outcome
}

func (client *integrationDoctorGitHubReadClient) TagReference(
	ctx context.Context,
	identity integrationDoctorRepositoryIdentity,
	tag string,
	token GitHubActionsDispatchToken,
) (integrationDoctorGitHubTagReference, integrationDoctorGitHubReadOutcome) {
	segments := append(client.repositorySegments(identity), "git", "ref", "tags", tag)
	body, outcome := client.get(ctx, client.endpoint(segments, nil), token)
	if outcome.State != integrationDoctorVerified {
		return integrationDoctorGitHubTagReference{}, outcome
	}
	var payload struct {
		Ref    string `json:"ref"`
		Object struct {
			Type string `json:"type"`
			SHA  string `json:"sha"`
		} `json:"object"`
	}
	if !decodeIntegrationDoctorGitHubResponse(body, &payload) || payload.Ref != "refs/tags/"+tag ||
		payload.Object.Type == "" || payload.Object.SHA == "" {
		return integrationDoctorGitHubTagReference{}, integrationDoctorMalformedGitHubOutcome(outcome.HTTPStatus)
	}
	return integrationDoctorGitHubTagReference{
		Reference: payload.Ref, ObjectType: payload.Object.Type, ObjectSHA: payload.Object.SHA,
	}, outcome
}

func (client *integrationDoctorGitHubReadClient) WorkflowRun(
	ctx context.Context,
	identity integrationDoctorRepositoryIdentity,
	runID string,
	token GitHubActionsDispatchToken,
) (integrationDoctorGitHubWorkflowRun, integrationDoctorGitHubReadOutcome) {
	segments := append(client.repositorySegments(identity), "actions", "runs", runID)
	body, outcome := client.get(ctx, client.endpoint(segments, nil), token)
	if outcome.State != integrationDoctorVerified {
		return integrationDoctorGitHubWorkflowRun{}, outcome
	}
	var payload struct {
		ID         json.Number `json:"id"`
		WorkflowID json.Number `json:"workflow_id"`
		Status     string      `json:"status"`
		Conclusion string      `json:"conclusion"`
	}
	if !decodeIntegrationDoctorGitHubResponse(body, &payload) || payload.ID.String() != runID ||
		payload.WorkflowID.String() == "" || payload.Status == "" {
		return integrationDoctorGitHubWorkflowRun{}, integrationDoctorMalformedGitHubOutcome(outcome.HTTPStatus)
	}
	return integrationDoctorGitHubWorkflowRun{
		ID: payload.ID.String(), WorkflowID: payload.WorkflowID.String(),
		Status: payload.Status, Conclusion: payload.Conclusion,
	}, outcome
}

func (client *integrationDoctorGitHubReadClient) get(
	ctx context.Context,
	endpoint string,
	token GitHubActionsDispatchToken,
) ([]byte, integrationDoctorGitHubReadOutcome) {
	requestContext, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, integrationDoctorGitHubReadOutcome{State: integrationDoctorUnavailable}
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	request.Header.Set("User-Agent", client.userAgent)
	if secret := token.secretValue(); secret != "" {
		request.Header.Set("Authorization", "Bearer "+secret)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, integrationDoctorTransportOutcome(err)
	}
	defer func() { _ = response.Body.Close() }()
	outcome := integrationDoctorOutcomeFromResponse(response)
	if outcome.State != integrationDoctorVerified {
		return nil, outcome
	}
	body, bounded := readIntegrationDoctorGitHubBody(response.Body)
	if !bounded {
		return nil, integrationDoctorGitHubReadOutcome{
			State: integrationDoctorUnavailable, HTTPStatus: response.StatusCode,
		}
	}
	return body, outcome
}

func (client *integrationDoctorGitHubReadClient) repositoryEndpoint(
	identity integrationDoctorRepositoryIdentity,
) string {
	return client.endpoint(client.repositorySegments(identity), nil)
}

func (*integrationDoctorGitHubReadClient) repositorySegments(
	identity integrationDoctorRepositoryIdentity,
) []string {
	return []string{"repos", identity.Owner, identity.Repository}
}

func (client *integrationDoctorGitHubReadClient) endpoint(
	segments []string,
	query url.Values,
) string {
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		escaped = append(escaped, url.PathEscape(segment))
	}
	endpoint := strings.TrimRight(client.baseURL, "/") + "/" + strings.Join(escaped, "/")
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	return endpoint
}

func decodeIntegrationDoctorGitHubResponse(body []byte, target any) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

func readIntegrationDoctorGitHubBody(reader io.Reader) ([]byte, bool) {
	if reader == nil {
		return nil, true
	}
	body, err := io.ReadAll(io.LimitReader(reader, integrationDoctorGitHubReadBodyLimit+1))
	return body, err == nil && len(body) <= integrationDoctorGitHubReadBodyLimit
}

func integrationDoctorOutcomeFromResponse(response *http.Response) integrationDoctorGitHubReadOutcome {
	outcome := integrationDoctorGitHubReadOutcome{
		HTTPStatus:     response.StatusCode,
		RetryAfter:     integrationDoctorSafeRetryAfter(response.Header.Get("Retry-After")),
		RateLimitReset: integrationDoctorSafeRateLimitReset(response.Header.Get("X-RateLimit-Reset")),
	}
	switch {
	case response.StatusCode >= 200 && response.StatusCode <= 299:
		outcome.State = integrationDoctorVerified
	case response.StatusCode == http.StatusNotFound:
		outcome.State = integrationDoctorMissing
	case response.StatusCode == http.StatusUnauthorized:
		outcome.State = integrationDoctorUnauthorized
	case response.StatusCode == http.StatusTooManyRequests:
		outcome.State = integrationDoctorRateLimited
	case response.StatusCode == http.StatusForbidden && integrationDoctorResponseIsRateLimited(response):
		outcome.State = integrationDoctorRateLimited
	case response.StatusCode == http.StatusForbidden:
		outcome.State = integrationDoctorUnauthorized
	default:
		outcome.State = integrationDoctorUnavailable
	}
	return outcome
}

func integrationDoctorResponseIsRateLimited(response *http.Response) bool {
	return response.Header.Get("Retry-After") != "" || response.Header.Get("X-RateLimit-Remaining") == "0" ||
		response.Header.Get("X-RateLimit-Reset") != ""
}

func integrationDoctorTransportOutcome(err error) integrationDoctorGitHubReadOutcome {
	state := integrationDoctorUnavailable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return integrationDoctorGitHubReadOutcome{State: state}
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return integrationDoctorGitHubReadOutcome{State: state}
	}
	return integrationDoctorGitHubReadOutcome{State: state}
}

func integrationDoctorMalformedGitHubOutcome(status int) integrationDoctorGitHubReadOutcome {
	return integrationDoctorGitHubReadOutcome{State: integrationDoctorUnavailable, HTTPStatus: status}
}

func integrationDoctorSafeRetryAfter(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return strconv.Itoa(seconds)
	}
	if parsed, err := http.ParseTime(value); err == nil {
		return parsed.UTC().Format(time.RFC3339)
	}
	return ""
}

func integrationDoctorSafeRateLimitReset(value string) string {
	value = strings.TrimSpace(value)
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds < 0 {
		return ""
	}
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

var _ integrationDoctorGitHubReader = (*integrationDoctorGitHubReadClient)(nil)
