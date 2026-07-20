package githubdispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
)

const testSecret = "dispatch-secret-sentinel"

func TestClientPostsExactWorkflowDispatchRequest(t *testing.T) {
	var seenMethod, seenPath, seenBody string
	var seenHeaders http.Header
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		seenMethod = request.Method
		seenPath = request.URL.EscapedPath()
		seenHeaders = request.Header.Clone()
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		seenBody = string(body)
		return testHTTPResponse(http.StatusNoContent, ""), nil
	})
	client := newTestClient(t, WithTransport(transport))

	result, err := client.Post(context.Background(), testTarget("https://api.github.test"), Request{
		WorkflowFilename: " release-api.yml ",
		Ref:              "api/v0.2.1",
		Inputs: map[string]string{
			"unit": "api", "version": "0.2.1", "tag": "api/v0.2.1", "release_sha": strings.Repeat("a", 40),
		},
	}, newTestToken(t))
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if result.Outcome != OutcomeAccepted || result.HTTPStatus != http.StatusNoContent {
		t.Fatalf("result = %#v", result)
	}
	if seenMethod != http.MethodPost || seenPath != "/repos/neko%20man/repo.one/actions/workflows/release-api.yml/dispatches" {
		t.Fatalf("request = %s %s", seenMethod, seenPath)
	}
	if seenHeaders.Get("Authorization") != "Bearer "+testSecret ||
		seenHeaders.Get("Accept") != "application/vnd.github+json" ||
		seenHeaders.Get("Content-Type") != "application/json" ||
		seenHeaders.Get("X-GitHub-Api-Version") != "2026-03-10" ||
		seenHeaders.Get("User-Agent") != "neko-cli/test" {
		t.Fatalf("headers = %#v", seenHeaders)
	}
	if !strings.HasPrefix(seenBody, `{"ref":"api/v0.2.1","inputs":{`) {
		t.Fatalf("JSON field order changed: %s", seenBody)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(seenBody), &payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(payload) != 2 || payload["ref"] != "api/v0.2.1" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestClientClassifiesHTTPResponses(t *testing.T) {
	tests := []struct {
		name    string
		outcome Outcome
		status  int
	}{
		{name: "200", status: http.StatusOK, outcome: OutcomeAccepted},
		{name: "204", status: http.StatusNoContent, outcome: OutcomeAccepted},
		{name: "400", status: http.StatusBadRequest, outcome: OutcomeRejected},
		{name: "401", status: http.StatusUnauthorized, outcome: OutcomeRejected},
		{name: "403", status: http.StatusForbidden, outcome: OutcomeRejected},
		{name: "404", status: http.StatusNotFound, outcome: OutcomeRejected},
		{name: "422", status: http.StatusUnprocessableEntity, outcome: OutcomeRejected},
		{name: "429", status: http.StatusTooManyRequests, outcome: OutcomeRejected},
		{name: "302", status: http.StatusFound, outcome: OutcomeUnknown},
		{name: "418", status: http.StatusTeapot, outcome: OutcomeUnknown},
		{name: "500", status: http.StatusInternalServerError, outcome: OutcomeUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return testHTTPResponse(tt.status, `{"message":"rejected `+testSecret+`"}`), nil
			})
			result, err := newTestClient(t, WithTransport(transport)).Post(
				context.Background(), testTarget("https://api.github.test"), testRequest(), newTestToken(t),
			)
			if err != nil {
				t.Fatalf("Post: %v", err)
			}
			if result.Outcome != tt.outcome || result.HTTPStatus != tt.status {
				t.Fatalf("result = %#v, want %s", result, tt.outcome)
			}
			if strings.Contains(result.Error, testSecret) {
				t.Fatalf("token leaked in %q", result.Error)
			}
			if tt.status == http.StatusTooManyRequests && !strings.Contains(result.Error, "later explicit retry or resume decision") {
				t.Fatalf("429 guidance changed: %q", result.Error)
			}
		})
	}
}

func TestClientCapturesOptionalAcceptedMetadata(t *testing.T) {
	transport := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		response := testHTTPResponse(http.StatusOK, `{"id":42,"run_id":123,"url":"https://api.github.com/runs/123","html_url":"https://github.com/o/r/actions/runs/123","created_at":"2026-07-07T10:00:00Z"}`)
		response.Header.Set("Date", "Tue, 07 Jul 2026 09:00:00 GMT")
		return response, nil
	})
	result, err := newTestClient(t, WithTransport(transport)).Post(
		context.Background(), testTarget("https://api.github.test"), testRequest(), newTestToken(t),
	)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if result.Outcome != OutcomeAccepted || result.WorkflowRunID != "123" ||
		result.RunURL != "https://api.github.com/runs/123" ||
		result.HTMLURL != "https://github.com/o/r/actions/runs/123" ||
		result.ResponseTimestamp != "2026-07-07T10:00:00Z" {
		t.Fatalf("metadata = %#v", result)
	}
}

