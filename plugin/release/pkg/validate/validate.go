// Package validate includes the validate command handler
//
//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package validate

//lint:file-ignore SA1019 V1 validation compatibility intentionally uses deprecated V1 APIs during migration

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      03.02.2026
*/

import (
	"fmt"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

// HandleValidate validates the release configuration
func HandleValidate(req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Config, "Validating release configuration")

	repository, err := config.LoadReleaseRepository(".")
	if err == nil && repository.SourceFormat == config.SourceFormatV2 {
		return validateV2Response(req, repository), nil
	}
	if err == nil && repository.SourceFormat == config.SourceFormatV1 {
		return validateV1Response(req, repository)
	}

	if config.V2ConfigExists(".") || config.V1Exists() {
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

	// Check if config exists
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
			Message: "No release configuration found",
			Details: map[string]any{
				"hint": "Run 'neko release init' for V1 or add .neko/release.config.json and .neko/release.state.json for V2",
			},
		},
	}, nil
}

func validateV1Response(req plugin.Request, repository *config.ReleaseRepository) (*plugin.Response, error) {
	unit, err := config.ResolveReleaseUnit(repository, getFlagString(req.Flags, "unit"), config.UnitResolutionOptions{})
	if err != nil {
		return validationError("UNIT_RESOLUTION_FAILED", err.Error()), nil
	}

	cfg := repository.Legacy
	// Validate the config
	if err := config.V1Validate(cfg); err != nil {
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

	if err := release.ValidateRequirements(cfg); err != nil {
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
				{
					"property": "Unit",
					"value":    unit.ID,
				},
			},
		},
		RendererHint: "table",
	}, nil
}

func validateV2Response(req plugin.Request, repository *config.ReleaseRepository) *plugin.Response {
	log.PluginPrint(log.Config, "V2 release configuration is valid")

	showConfig := getFlagBool(req.Flags, "show")
	requestedUnit := getFlagString(req.Flags, "unit")
	var focusedUnit *config.ReleaseUnit
	if requestedUnit != "" {
		unit, err := config.ResolveReleaseUnit(repository, requestedUnit, config.UnitResolutionOptions{})
		if err != nil {
			return validationError("UNIT_RESOLUTION_FAILED", err.Error())
		}
		focusedUnit = unit
	}
	items := []map[string]any{
		{
			"property": "Configuration",
			"value":    ".neko/release.config.json",
		},
		{
			"property": "Schema",
			"value":    "v2",
		},
		{
			"property": "Status",
			"value":    "✓ Valid",
		},
	}

	if showConfig {
		items = []map[string]any{
			{
				"property": "Schema",
				"value":    "v2",
			},
		}
		units := repository.Units
		if focusedUnit != nil {
			units = []config.ReleaseUnit{*focusedUnit}
		}
		for _, unit := range units {
			items = append(items, unitShowRow(unit))
		}
	}

	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "validate",
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"items": items,
		},
		RendererHint: "table",
	}
}

func unitShowRow(unit config.ReleaseUnit) map[string]any {
	parts := []string{
		fmt.Sprintf("version=%s", unit.Version),
		fmt.Sprintf("workingDirectory=%s", unit.WorkingDirectory),
		fmt.Sprintf("tagPrefix=%s", unit.TagPrefix),
		fmt.Sprintf("executor=%s", unit.ExecutorType),
		fmt.Sprintf("delivery=%s", unit.Delivery),
		fmt.Sprintf("workflow=%s", workflowShowValue(unit)),
		fmt.Sprintf("paths=%v", unit.Paths),
	}
	if unit.Kind != "" {
		parts = append(parts, fmt.Sprintf("kind=%s", unit.Kind))
	}
	if unit.IsPlugin {
		parts = append(parts,
			fmt.Sprintf("plugin=%s", unit.PluginName),
			fmt.Sprintf("pluginManifest=%s", unit.PluginManifestPath),
			fmt.Sprintf("pluginAssetPrefix=%s", unit.PluginAssetPrefix),
			fmt.Sprintf("pluginBinary=%s", unit.PluginBinaryName),
		)
	}
	return map[string]any{
		"property": fmt.Sprintf("Unit %s", unit.ID),
		"value":    strings.Join(parts, " "),
	}
}

func workflowShowValue(unit config.ReleaseUnit) string {
	if unit.Workflow == "" {
		return "not applicable"
	}
	return unit.Workflow
}

func validationError(code, message string) *plugin.Response {
	return &plugin.Response{
		Status: "error",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "validate",
			Timestamp: time.Now(),
		},
		Error: &plugin.ResponseError{
			Code:    code,
			Message: message,
		},
	}
}

func getFlagBool(flags map[string]any, name string) bool {
	if v, ok := flags[name]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func getFlagString(flags map[string]any, name string) string {
	if v, ok := flags[name]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
