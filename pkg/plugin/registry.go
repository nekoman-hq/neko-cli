package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	// DefaultRegistry is the default GitHub releases URL for fetching plugins
	DefaultRegistry = "https://api.github.com/repos/nekoman-hq/neko-cli/releases"
)

// AvailablePlugin represents a plugin available in the registry
type AvailablePlugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Registry handles plugin discovery from a remote registry
type Registry struct {
	baseURL string
}

// NewRegistry creates a new registry client
func NewRegistry() *Registry {
	return &Registry{
		baseURL: DefaultRegistry,
	}
}

// NewRegistryWithURL creates a new registry client with a custom URL
func NewRegistryWithURL(url string) *Registry {
	return &Registry{
		baseURL: url,
	}
}

// FetchAvailablePlugins fetches the list of available plugins from the registry
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

// GetLatestVersion fetches the latest metadata tag from the registry
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

// GetDownloadURL returns the download URL for a specific plugin version
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
	// We match by prefix and suffix since version is embedded in filename
	prefix := fmt.Sprintf("plugin-%s_", pluginName)
	suffix := fmt.Sprintf("_%s_%s.tar.gz", osName, archName)

	for _, asset := range release.Assets {
		if strings.HasPrefix(asset.Name, prefix) && strings.HasSuffix(asset.Name, suffix) {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("plugin '%s' not found for %s/%s in release %s", pluginName, osName, archName, releaseTag)
}

// httpGetWithAuth performs an HTTP GET request with optional GitHub authentication
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
