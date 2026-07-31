package init

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

// GetAvailableOptions returns the available options for init configuration.
func GetAvailableOptions() (*plugin.Response, error) {
	items := []map[string]any{
		{
			"option":      "unit",
			"values":      "cli, api, plugin-release, ...",
			"required":    false,
			"description": "Release unit id",
		},
		{
			"option":      "display-name",
			"values":      "string",
			"required":    false,
			"description": "Release unit display name",
		},
		{
			"option":      "version",
			"values":      "semver, default 0.1.0",
			"required":    false,
			"description": "Initial version",
		},
		{
			"option":      "executor",
			"values":      supportedExecutorValues(", "),
			"required":    true,
			"description": "Release executor",
		},
		{
			"option":      "delivery",
			"values":      "github-actions",
			"required":    true,
			"description": "V2 release delivery mode",
		},
		{
			"option":      "workflow",
			"values":      ".github/workflows/*.yml",
			"required":    true,
			"description": "GitHub Actions workflow path",
		},
		{
			"option":      "tag-prefix",
			"values":      "v",
			"required":    false,
			"description": "Release tag prefix",
		},
		{
			"option":      "working-directory",
			"values":      ".",
			"required":    false,
			"description": "Unit working directory",
		},
		{
			"option":      "paths",
			"values":      "comma-separated globs",
			"required":    false,
			"description": "Unit path scope",
		},
		{
			"option":      "kind",
			"values":      "release, plugin",
			"required":    false,
			"description": "release is the default for normal release units; plugin is only for Neko CLI plugins. Plugin fields are invalid unless kind=plugin.",
		},
		{
			"option":      "plugin-name",
			"values":      "release, ui, ...",
			"required":    "when kind=plugin",
			"description": "Only with kind=plugin; public Neko CLI plugin name. Normal repositories do not use plugin fields.",
		},
		{
			"option":      "plugin-manifest",
			"values":      "plugin/<name>/manifest.json",
			"required":    "when kind=plugin",
			"description": "Only with kind=plugin; repository-root-relative Neko CLI plugin manifest path.",
		},
		{
			"option":      "plugin-asset-prefix",
			"values":      "plugin-<name>",
			"required":    "when kind=plugin",
			"description": "Only with kind=plugin; Neko CLI plugin asset prefix, required there and must match unit id.",
		},
		{
			"option":      "plugin-binary-name",
			"values":      "plugin-<name>",
			"required":    "when kind=plugin",
			"description": "Only with kind=plugin; Neko CLI plugin executable name in release archives.",
		},
		{
			"option":      "force",
			"values":      "true, false",
			"required":    false,
			"description": "Overwrite existing V2 config/state",
		},
	}

	response := &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "init-options",
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"items": items,
		},
		RendererHint: "table",
	}
	response.SetExitCode(0)
	return response, nil
}
