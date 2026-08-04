package github

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mixedRepositoryReleases mirrors the real multi-unit repository: the newest
// release by API ordering is a plugin unit, plugin-registry is mutable, and
// drafts, prereleases, and malformed tags share the same list.
func mixedRepositoryReleases() []Release {
	return []Release{
		{Name: "plugin-release 4.4.5", TagName: "plugin-release/v4.4.5", HTMLURL: "https://example.com/plugin-release"},
		{Name: "plugin-ui 1.3.0", TagName: "plugin-ui/v1.3.0", HTMLURL: "https://example.com/plugin-ui"},
		{Name: "Neko Plugin Registry", TagName: "plugin-registry", HTMLURL: "https://example.com/plugin-registry"},
		{Name: "Nekocli 3.2.0", TagName: "v3.2.0", Draft: true},
		{Name: "Nekocli 3.1.3 RC1", TagName: "v3.1.3-rc.1", PreRelease: true},
		{Name: "Nekocli 3.1.9 mistagged", TagName: "3.1.9"},
		{Name: "Nekocli 3.9 mistagged", TagName: "v3.9"},
		{Name: "Nekocli 3.0.10", TagName: "v3.0.10"},
		{
			Name:        "Nekocli 3.1.2",
			TagName:     "v3.1.2",
			PublishedAt: "2026-08-01T18:30:00Z",
			HTMLURL:     "https://github.com/nekoman-hq/neko-cli/releases/tag/v3.1.2",
			Author:      Author{Login: "release-bot"},
			Assets: []Asset{
				{Name: "neko-cli_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://download.example/neko-cli_Darwin_arm64.tar.gz"},
				{Name: "neko-cli_3.1.2_checksums.txt", BrowserDownloadURL: "https://download.example/neko-cli_3.1.2_checksums.txt"},
			},
		},
		{Name: "Nekocli 3.1.1", TagName: "v3.1.1"},
		{Name: "Nekocli 3.0.9", TagName: "v3.0.9"},
	}
}

func TestSelectStableCLIReleaseIgnoresEveryNonStableCLITag(t *testing.T) {
	selected, err := SelectStableCLIRelease(mixedRepositoryReleases())
	if err != nil {
		t.Fatalf("SelectStableCLIRelease: %v", err)
	}

	if selected.TagName != "v3.1.2" {
		t.Fatalf("selected tag = %q, want v3.1.2", selected.TagName)
	}
	if selected.Version != "v3.1.2" {
		t.Fatalf("selected version = %q, want v3.1.2", selected.Version)
	}
	if selected.HTMLURL != "https://github.com/nekoman-hq/neko-cli/releases/tag/v3.1.2" {
		t.Fatalf("selected url = %q", selected.HTMLURL)
	}
	if selected.PublishedAt != "2026-08-01T18:30:00Z" {
		t.Fatalf("selected published = %q", selected.PublishedAt)
	}
	if selected.Author != "release-bot" {
		t.Fatalf("selected author = %q", selected.Author)
	}
	if len(selected.Assets) != 2 {
		t.Fatalf("selected assets = %v", selected.Assets)
	}
}

func TestSelectStableCLIReleaseRejectsEachIgnoredTagIndividually(t *testing.T) {
	for _, tag := range []string{
		"plugin-release/v4.4.5",
		"plugin-ui/v1.3.0",
		"plugin-registry",
		"3.1.9",
		"v3.9",
		"v3.1.2+build",
		"v3.1.2-rc.1",
		"latest",
		"",
	} {
		if isStableCLITag(tag) {
			t.Fatalf("tag %q must not be treated as a stable CLI tag", tag)
		}
		if _, err := SelectStableCLIRelease([]Release{{TagName: tag}}); !stderrors.Is(err, ErrNoReleases) {
			t.Fatalf("SelectStableCLIRelease(%q) error = %v, want ErrNoReleases", tag, err)
		}
	}
}

