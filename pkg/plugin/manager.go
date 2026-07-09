package plugin

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      04.02.2026
*/

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
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Manager handles plugin installation, uninstallation, and management.
// It provides functionality to download plugins from a registry, install them
// to a local directory, and add their lifecycle.
type Manager struct {
	registry  *Registry
	PluginDir string
}

// NewManager creates a new plugin manager with the default registry.
//
// Args:
//   - pluginDir: The directory where plugins will be installed
//
// Returns a configured Manager instance.
func NewManager(pluginDir string) *Manager {
	return &Manager{
		PluginDir: pluginDir,
		registry:  NewRegistry(),
	}
}

// NewManagerWithRegistry creates a new plugin manager with a custom registry.
// This is useful for testing or using alternative plugin sources.
//
// Args:
//   - pluginDir: The directory where plugins will be installed
//   - registry: A custom Registry instance to use for plugin lookups
//
// Returns a configured Manager instance with the specified registry.
func NewManagerWithRegistry(pluginDir string, registry *Registry) *Manager {
	return &Manager{
		PluginDir: pluginDir,
		registry:  registry,
	}
}

// EnsurePluginDir creates the plugin directory if it doesn't exist.
// The directory is created with permissions 0755 (rwxr-xr-x).
//
// Returns an error if directory creation fails, or nil on success.
func (m *Manager) EnsurePluginDir() error {
	return os.MkdirAll(m.PluginDir, 0755)
}

// Install downloads and installs a plugin from the registry.
// If version is "latest" or empty, the most recent version is installed.
// The plugin is downloaded as a tar.gz archive and extracted to the plugin directory.
//
// Args:
//   - pluginName: The name of the plugin to install
//   - version: The version to install, or "latest" for the newest version
//
// Returns an error if:
//   - The plugin directory cannot be created
//   - The version cannot be resolved
//   - The download URL cannot be constructed
//   - The download or installation fails
func (m *Manager) Install(pluginName, version string) error {
	// Ensure plugin directory exists
	if err := m.EnsurePluginDir(); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Resolve plugin versions to V2 unit release tags such as plugin-release/v4.0.2.
	releaseTag, err := m.registry.ResolveReleaseTag(pluginName, version)
	if err != nil {
		return fmt.Errorf("failed to resolve plugin release: %w", err)
	}

	// Build download URL
	downloadURL, err := m.getPluginDownloadURL(pluginName, releaseTag)
	if err != nil {
		// Check if plugin exists at all
		available, listErr := m.GetAvailablePlugins()
		if listErr == nil {
			var pluginNames []string
			for _, p := range available {
				pluginNames = append(pluginNames, p.Name)
			}
			return fmt.Errorf("%w\n\nAvailable plugins: %s\n\nRun 'neko plugin list' to see all available plugins",
				err, strings.Join(pluginNames, ", "))
		}
		return err
	}

	// Download and extract
	if err := m.downloadAndInstall(pluginName, downloadURL); err != nil {
		return fmt.Errorf("failed to install plugin: %w", err)
	}

	return nil
}

// Uninstall removes an installed plugin and all its files.
//
// Args:
//   - pluginName: The name of the plugin to uninstall
//
// Returns an error if:
//   - The plugin is not installed
//   - The plugin directory cannot be removed
func (m *Manager) Uninstall(pluginName string) error {
	installPath := filepath.Join(m.PluginDir, pluginName)
	if _, err := os.Stat(installPath); os.IsNotExist(err) {
		return fmt.Errorf("plugin '%s' is not installed", pluginName)
	}

	if err := os.RemoveAll(installPath); err != nil {
		return fmt.Errorf("failed to uninstall plugin: %w", err)
	}

	return nil
}

// IsInstalled checks whether a plugin is currently installed.
//
// Args:
//   - pluginName: The name of the plugin to check
//
// Returns true if the plugin directory exists, false otherwise.
func (m *Manager) IsInstalled(pluginName string) bool {
	installPath := filepath.Join(m.PluginDir, pluginName)
	_, err := os.Stat(installPath)
	return err == nil
}

// GetManifest reads and parses the manifest file for an installed plugin.
//
// Args:
//   - pluginName: The name of the plugin
//
// Returns:
//   - A pointer to the parsed Manifest
//   - An error if the manifest file cannot be read or parsed
func (m *Manager) GetManifest(pluginName string) (*Manifest, error) {
	manifestPath := filepath.Join(m.PluginDir, pluginName, "manifest.json")
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

// ListInstalled returns a map of all installed plugins with their versions.
// The map keys are plugin names, and values are version strings.
//
// Returns:
//   - A map of plugin names to versions
//   - An error if the plugin directory cannot be read (except if it doesn't exist,
//     in which case an empty map is returned)
func (m *Manager) ListInstalled() (map[string]string, error) {
	installed := make(map[string]string)

	entries, err := os.ReadDir(m.PluginDir)
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

// GetAvailablePlugins fetches the list of all plugins available in the registry.
//
// Returns:
//   - A slice of AvailablePlugin structs containing plugin metadata
//   - An error if the registry cannot be contacted or the response is invalid
func (m *Manager) GetAvailablePlugins() ([]AvailablePlugin, error) {
	return m.registry.FetchAvailablePlugins()
}

// getPluginDownloadURL constructs the download URL for a specific plugin version.
// It automatically detects the current OS and architecture and maps them to the
// appropriate naming conventions used by goreleaser.
//
// Args:
//   - pluginName: The name of the plugin
//   - version: The version to download
//
// Returns:
//   - The complete download URL
//   - An error if the URL cannot be constructed
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

// downloadAndInstall downloads a plugin from the specified URL and extracts it
// to the plugin directory. The plugin archive is expected to be in tar.gz format.
// Any existing installation of the plugin is removed before extraction.
//
// The function flattens the archive structure, extracting all files directly into
// the plugin directory regardless of their path in the archive.
//
// Args:
//   - pluginName: The name of the plugin being installed
//   - downloadURL: The URL to download the plugin archive from
//
// Returns an error if:
//   - The HTTP request fails
//   - The response status is not 200 OK
//   - The existing plugin directory cannot be removed
//   - The archive cannot be extracted
//   - File creation or copying fails
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
	installPath := filepath.Join(m.PluginDir, pluginName)
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

// httpGetWithAuth performs an HTTP GET request with optional GitHub authentication.
// If the GITHUB_TOKEN environment variable is set, it will be used for authentication,
// allowing access to private repositories.
//
// Args:
//   - url: The URL to request
//
// Returns:
//   - The HTTP response
//   - An error if the request cannot be created or executed
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

// Truncate shortens a string to a maximum length, appending "..." if truncated.
// If the string is already shorter than maxLen, it is returned unchanged.
//
// Args:
//   - s: The string to truncate
//   - maxLen: The maximum length (including the "..." if added)
//
// Returns the truncated string.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
