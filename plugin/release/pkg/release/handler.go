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

	repository, err := config.LoadReleaseRepository(".")
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

	dryRun := getFlagBool(req.Flags, "dry-run")
	unit, err := config.ResolveReleaseUnit(repository, getFlagString(req.Flags, "unit"), config.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		return commandErrorResponse(string(releaseType), "UNIT_RESOLUTION_FAILED", err.Error()), nil
	}

	if repository.SourceFormat == config.SourceFormatV2 {
		if dryRun {
			return v2DryRunPlanResponse(string(releaseType), *unit, releaseType)
		}
		return V2ExecutionUnavailableResponse(string(releaseType)), nil
	}

	cfg := repository.Legacy

	// Create release service
	svc := NewReleaseService(cfg)

	// Dry-run planning must stay read-only: no release-tool requirements, no
	// executor lookup, no remote refresh, and no file writes are allowed.

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

	if err = ValidateRequirements(cfg); err != nil {
		return &plugin.Response{
			Status: "error",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   string(releaseType),
				Timestamp: time.Now(),
			},
			Error: &plugin.ResponseError{
				Code:    "VALIDATION_FAILED",
				Message: err.Error(),
			},
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

func getFlagString(flags map[string]any, name string) string {
	if v, ok := flags[name]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func v2DryRunPlanResponse(command string, unit config.ReleaseUnit, releaseType Type) (*plugin.Response, error) {
	plan, err := PlanUnitVersionBump(unit, releaseType)
	if err != nil {
		return commandErrorResponse(command, "VERSION_ERROR", err.Error()), nil
	}
	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   command,
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"items": []map[string]any{
				{"property": "Release Type", "value": command},
				{"property": "Unit", "value": plan.UnitID},
				{"property": "Current Version", "value": plan.CurrentVersion},
				{"property": "New Version", "value": plan.NextVersion},
				{"property": "Tag", "value": plan.Tag},
				{"property": "Executor", "value": plan.Executor},
				{"property": "Delivery", "value": plan.Delivery},
				{"property": "Working Directory", "value": plan.WorkingDirectory},
				{"property": "Dry Run", "value": "yes"},
				{"property": "Status", "value": "V2 preview - no changes made"},
			},
		},
		RendererHint: "table",
	}, nil
}

func commandErrorResponse(command, code, message string) *plugin.Response {
	return &plugin.Response{
		Status: "error",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   command,
			Timestamp: time.Now(),
		},
		Error: &plugin.ResponseError{
			Code:    code,
			Message: message,
		},
	}
}

// V2ExecutionUnavailableResponse documents the temporary Milestone-2 boundary:
// V2 can plan releases and read unit-aware history, but mutating execution is
// not active yet.
func V2ExecutionUnavailableResponse(command string) *plugin.Response {
	return &plugin.Response{
		Status: "error",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   command,
			Timestamp: time.Now(),
		},
		Error: &plugin.ResponseError{
			Code:    "V2_EXECUTION_UNAVAILABLE",
			Message: "release schema v2 supports planning and read-only history, but release execution is not available yet",
		},
	}
}