func TestSelectStableCLIReleaseIgnoresDraftsAndPrereleasesEvenWhenNewest(t *testing.T) {
	selected, err := SelectStableCLIRelease([]Release{
		{TagName: "v9.9.9", Draft: true},
		{TagName: "v9.9.8", PreRelease: true},
		{TagName: "v3.1.2"},
	})
	if err != nil {
		t.Fatalf("SelectStableCLIRelease: %v", err)
	}
	if selected.TagName != "v3.1.2" {
		t.Fatalf("selected tag = %q, want v3.1.2", selected.TagName)
	}
}

func TestSelectStableCLIReleaseComparesNumericallyNotLexicographically(t *testing.T) {
	selected, err := SelectStableCLIRelease([]Release{
		{TagName: "v3.1.9"},
		{TagName: "v3.1.10"},
		{TagName: "v3.1.2"},
	})
	if err != nil {
		t.Fatalf("SelectStableCLIRelease: %v", err)
	}
	if selected.TagName != "v3.1.10" {
		t.Fatalf("selected tag = %q, want v3.1.10 (v3.1.10 sorts after v3.1.9)", selected.TagName)
	}
}

func TestSelectStableCLIReleaseDoesNotTrustAPIOrdering(t *testing.T) {
	ascending := []Release{{TagName: "v3.0.9"}, {TagName: "v3.1.1"}, {TagName: "v3.1.2"}}
	descending := []Release{{TagName: "v3.1.2"}, {TagName: "v3.1.1"}, {TagName: "v3.0.9"}}
	shuffled := []Release{{TagName: "v3.1.1"}, {TagName: "v3.0.9"}, {TagName: "v3.1.2"}}

	for name, releases := range map[string][]Release{"ascending": ascending, "descending": descending, "shuffled": shuffled} {
		selected, err := SelectStableCLIRelease(releases)
		if err != nil {
			t.Fatalf("%s: SelectStableCLIRelease: %v", name, err)
		}
		if selected.TagName != "v3.1.2" {
			t.Fatalf("%s: selected tag = %q, want v3.1.2", name, selected.TagName)
		}
	}
}

func TestResolveLatestCLIReleaseFollowsEveryReleasePage(t *testing.T) {
	firstPage := make([]Release, 0, releasesPerPage)
	for index := 0; index < releasesPerPage; index++ {
		firstPage = append(firstPage, Release{TagName: fmt.Sprintf("plugin-release/v4.4.%d", index)})
	}
	secondPage := []Release{{TagName: "v3.1.2", HTMLURL: "https://example.com/v3.1.2"}}

	var requested []string
	server := releaseListServer(t, &requested, func(page int) any {
		switch page {
		case 1:
			return firstPage
		case 2:
			return secondPage
		default:
			return []Release{}
		}
	})

	selected, err := clientFor(server).ResolveLatestCLIRelease(context.Background(), &RepoInfo{Owner: "nekoman-hq", Repo: "neko-cli"})
	if err != nil {
		t.Fatalf("ResolveLatestCLIRelease: %v", err)
	}
	if selected.TagName != "v3.1.2" {
		t.Fatalf("selected tag = %q, want v3.1.2 from the second page", selected.TagName)
	}
	if len(requested) != 2 {
		t.Fatalf("requested pages = %v, want exactly two pages", requested)
	}
	for index, path := range requested {
		want := fmt.Sprintf("per_page=%d&page=%d", releasesPerPage, index+1)
		if !strings.HasSuffix(path, want) {
			t.Fatalf("request %d = %q, want suffix %q", index+1, path, want)
		}
	}
}

func TestResolveLatestCLIReleaseStopsPaginationOnAShortPage(t *testing.T) {
	var requested []string
	server := releaseListServer(t, &requested, func(int) any {
		return []Release{{TagName: "v3.1.2"}}
	})

	if _, err := clientFor(server).ResolveLatestCLIRelease(context.Background(), &RepoInfo{Owner: "nekoman-hq", Repo: "neko-cli"}); err != nil {
		t.Fatalf("ResolveLatestCLIRelease: %v", err)
	}
	if len(requested) != 1 {
		t.Fatalf("requested pages = %v, want exactly one page", requested)
	}
}

