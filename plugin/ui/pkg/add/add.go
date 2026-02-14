package add

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

// Handle processes the add command
func Handle(req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Exec, "Adding UI components")

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

	// Ensure components directory exists
	if err := os.MkdirAll(localComponentsPath, 0755); err != nil {
		return errorResponse(req, "CREATE_DIR_FAILED", "Failed to create components directory", err)
	}

	// Get flags and arguments
	addAll := getFlagBool(req.Flags, "all")

	// Component name can come from argument or flag (for backwards compatibility)
	var componentName string
	if len(req.Args) > 0 {
		componentName = req.Args[0]
	} else {
		componentName = getFlagString(req.Flags, "name")
	}

	// Validate flags
	if !addAll && componentName == "" {
		return errorResponse(req, "MISSING_ARGUMENT",
			"Either component name or --all must be specified. Usage: neko ui add <component-name> or neko ui add --all", nil)
	}

	if addAll && componentName != "" {
		return errorResponse(req, "CONFLICTING_FLAGS",
			"Cannot use both component name and --all flag together", nil)
	}

	var addedComponents []string
	var skippedComponents []string

	if addAll {
		log.PluginPrint(log.Exec, "Adding all components from GitHub")

		// Fetch all available components
		availableComponents, err := fetchGitHubComponents()
		if err != nil {
			return errorResponse(req, "GITHUB_FETCH_FAILED", "Failed to fetch components from GitHub", err)
		}

		log.PluginV(log.Exec, "Found %d components to add", len(availableComponents))

		// Download each component
		for _, comp := range availableComponents {
			if err := downloadComponent(comp.Name, localComponentsPath); err != nil {
				log.PluginPrint(log.Exec, "Failed to add component %s: %v", comp.Name, err)
				skippedComponents = append(skippedComponents, comp.Name)
				continue
			}
			log.PluginPrint(log.Exec, "Added component: %s", comp.Name)
			addedComponents = append(addedComponents, comp.Name)
		}
	} else {
		log.PluginPrint(log.Exec, "Adding component: %s", componentName)

		// Download single component
		if err := downloadComponent(componentName, localComponentsPath); err != nil {
			return errorResponse(req, "DOWNLOAD_FAILED",
				fmt.Sprintf("Failed to add component '%s'", componentName), err)
		}

		addedComponents = append(addedComponents, componentName)
		log.PluginPrint(log.Exec, "Successfully added component: %s", componentName)
	}

	// Build response data
	items := make([]map[string]any, 0)
	for _, comp := range addedComponents {
		items = append(items, map[string]any{
			"component": comp,
			"status":    "\uF00C Added",
		})
	}
	for _, comp := range skippedComponents {
		items = append(items, map[string]any{
			"component": comp,
			"status":    "\uF467 Failed",
		})
	}

	message := fmt.Sprintf("Successfully added %d component(s)", len(addedComponents))
	if len(skippedComponents) > 0 {
		message += fmt.Sprintf(", %d failed", len(skippedComponents))
	}

	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "add",
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"items":          items,
			"message":        message,
			"addedCount":     len(addedComponents),
			"skippedCount":   len(skippedComponents),
			"componentsPath": localComponentsPath,
		},
		RendererHint: "table",
	}, nil
}

// downloadComponent downloads a single component from GitHub to the local path
func downloadComponent(componentName, destPath string) error {
	// Create URL for downloading the component folder as zip
	// We'll download the entire repo and extract just the component folder
	zipURL := fmt.Sprintf("https://github.com/%s/archive/refs/heads/%s.zip", githubRepo, githubBranch)

	log.PluginV(log.Exec, "Downloading from: %s", zipURL)

	// Download the zip file
	resp, err := http.Get(zipURL)
	if err != nil {
		return fmt.Errorf("failed to download repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub returned status %d", resp.StatusCode)
	}

	// Read zip into memory
	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read zip data: %w", err)
	}

	// Open zip reader
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}

	// The zip contains files like: neko-ui-main/src/components/Button/...
	// We need to extract: neko-ui-main/src/components/{componentName}/*
	componentPrefix := fmt.Sprintf("neko-ui-%s/%s/%s/", githubBranch, componentsDir, componentName)

	componentFound := false
	destComponentPath := filepath.Join(destPath, componentName)

	// Extract component files
	for _, file := range zipReader.File {
		// Check if file is in the component directory
		if !strings.HasPrefix(file.Name, componentPrefix) {
			continue
		}

		componentFound = true

		// Calculate relative path within component
		relativePath := strings.TrimPrefix(file.Name, componentPrefix)
		if relativePath == "" {
			continue // Skip the component directory itself
		}

		// Skip storybook stories
		if strings.HasSuffix(relativePath, ".stories.tsx") || strings.HasSuffix(relativePath, ".stories.ts") {
			log.PluginV(log.Exec, "Skipping story file: %s", relativePath)
			continue
		}

		targetPath := filepath.Join(destComponentPath, relativePath)

		if file.FileInfo().IsDir() {
			// Create directory
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Extract file
		if err := extractFile(file, targetPath); err != nil {
			return fmt.Errorf("failed to extract file %s: %w", file.Name, err)
		}

		log.PluginV(log.Exec, "Extracted: %s", relativePath)
	}

	if !componentFound {
		return fmt.Errorf("component '%s' not found in repository", componentName)
	}

	return nil
}

// extractFile extracts a single file from zip to the target path
func extractFile(file *zip.File, targetPath string) error {
	// Open file in zip
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Create target file
	outFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Copy contents
	_, err = io.Copy(outFile, rc)
	return err
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
	defer resp.Body.Close()

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
			Command:   "add",
			Timestamp: time.Now(),
		},
		Error: &plugin.ResponseError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}, nil
}

// getFlagString safely extracts a string flag value
func getFlagString(flags map[string]any, name string) string {
	if val, ok := flags[name]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getFlagBool safely extracts a boolean flag value
func getFlagBool(flags map[string]any, name string) bool {
	if val, ok := flags[name]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}
