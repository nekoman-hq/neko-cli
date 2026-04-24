package github

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNewGitHubRequestAddsOptionalToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "secret-token")

	req, err := newGitHubRequest("https://example.com")
	if err != nil {
		t.Fatalf("newGitHubRequest: %v", err)
	}

	if got := req.Header.Get("Authorization"); got != "token secret-token" {
		t.Fatalf("expected authorization header, got %q", got)
	}

	if got := req.Header.Get("Accept"); got != "application/vnd.github.v3+json" {
		t.Fatalf("expected GitHub accept header, got %q", got)
	}
}

func TestLatestReleaseFallsBackToReleaseList(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "secret-token")

	withGitHubTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Authorization"); got != "token secret-token" {
			t.Fatalf("expected authorization header, got %q", got)
		}

		switch req.URL.Path {
		case "/repos/nekoman-hq/neko-web/releases/latest":
			return jsonResponse(t, http.StatusNotFound, map[string]string{"message": "Not Found"}), nil
		case "/repos/nekoman-hq/neko-web/releases":
			releases := []Release{
				{Name: "draft", TagName: "draft", Draft: true},
				{Name: "trainity-web@1.0.6", TagName: "1.0.6", HTMLURL: "https://example.com/releases/1.0.6", PreRelease: true},
			}
			return jsonResponse(t, http.StatusOK, releases), nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	}))

	release, err := LatestRelease(&RepoInfo{
		Owner: "nekoman-hq",
		Repo:  "neko-web",
	})
	if err != nil {
		t.Fatalf("LatestRelease: %v", err)
	}

	if release == nil {
		t.Fatal("expected release")
	}

	if release.TagName != "1.0.6" {
		t.Fatalf("expected fallback release tag 1.0.6, got %q", release.TagName)
	}
}

func TestLatestReleaseReturnsNoReleasesWhenReleaseListIsEmpty(t *testing.T) {
	withGitHubTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/repos/nekoman-hq/empty/releases/latest":
			return jsonResponse(t, http.StatusNotFound, map[string]string{"message": "Not Found"}), nil
		case "/repos/nekoman-hq/empty/releases":
			return jsonResponse(t, http.StatusOK, []Release{}), nil
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	}))

	_, err := LatestRelease(&RepoInfo{
		Owner: "nekoman-hq",
		Repo:  "empty",
	})
	if err == nil {
		t.Fatal("expected no releases error")
	}

	if !stderrors.Is(err, ErrNoReleases) {
		t.Fatalf("expected ErrNoReleases, got %v", err)
	}

	if !strings.Contains(err.Error(), "has no releases yet") {
		t.Fatalf("expected no releases message, got %q", err.Error())
	}
}

func TestLatestReleaseReturnsRepositoryAccessHintForPrivateReposWithoutToken(t *testing.T) {
	withGitHubTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(t, http.StatusNotFound, map[string]string{"message": "Not Found"}), nil
	}))

	_, err := LatestRelease(&RepoInfo{
		Owner: "nekoman-hq",
		Repo:  "private-repo",
	})
	if err == nil {
		t.Fatal("expected repository access error")
	}

	if !strings.Contains(err.Error(), "set GITHUB_TOKEN") {
		t.Fatalf("expected GITHUB_TOKEN hint, got %q", err.Error())
	}
}

func withGitHubTestClient(t *testing.T, transport http.RoundTripper) {
	t.Helper()

	previousBaseURL := githubAPIBase
	previousClient := githubHTTPClient

	githubAPIBase = "https://api.github.com"
	githubHTTPClient = &http.Client{Transport: transport}

	t.Cleanup(func() {
		githubAPIBase = previousBaseURL
		githubHTTPClient = previousClient
	})
}

func jsonResponse(t *testing.T, statusCode int, payload any) *http.Response {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal response payload: %v", err)
	}

	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
