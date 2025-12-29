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
	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/git/github"
	"github.com/nekoman-hq/neko-cli/internal/log"
)

func LatestRelease(repoInfo *RepoInfo) *github.Release {
	token := config.GetPAT()
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoInfo.Owner, repoInfo.Repo)

	log.V(log.Release, fmt.Sprintf("Fetching latest release from remote: %s",
		log.ColorText(log.ColorGreen, url),
	))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		errors.Fatal(
			"Request Creation Failed",
			fmt.Sprintf("Could not create API request: %v", err),
			errors.ErrAPIRequest,
		)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		errors.Fatal(
			"API Request Failed",
			fmt.Sprintf("Could not fetch latest release: %v", err),
			errors.ErrAPIRequest,
		)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			// Error log not needed normally
			return
		}
	}(resp.Body)

	if resp.StatusCode == 404 {
		errors.Warning(
			"No Releases Found",
			fmt.Sprintf("Repository %s/%s has no releases yet.", repoInfo.Owner, repoInfo.Repo),
		)
		return nil
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		errors.Fatal(
			"API Error",
			fmt.Sprintf("GitHub API returned status %d: %s", resp.StatusCode, string(body)),
			errors.ErrAPIResponse,
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		errors.Fatal(
			"Response Read Failed",
			fmt.Sprintf("Could not read API response: %v", err),
			errors.ErrAPIResponse,
		)
	}

	var release github.Release
	if err := json.Unmarshal(body, &release); err != nil {
		errors.Fatal(
			"JSON Parse Failed",
			fmt.Sprintf("Could not parse API response: %v", err),
			errors.ErrAPIResponse,
		)
	}

	log.V(log.Release, "\uF00C Successfully received release information from remote!")
	return &release
}
