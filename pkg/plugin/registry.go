package plugin

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      04.02.2026
*/

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	// DefaultRegistry is the default GitHub releases API URL for fetching plugins.
	// This points to the neko-cli repository's releases endpoint.
	DefaultRegistry = "https://api.github.com/repos/nekoman-hq/neko-cli/releases"

	maxReleasePages = 10
)

// AvailablePlugin represents a plugin that is available for installation
// from the registry. It contains basic metadata about the plugin.
type AvailablePlugin struct {
	Name    string `json:"name"`    // The name of the plugin
	Version string `json:"version"` // The version string (e.g., "1.2.3")
}

// Registry handles plugin discovery and retrieval from a remote registry.
// It communicates with the GitHub Releases API to fetch plugin information
// and download URLs.
type Registry struct {
	baseURL string
}

type pluginReleaseIdentity struct {
	PublicName  string
	UnitID      string
	TagPrefix   string
	AssetPrefix string
	BinaryName  string
}

type registryAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ID                 int    `json:"id"`
}

type registryRelease struct {
	TagName    string          `json:"tag_name"`
	Assets     []registryAsset `json:"assets"`
	Draft      bool            `json:"draft"`
	PreRelease bool            `json:"prerelease"`
}

var pluginReleaseIdentities = []pluginReleaseIdentity{
	{
		PublicName:  "release",
		UnitID:      "plugin-release",
		TagPrefix:   "plugin-release/v",
		AssetPrefix: "plugin-release",
		BinaryName:  "plugin-release",
	},
	{
		PublicName:  "ui",
		UnitID:      "plugin-ui",
		TagPrefix:   "plugin-ui/v",
		AssetPrefix: "plugin-ui",
		BinaryName:  "plugin-ui",
	},
}

// NewRegistry creates a new registry client using the default registry URL.
// The default registry points to the neko-cli GitHub releases.
//
// Returns a configured Registry instance.
func NewRegistry() *Registry {
	return &Registry{
		baseURL: DefaultRegistry,
	}
}

// NewRegistryWithURL creates a new registry client with a custom base URL.
// This is useful for testing or using alternative plugin sources.
//
// Args:
//   - url: The base URL for the registry API (should be a GitHub releases endpoint)
//
// Returns a configured Registry instance.
func NewRegistryWithURL(url string) *Registry {
	return &Registry{
		baseURL: url,
	}
}

// FetchAvailablePlugins retrieves the list of all known plugins available in the registry.
// It lists repository releases, filters them by each plugin's V2 unit tag prefix,
// and returns the highest semantic version for each plugin.
func (r *Registry) FetchAvailablePlugins() ([]AvailablePlugin, error) {
	releases, err := r.listReleases()
	if err != nil {
		return nil, err
	}

	var plugins []AvailablePlugin
	for _, identity := range pluginReleaseIdentities {
		if _, version, ok := selectLatestPluginRelease(releases, identity); ok {
			plugins = append(plugins, AvailablePlugin{
				Name:    identity.PublicName,
				Version: version,
			})
		}
	}

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Name < plugins[j].Name
	})

	return plugins, nil
}

// GetLatestVersion retrieves the tag name of the latest release for a plugin.
//
// Returns:
//   - The tag name string (e.g., "plugin-release/v4.0.2")
//   - An error if:
//   - the plugin is unknown
//   - the HTTP request fails
//   - no matching plugin-specific release exists
func (r *Registry) GetLatestVersion(pluginName string) (string, error) {
	release, _, err := r.latestPluginRelease(pluginName)
	if err != nil {
		return "", err
	}
	return release.TagName, nil
}

// ResolveReleaseTag maps a user-supplied plugin version to the V2 unit release tag.
// It accepts "latest", "4.0.2", "v4.0.2", or the exact unit tag.
func (r *Registry) ResolveReleaseTag(pluginName, version string) (string, error) {
	if version == "" || version == "latest" {
		return r.GetLatestVersion(pluginName)
	}

	identity, err := pluginIdentity(pluginName)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(version, identity.TagPrefix) {
		suffix := strings.TrimPrefix(version, identity.TagPrefix)
		if !semver.IsValid("v" + suffix) {
			return "", fmt.Errorf("invalid version %q for plugin %q", version, pluginName)
		}
		return version, nil
	}

	suffix := strings.TrimPrefix(version, "v")
	if !semver.IsValid("v" + suffix) {
		return "", fmt.Errorf("invalid version %q for plugin %q", version, pluginName)
	}
	return identity.TagPrefix + suffix, nil
}

