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
	"os"
	"strings"
)

const (
	// DefaultRegistry is the default GitHub releases API URL for fetching plugins.
	// This points to the neko-cli repository's releases endpoint.
	DefaultRegistry = "https://api.github.com/repos/nekoman-hq/neko-cli/releases"
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

// FetchAvailablePlugins retrieves the list of all plugins available in the registry.
// It fetches the latest release and parses plugin information from the release assets.
//
// Plugin assets are expected to follow the naming pattern:
// plugin-{name}_{version}_{OS}_{Arch}.tar.gz
//
// Returns:
//   - A slice of AvailablePlugin structs containing plugin names and versions
//   - An error if:
//   - The latest version cannot be determined
//   - The HTTP request fails
//   - The response cannot be decoded
func (r *Registry) FetchAvailablePlugins() ([]AvailablePlugin, error) {
	latestVersion, err := r.GetLatestVersion()
	if err != nil {
		return nil, err
	}

	// Get release assets
	url := fmt.Sprintf("%s/tags/%s", r.baseURL, latestVersion)

	resp, err := r.httpGetWithAuth(url)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch release: %s", resp.Status)
	}

	var release struct {
		Assets []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	// Parse plugin names and versions from assets
	// Pattern: plugin-{name}_{version}_{OS}_{Arch}.tar.gz
	pluginMap := make(map[string]string) // name -> version
	for _, asset := range release.Assets {
		if strings.HasPrefix(asset.Name, "plugin-") && strings.HasSuffix(asset.Name, ".tar.gz") {
			// Remove extension
			name := strings.TrimSuffix(asset.Name, ".tar.gz")
			// Split: plugin-release_2.3.0_Darwin_arm64 -> [plugin-release, 2.3.0, Darwin, arm64]
			parts := strings.Split(name, "_")
			if len(parts) >= 2 {
				pluginName := strings.TrimPrefix(parts[0], "plugin-")
				pluginVersion := parts[1]
				// Only store if we haven't seen this plugin yet
				if _, exists := pluginMap[pluginName]; !exists {
					pluginMap[pluginName] = pluginVersion
				}
			}
		}
	}

	var plugins []AvailablePlugin
	for name, version := range pluginMap {
		plugins = append(plugins, AvailablePlugin{
			Name:    name,
			Version: version,
		})
	}

	return plugins, nil
}

// GetLatestVersion retrieves the tag name of the latest release from the registry.
//
// Returns:
//   - The tag name string (e.g., "v1.2.3")
//   - An error if:
//   - The HTTP request fails
//   - The response status is not 200 OK
//   - The response body cannot be decoded
func (r *Registry) GetLatestVersion() (string, error) {
	url := fmt.Sprintf("%s/latest", r.baseURL)

	resp, err := r.httpGetWithAuth(url)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch latest release: %s", resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return release.TagName, nil
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
	url := fmt.Sprintf("%s/tags/%s", r.baseURL, releaseTag)
	resp, err := r.httpGetWithAuth(url)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			ID                 int    `json:"id"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	// Find asset matching pattern: plugin-{name}_{version}_{OS}_{Arch}.tar.gz
	prefix := fmt.Sprintf("plugin-%s_", pluginName)
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
		return "", fmt.Errorf("plugin '%s' is not available for %s/%s (only available for other platforms)",
			pluginName, osName, archName)
	}

	return "", fmt.Errorf("plugin '%s' does not exist in release %s", pluginName, releaseTag)
}

// GetPluginVersion retrieves the version of a specific plugin from the latest release.
// It reuses the plugin parsing logic from FetchAvailablePlugins.
//
// Args:
//   - pluginName: The name of the plugin to get the version for
//
// Returns:
//   - The plugin version string (e.g., "2.3.0")
//   - An error if the plugin cannot be found
func (r *Registry) GetPluginVersion(pluginName string) (string, error) {
	availablePlugins, err := r.FetchAvailablePlugins()
	if err != nil {
		return "", err
	}

	for _, plugin := range availablePlugins {
		if plugin.Name == pluginName {
			return plugin.Version, nil
		}
	}

	return "", fmt.Errorf("plugin '%s' not found in latest release", pluginName)
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
