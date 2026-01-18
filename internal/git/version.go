// Package git includes operations using git or git-cli
package git

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/git/github"
	"github.com/nekoman-hq/neko-cli/internal/log"
)

func LatestRelease(repoInfo *RepoInfo) (*github.Release, error) {
	token, err := config.GetPAT()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoInfo.Owner, repoInfo.Repo)

	log.V(log.Release, fmt.Sprintf("Fetching latest release from remote: %s",
		log.ColorText(log.ColorGreen, url),
	))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf(
			"request Creation Failed: %w", err,
		)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"API Request Failed: %w", err,
		)
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			// Error log not needed normally
			return
		}
	}(resp.Body)

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("repository %s/%s has no releases yet", repoInfo.Owner, repoInfo.Repo)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"GitHub API returned status %d: %s", resp.StatusCode, string(body),
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"response Read Failed: %w", err,
		)
	}

	var release github.Release
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf(
			"JSON Parse Failed: %w", err,
		)
	}

	log.V(log.Release, "\uF00C Successfully received release information from remote!")
	return &release, nil
}
