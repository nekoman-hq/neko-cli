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
	neturl "net/url"
	"os"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	// DefaultRegistry is the default GitHub releases API URL for fetching plugins.
	// This points to the neko-cli repository's releases endpoint.
	DefaultRegistry = "https://api.github.com/repos/nekoman-hq/neko-cli/releases"

	DefaultPluginIndexReleaseTag = "plugin-registry"
	DefaultPluginIndexAssetName  = "plugin-index.json"
)

// AvailablePlugin represents a plugin that is available for installation
// from the registry. It contains basic metadata about the plugin.
type AvailablePlugin struct {
	Name        string `json:"name"`                  // The name of the plugin
	Version     string `json:"version"`               // The version string (e.g., "1.2.3")
	Description string `json:"description,omitempty"` // Human-readable plugin description
}

// Registry handles plugin discovery and retrieval from a remote registry.
// It reads the public plugin-index.json from the plugin-registry release and
// resolves release assets from GitHub Releases by exact tag.
//
//nolint:govet // Field order groups registry configuration for readability.
type Registry struct {
	baseURL         string
	indexURL        string
	indexReleaseTag string
	indexAssetName  string
	httpClient      *http.Client
}

//nolint:govet // JSON schema order mirrors the generated plugin-index artifact.
type pluginIndex struct {
	SchemaVersion int                `json:"schemaVersion"`
	Repository    string             `json:"repository"`
	Plugins       []pluginIndexEntry `json:"plugins"`
}

type pluginIndexEntry struct {
	Name        string `json:"name"`
	Unit        string `json:"unit"`
	Version     string `json:"version"`
	Tag         string `json:"tag"`
	TagPrefix   string `json:"tagPrefix"`
	Manifest    string `json:"manifest"`
	AssetPrefix string `json:"assetPrefix"`
	BinaryName  string `json:"binaryName"`
	Description string `json:"description"`
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

// NewRegistry creates a new registry client using the default registry URL.
// The default registry points to the neko-cli GitHub releases.
func NewRegistry() *Registry {
	return NewRegistryWithURL(DefaultRegistry)
}

// NewRegistryWithURL creates a new registry client with a custom GitHub
// releases base URL. This is useful for tests or alternative registries.
func NewRegistryWithURL(url string) *Registry {
	return &Registry{
		baseURL:         strings.TrimRight(url, "/"),
		indexReleaseTag: DefaultPluginIndexReleaseTag,
		indexAssetName:  DefaultPluginIndexAssetName,
		httpClient:      http.DefaultClient,
	}
}

// NewRegistryWithIndexURL creates a registry that downloads plugin-index.json
// directly from indexURL while still using DefaultRegistry for release assets.
func NewRegistryWithIndexURL(indexURL string) *Registry {
	registry := NewRegistry()
	registry.indexURL = indexURL
	return registry
}

// NewRegistryWithHTTPClient creates a registry with an injected HTTP client.
func NewRegistryWithHTTPClient(client *http.Client) *Registry {
	registry := NewRegistry()
	if client != nil {
		registry.httpClient = client
	}
	return registry
}

// FetchAvailablePlugins retrieves available plugins from plugin-index.json.
func (r *Registry) FetchAvailablePlugins() ([]AvailablePlugin, error) {
	index, err := r.loadPluginIndex()
	if err != nil {
		return nil, err
	}

	plugins := make([]AvailablePlugin, 0, len(index.Plugins))
	for _, entry := range index.Plugins {
		plugins = append(plugins, AvailablePlugin{
			Name:        entry.Name,
			Version:     entry.Version,
			Description: entry.Description,
		})
	}
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Name < plugins[j].Name
	})
	return plugins, nil
}

// GetLatestVersion retrieves the latest release tag for a plugin from the index.
func (r *Registry) GetLatestVersion(pluginName string) (string, error) {
	entry, err := r.pluginEntry(pluginName)
	if err != nil {
		return "", err
	}
	return entry.Tag, nil
}

