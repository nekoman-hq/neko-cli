package plugin

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Manager handles plugin installation, uninstallation, and management
type Manager struct {
	registry  *Registry
	pluginDir string
}

// NewManager creates a new plugin manager
func NewManager(pluginDir string) *Manager {
	return &Manager{
		pluginDir: pluginDir,
		registry:  NewRegistry(),
	}
}

// NewManagerWithRegistry creates a new plugin manager with a custom registry
func NewManagerWithRegistry(pluginDir string, registry *Registry) *Manager {
	return &Manager{
		pluginDir: pluginDir,
		registry:  registry,
	}
}

// EnsurePluginDir creates the plugin directory if it doesn't exist
func (m *Manager) EnsurePluginDir() error {
	return os.MkdirAll(m.pluginDir, 0755)
}

// Install installs a plugin from the registry
func (m *Manager) Install(pluginName, version string) error {
	// Ensure plugin directory exists
	if err := m.EnsurePluginDir(); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Determine metadata to install
	actualVersion := version
	if version == "latest" || version == "" {
		latestVersion, err := m.registry.GetLatestVersion()
		if err != nil {
			return fmt.Errorf("failed to get latest metadata: %w", err)
		}
		actualVersion = latestVersion
	}

	// Build download URL
	downloadURL, err := m.getPluginDownloadURL(pluginName, actualVersion)
	if err != nil {
		return fmt.Errorf("failed to get download URL: %w", err)
	}

	// Download and extract
	if err := m.downloadAndInstall(pluginName, downloadURL); err != nil {
		return fmt.Errorf("failed to install plugin: %w", err)
	}

	return nil
}

// Uninstall removes an installed plugin
func (m *Manager) Uninstall(pluginName string) error {
	installPath := filepath.Join(m.pluginDir, pluginName)
	if _, err := os.Stat(installPath); os.IsNotExist(err) {
		return fmt.Errorf("plugin '%s' is not installed", pluginName)
	}

	if err := os.RemoveAll(installPath); err != nil {
		return fmt.Errorf("failed to uninstall plugin: %w", err)
	}

	return nil
}

// IsInstalled checks if a plugin is installed
func (m *Manager) IsInstalled(pluginName string) bool {
	installPath := filepath.Join(m.pluginDir, pluginName)
	_, err := os.Stat(installPath)
	return err == nil
}

// GetManifest returns the manifest for an installed plugin
func (m *Manager) GetManifest(pluginName string) (*Manifest, error) {
	manifestPath := filepath.Join(m.pluginDir, pluginName, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// ListInstalled returns a map of installed plugin names to their versions
func (m *Manager) ListInstalled() (map[string]string, error) {
	installed := make(map[string]string)

	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return installed, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			manifest, err := m.GetManifest(entry.Name())
			if err == nil {
				installed[manifest.Name] = manifest.Version
			}
		}
	}

	return installed, nil
}

// GetAvailablePlugins returns the list of available plugins from the registry
func (m *Manager) GetAvailablePlugins() ([]AvailablePlugin, error) {
	return m.registry.FetchAvailablePlugins()
}

// getPluginDownloadURL builds the download URL for a plugin
func (m *Manager) getPluginDownloadURL(pluginName, version string) (string, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Map arch names to match goreleaser output
	archName := arch
	if arch == "amd64" {
		archName = "x86_64"
	}

	// Capitalize OS name
	caser := cases.Title(language.English)
	osName = caser.String(osName)

	return m.registry.GetDownloadURL(pluginName, version, osName, archName)
}

// downloadAndInstall downloads and extracts a plugin
func (m *Manager) downloadAndInstall(pluginName, downloadURL string) error {
	resp, err := m.httpGetWithAuth(downloadURL)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download plugin: %s", resp.Status)
	}

	// Remove existing plugin directory if it exists
	installPath := filepath.Join(m.pluginDir, pluginName)
	if err = os.RemoveAll(installPath); err != nil {
		return fmt.Errorf("failed to remove existing plugin: %w", err)
	}

	// Create plugin directory
	if err = os.MkdirAll(installPath, 0755); err != nil {
		return err
	}

	// Extract tar.gz
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer func(gzr *gzip.Reader) {
		_ = gzr.Close()
	}(gzr)

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Skip empty names or current directory entries
		name := filepath.Clean(header.Name)
		if name == "" || name == "." {
			continue
		}

		// Get just the base name (in case archive has nested structure)
		// This flattens any directory structure in the archive
		baseName := filepath.Base(name)
		target := filepath.Join(installPath, baseName)

		switch header.Typeflag {
		case tar.TypeDir:
			// Skip directories - we already created installPath
			continue
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", target, err)
			}
			if _, err = io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			if err = f.Close(); err != nil {
				return err
			}
		}
	}

	return nil
}

// httpGetWithAuth performs an HTTP GET request with optional GitHub authentication
func (m *Manager) httpGetWithAuth(url string) (*http.Response, error) {
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

// Truncate truncates a string to a maximum length, adding "..." if truncated
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