func TestClientDoesNotFollowRedirectsOrRetry(t *testing.T) {
	requests := 0
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.Path == "/elsewhere" {
			t.Fatal("redirect target must not be requested")
		}
		response := testHTTPResponse(http.StatusFound, "")
		response.Header.Set("Location", "/elsewhere")
		return response, nil
	})
	result, err := newTestClient(t, WithTransport(transport)).Post(
		context.Background(), testTarget("https://api.github.test"), testRequest(), newTestToken(t),
	)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if requests != 1 || result.Outcome != OutcomeUnknown {
		t.Fatalf("requests=%d result=%#v", requests, result)
	}
}

func TestClientClassifiesTimeoutAndSanitizesTransportText(t *testing.T) {
	requests := 0
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		<-request.Context().Done()
		return nil, fmt.Errorf("%s: %w", testSecret, request.Context().Err())
	})
	client := newTestClient(t, WithTransport(transport), WithTimeout(time.Millisecond))
	result, err := client.Post(context.Background(), testTarget("https://api.github.test"), testRequest(), newTestToken(t))
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if requests != 1 || result.Outcome != OutcomeUnknown || !strings.Contains(result.Error, "timed out") || strings.Contains(result.Error, testSecret) {
		t.Fatalf("requests=%d result=%#v", requests, result)
	}
}

func TestClientPreservesPreRequestValidationErrors(t *testing.T) {
	client := newTestClient(t)
	for _, filename := range []string{"", "../release.yml", `nested\\release.yml`} {
		request := testRequest()
		request.WorkflowFilename = filename
		if _, err := client.Post(context.Background(), testTarget("https://api.github.test"), request, newTestToken(t)); err == nil ||
			!strings.Contains(err.Error(), "workflow filename is invalid") {
			t.Fatalf("filename %q error = %v", filename, err)
		}
	}
	if _, err := NewClient(testProtocol(), WithAPIBaseURL("ftp://example.test")); err == nil ||
		!strings.Contains(err.Error(), "must be http or https") {
		t.Fatalf("base URL error = %v", err)
	}
	if _, err := NewClient(testProtocol(), WithTransport(nil)); err == nil ||
		!strings.Contains(err.Error(), "transport is missing") {
		t.Fatalf("transport error = %v", err)
	}
	if _, err := NewClient(testProtocol(), WithTimeout(0)); err == nil ||
		!strings.Contains(err.Error(), "timeout must be positive") {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestBearerTokenFormattingAndStoredTextRemainSecretFree(t *testing.T) {
	token := newTestToken(t)
	formatted := fmt.Sprintf("%s %v %+v %#v", token, token, token, token)
	if strings.Contains(formatted, testSecret) || !strings.Contains(formatted, "[redacted]") {
		t.Fatalf("token formatting = %q", formatted)
	}
	bounded := SanitizeText(testSecret+strings.Repeat("x", 1100), token)
	if strings.Contains(bounded, testSecret) || !strings.HasSuffix(bounded, "...[truncated]") {
		t.Fatalf("sanitized text = %q", bounded)
	}
}

func newTestClient(t *testing.T, options ...ClientOption) *Client {
	t.Helper()
	client, err := NewClient(testProtocol(), options...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func testProtocol() Protocol {
	return Protocol{APIVersion: "2026-03-10", UserAgent: "neko-cli/test"}
}

func newTestToken(t *testing.T) BearerToken {
	t.Helper()
	token, err := NewBearerToken(testSecret)
	if err != nil {
		t.Fatalf("NewBearerToken: %v", err)
	}
	return token
}

func testTarget(apiBaseURL string) releaseworkflow.GitHubRepositoryTarget {
	return releaseworkflow.GitHubRepositoryTarget{
		Owner: "neko man", Repository: "repo.one", APIBaseURL: apiBaseURL,
	}
}

func testRequest() Request {
	return Request{
		WorkflowFilename: "release-api.yml",
		Ref:              "api/v0.2.1",
		Inputs:           map[string]string{"unit": "api"},
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func testHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
