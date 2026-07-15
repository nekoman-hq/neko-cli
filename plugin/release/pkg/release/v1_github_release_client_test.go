package release

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestBoundedV1GitHubReleaseClientUsesExplicitRootAndVerifiesDeletion(t *testing.T) {
	const secret = "NEKO_V1_GITHUB_SECRET_MUST_NOT_APPEAR"
	remotes := &recordingV1GitHubRemoteReader{remoteURL: "git@github.com:acme/example.git"}
	httpClient := &sequenceV1GitHubHTTPClient{responses: []*http.Response{
		v1GitHubResponse(http.StatusOK, `{"id":42}`),
		v1GitHubResponse(http.StatusNoContent, ""),
		v1GitHubResponse(http.StatusNotFound, `{"message":"not found"}`),
	}}
	client := boundedV1GitHubReleaseClient{http: httpClient, remotes: remotes}

	if err := client.Delete("/explicit/repository", "v1.2.4", v1GitHubToken{value: secret}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if remotes.root != "/explicit/repository" {
		t.Fatalf("remote root = %q", remotes.root)
	}
	want := []string{
		"GET https://api.github.com/repos/acme/example/releases/tags/v1.2.4",
		"DELETE https://api.github.com/repos/acme/example/releases/42",
		"GET https://api.github.com/repos/acme/example/releases/tags/v1.2.4",
	}
	if got := strings.Join(httpClient.requests, "\n"); got != strings.Join(want, "\n") {
		t.Fatalf("requests:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
	for _, authorization := range httpClient.authorizations {
		if authorization != "Bearer "+secret {
			t.Fatalf("authorization header = %q", authorization)
		}
	}
}

func TestBoundedV1GitHubReleaseClientHasFiniteTimeout(t *testing.T) {
	client := newBoundedV1GitHubReleaseClient()
	httpClient, ok := client.http.(*http.Client)
	if !ok {
		t.Fatalf("HTTP client = %T", client.http)
	}
	if httpClient.Timeout != v1GitHubReleaseRequestTimeout || httpClient.Timeout <= 0 {
		t.Fatalf("HTTP timeout = %s", httpClient.Timeout)
	}
}

func TestBoundedV1GitHubReleaseClientBoundsBodiesAndDoesNotEchoSecrets(t *testing.T) {
	const secret = "NEKO_V1_HTTP_BODY_SECRET_MUST_NOT_APPEAR"
	tests := []struct {
		name     string
		response *http.Response
		want     string
	}{
		{
			name:     "unexpected status body",
			response: v1GitHubResponse(http.StatusInternalServerError, secret),
			want:     "unexpected status 500",
		},
		{
			name:     "oversized body",
			response: v1GitHubResponse(http.StatusOK, strings.Repeat("x", v1GitHubReleaseBodyLimit+1)+secret),
			want:     "body exceeds",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := boundedV1GitHubReleaseClient{
				http: &sequenceV1GitHubHTTPClient{responses: []*http.Response{tt.response}},
				remotes: &recordingV1GitHubRemoteReader{
					remoteURL: "https://github.com/acme/example.git",
				},
			}
			err := client.Delete("/repo", "v1.2.4", v1GitHubToken{value: secret})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Delete error = %v", err)
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("secret appeared in HTTP diagnostic: %v", err)
			}
		})
	}
}

type recordingV1GitHubRemoteReader struct {
	remoteURL string
	root      string
}

func (reader *recordingV1GitHubRemoteReader) ReadOriginURL(root string) (string, error) {
	reader.root = root
	return reader.remoteURL, nil
}

type sequenceV1GitHubHTTPClient struct {
	responses      []*http.Response
	requests       []string
	authorizations []string
}

func (client *sequenceV1GitHubHTTPClient) Do(request *http.Request) (*http.Response, error) {
	client.requests = append(client.requests, request.Method+" "+request.URL.String())
	client.authorizations = append(client.authorizations, request.Header.Get("Authorization"))
	if len(client.responses) == 0 {
		return nil, io.EOF
	}
	response := client.responses[0]
	client.responses = client.responses[1:]
	response.Request = request
	return response, nil
}

func v1GitHubResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}
