package remove

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/ui/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/ui/pkg/metadata"
)

// Handle processes the remove command
func Handle(req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Exec, "Removing UI components")

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

	// Get flags and arguments
	removeAll := getFlagBool(req.Flags, "all")

	// Component name can come from argument or flag (for backwards compatibility)
	var componentName string
	if len(req.Args) > 0 {
		componentName = req.Args[0]
	} else {
		componentName = getFlagString(req.Flags, "name")
	}

	// Validate flags
	if !removeAll && componentName == "" {
		return errorResponse(req, "MISSING_ARGUMENT",
			"Either component name or --all must be specified. Usage: neko ui remove <component-name> or neko ui remove --all", nil)
	}

	if removeAll && componentName != "" {
		return errorResponse(req, "CONFLICTING_FLAGS",
			"Cannot use both component name and --all flag together", nil)
	}

	var removedComponents []string
	var skippedComponents []string

	if removeAll {
		log.PluginPrint(log.Exec, "Removing all components")

		// Get all component directories
		entries, err := os.ReadDir(localComponentsPath)
		if err != nil {
			return errorResponse(req, "READ_DIR_FAILED", "Failed to read components directory", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			compName := entry.Name()
			if err := removeComponent(localComponentsPath, compName); err != nil {
				log.PluginPrint(log.Exec, "Failed to remove component %s: %v", compName, err)
				skippedComponents = append(skippedComponents, compName)
				continue
			}

			log.PluginPrint(log.Exec, "Removed component: %s", compName)
			removedComponents = append(removedComponents, compName)
		}

		if len(removedComponents) == 0 && len(skippedComponents) == 0 {
			return errorResponse(req, "NO_COMPONENTS",
				fmt.Sprintf("No components found in %s", localComponentsPath), nil)
		}
	} else {
		log.PluginPrint(log.Exec, "Removing component: %s", componentName)

		// Check if component exists
		if !componentExists(localComponentsPath, componentName) {
			return errorResponse(req, "COMPONENT_NOT_FOUND",
				fmt.Sprintf("Component '%s' not found in %s", componentName, localComponentsPath), nil)
		}

		// Remove single component
		if err := removeComponent(localComponentsPath, componentName); err != nil {
			return errorResponse(req, "REMOVE_FAILED",
				fmt.Sprintf("Failed to remove component '%s'", componentName), err)
		}

		removedComponents = append(removedComponents, componentName)
		log.PluginPrint(log.Exec, "Successfully removed component: %s", componentName)
	}

	// Build response data
	items := make([]map[string]any, 0)
	for _, comp := range removedComponents {
		items = append(items, map[string]any{
			"component": comp,
			"status":    "\uF00C Removed",
		})
	}
	for _, comp := range skippedComponents {
		items = append(items, map[string]any{
			"component": comp,
			"status":    "\uF467 Failed",
		})
	}

	message := fmt.Sprintf("Successfully removed %d component(s)", len(removedComponents))
	if len(skippedComponents) > 0 {
		message += fmt.Sprintf(", %d failed", len(skippedComponents))
	}

	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "remove",
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"items":          items,
			"message":        message,
			"removedCount":   len(removedComponents),
			"skippedCount":   len(skippedComponents),
			"componentsPath": localComponentsPath,
		},
		RendererHint: "table",
	}, nil
}

// removeComponent removes a component directory from the local path
func removeComponent(componentsPath, componentName string) error {
	componentPath := filepath.Join(componentsPath, componentName)

	// Remove the entire component directory
	if err := os.RemoveAll(componentPath); err != nil {
		return fmt.Errorf("failed to remove directory: %w", err)
	}

	log.PluginV(log.Exec, "Removed directory: %s", componentPath)
	return nil
}

// componentExists checks if a component directory exists
func componentExists(componentsPath, componentName string) bool {
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
			Command:   "remove",
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
