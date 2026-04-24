package github

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

var (
	ErrNoReleases    = stderrors.New("repository has no releases")
	githubAPIBase    = "https://api.github.com"
	githubHTTPClient = http.DefaultClient
)

func LatestRelease(repoInfo *RepoInfo) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", githubAPIBase, repoInfo.Owner, repoInfo.Repo)

	log.PluginV(log.Exec, fmt.Sprintf("Fetching latest release from remote: %s",
		log.ColorText(log.ColorGreen, url),
	))

	body, statusCode, err := doGitHubRequest(url)
	if err == nil {
		var release Release
		if unmarshalErr := json.Unmarshal(body, &release); unmarshalErr != nil {
			return nil, fmt.Errorf(
				"JSON Parse Failed: %w", unmarshalErr,
			)
		}

		log.PluginV(log.Exec, "\uF00C Successfully received release information from remote!")
		return &release, nil
	}

	if statusCode != http.StatusNotFound {
		return nil, err
	}

	log.PluginV(log.Exec, "Latest release endpoint returned 404, falling back to release list")

	releases, err := listReleases(repoInfo)
	if err != nil {
		return nil, err
	}

	for i := range releases {
		if releases[i].Draft {
			continue
		}

		log.PluginV(log.Exec, "\uF00C Successfully received release information from release list fallback!")
		return &releases[i], nil
	}

	return nil, fmt.Errorf(
		"%w: repository %s/%s has no releases yet",
		ErrNoReleases,
		repoInfo.Owner,
		repoInfo.Repo,
	)
}

func listReleases(repoInfo *RepoInfo) ([]Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=10", githubAPIBase, repoInfo.Owner, repoInfo.Repo)

	log.PluginV(log.Exec, fmt.Sprintf("Fetching release list from remote: %s",
		log.ColorText(log.ColorGreen, url),
	))

	body, statusCode, err := doGitHubRequest(url)
	if err != nil {
		if statusCode == http.StatusNotFound {
			return nil, repositoryAccessError(repoInfo)
		}
		return nil, err
	}

	var releases []Release
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf(
			"JSON Parse Failed: %w", err,
		)
	}

	return releases, nil
}

func doGitHubRequest(url string) ([]byte, int, error) {
	req, err := newGitHubRequest(url)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"request Creation Failed: %w", err,
		)
	}

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf(
			"API Request Failed: %w", err,
		)
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf(
			"response Read Failed: %w", err,
		)
	}

	if resp.StatusCode != http.StatusOK {
		return body, resp.StatusCode, fmt.Errorf(
			"GitHub API returned status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	return body, resp.StatusCode, nil
}

func newGitHubRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	return req, nil
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
