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

func TestLatestReleaseSelectsLatestStableCLITagFromReleaseList(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "secret-token")

	withGitHubTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Authorization"); got != "token secret-token" {
			t.Fatalf("expected authorization header, got %q", got)
		}

		switch req.URL.Path {
		case "/repos/nekoman-hq/neko-web/releases":
			if req.URL.RawQuery != "per_page=100" {
				t.Fatalf("expected per_page=100 query, got %q", req.URL.RawQuery)
			}
			releases := []Release{
				{Name: "draft", TagName: "draft", Draft: true},
				{Name: "plugin release", TagName: "plugin-release/v4.0.4", HTMLURL: "https://example.com/releases/plugin-release/v4.0.4"},
				{Name: "plugin ui", TagName: "plugin-ui/v1.0.1", HTMLURL: "https://example.com/releases/plugin-ui/v1.0.1"},
				{Name: "registry", TagName: "plugin-registry", HTMLURL: "https://example.com/releases/plugin-registry"},
				{Name: "invalid", TagName: "3.0.11", HTMLURL: "https://example.com/releases/3.0.11"},
				{Name: "prerelease", TagName: "v3.0.11", PreRelease: true, HTMLURL: "https://example.com/releases/v3.0.11"},
				{Name: "Nekocli 3.0.4", TagName: "v3.0.4", HTMLURL: "https://example.com/releases/v3.0.4"},
				{Name: "Nekocli 3.0.10", TagName: "v3.0.10", HTMLURL: "https://example.com/releases/v3.0.10"},
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

	if release.TagName != "v3.0.10" {
		t.Fatalf("expected latest CLI release tag v3.0.10, got %q", release.TagName)
	}
}

func TestLatestReleaseReturnsNoReleasesWhenReleaseListIsEmpty(t *testing.T) {
	withGitHubTestClient(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
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

	if !strings.Contains(err.Error(), "has no stable CLI releases matching vX.Y.Z") {
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

func TestLatestStableCLIReleaseIgnoresNonCLITags(t *testing.T) {
	release, err := LatestStableCLIRelease([]Release{
		{Name: "plugin release", TagName: "plugin-release/v4.0.4"},
		{Name: "plugin ui", TagName: "plugin-ui/v1.0.1"},
		{Name: "registry", TagName: "plugin-registry"},
		{Name: "invalid", TagName: "v3.0"},
		{Name: "draft cli", TagName: "v3.0.12", Draft: true},
		{Name: "prerelease cli", TagName: "v3.0.11", PreRelease: true},
		{Name: "older cli", TagName: "v3.0.4"},
		{Name: "newer cli", TagName: "v3.0.10"},
	})
	if err != nil {
		t.Fatalf("LatestStableCLIRelease returned error: %v", err)
	}
	if release.TagName != "v3.0.10" {
		t.Fatalf("LatestStableCLIRelease selected %q, want v3.0.10", release.TagName)
	}
}

func TestLatestStableCLIReleaseErrorsWhenNoCLITagExists(t *testing.T) {
	_, err := LatestStableCLIRelease([]Release{
		{Name: "plugin release", TagName: "plugin-release/v4.0.4"},
		{Name: "plugin ui", TagName: "plugin-ui/v1.0.1"},
		{Name: "registry", TagName: "plugin-registry"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !stderrors.Is(err, ErrNoReleases) {
		t.Fatalf("expected ErrNoReleases, got %v", err)
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
