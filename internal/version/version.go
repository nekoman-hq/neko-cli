package version

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/repository"
)

var (
	// These variables are set via ldflags during build
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
	BuiltBy = "unknown"
)

func Latest(repoInfo *repository.RepoInfo, token string) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoInfo.Owner, repoInfo.Repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		errors.Fatal(
			"Request Creation Failed",
			fmt.Sprintf("Could not create API request: %v", err),
			errors.ErrAPIRequest,
		)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

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
		}
	}(resp.Body)

	if resp.StatusCode == 404 {
		errors.Fatal(
			"No Releases Found",
			fmt.Sprintf("Repository %s/%s has no releases yet.", repoInfo.Owner, repoInfo.Repo),
			errors.ErrNoReleases,
		)
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

	var release GithubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		errors.Fatal(
			"JSON Parse Failed",
			fmt.Sprintf("Could not parse API response: %v", err),
			errors.ErrAPIResponse,
		)
	}

	displayCLIVersion()
	displayRelease(repoInfo, &release)
}

func displayCLIVersion() {
	fmt.Println()
	fmt.Printf("┌─ neko-cli\n")
	fmt.Printf("│\n")
	fmt.Printf("├─ Version:   %s\n", Version)
	fmt.Printf("├─ Commit:    %s\n", Commit)
	fmt.Printf("├─ Built:     %s\n", Date)
	fmt.Printf("└─ Built by:  %s\n", BuiltBy)
	fmt.Println()
}

func displayRelease(repoInfo *repository.RepoInfo, release *GithubRelease) {
	// Parse and format the date
	publishedTime, err := time.Parse(time.RFC3339, release.PublishedAt)
	var formattedDate string
	if err == nil {
		formattedDate = publishedTime.Format("2006-01-02 15:04 MST")
	} else {
		formattedDate = release.PublishedAt
	}

	fmt.Println()
	fmt.Printf("┌─ Latest Release\n")
	fmt.Printf("│\n")
	fmt.Printf("├─ Repository: %s/%s\n", repoInfo.Owner, repoInfo.Repo)
	fmt.Printf("├─ Version:    %s", release.Name)
	if release.TagName != "" && release.TagName != release.Name {
		fmt.Printf(" (%s)", release.TagName)
	}
	fmt.Println()

	if release.PreRelease {
		fmt.Printf("├─ Type:       Pre-release\n")
	}

	fmt.Printf("├─ Published:  %s", formattedDate)
	if release.Author.Login != "" {
		fmt.Printf(" by %s", release.Author.Login)
	}
	fmt.Println()

	fmt.Printf("└─ URL:        %s\n", release.HTMLURL)
	fmt.Println()
}
