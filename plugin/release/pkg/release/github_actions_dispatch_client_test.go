package release

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEnvironmentGitHubActionsDispatchTokenResolverUsesOnlyGitHubTokenAndRedactsFormatting(t *testing.T) {
	t.Setenv("GH_TOKEN", "ignored-gh-token")
	t.Setenv("GITHUB_PAT", "ignored-github-pat")
	t.Setenv("GITHUB_TOKEN", "  canonical-github-token  ")

	token, err := (EnvironmentGitHubActionsDispatchTokenResolver{}).ResolveGitHubActionsDispatchToken(context.Background())
	if err != nil {
		t.Fatalf("ResolveGitHubActionsDispatchToken: %v", err)
	}
	if got := token.secretValue(); got != "canonical-github-token" {
		t.Fatalf("resolved token = %q, want trimmed GITHUB_TOKEN", got)
	}
	formatted := fmt.Sprintf("%s %v %+v %#v", token, token, token, token)
	if strings.Contains(formatted, token.secretValue()) || !strings.Contains(formatted, "[redacted]") {
		t.Fatalf("typed token formatting was not redacted: %s", formatted)
	}
}

func TestEnvironmentGitHubActionsDispatchTokenResolverPreservesMissingTokenError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", " \t ")

	_, err := (EnvironmentGitHubActionsDispatchTokenResolver{}).ResolveGitHubActionsDispatchToken(context.Background())
	if err == nil || err.Error() != "GitHub Actions dispatch requires GITHUB_TOKEN with the appropriate repository Actions write permission" {
		t.Fatalf("missing-token error changed: %v", err)
	}
}

