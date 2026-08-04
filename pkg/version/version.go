// Package version exposes neko-cli build metadata and version reporting.
package version

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"context"
	"fmt"
	"strings"
	"time"

	clierrors "github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/pkg/git"
	"github.com/nekoman-hq/neko-cli/pkg/log"
)

var (
	// Version variables are set via ldflags during build
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
	BuiltBy = "unknown"
)

// Latest prints local build facts and the newest stable CLI release for the
// configured CLI repository. Discovery uses github.ResolveLatestCLIRelease, the
// same resolver `neko update` uses, so both commands report the same release.
// A discovery failure is reported verbatim; the installed version is never
// presented as the latest release.
func Latest(ctx context.Context, repoInfo *github.RepoInfo) error {
	displayCLIVersion()

	if repoInfo == nil {
		repoInfo = github.CLIRepository()
	}

	release, err := github.ResolveLatestCLIRelease(ctx, repoInfo)
	if err != nil {
		clierrors.Warning("Latest CLI Release Unavailable", err.Error())
		return nil
	}

	if release != nil {
		displayRelease(repoInfo, release)
	}
	return nil
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

func displayRelease(repoInfo *github.RepoInfo, release *github.SelectedCLIRelease) {
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
		log.ColorText(log.ColorBold, "Latest CLI Release"))
	fmt.Printf("%s\n", log.ColorText(log.ColorPurple, "│"))
	fmt.Printf("%s %s %s\n",
		log.ColorText(log.ColorPurple, "├─"),
		log.ColorText(log.ColorPurple, "\uF09B Repository:"),
		log.ColorText(log.ColorYellow, fmt.Sprintf("%s/%s", repoInfo.Owner, repoInfo.Repo)))

	// A release title and GitHub's "Latest" label never identify a CLI release.
	// Only the resolved stable vX.Y.Z tag does.
	versionStr := strings.TrimPrefix(release.Version, "v")
	if release.TagName != "" {
		versionStr = fmt.Sprintf("%s (%s)", versionStr, release.TagName)
	}
	fmt.Printf("%s %s %s\n",
		log.ColorText(log.ColorPurple, "├─"),
		log.ColorText(log.ColorPurple, "\uF02B Version:   "),
		log.ColorText(log.ColorGreen, versionStr))

	publishedStr := formattedDate
	if release.Author != "" {
		publishedStr = fmt.Sprintf("%s by %s", formattedDate,
			log.ColorText(log.ColorCyan, release.Author))
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
