// Package init includes the init handler for plugin-based execution.
package init

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
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

const (
	defaultUnitID           = "cli"
	defaultInitialVersion   = "0.1.0"
	defaultTagPrefix        = "v"
	defaultWorkingDirectory = "."
	defaultPaths            = "**"
	defaultKind             = "release"
	pluginKind              = "plugin"
	legacyV1ConfigFileName  = ".release.neko.json"
)

var pluginInitFlagNames = []string{
	"plugin-name",
	"plugin-manifest",
	"plugin-asset-prefix",
	"plugin-binary-name",
}

// HandleInit handles the init command in plugin mode.
func HandleInit(req plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleInitAt(root, req)
}

// HandleInitAt handles the init command at an explicit repository root without
// changing process cwd.
func HandleInitAt(root workspace.RepositoryRoot, req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Init, "Starting release initialization")
	repository := newV2Repository(root.Path())
	useCase := initializeV2RepositoryUseCase{
		presenceReader: repository,
		validator:      repository,
		writer:         repository,
	}
	result, failure := useCase.Execute(parseInitCommandRequest(req.Flags))
	response := mapInitializeV2Response(result, failure, time.Now())
	if failure != nil {
		if failure.origin == commandFailureFromPresencePolicy {
			log.PluginV(log.Init, "Existing release configuration prevents init: %s", failure.message)
		}
		return response, nil
	}

	log.PluginPrint(log.Init, "Configuration saved to %s", config.V2ConfigPath("."))
	log.PluginPrint(log.Init, "State saved to %s", config.V2StatePath("."))
	log.PluginPrint(log.Init, "Initialization completed successfully")
	return response, nil
}

// HandleUnitAdd appends one unit to an existing V2 release configuration.
func HandleUnitAdd(req plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleUnitAddAt(root, req)
}

// HandleUnitAddAt appends one unit at an explicit repository root without
// changing process cwd.
func HandleUnitAddAt(root workspace.RepositoryRoot, req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Init, "Starting release unit append")
	repository := newV2Repository(root.Path())
	useCase := addV2ReleaseUnitUseCase{
		presenceReader: repository,
		loader:         repository,
		validator:      repository,
		writer:         repository,
	}
	result, failure := useCase.Execute(parseUnitAddCommandRequest(req.Flags))
	response := mapAddV2UnitResponse(result, failure, time.Now())
	if failure != nil {
		if failure.origin == commandFailureFromPresencePolicy {
			log.PluginV(log.Init, "Existing release configuration prevents unit-add: %s", failure.message)
		}
		return response, nil
	}

	log.PluginPrint(log.Init, "Release unit %s appended to %s", result.Unit.UnitID, config.V2ConfigPath("."))
	log.PluginPrint(log.Init, "State entry saved to %s", config.V2StatePath("."))
	return response, nil
}

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

	return &plugin.Response{
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
	}, nil
}