// ResolveReleaseTag maps a user-supplied plugin version to a V2 unit release tag.
// It accepts "latest", "4.0.2", "v4.0.2", or the exact unit tag.
func (r *Registry) ResolveReleaseTag(pluginName, version string) (string, error) {
	entry, err := r.pluginEntry(pluginName)
	if err != nil {
		return "", err
	}

	if version == "" || version == "latest" {
		return entry.Tag, nil
	}
	if strings.HasPrefix(version, entry.TagPrefix) {
		suffix := strings.TrimPrefix(version, entry.TagPrefix)
		if !validIndexVersion(suffix) {
			return "", fmt.Errorf("invalid version %q for plugin %q", version, pluginName)
		}
		return version, nil
	}
	if strings.Contains(version, "/") {
		return "", fmt.Errorf("release tag %q does not belong to plugin %q; expected prefix %q", version, pluginName, entry.TagPrefix)
	}

	suffix := strings.TrimPrefix(version, "v")
	if !validIndexVersion(suffix) {
		return "", fmt.Errorf("invalid version %q for plugin %q", version, pluginName)
	}
	return entry.TagPrefix + suffix, nil
}

// GetDownloadURL constructs the browser download URL for a specific plugin release tag.
func (r *Registry) GetDownloadURL(pluginName, releaseTag, osName, archName string) (string, error) {
	entry, err := r.pluginMetadata(pluginName)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(releaseTag, entry.TagPrefix) {
		return "", fmt.Errorf("release tag %q does not belong to plugin %q; expected prefix %q", releaseTag, pluginName, entry.TagPrefix)
	}

	release, err := r.fetchReleaseByTag(releaseTag)
	if err != nil {
		return "", err
	}

	prefix := fmt.Sprintf("%s_", entry.AssetPrefix)
	suffix := fmt.Sprintf("_%s_%s.tar.gz", osName, archName)
	for _, asset := range release.Assets {
		if strings.HasPrefix(asset.Name, prefix) && strings.HasSuffix(asset.Name, suffix) {
			return asset.BrowserDownloadURL, nil
		}
	}

	for _, asset := range release.Assets {
		if strings.HasPrefix(asset.Name, prefix) && strings.HasSuffix(asset.Name, ".tar.gz") {
			return "", fmt.Errorf("plugin %q release %q has no asset for %s/%s; available assets: %s",
				pluginName, releaseTag, osName, archName, formatAssetNames(release.Assets))
		}
	}

	return "", fmt.Errorf("plugin %q release %q has no assets with prefix %q; available assets: %s",
		pluginName, releaseTag, prefix, formatAssetNames(release.Assets))
}

// GetPluginVersion retrieves the version of a specific plugin from plugin-index.json.
func (r *Registry) GetPluginVersion(pluginName string) (string, error) {
	entry, err := r.pluginEntry(pluginName)
	if err != nil {
		return "", err
	}
	return entry.Version, nil
}

func (r *Registry) pluginMetadata(pluginName string) (pluginIndexEntry, error) {
	index, err := r.loadPluginIndex()
	if err != nil {
		return pluginIndexEntry{}, err
	}
	return index.entry(pluginName)
}

func (r *Registry) pluginEntry(pluginName string) (pluginIndexEntry, error) {
	index, err := r.loadPluginIndex()
	if err != nil {
		return pluginIndexEntry{}, err
	}
	return index.entry(pluginName)
}

func (idx *pluginIndex) entry(pluginName string) (pluginIndexEntry, error) {
	for _, entry := range idx.Plugins {
		if entry.Name == pluginName {
			return entry, nil
		}
	}
	return pluginIndexEntry{}, fmt.Errorf("unknown plugin %q; available plugins: %s", pluginName, idx.availablePluginNames())
}

