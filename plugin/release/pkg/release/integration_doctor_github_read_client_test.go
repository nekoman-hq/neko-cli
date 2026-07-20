package release

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestIntegrationDoctorGitHubReadClientUsesOnlyExactGETOperations(t *testing.T) {
	identity := integrationDoctorRepositoryIdentity{Owner: "acme", Repository: "example"}
	workflow := ".github/workflows/release.yml"
	workflowBytes := []byte("name: release\n")
	var requests []string
	var methods []string
	var authorizations []string
	var lock sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		lock.Lock()
		requests = append(requests, request.URL.RequestURI())
		methods = append(methods, request.Method)
		authorizations = append(authorizations, request.Header.Get("Authorization"))
		lock.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/repos/acme/example":
			fmt.Fprint(writer, `{"name":"example","owner":{"login":"acme"},"default_branch":"main","visibility":"private","private":true}`)
		case "/repos/acme/example/contents/.github/workflows/release.yml":
			fmt.Fprintf(writer, `{"type":"file","path":%q,"encoding":"base64","content":%q}`, workflow, base64.StdEncoding.EncodeToString(workflowBytes))
		case "/repos/acme/example/actions/workflows/release.yml":
			fmt.Fprintf(writer, `{"path":%q,"state":"active"}`, workflow)
		case "/repos/acme/example/actions/variables/NEKO_VERSION":
			fmt.Fprint(writer, `{"name":"NEKO_VERSION","value":"3.0.4"}`)
		case "/repos/acme/example/actions/secrets/SIGNING_KEY":
			fmt.Fprint(writer, `{"name":"SIGNING_KEY","created_at":"2026-07-01T00:00:00Z","updated_at":"2026-07-02T00:00:00Z","value":"must-never-enter-model"}`)
		case "/repos/acme/example/actions/permissions":
			fmt.Fprint(writer, `{"enabled":true,"allowed_actions":"selected"}`)
		case "/repos/acme/example/releases/tags/plugin-release/v4.2.0":
			fmt.Fprint(writer, `{"tag_name":"plugin-release/v4.2.0","draft":false,"prerelease":false,"assets":[{"name":"plugin-release_4.2.0_Darwin_arm64.tar.gz"}]}`)
		case "/repos/acme/example/git/ref/tags/plugin-release/v4.2.0":
			fmt.Fprint(writer, `{"ref":"refs/tags/plugin-release/v4.2.0","object":{"type":"commit","sha":"abc123"}}`)
		case "/repos/acme/example/actions/runs/42":
			fmt.Fprint(writer, `{"id":42,"workflow_id":7,"status":"completed","conclusion":"success"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := newIntegrationDoctorGitHubReadClientForTest(t, server.URL)
	token, err := NewGitHubActionsDispatchToken("read-client-secret")
	if err != nil {
		t.Fatal(err)
	}

	repository, outcome := client.Repository(context.Background(), identity, token)
	if outcome.State != integrationDoctorVerified || repository.DefaultBranch != "main" || !repository.Private {
		t.Fatalf("repository=%#v outcome=%#v", repository, outcome)
	}
	content, outcome := client.WorkflowContent(context.Background(), identity, workflow, "main", token)
	if outcome.State != integrationDoctorVerified || !reflect.DeepEqual(content.Content, workflowBytes) {
		t.Fatalf("content=%#v outcome=%#v", content, outcome)
	}
	metadata, outcome := client.WorkflowMetadata(context.Background(), identity, workflow, token)
	if outcome.State != integrationDoctorVerified || metadata.State != "active" {
		t.Fatalf("metadata=%#v outcome=%#v", metadata, outcome)
	}
	variable, outcome := client.RepositoryVariable(context.Background(), identity, "NEKO_VERSION", token)
	if outcome.State != integrationDoctorVerified || variable.Value != "3.0.4" {
		t.Fatalf("variable=%#v outcome=%#v", variable, outcome)
	}
	secret, outcome := client.RepositorySecret(context.Background(), identity, "SIGNING_KEY", token)
	if outcome.State != integrationDoctorVerified || secret.Name != "SIGNING_KEY" || secret.UpdatedAt == "" {
		t.Fatalf("secret=%#v outcome=%#v", secret, outcome)
	}
	if strings.Contains(fmt.Sprintf("%#v", secret), "must-never-enter-model") {
		t.Fatal("secret value entered metadata model")
	}
	policy, outcome := client.ActionsPolicy(context.Background(), identity, token)
	if outcome.State != integrationDoctorVerified || !policy.Enabled {
		t.Fatalf("policy=%#v outcome=%#v", policy, outcome)
	}
	release, outcome := client.ReleaseByTag(context.Background(), identity, "plugin-release/v4.2.0", token)
	if outcome.State != integrationDoctorVerified || release.TagName != "plugin-release/v4.2.0" || len(release.Assets) != 1 {
		t.Fatalf("release=%#v outcome=%#v", release, outcome)
	}
	reference, outcome := client.TagReference(context.Background(), identity, "plugin-release/v4.2.0", token)
	if outcome.State != integrationDoctorVerified || reference.ObjectSHA != "abc123" {
		t.Fatalf("reference=%#v outcome=%#v", reference, outcome)
	}
	run, outcome := client.WorkflowRun(context.Background(), identity, "42", token)
	if outcome.State != integrationDoctorVerified || run.ID != "42" || run.Conclusion != "success" {
		t.Fatalf("run=%#v outcome=%#v", run, outcome)
	}

	for _, method := range methods {
		if method != http.MethodGet {
			t.Fatalf("HTTP method = %q, want GET", method)
		}
	}
	for _, authorization := range authorizations {
		if authorization != "Bearer read-client-secret" {
			t.Fatalf("authorization = %q", authorization)
		}
	}
	wantRequests := []string{
		"/repos/acme/example",
		"/repos/acme/example/contents/.github/workflows/release.yml?ref=main",
		"/repos/acme/example/actions/workflows/release.yml",
		"/repos/acme/example/actions/variables/NEKO_VERSION",
		"/repos/acme/example/actions/secrets/SIGNING_KEY",
		"/repos/acme/example/actions/permissions",
		"/repos/acme/example/releases/tags/plugin-release%2Fv4.2.0",
		"/repos/acme/example/git/ref/tags/plugin-release%2Fv4.2.0",
		"/repos/acme/example/actions/runs/42",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("requests = %#v, want %#v", requests, wantRequests)
	}
	for _, request := range requests {
		if strings.Contains(request, "/releases/latest") || strings.Contains(request, "per_page") {
			t.Fatalf("non-exact discovery request emitted: %s", request)
		}
	}
}

func TestIntegrationDoctorGitHubReadClientClassifiesFailuresWithoutRetryOrBodyLeak(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		headers   map[string]string
		wantState integrationDoctorVerificationState
	}{
		{name: "missing", status: http.StatusNotFound, wantState: integrationDoctorMissing},
		{name: "unauthorized", status: http.StatusUnauthorized, wantState: integrationDoctorUnauthorized},
		{name: "forbidden", status: http.StatusForbidden, wantState: integrationDoctorUnauthorized},
		{name: "primary rate limit", status: http.StatusForbidden, headers: map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "1784505600"}, wantState: integrationDoctorRateLimited},
		{name: "secondary rate limit", status: http.StatusTooManyRequests, headers: map[string]string{"Retry-After": "30"}, wantState: integrationDoctorRateLimited},
		{name: "server unavailable", status: http.StatusBadGateway, wantState: integrationDoctorUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				requests++
				for name, value := range test.headers {
					writer.Header().Set(name, value)
				}
				writer.WriteHeader(test.status)
				fmt.Fprint(writer, `{"message":"private-body-read-client-secret"}`)
			}))
			defer server.Close()
			client := newIntegrationDoctorGitHubReadClientForTest(t, server.URL)
			token, _ := NewGitHubActionsDispatchToken("read-client-secret")
			_, outcome := client.Repository(context.Background(), integrationDoctorRepositoryIdentity{Owner: "acme", Repository: "example"}, token)
			if outcome.State != test.wantState || requests != 1 {
				t.Fatalf("outcome=%#v requests=%d", outcome, requests)
			}
			serialized := fmt.Sprintf("%#v", outcome)
			for _, forbidden := range []string{"private-body", "read-client-secret", "Authorization", "Bearer"} {
				if strings.Contains(serialized, forbidden) {
					t.Fatalf("outcome contains %q: %s", forbidden, serialized)
				}
			}
			if test.wantState == integrationDoctorRateLimited && outcome.RetryAfter == "" && outcome.RateLimitReset == "" {
				t.Fatalf("rate-limit metadata missing: %#v", outcome)
			}
		})
	}
}

func TestIntegrationDoctorGitHubReadClientBoundsBodiesAndHonorsCancellation(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.URL.Query().Get("slow") == "true" {
			<-request.Context().Done()
			return
		}
		fmt.Fprint(writer, strings.Repeat("x", integrationDoctorGitHubReadBodyLimit+1))
	}))
	defer server.Close()
	client := newIntegrationDoctorGitHubReadClientForTest(t, server.URL)
	_, outcome := client.Repository(context.Background(), integrationDoctorRepositoryIdentity{Owner: "acme", Repository: "example"}, GitHubActionsDispatchToken{})
	if outcome.State != integrationDoctorUnavailable || requests != 1 {
		t.Fatalf("bounded outcome=%#v requests=%d", outcome, requests)
	}

	transport := integrationDoctorRoundTripper(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	client, err := newIntegrationDoctorGitHubReadClient(
		withIntegrationDoctorGitHubReadTransport(transport),
		withIntegrationDoctorGitHubReadTimeout(10*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, outcome = client.Repository(context.Background(), integrationDoctorRepositoryIdentity{Owner: "acme", Repository: "example"}, GitHubActionsDispatchToken{})
	if outcome.State != integrationDoctorUnavailable {
		t.Fatalf("timeout outcome=%#v", outcome)
	}
}

func TestIntegrationDoctorGitHubReadClientRejectsMalformedSuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(writer, `{"name":"example","owner":{"login":"acme"}}`)
	}))
	defer server.Close()
	client := newIntegrationDoctorGitHubReadClientForTest(t, server.URL)
	_, outcome := client.Repository(context.Background(), integrationDoctorRepositoryIdentity{Owner: "acme", Repository: "example"}, GitHubActionsDispatchToken{})
	if outcome.State != integrationDoctorUnavailable {
		t.Fatalf("outcome=%#v", outcome)
	}
}

type integrationDoctorRoundTripper func(*http.Request) (*http.Response, error)

func (transport integrationDoctorRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func newIntegrationDoctorGitHubReadClientForTest(
	t *testing.T,
	baseURL string,
) *integrationDoctorGitHubReadClient {
	t.Helper()
	client, err := newIntegrationDoctorGitHubReadClient(withIntegrationDoctorGitHubReadBaseURL(baseURL))
	if err != nil {
		t.Fatalf("new read client: %v", err)
	}
	return client
}
