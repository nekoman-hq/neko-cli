package initialize

import (
	"fmt"
	"os"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/ui/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/ui/pkg/metadata"
)

// Handle processes the init command
func Handle(req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Init, "Starting UI plugin initialization")

	// Get project root - use current directory
	projectRoot, err := os.Getwd()
	if err != nil {
		return errorResponse(req, "GETWD_FAILED", "Failed to get current directory", err)
	}

	log.PluginV(log.Init, "Project root: %s", projectRoot)

	// Check if config already exists
	if config.Exists(projectRoot) {
		force := getFlagBool(req.Flags, "force")
		if !force {
			return errorResponse(req, "CONFIG_EXISTS",
				fmt.Sprintf("Config file already exists at %s/%s. Use --force to overwrite.", projectRoot, config.Filename), nil)
		}
		log.PluginPrint(log.Init, "Force flag set, overwriting existing config")
	}

	// Get components path from flags - check if it was actually provided by user
	// Check if flag exists in the map first
	componentsPathRaw, exists := req.Flags["components-path"]
	if !exists {
		return errorResponse(req, "MISSING_REQUIRED_FLAG",
			"The --components-path flag is required. Example: neko ui init --components-path=shared/ui", nil)
	}

	// Then check if it's a non-empty string
	componentsPath, ok := componentsPathRaw.(string)
	if !ok || componentsPath == "" {
		return errorResponse(req, "MISSING_REQUIRED_FLAG",
			"The --components-path flag is required and must be a non-empty string. Example: neko ui init --components-path=shared/ui", nil)
	}

	log.PluginV(log.Init, "Components path: %s", componentsPath)

	// Create or update config
	if config.Exists(projectRoot) {
		err = config.Update(projectRoot, componentsPath)
	} else {
		err = config.Create(projectRoot, componentsPath)
	}

	if err != nil {
		return errorResponse(req, "CONFIG_CREATE_FAILED", "Failed to create config file", err)
	}

	// Load config to verify
	cfg, err := config.Load(projectRoot)
	if err != nil {
		return errorResponse(req, "CONFIG_LOAD_FAILED", "Failed to load created config", err)
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		return errorResponse(req, "CONFIG_INVALID", "Config validation failed", err)
	}

	// Create components directory if it doesn't exist
	componentsDir := cfg.GetComponentsPath(projectRoot)
	if err := os.MkdirAll(componentsDir, 0755); err != nil {
		log.PluginPrint(log.Init, "Warning: Failed to create components directory: %s", err.Error())
		// Don't fail here, just warn
	} else {
		log.PluginV(log.Init, "Created components directory: %s", componentsDir)
	}

	log.PluginPrint(log.Init, "UI plugin initialized successfully")

	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "init",
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"items": []map[string]any{
				{
					"setting": "Config File",
					"value":   config.Filename,
				},
				{
					"setting": "Components Path",
					"value":   cfg.ComponentsPath,
				},
				{
					"setting": "Full Path",
					"value":   componentsDir,
				},
			},
			"message": "UI plugin initialized successfully",
		},
		RendererHint: "table",
	}, nil
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
			Command:   "init",
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