func (idx *pluginIndex) availablePluginNames() string {
	names := make([]string, 0, len(idx.Plugins))
	for _, entry := range idx.Plugins {
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func (r *Registry) loadPluginIndex() (*pluginIndex, error) {
	var indexURL string
	if r.indexURL != "" {
		indexURL = r.indexURL
	} else {
		downloadURL, err := r.pluginIndexDownloadURL()
		if err != nil {
			return nil, err
		}
		indexURL = downloadURL
	}

	resp, err := r.httpGetWithAuth(indexURL)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin index from plugin-registry release: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to load plugin index from plugin-registry release: download %s returned %s", indexURL, resp.Status)
	}

	var index pluginIndex
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, fmt.Errorf("invalid plugin index: malformed JSON: %w", err)
	}
	if err := validatePluginIndex(&index); err != nil {
		return nil, fmt.Errorf("invalid plugin index: %w", err)
	}
	return &index, nil
}

func (r *Registry) pluginIndexDownloadURL() (string, error) {
	releaseURL := fmt.Sprintf("%s/tags/%s", r.baseURL, neturl.PathEscape(r.indexReleaseTag))
	resp, err := r.httpGetWithAuth(releaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to load plugin index from plugin-registry release: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return "", fmt.Errorf("failed to load plugin index from plugin-registry release: release or asset not found")
		}
		return "", fmt.Errorf("failed to load plugin index from plugin-registry release: %s", resp.Status)
	}

	var release registryRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to load plugin index from plugin-registry release: malformed release response: %w", err)
	}
	for _, asset := range release.Assets {
		if asset.Name == r.indexAssetName {
			return asset.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("failed to load plugin index from plugin-registry release: release or asset not found")
}

func validatePluginIndex(index *pluginIndex) error {
	if index.SchemaVersion != 1 {
		return fmt.Errorf("schemaVersion must be 1, got %d", index.SchemaVersion)
	}
	if strings.TrimSpace(index.Repository) == "" {
		return fmt.Errorf("repository must not be empty")
	}

	names := map[string]struct{}{}
	for _, entry := range index.Plugins {
		if strings.TrimSpace(entry.Name) == "" {
			return fmt.Errorf("plugin name must not be empty")
		}
		if _, exists := names[entry.Name]; exists {
			return fmt.Errorf("duplicate plugin name %q", entry.Name)
		}
		names[entry.Name] = struct{}{}

		if strings.TrimSpace(entry.Unit) == "" {
			return fmt.Errorf("plugin %q unit must not be empty", entry.Name)
		}
		if !validIndexVersion(entry.Version) {
			return fmt.Errorf("plugin %q version %q is not valid semver", entry.Name, entry.Version)
		}
		if entry.Tag != entry.TagPrefix+entry.Version {
			return fmt.Errorf("plugin %q tag %q must equal tagPrefix + version %q", entry.Name, entry.Tag, entry.TagPrefix+entry.Version)
		}
		if strings.TrimSpace(entry.AssetPrefix) == "" {
			return fmt.Errorf("plugin %q assetPrefix must not be empty", entry.Name)
		}
		if strings.TrimSpace(entry.BinaryName) == "" {
			return fmt.Errorf("plugin %q binaryName must not be empty", entry.Name)
		}
	}
	return nil
}

func validIndexVersion(version string) bool {
	return version != "" && !strings.HasPrefix(version, "v") && semver.IsValid("v"+version)
}

func (r *Registry) fetchReleaseByTag(releaseTag string) (registryRelease, error) {
	releaseURL := fmt.Sprintf("%s/tags/%s", r.baseURL, neturl.PathEscape(releaseTag))
	resp, err := r.httpGetWithAuth(releaseURL)
	if err != nil {
		return registryRelease{}, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return registryRelease{}, fmt.Errorf("failed to fetch release %s: %s", releaseTag, resp.Status)
	}

	var release registryRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return registryRelease{}, err
	}
	return release, nil
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
func (r *Registry) httpGetWithAuth(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	return r.httpClient.Do(req)
}
