package github

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      04.08.2026
*/

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"golang.org/x/mod/semver"
)

// The CLI, the Release Plugin, the UI Plugin, and the mutable plugin registry
// all publish into the same GitHub release list. Only a tag matching exactly
// vX.Y.Z identifies a CLI release.
var stableCLITagRE = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

const (
	defaultCLIRepositoryOwner = "nekoman-hq"
	defaultCLIRepositoryName  = "neko-cli"
	releasesPerPage           = 100
	// maxReleasePages bounds pagination so a pathological repository cannot make
	// discovery unbounded. It covers 2000 releases.
	maxReleasePages = 20
)

// ErrCLIReleaseDiscovery marks every failure to determine the newest stable CLI
// release. Callers must surface it; they must never substitute the installed
// version for a failed discovery.
var ErrCLIReleaseDiscovery = stderrors.New("unable to determine the latest CLI release")

// ErrReleaseListTruncated reports that the release list did not end within the
// pagination limit. Selection is refused because an unread page may hold a
// greater stable CLI tag.
var ErrReleaseListTruncated = stderrors.New("release list exceeds the pagination limit")

// SelectedCLIRelease is the single typed CLI release that `neko version` and
// `neko update` both resolve. It keeps the selected release metadata and its
// assets together so no caller re-queries GitHub to complete the selection.
type SelectedCLIRelease struct {
	// Version is the normalized semantic version of TagName, for example v3.1.2.
	Version string
	// TagName is the original GitHub tag the release was published under.
	TagName     string
	HTMLURL     string
	PublishedAt string
	Author      string
	Assets      []Asset
}

// APIBaseURL reports the GitHub API base used for CLI release discovery.
// NEKO_GITHUB_API_BASE overrides it, the same override the install script
// accepts, so the installer and the built-in updater can be pointed at one
// controlled endpoint together.
func APIBaseURL() string {
	if configured := strings.TrimSpace(os.Getenv("NEKO_GITHUB_API_BASE")); configured != "" {
		return strings.TrimRight(configured, "/")
	}
	return githubAPIBase
}

// CLIRepository reports the configured CLI release repository. NEKO_REPOSITORY
// overrides the default in the same owner/repo form the install script accepts,
// which keeps the installer and the built-in updater on one repository.
func CLIRepository() *RepoInfo {
	configured := strings.TrimSpace(os.Getenv("NEKO_REPOSITORY"))
	if owner, repo, ok := strings.Cut(configured, "/"); ok {
		owner = strings.TrimSpace(owner)
		repo = strings.TrimSpace(repo)
		if owner != "" && repo != "" && !strings.Contains(repo, "/") {
			return &RepoInfo{Owner: owner, Repo: repo}
		}
	}
	return &RepoInfo{Owner: defaultCLIRepositoryOwner, Repo: defaultCLIRepositoryName}
}

// SelectStableCLIRelease picks the greatest stable CLI release from a release
// list. Drafts, prereleases, plugin unit tags, the plugin-registry tag, and
// malformed tags are ignored, and API ordering is not trusted. Release titles
// and GitHub's "Latest" label are never consulted.
func SelectStableCLIRelease(releases []Release) (*SelectedCLIRelease, error) {
	var selected *Release

	for index := range releases {
		release := &releases[index]
		if release.Draft || release.PreRelease || !isStableCLITag(release.TagName) {
			continue
		}
		if selected == nil || semver.Compare(release.TagName, selected.TagName) > 0 {
			selected = release
		}
	}

	if selected == nil {
		return nil, ErrNoReleases
	}

	return &SelectedCLIRelease{
		Version:     semver.Canonical(selected.TagName),
		TagName:     selected.TagName,
		HTMLURL:     selected.HTMLURL,
		PublishedAt: selected.PublishedAt,
		Author:      selected.Author.Login,
		Assets:      selected.Assets,
	}, nil
}

// ResolveLatestCLIRelease resolves the newest stable CLI release through the
// package default client.
func ResolveLatestCLIRelease(ctx context.Context, repoInfo *RepoInfo) (*SelectedCLIRelease, error) {
	return resolveLatestCLIRelease(ctx, repoInfo, APIBaseURL(), githubHTTPClient)
}

// ResolveLatestCLIRelease resolves the newest stable CLI release through this
// client. `neko update` and `neko version` share this one resolver.
func (client *Client) ResolveLatestCLIRelease(ctx context.Context, repoInfo *RepoInfo) (*SelectedCLIRelease, error) {
	if client == nil {
		client = NewClient(nil)
	}
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	apiBase := client.APIBase
	if apiBase == "" {
		apiBase = APIBaseURL()
	}
	return resolveLatestCLIRelease(ctx, repoInfo, apiBase, httpClient)
}

func resolveLatestCLIRelease(ctx context.Context, repoInfo *RepoInfo, apiBase string, httpClient *http.Client) (*SelectedCLIRelease, error) {
	if repoInfo == nil {
		repoInfo = CLIRepository()
	}

	releases, err := listReleases(ctx, repoInfo, apiBase, httpClient)
	if err != nil {
		return nil, cliDiscoveryError(repoInfo, err)
	}

	selected, err := SelectStableCLIRelease(releases)
	if err != nil {
		return nil, noStableCLIReleaseError(repoInfo)
	}

	log.PluginV(log.Exec, " Successfully resolved latest stable CLI release!")
	return selected, nil
}

func isStableCLITag(tag string) bool {
	return stableCLITagRE.MatchString(tag) && semver.IsValid(tag)
}

func cliDiscoveryError(repoInfo *RepoInfo, cause error) error {
	return fmt.Errorf("%w for %s/%s: %w", ErrCLIReleaseDiscovery, repoInfo.Owner, repoInfo.Repo, cause)
}

func noStableCLIReleaseError(repoInfo *RepoInfo) error {
	return fmt.Errorf(
		"%w: repository %s/%s publishes no stable CLI release matching vX.Y.Z; check the repository releases or pin a version with NEKO_VERSION: %w",
		ErrCLIReleaseDiscovery,
		repoInfo.Owner,
		repoInfo.Repo,
		ErrNoReleases,
	)
}
