// Package validate includes the validate command handler
package validate

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      03.02.2026
*/

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

// HandleValidate validates the release configuration
func HandleValidate(req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Config, "Validating release configuration")

	// Check if config exists
	if !config.Exists() {
		return &plugin.Response{
			Status: "error",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   "validate",
				Timestamp: time.Now(),
			},
			Error: &plugin.ResponseError{
				Code:    "CONFIG_NOT_FOUND",
				Message: "No .release.neko.json configuration found",
				Details: map[string]any{
					"hint": "Run 'neko release init' first to initialize the release configuration",
				},
			},
		}, nil
	}

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return &plugin.Response{
			Status: "error",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   "validate",
				Timestamp: time.Now(),
			},
			Error: &plugin.ResponseError{
				Code:    "CONFIG_INVALID",
				Message: err.Error(),
			},
		}, nil
	}

	// Validate the config
	if err := config.Validate(cfg); err != nil {
		return &plugin.Response{
			Status: "error",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   "validate",
				Timestamp: time.Now(),
			},
			Error: &plugin.ResponseError{
				Code:    "VALIDATION_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	log.PluginPrint(log.Config, "Configuration is valid")

	// Check if --show flag is set
	showConfig := getFlagBool(req.Flags, "show")

	if showConfig {
		return &plugin.Response{
			Status: "success",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   "validate",
				Timestamp: time.Now(),
			},
			Data: map[string]any{
				"items": []map[string]any{
					{
						"property": "Project Name",
						"value":    cfg.ProjectName,
					},
					{
						"property": "Project Owner",
						"value":    cfg.ProjectOwner,
					},
					{
						"property": "Project Type",
						"value":    string(cfg.ProjectType),
					},
					{
						"property": "Release System",
						"value":    string(cfg.ReleaseSystem),
					},
					{
						"property": "Version",
						"value":    cfg.Version,
					},
					{
						"property": "Status",
						"value":    "✓ Valid",
					},
				},
			},
			RendererHint: "table",
		}, nil
	}

	// Simple validation response
	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "validate",
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"items": []map[string]any{
				{
					"property": "Configuration",
					"value":    ".release.neko.json",
				},
				{
					"property": "Status",
					"value":    "✓ Valid",
				},
			},
		},
		RendererHint: "table",
	}, nil
}

func getFlagBool(flags map[string]any, name string) bool {
	if v, ok := flags[name]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
