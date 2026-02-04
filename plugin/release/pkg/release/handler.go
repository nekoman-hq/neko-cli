// Package release includes all neko cli release logic
package release

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

// HandleRelease handles the patch, minor, major release commands
func HandleRelease(req plugin.Request, releaseType Type) (*plugin.Response, error) {
	log.PluginPrint(log.Exec, "Starting %s release", string(releaseType))

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return &plugin.Response{
			Status: "error",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   string(releaseType),
				Timestamp: time.Now(),
			},
			Error: &plugin.ResponseError{
				Code:    "CONFIG_NOT_FOUND",
				Message: err.Error(),
				Details: map[string]any{
					"hint": "Run 'neko release init' first to initialize the release configuration",
				},
			},
		}, nil
	}

	// Create release service
	svc := NewReleaseService(cfg)

	// Get metadata info for response
	oldVersion, newVersion, err := svc.GetNewVersion(releaseType)
	if err != nil {
		return &plugin.Response{
			Status: "error",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   string(releaseType),
				Timestamp: time.Now(),
			},
			Error: &plugin.ResponseError{
				Code:    "VERSION_ERROR",
				Message: err.Error(),
			},
		}, nil
	}

	// Check for dry-run flag
	dryRun := getFlagBool(req.Flags, "dry-run")
	if dryRun {
		log.PluginPrint(log.Exec, "Dry run mode - no changes will be made")
		return &plugin.Response{
			Status: "success",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   string(releaseType),
				Timestamp: time.Now(),
			},
			Data: map[string]any{
				"items": []map[string]any{
					{
						"property": "Release Type",
						"value":    string(releaseType),
					},
					{
						"property": "Current Version",
						"value":    oldVersion.String(),
					},
					{
						"property": "New Version",
						"value":    newVersion.String(),
					},
					{
						"property": "Release System",
						"value":    string(cfg.ReleaseSystem),
					},
					{
						"property": "Dry Run",
						"value":    "yes",
					},
					{
						"property": "Status",
						"value":    "Preview - no changes made",
					},
				},
			},
			RendererHint: "table",
		}, nil
	}

	// Execute release
	if err := svc.Run(releaseType); err != nil {
		return &plugin.Response{
			Status: "error",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   string(releaseType),
				Timestamp: time.Now(),
			},
			Error: &plugin.ResponseError{
				Code:    "RELEASE_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   string(releaseType),
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"items": []map[string]any{
				{
					"property": "Release Type",
					"value":    string(releaseType),
				},
				{
					"property": "Previous Version",
					"value":    oldVersion.String(),
				},
				{
					"property": "New Version",
					"value":    newVersion.String(),
				},
				{
					"property": "Release System",
					"value":    string(cfg.ReleaseSystem),
				},
				{
					"property": "Status",
					"value":    "Released successfully",
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
