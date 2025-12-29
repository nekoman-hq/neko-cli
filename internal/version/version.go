// Package version includes build and status information of neko cli and the current Repository
package version

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"fmt"
	"time"

	"github.com/nekoman-hq/neko-cli/internal/git"
	"github.com/nekoman-hq/neko-cli/internal/git/github"
	"github.com/nekoman-hq/neko-cli/internal/log"
)

var (
	// These variables are set via ldflags during build
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
	BuiltBy = "unknown"
)

func Latest(repoInfo *git.RepoInfo) {
	release := git.LatestRelease(repoInfo)
	displayCLIVersion()

	if release != nil {
		displayRelease(repoInfo, release)
	}
}

func displayCLIVersion() {
	fmt.Println()
	fmt.Printf("%s %s\n",
		log.ColorText(log.ColorCyan, "┌─"),
		log.ColorText(log.ColorBold, "neko-cli"))
	fmt.Printf("%s\n", log.ColorText(log.ColorCyan, "│"))
	fmt.Printf("%s %s %s\n",
		log.ColorText(log.ColorCyan, "├─"),
		log.ColorText(log.ColorCyan, "\uF02B Version:  "),
		log.ColorText(log.ColorGreen, Version))
	fmt.Printf("%s %s %s\n",
		log.ColorText(log.ColorCyan, "├─"),
		log.ColorText(log.ColorCyan, "\uF1D3 Commit:   "),
		log.ColorText(log.ColorYellow, Commit))
	fmt.Printf("%s %s %s\n",
		log.ColorText(log.ColorCyan, "├─"),
		log.ColorText(log.ColorCyan, "\uF133 Built:    "),
		log.ColorText(log.ColorYellow, Date))
	fmt.Printf("%s %s %s\n",
		log.ColorText(log.ColorCyan, "└─"),
		log.ColorText(log.ColorCyan, "\uF007 Built by: "),
		log.ColorText(log.ColorYellow, BuiltBy))
	fmt.Println()
}

func displayRelease(repoInfo *git.RepoInfo, release *github.Release) {
	// Parse and format the date
	publishedTime, err := time.Parse(time.RFC3339, release.PublishedAt)
	var formattedDate string
	if err == nil {
		formattedDate = publishedTime.Format("2006-01-02 15:04 MST")
	} else {
		formattedDate = release.PublishedAt
	}

	fmt.Println()
	fmt.Printf("%s %s\n",
		log.ColorText(log.ColorPurple, "┌─"),
		log.ColorText(log.ColorBold, "Latest Release"))
	fmt.Printf("%s\n", log.ColorText(log.ColorPurple, "│"))
	fmt.Printf("%s %s %s\n",
		log.ColorText(log.ColorPurple, "├─"),
		log.ColorText(log.ColorPurple, "\uF09B Repository:"),
		log.ColorText(log.ColorYellow, fmt.Sprintf("%s/%s", repoInfo.Owner, repoInfo.Repo)))

	versionStr := release.Name
	if release.TagName != "" && release.TagName != release.Name {
		versionStr = fmt.Sprintf("%s (%s)", release.Name, release.TagName)
	}
	fmt.Printf("%s %s %s\n",
		log.ColorText(log.ColorPurple, "├─"),
		log.ColorText(log.ColorPurple, "\uF02B Version:   "),
		log.ColorText(log.ColorGreen, versionStr))

	if release.PreRelease {
		fmt.Printf("%s %s %s\n",
			log.ColorText(log.ColorPurple, "├─"),
			log.ColorText(log.ColorPurple, "\uF12A Type:      "),
			log.ColorText(log.ColorYellow, "Pre-release"))
	}

	publishedStr := formattedDate
	if release.Author.Login != "" {
		publishedStr = fmt.Sprintf("%s by %s", formattedDate,
			log.ColorText(log.ColorCyan, release.Author.Login))
	}
	fmt.Printf("%s %s %s\n",
		log.ColorText(log.ColorPurple, "├─"),
		log.ColorText(log.ColorPurple, "\uF133 Published: "),
		publishedStr)

	fmt.Printf("%s %s %s\n",
		log.ColorText(log.ColorPurple, "└─"),
		log.ColorText(log.ColorPurple, "\uF0C1 URL:       "),
		log.ColorText(log.ColorBlue, release.HTMLURL))
	fmt.Println()
}