func TestGitHubActionsDispatchClientPreservesOptionValidation(t *testing.T) {
	for _, test := range []struct {
		name   string
		option GitHubActionsDispatchClientOption
		want   string
	}{
		{name: "empty base URL", option: WithGitHubActionsDispatchAPIBaseURL(" "), want: "API base URL is empty"},
		{name: "invalid scheme", option: WithGitHubActionsDispatchAPIBaseURL("ftp://example.test"), want: "must be http or https"},
		{name: "missing transport", option: WithGitHubActionsDispatchTransport(nil), want: "transport is missing"},
		{name: "nonpositive timeout", option: WithGitHubActionsDispatchTimeout(0), want: "timeout must be positive"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewGitHubActionsDispatchClient(test.option); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewGitHubActionsDispatchClient error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestGitHubActionsDispatchClientOptionCanConfigureConstructedClient(t *testing.T) {
	client, err := NewGitHubActionsDispatchClient()
	if err != nil {
		t.Fatalf("NewGitHubActionsDispatchClient: %v", err)
	}
	requests := 0
	transport := dispatchClientRoundTripper(func(request *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})
	if optionErr := WithGitHubActionsDispatchTransport(transport)(client); optionErr != nil {
		t.Fatalf("configure constructed client: %v", optionErr)
	}
	result, err := client.Dispatch(
		context.Background(),
		GitHubRepositoryTarget{Owner: "owner", Repository: "repo", APIBaseURL: "https://api.github.test"},
		newDispatchClientTestRequest(t),
		GitHubActionsDispatchToken{secret: "secret-token"},
	)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if requests != 1 || result.State != DispatchJournalAccepted {
		t.Fatalf("requests=%d result=%#v", requests, result)
	}
}

func TestGitHubActionsDispatchClientBuildsWorkflowDispatchRequest(t *testing.T) {
	request := newDispatchClientTestRequest(t)
	var seenPath string
	var seenHeaders http.Header
	var seenBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.EscapedPath()
		seenHeaders = r.Header.Clone()
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client, err := NewGitHubActionsDispatchClient(WithGitHubActionsDispatchAPIBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewGitHubActionsDispatchClient: %v", err)
	}
	target := GitHubRepositoryTarget{Owner: "neko man", Repository: "repo.one", APIBaseURL: server.URL}

	result, err := client.Dispatch(context.Background(), target, request, GitHubActionsDispatchToken{secret: "super-secret-token"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if result.State != DispatchJournalAccepted || result.HTTPStatus != http.StatusNoContent {
		t.Fatalf("unexpected result: %#v", result)
	}
	if seenPath != "/repos/neko%20man/repo.one/actions/workflows/release-api.yml/dispatches" {
		t.Fatalf("unexpected endpoint path %s", seenPath)
	}
	if seenHeaders.Get("Authorization") != "Bearer super-secret-token" ||
		seenHeaders.Get("Accept") != "application/vnd.github+json" ||
		seenHeaders.Get("Content-Type") != "application/json" ||
		seenHeaders.Get("X-GitHub-Api-Version") != githubAPIVersion ||
		seenHeaders.Get("User-Agent") == "" {
		t.Fatalf("unexpected headers: %#v", seenHeaders)
	}
	assertWorkflowDispatchBody(t, seenBody)
}

func TestGitHubActionsDispatchClientClassifiesResponses(t *testing.T) {
	tests := []struct { //nolint:govet // Table order mirrors dispatch outcomes.
		name          string
		status        int
		body          string
		expectedState DispatchJournalState
	}{
		{
			name:          "metadata accepted",
			status:        http.StatusOK,
			body:          `{"run_id":123,"url":"https://api.github.com/runs/123","html_url":"https://github.com/o/r/actions/runs/123","created_at":"2026-07-07T10:00:00Z"}`,
			expectedState: DispatchJournalAccepted,
		},
		{name: "other 2xx accepted", status: http.StatusAccepted, body: `not-json`, expectedState: DispatchJournalAccepted},
		{name: "400 rejected", status: http.StatusBadRequest, body: `{"message":"bad request"}`, expectedState: DispatchJournalRejected},
		{name: "401 rejected", status: http.StatusUnauthorized, body: `{"message":"unauthorized"}`, expectedState: DispatchJournalRejected},
		{name: "403 rejected", status: http.StatusForbidden, body: `{"message":"forbidden"}`, expectedState: DispatchJournalRejected},
		{name: "404 rejected", status: http.StatusNotFound, body: `{"message":"missing"}`, expectedState: DispatchJournalRejected},
		{name: "422 rejected", status: http.StatusUnprocessableEntity, body: `{"message":"invalid"}`, expectedState: DispatchJournalRejected},
		{name: "429 rejected", status: http.StatusTooManyRequests, body: `{"message":"rate limited"}`, expectedState: DispatchJournalRejected},
		{name: "500 unknown", status: http.StatusInternalServerError, body: `{"message":"server"}`, expectedState: DispatchJournalUnknown},
		{name: "redirect unknown", status: http.StatusFound, body: ``, expectedState: DispatchJournalUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := newDispatchClientTestRequest(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.status == http.StatusFound {
					w.Header().Set("Location", "/elsewhere")
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			client, err := NewGitHubActionsDispatchClient(WithGitHubActionsDispatchAPIBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewGitHubActionsDispatchClient: %v", err)
			}
			result, err := client.Dispatch(context.Background(), GitHubRepositoryTarget{Owner: "owner", Repository: "repo", APIBaseURL: server.URL}, request, GitHubActionsDispatchToken{secret: "secret-token"})
			if err != nil {
				t.Fatalf("Dispatch: %v", err)
			}
			if result.State != tt.expectedState {
				t.Fatalf("expected %s, got %#v", tt.expectedState, result)
			}
			if tt.name == "metadata accepted" && (result.WorkflowRunID != "123" || result.RunURL == "" || result.HTMLURL == "") {
				t.Fatalf("metadata not captured: %#v", result)
			}
			if strings.Contains(result.Error, "secret-token") {
				t.Fatalf("token leaked in error: %q", result.Error)
			}
		})
	}
}

func TestGitHubActionsDispatchClientDoesNotFollowRedirects(t *testing.T) {
	requests := 0
	request := newDispatchClientTestRequest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path == "/elsewhere" {
			t.Fatal("redirect target must not be requested")
		}
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	defer server.Close()
	client, err := NewGitHubActionsDispatchClient(WithGitHubActionsDispatchAPIBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewGitHubActionsDispatchClient: %v", err)
	}
	result, err := client.Dispatch(context.Background(), GitHubRepositoryTarget{Owner: "owner", Repository: "repo", APIBaseURL: server.URL}, request, GitHubActionsDispatchToken{secret: "secret-token"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if requests != 1 || result.State != DispatchJournalUnknown {
		t.Fatalf("expected one unknown redirect result, requests=%d result=%#v", requests, result)
	}
}

func TestGitHubActionsDispatchClientTimeoutIsUnknown(t *testing.T) {
	request := newDispatchClientTestRequest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client, err := NewGitHubActionsDispatchClient(
		WithGitHubActionsDispatchAPIBaseURL(server.URL),
		WithGitHubActionsDispatchTimeout(time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewGitHubActionsDispatchClient: %v", err)
	}
	result, err := client.Dispatch(context.Background(), GitHubRepositoryTarget{Owner: "owner", Repository: "repo", APIBaseURL: server.URL}, request, GitHubActionsDispatchToken{secret: "secret-token"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if result.State != DispatchJournalUnknown || !strings.Contains(result.Error, "timed out") {
		t.Fatalf("expected timeout unknown, got %#v", result)
	}
}

func newDispatchClientTestRequest(t *testing.T) *ReleaseDispatchRequest {
	t.Helper()
	identity, err := newReleaseDispatchIdentity(
		"origin",
		"https://github.com/owner/repo",
		"api",
		"0.2.1",
		"api/v0.2.1",
		strings.Repeat("a", 40),
		".github/workflows/release-api.yml",
		"goreleaser",
		"github-actions",
	)
	if err != nil {
		t.Fatalf("newReleaseDispatchIdentity: %v", err)
	}
	return &ReleaseDispatchRequest{
		RepositoryRemoteName: "origin",
		UnitID:               "api",
		Version:              "0.2.1",
		Tag:                  "api/v0.2.1",
		ReleaseCommitSHA:     strings.Repeat("a", 40),
		WorkflowPath:         ".github/workflows/release-api.yml",
		WorkflowFileName:     "release-api.yml",
		Delivery:             "github-actions",
		Executor:             "goreleaser",
		Inputs: map[string]string{
			"unit":        "api",
			"version":     "0.2.1",
			"tag":         "api/v0.2.1",
			"release_sha": strings.Repeat("a", 40),
			"ignored":     "must-not-be-sent",
		},
		Identity: identity,
	}
}

func assertWorkflowDispatchBody(t *testing.T, body map[string]any) {
	t.Helper()
	if len(body) != 2 || body["ref"] != "api/v0.2.1" {
		t.Fatalf("unexpected body: %#v", body)
	}
	inputs, ok := body["inputs"].(map[string]any)
	if !ok {
		t.Fatalf("inputs missing: %#v", body)
	}
	if len(inputs) != 4 ||
		inputs["unit"] != "api" ||
		inputs["version"] != "0.2.1" ||
		inputs["tag"] != "api/v0.2.1" ||
		inputs["release_sha"] != strings.Repeat("a", 40) {
		t.Fatalf("unexpected inputs: %#v", inputs)
	}
	if _, ok := inputs["ignored"]; ok {
		t.Fatalf("unexpected ignored input leaked: %#v", inputs)
	}
}

type dispatchClientRoundTripper func(*http.Request) (*http.Response, error)

func (transport dispatchClientRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}
