package github

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

var (
	ErrNoReleases    = stderrors.New("repository has no releases")
	githubAPIBase    = "https://api.github.com"
	githubHTTPClient = &http.Client{Timeout: 30 * time.Second}
)

const maxReleaseMetadataBytes int64 = 8 << 20

// Client owns bounded, context-aware GitHub release metadata and asset reads.
type Client struct {
	HTTPClient *http.Client
	APIBase    string
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{HTTPClient: httpClient, APIBase: APIBaseURL()}
}

// listReleases reads every published release page for a repository. GitHub
// paginates the release list, and the repository publishes CLI and plugin units
// into the same list, so a stable CLI tag can appear beyond the first page.
func listReleases(ctx context.Context, repoInfo *RepoInfo, apiBase string, httpClient *http.Client) ([]Release, error) {
	var releases []Release

	for page := 1; page <= maxReleasePages; page++ {
		url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=%d&page=%d", apiBase, repoInfo.Owner, repoInfo.Repo, releasesPerPage, page)

		log.PluginV(log.Exec, fmt.Sprintf("Fetching release list from remote: %s",
			log.ColorText(log.ColorGreen, url),
		))

		body, statusCode, err := doGitHubRequest(ctx, httpClient, url)
		if err != nil {
			if statusCode == http.StatusNotFound {
				return nil, repositoryAccessError(repoInfo)
			}
			return nil, err
		}

		var pageReleases []Release
		if err := json.Unmarshal(body, &pageReleases); err != nil {
			return nil, fmt.Errorf(
				"JSON Parse Failed: %w", err,
			)
		}

		releases = append(releases, pageReleases...)
		if len(pageReleases) < releasesPerPage {
			return releases, nil
		}
	}

	// The final page was full, so the release list was truncated. The greatest
	// tag seen so far is not provably the greatest tag published, so discovery
	// fails instead of selecting a possibly non-maximum CLI release.
	return nil, fmt.Errorf(
		"%w: %d release pages (%d releases) were read and more remain; pin a version instead of resolving the newest release",
		ErrReleaseListTruncated,
		maxReleasePages,
		maxReleasePages*releasesPerPage,
	)
}

func doGitHubRequest(ctx context.Context, httpClient *http.Client, url string) ([]byte, int, error) {
	req, err := newGitHubRequestWithContext(ctx, url)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"request Creation Failed: %w", err,
		)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"API Request Failed: %w", err,
		)
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReleaseMetadataBytes+1))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf(
			"response Read Failed: %w", err,
		)
	}
	if int64(len(body)) > maxReleaseMetadataBytes {
		return nil, resp.StatusCode, fmt.Errorf("GitHub API response exceeds %d bytes", maxReleaseMetadataBytes)
	}

	return body, resp.StatusCode, nil
}

func newGitHubRequest(url string) (*http.Request, error) {
	return newGitHubRequestWithContext(context.Background(), url)
}

func newGitHubRequestWithContext(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	return req, nil
}

// Download reads a release asset through the configured client and rejects
// responses larger than maxBytes. Callers own integrity verification.
func (client *Client) Download(ctx context.Context, assetURL string, maxBytes int64) ([]byte, error) {
	if client == nil {
		client = NewClient(nil)
	}
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create asset request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download release asset %s: %w", safeRequestURL(req), err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download release asset %s: HTTP %d", safeRequestURL(req), resp.StatusCode)
	}
	if maxBytes <= 0 {
		return nil, fmt.Errorf("download release asset %s: invalid size limit", safeRequestURL(req))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read release asset %s: %w", safeRequestURL(req), err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("download release asset %s exceeds %d bytes", safeRequestURL(req), maxBytes)
	}
	return body, nil
}

func safeRequestURL(req *http.Request) string {
	if req == nil || req.URL == nil {
		return "<unknown>"
	}
	return req.URL.Scheme + "://" + req.URL.Host + req.URL.EscapedPath()
}

func repositoryAccessError(repoInfo *RepoInfo) error {
	if os.Getenv("GITHUB_TOKEN") == "" {
		return fmt.Errorf(
			"unable to access releases for %s/%s. If this is a private repository, set GITHUB_TOKEN",
			repoInfo.Owner,
			repoInfo.Repo,
		)
	}

	return fmt.Errorf(
		"unable to access releases for %s/%s",
		repoInfo.Owner,
		repoInfo.Repo,
	)
}
