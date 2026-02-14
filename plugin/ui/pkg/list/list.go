package list

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/ui/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/ui/pkg/metadata"
)

const (
	githubRepo    = "nekoman-hq/neko-ui"
	githubBranch  = "main"
	componentsDir = "src/components"
)

// GitHubContent represents a file or directory in GitHub API response
type GitHubContent struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "file" or "dir"
	URL  string `json:"url"`
}

// Handle processes the list command
func Handle(req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Exec, "Listing available UI components")

	// Get project root
	projectRoot, err := os.Getwd()
	if err != nil {
		return errorResponse(req, "GETWD_FAILED", "Failed to get current directory", err)
	}

	// Check if config exists
	if !config.Exists(projectRoot) {
		return errorResponse(req, "CONFIG_NOT_FOUND",
			"Config file not found. Run 'neko ui init --components-path=<path>' first.", nil)
	}

	// Load config to get components path
	cfg, err := config.Load(projectRoot)
	if err != nil {
		return errorResponse(req, "CONFIG_LOAD_FAILED", "Failed to load config", err)
	}

	localComponentsPath := cfg.GetComponentsPath(projectRoot)
	log.PluginV(log.Exec, "Local components path: %s", localComponentsPath)

	// Fetch available components from GitHub
	availableComponents, err := fetchGitHubComponents()
	if err != nil {
		return errorResponse(req, "GITHUB_FETCH_FAILED", "Failed to fetch components from GitHub", err)
	}

	log.PluginV(log.Exec, "Found %d components in GitHub repo", len(availableComponents))

	// Check which components are installed locally
	installedCount := 0
	items := make([]map[string]any, 0, len(availableComponents))
	for _, comp := range availableComponents {
		installed := isComponentInstalled(localComponentsPath, comp.Name)

		var status, statusText string
		if installed {
			status = "✓"
			statusText = "Installed"
			installedCount++
		} else {
			status = "✗"
			statusText = "Not installed"
		}

		items = append(items, map[string]any{
			"status":    status,
			"name":      comp.Name,
			"installed": statusText,
		})

		log.PluginV(log.Exec, "Component %s: %s", comp.Name, statusText)
	}

	summary := fmt.Sprintf("%d/%d components installed", installedCount, len(availableComponents))

	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "list",
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"items":          items,
			"summary":        summary,
			"repository":     fmt.Sprintf("github.com/%s", githubRepo),
			"branch":         githubBranch,
			"componentsPath": cfg.ComponentsPath,
		},
		RendererHint: "table",
	}, nil
}

// fetchGitHubComponents fetches the list of component directories from GitHub
func fetchGitHubComponents() ([]GitHubContent, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s?ref=%s",
		githubRepo, componentsDir, githubBranch)

	log.PluginV(log.Exec, "Fetching from GitHub API: %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add GitHub API headers
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Check for GitHub token in environment (optional, for higher rate limits)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		log.PluginV(log.Exec, "Using GitHub token for authentication")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from GitHub: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var contents []GitHubContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub response: %w", err)
	}

	// Filter to only include directories (components)
	var components []GitHubContent
	for _, item := range contents {
		if item.Type == "dir" {
			components = append(components, item)
		}
	}

	return components, nil
}

// isComponentInstalled checks if a component directory exists locally
func isComponentInstalled(componentsPath, componentName string) bool {
	componentPath := filepath.Join(componentsPath, componentName)
	info, err := os.Stat(componentPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// errorResponse creates a standardized error response
func errorResponse(req plugin.Request, code, message string, err error) (*plugin.Response, error) {
	details := map[string]any{}
	if err != nil {
		details["error"] = err.Error()
		log.PluginPrint(log.Exec, "%s: %s - %v", code, message, err)
	} else {
		log.PluginPrint(log.Exec, "%s: %s", code, message)
	}

	return &plugin.Response{
		Status: "error",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "list",
			Timestamp: time.Now(),
		},
		Error: &plugin.ResponseError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}, nil
}