// GetDownloadURL constructs the browser download URL for a specific plugin version.
// It searches the release assets for a file matching the expected naming pattern
// and returns its download URL.
//
// Expected asset naming pattern: plugin-{name}_{version}_{OS}_{Arch}.tar.gz
// Example: plugin-release_2.3.0_Darwin_arm64.tar.gz
//
// Args:
//   - pluginName: The name of the plugin
//   - releaseTag: The release tag (e.g., "v1.2.3")
//   - osName: The operating system name (e.g., "Darwin", "Linux", "Windows")
//   - archName: The architecture name (e.g., "arm64", "x86_64")
//
// Returns:
//   - The browser download URL for the matching asset
//   - An error if:
//   - The HTTP request fails
//   - The response cannot be decoded
//   - No matching asset is found for the given OS/architecture combination
func (r *Registry) GetDownloadURL(pluginName, releaseTag, osName, archName string) (string, error) {
	identity, err := pluginIdentity(pluginName)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/tags/%s", r.baseURL, url.PathEscape(releaseTag))
	resp, err := r.httpGetWithAuth(url)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch release %s for plugin %q: %s", releaseTag, pluginName, resp.Status)
	}

	var release registryRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	// Find asset matching pattern: plugin-{unit}_{version}_{OS}_{Arch}.tar.gz
	prefix := fmt.Sprintf("%s_", identity.AssetPrefix)
	suffix := fmt.Sprintf("_%s_%s.tar.gz", osName, archName)

	for _, asset := range release.Assets {
		if strings.HasPrefix(asset.Name, prefix) && strings.HasSuffix(asset.Name, suffix) {
			return asset.BrowserDownloadURL, nil
		}
	}

	// Check if the plugin exists at all (any platform)
	pluginExists := false
	for _, asset := range release.Assets {
		if strings.HasPrefix(asset.Name, prefix) && strings.HasSuffix(asset.Name, ".tar.gz") {
			pluginExists = true
			break
		}
	}

	if pluginExists {
		return "", fmt.Errorf("plugin %q in release %s is not available for %s/%s; available assets: %s",
			pluginName, releaseTag, osName, archName, formatAssetNames(release.Assets))
	}

	return "", fmt.Errorf("plugin %q does not exist in release %s for %s/%s; available assets: %s",
		pluginName, releaseTag, osName, archName, formatAssetNames(release.Assets))
}

// GetPluginVersion retrieves the version of a specific plugin from its latest V2 unit release.
//
// Args:
//   - pluginName: The name of the plugin to get the version for
//
// Returns:
//   - The plugin version string (e.g., "2.3.0")
//   - An error if the plugin cannot be found
func (r *Registry) GetPluginVersion(pluginName string) (string, error) {
	_, version, err := r.latestPluginRelease(pluginName)
	return version, err
}

func (r *Registry) latestPluginRelease(pluginName string) (registryRelease, string, error) {
	identity, err := pluginIdentity(pluginName)
	if err != nil {
		return registryRelease{}, "", err
	}

	releases, err := r.listReleases()
	if err != nil {
		return registryRelease{}, "", err
	}

	release, version, ok := selectLatestPluginRelease(releases, identity)
	if !ok {
		return registryRelease{}, "", fmt.Errorf("no plugin-specific release found for plugin %q with tag prefix %q",
			pluginName, identity.TagPrefix)
	}

	return release, version, nil
}

func pluginIdentity(pluginName string) (pluginReleaseIdentity, error) {
	for _, identity := range pluginReleaseIdentities {
		if identity.PublicName == pluginName {
			return identity, nil
		}
	}
	return pluginReleaseIdentity{}, fmt.Errorf("unknown plugin %q; known plugins: %s", pluginName, knownPluginNames())
}

func knownPluginNames() string {
	names := make([]string, 0, len(pluginReleaseIdentities))
	for _, identity := range pluginReleaseIdentities {
		names = append(names, identity.PublicName)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func (r *Registry) listReleases() ([]registryRelease, error) {
	nextURL := fmt.Sprintf("%s?per_page=100", r.baseURL)
	var releases []registryRelease

	for page := 0; page < maxReleasePages && nextURL != ""; page++ {
		resp, err := r.httpGetWithAuth(nextURL)
		if err != nil {
			return nil, err
		}

		pageReleases, linkHeader, err := decodeReleaseListResponse(resp)
		if err != nil {
			return nil, err
		}
		releases = append(releases, pageReleases...)
		nextURL = nextPageURL(linkHeader)
	}

	return releases, nil
}

func decodeReleaseListResponse(resp *http.Response) ([]registryRelease, string, error) {
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to fetch releases: %s", resp.Status)
	}

	var releases []registryRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, "", err
	}

	return releases, resp.Header.Get("Link"), nil
}

func selectLatestPluginRelease(releases []registryRelease, identity pluginReleaseIdentity) (registryRelease, string, bool) {
	var selected registryRelease
	var selectedSemver string
	var selectedVersion string

	for _, release := range releases {
		if release.Draft || release.PreRelease {
			continue
		}
		if !strings.HasPrefix(release.TagName, identity.TagPrefix) {
			continue
		}

		version := strings.TrimPrefix(release.TagName, identity.TagPrefix)
		versionSemver := "v" + version
		if !semver.IsValid(versionSemver) {
			continue
		}

		if selectedSemver == "" || semver.Compare(versionSemver, selectedSemver) > 0 {
			selected = release
			selectedSemver = versionSemver
			selectedVersion = version
		}
	}

	return selected, selectedVersion, selectedSemver != ""
}

func nextPageURL(linkHeader string) string {
	for _, link := range strings.Split(linkHeader, ",") {
		link = strings.TrimSpace(link)
		if !strings.Contains(link, `rel="next"`) {
			continue
		}

		start := strings.Index(link, "<")
		end := strings.Index(link, ">")
		if start >= 0 && end > start {
			return link[start+1 : end]
		}
	}

	return ""
}

func formatAssetNames(assets []registryAsset) string {
	if len(assets) == 0 {
		return "none"
	}

	names := make([]string, 0, len(assets))
	for _, asset := range assets {
		names = append(names, asset.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// httpGetWithAuth performs an HTTP GET request with optional GitHub authentication.
// If the GITHUB_TOKEN environment variable is set, it is included in the request
// headers as a Bearer token, enabling access to private repositories and higher
// rate limits.
//
// Args:
//   - url: The URL to request
//
// Returns:
//   - The HTTP response
//   - An error if the request cannot be created or executed
func (r *Registry) httpGetWithAuth(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add GitHub token if available (for private repos)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	return http.DefaultClient.Do(req)
}
