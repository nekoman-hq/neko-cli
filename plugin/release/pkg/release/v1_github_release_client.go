package release

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	v1GitHubReleaseRequestTimeout = 15 * time.Second
	v1GitHubReleaseBodyLimit      = 64 * 1024
)

type v1GitHubHTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type v1GitHubRemoteURLReader interface {
	ReadOriginURL(string) (string, error)
}

type systemV1GitHubRemoteURLReader struct {
	runner v1GitCommandRunner
}

func (reader systemV1GitHubRemoteURLReader) ReadOriginURL(repositoryRoot string) (string, error) {
	output, err := reader.runner.CombinedOutput(repositoryRoot, "config", "--get", "remote.origin.url")
	if err != nil {
		return "", fmt.Errorf("resolve V1 origin remote: %s: %w", strings.TrimSpace(string(output)), err)
	}
	remoteURL := strings.TrimSpace(string(output))
	if remoteURL == "" {
		return "", fmt.Errorf("resolve V1 origin remote: URL is empty")
	}
	return remoteURL, nil
}

type boundedV1GitHubReleaseClient struct {
	http    v1GitHubHTTPClient
	remotes v1GitHubRemoteURLReader
}

func newBoundedV1GitHubReleaseClient() boundedV1GitHubReleaseClient {
	runner := newSystemV1GitCommandRunner()
	return boundedV1GitHubReleaseClient{
		http:    &http.Client{Timeout: v1GitHubReleaseRequestTimeout},
		remotes: systemV1GitHubRemoteURLReader{runner: runner},
	}
}

func (client boundedV1GitHubReleaseClient) Delete(repositoryRoot, tag string, token v1GitHubToken) error {
	remoteURL, err := client.remotes.ReadOriginURL(repositoryRoot)
	if err != nil {
		return err
	}
	target, err := ResolveGitHubRepositoryTarget("origin", remoteURL)
	if err != nil {
		return fmt.Errorf("resolve V1 GitHub repository: %w", err)
	}
	releaseID, found, err := client.findRelease(target, tag, token)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	deleteURL := fmt.Sprintf("%s/repos/%s/%s/releases/%d", target.APIBaseURL, url.PathEscape(target.Owner), url.PathEscape(target.Repository), releaseID)
	request, err := newAuthorizedV1GitHubRequest(http.MethodDelete, deleteURL, token)
	if err != nil {
		return err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return fmt.Errorf("delete V1 GitHub release: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if _, bodyErr := readBoundedV1GitHubBody(response.Body); bodyErr != nil {
		return bodyErr
	}
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete V1 GitHub release: unexpected status %d", response.StatusCode)
	}
	_, stillPresent, err := client.findRelease(target, tag, token)
	if err != nil {
		return fmt.Errorf("verify V1 GitHub release deletion: %w", err)
	}
	if stillPresent {
		return fmt.Errorf("verify V1 GitHub release deletion: release is still present")
	}
	return nil
}

func (client boundedV1GitHubReleaseClient) findRelease(target GitHubRepositoryTarget, tag string, token v1GitHubToken) (int64, bool, error) {
	releaseURL := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", target.APIBaseURL, url.PathEscape(target.Owner), url.PathEscape(target.Repository), url.PathEscape(tag))
	request, err := newAuthorizedV1GitHubRequest(http.MethodGet, releaseURL, token)
	if err != nil {
		return 0, false, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return 0, false, fmt.Errorf("look up V1 GitHub release: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := readBoundedV1GitHubBody(response.Body)
	if err != nil {
		return 0, false, err
	}
	if response.StatusCode == http.StatusNotFound {
		return 0, false, nil
	}
	if response.StatusCode != http.StatusOK {
		return 0, false, fmt.Errorf("look up V1 GitHub release: unexpected status %d", response.StatusCode)
	}
	var release struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return 0, false, fmt.Errorf("decode V1 GitHub release response: %w", err)
	}
	if release.ID <= 0 {
		return 0, false, fmt.Errorf("decode V1 GitHub release response: release ID is missing")
	}
	return release.ID, true, nil
}

func newAuthorizedV1GitHubRequest(method, requestURL string, token v1GitHubToken) (*http.Request, error) {
	request, err := http.NewRequest(method, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create V1 GitHub release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+token.value)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	return request, nil
}

func readBoundedV1GitHubBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, v1GitHubReleaseBodyLimit+1))
	if err != nil {
		return nil, fmt.Errorf("read V1 GitHub release response: %w", err)
	}
	if len(data) > v1GitHubReleaseBodyLimit {
		return nil, fmt.Errorf("read V1 GitHub release response: body exceeds %d bytes", v1GitHubReleaseBodyLimit)
	}
	return data, nil
}