// TestResolveLatestCLIReleaseRefusesATruncatedReleaseList pins the pagination
// boundary: when the last permitted page is still full the list did not end, so
// an unread page may hold a greater stable CLI tag. Discovery must fail rather
// than select the greatest tag seen so far.
func TestResolveLatestCLIReleaseRefusesATruncatedReleaseList(t *testing.T) {
	fullPage := make([]Release, 0, releasesPerPage)
	fullPage = append(fullPage, Release{TagName: "v3.1.2"})
	for index := 1; index < releasesPerPage; index++ {
		fullPage = append(fullPage, Release{TagName: fmt.Sprintf("plugin-release/v4.4.%d", index)})
	}

	var requested []string
	server := releaseListServer(t, &requested, func(int) any { return fullPage })

	selected, err := clientFor(server).ResolveLatestCLIRelease(context.Background(), &RepoInfo{Owner: "nekoman-hq", Repo: "neko-cli"})
	if selected != nil {
		t.Fatalf("selected = %#v, want no release from a truncated list", selected)
	}
	if !stderrors.Is(err, ErrCLIReleaseDiscovery) || !stderrors.Is(err, ErrReleaseListTruncated) {
		t.Fatalf("error = %v, want both ErrCLIReleaseDiscovery and ErrReleaseListTruncated", err)
	}
	for _, fragment := range []string{
		"unable to determine the latest CLI release for nekoman-hq/neko-cli",
		"release list exceeds the pagination limit",
		"and more remain",
		"pin a version",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("error = %q, want fragment %q", err.Error(), fragment)
		}
	}
	if len(requested) != maxReleasePages {
		t.Fatalf("requested %d page(s), want exactly the %d-page limit", len(requested), maxReleasePages)
	}
}

// TestResolveLatestCLIReleaseAcceptsAFullPageFollowedByAShortPage keeps the
// truncation guard from firing on a list that merely fills an intermediate page.
func TestResolveLatestCLIReleaseAcceptsAFullPageFollowedByAShortPage(t *testing.T) {
	fullPage := make([]Release, 0, releasesPerPage)
	for index := 0; index < releasesPerPage; index++ {
		fullPage = append(fullPage, Release{TagName: fmt.Sprintf("plugin-release/v4.4.%d", index)})
	}

	var requested []string
	server := releaseListServer(t, &requested, func(page int) any {
		if page < maxReleasePages {
			return fullPage
		}
		return []Release{{TagName: "v3.1.2"}}
	})

	selected, err := clientFor(server).ResolveLatestCLIRelease(context.Background(), &RepoInfo{Owner: "nekoman-hq", Repo: "neko-cli"})
	if err != nil {
		t.Fatalf("ResolveLatestCLIRelease: %v", err)
	}
	if selected.TagName != "v3.1.2" {
		t.Fatalf("selected tag = %q, want v3.1.2 from the final short page", selected.TagName)
	}
	if len(requested) != maxReleasePages {
		t.Fatalf("requested %d page(s), want %d", len(requested), maxReleasePages)
	}
}

func TestResolveLatestCLIReleaseReportsHTTPFailureAndNeverFallsBack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	selected, err := clientFor(server).ResolveLatestCLIRelease(context.Background(), &RepoInfo{Owner: "nekoman-hq", Repo: "neko-cli"})
	if selected != nil {
		t.Fatalf("selected = %#v, want no release on failure", selected)
	}
	if !stderrors.Is(err, ErrCLIReleaseDiscovery) {
		t.Fatalf("error = %v, want ErrCLIReleaseDiscovery", err)
	}
	if !strings.Contains(err.Error(), "unable to determine the latest CLI release for nekoman-hq/neko-cli") {
		t.Fatalf("error = %q, want actionable discovery text", err.Error())
	}
}

func TestResolveLatestCLIReleaseReportsMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, `{"tag_name": "v3.1.2"`)
	}))
	t.Cleanup(server.Close)

	selected, err := clientFor(server).ResolveLatestCLIRelease(context.Background(), &RepoInfo{Owner: "nekoman-hq", Repo: "neko-cli"})
	if selected != nil {
		t.Fatalf("selected = %#v, want no release on a parse failure", selected)
	}
	if !stderrors.Is(err, ErrCLIReleaseDiscovery) || !strings.Contains(err.Error(), "JSON Parse Failed") {
		t.Fatalf("error = %v, want a reported parse failure", err)
	}
}

func TestResolveLatestCLIReleaseReportsMissingStableCLIRelease(t *testing.T) {
	var requested []string
	server := releaseListServer(t, &requested, func(int) any {
		return []Release{
			{TagName: "plugin-release/v4.4.5"},
			{TagName: "plugin-ui/v1.3.0"},
			{TagName: "plugin-registry"},
		}
	})

	selected, err := clientFor(server).ResolveLatestCLIRelease(context.Background(), &RepoInfo{Owner: "nekoman-hq", Repo: "neko-cli"})
	if selected != nil {
		t.Fatalf("selected = %#v, want no release", selected)
	}
	if !stderrors.Is(err, ErrCLIReleaseDiscovery) || !stderrors.Is(err, ErrNoReleases) {
		t.Fatalf("error = %v, want both ErrCLIReleaseDiscovery and ErrNoReleases", err)
	}
	if !strings.Contains(err.Error(), "publishes no stable CLI release matching vX.Y.Z") {
		t.Fatalf("error = %q, want actionable text", err.Error())
	}
}

func TestResolveLatestCLIReleaseNeverRequestsTheLatestReleaseEndpoint(t *testing.T) {
	var requested []string
	server := releaseListServer(t, &requested, func(int) any {
		return []Release{{TagName: "v3.1.2"}}
	})

	if _, err := clientFor(server).ResolveLatestCLIRelease(context.Background(), &RepoInfo{Owner: "nekoman-hq", Repo: "neko-cli"}); err != nil {
		t.Fatalf("ResolveLatestCLIRelease: %v", err)
	}
	for _, path := range requested {
		if strings.Contains(path, "/releases/latest") {
			t.Fatalf("CLI discovery requested %q", path)
		}
	}
}

func TestCLIRepositoryDefaultsAndHonorsConfiguredRepository(t *testing.T) {
	t.Setenv("NEKO_REPOSITORY", "")
	if repo := CLIRepository(); repo.Owner != "nekoman-hq" || repo.Repo != "neko-cli" {
		t.Fatalf("default repository = %+v", repo)
	}

	t.Setenv("NEKO_REPOSITORY", " forkowner/forkrepo ")
	if repo := CLIRepository(); repo.Owner != "forkowner" || repo.Repo != "forkrepo" {
		t.Fatalf("configured repository = %+v", repo)
	}

	for _, invalid := range []string{"nekoman-hq", "/neko-cli", "nekoman-hq/", "a/b/c"} {
		t.Setenv("NEKO_REPOSITORY", invalid)
		if repo := CLIRepository(); repo.Owner != "nekoman-hq" || repo.Repo != "neko-cli" {
			t.Fatalf("repository for %q = %+v, want the default", invalid, repo)
		}
	}
}

func clientFor(server *httptest.Server) *Client {
	client := NewClient(server.Client())
	client.APIBase = server.URL
	return client
}

func releaseListServer(t *testing.T, requested *[]string, page func(int) any) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		*requested = append(*requested, request.URL.RequestURI())
		pageNumber := 1
		if raw := request.URL.Query().Get("page"); raw != "" {
			if _, err := fmt.Sscanf(raw, "%d", &pageNumber); err != nil {
				t.Errorf("unparsable page query %q", raw)
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(page(pageNumber)); err != nil {
			t.Errorf("encode release page: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}
