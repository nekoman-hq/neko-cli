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
	Version string `json:"metadata"`
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

	// Parse plugin names from assets
	pluginMap := make(map[string]bool)
	for _, asset := range release.Assets {
		// Plugin assets follow pattern: plugin-{name}_{OS}_{Arch}.tar.gz
		if strings.HasPrefix(asset.Name, "plugin-") {
			parts := strings.Split(asset.Name, "_")
			if len(parts) >= 1 {
				pluginName := strings.TrimPrefix(parts[0], "plugin-")
				pluginMap[pluginName] = true
			}
		}
	}

	var plugins []AvailablePlugin
	for name := range pluginMap {
		plugins = append(plugins, AvailablePlugin{
			Name:    name,
			Version: latestVersion,
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

// GetDownloadURL returns the download URL for a specific plugin metadata
func (r *Registry) GetDownloadURL(pluginName, version, osName, archName string) (string, error) {
	assetName := fmt.Sprintf("plugin-%s_%s_%s.tar.gz", pluginName, osName, archName)

	url := fmt.Sprintf("%s/tags/%s", r.baseURL, version)
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

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("plugin '%s' not found for %s/%s in metadata %s", pluginName, osName, archName, version)
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
