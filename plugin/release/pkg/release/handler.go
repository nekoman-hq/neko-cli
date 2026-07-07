// Package release includes all neko cli release logic
package release

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      03.02.2026
*/

import (
	"strings"
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
		ctx, ctxErr := BuildReleaseExecutionContext(repository, *unit, releaseType, dryRun)
		if ctxErr != nil {
			return commandErrorResponse(string(releaseType), "EXECUTION_CONTEXT_FAILED", ctxErr.Error()), nil
		}
		if dryRun {
			if validationErr := ValidateRequirementsForContext(ctx); validationErr != nil {
				return commandErrorResponse(string(releaseType), "VALIDATION_FAILED", validationErr.Error()), nil
			}
			return v2DryRunPlanResponse(string(releaseType), ctx)
		}
		return commandErrorResponse(string(releaseType), "V2_PUBLICATION_ADAPTERS_UNAVAILABLE", v2GitCoordinationUnavailableMessage), nil
	}

	cfg := repository.Legacy
	ctx, err := BuildReleaseExecutionContext(repository, *unit, releaseType, dryRun)
	if err != nil {
		return commandErrorResponse(string(releaseType), "EXECUTION_CONTEXT_FAILED", err.Error()), nil
	}

	// Create release service
	svc := NewReleaseServiceWithContext(cfg, ctx)

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

	if err = ValidateRequirementsForContext(ctx); err != nil {
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

func v2DryRunPlanResponse(command string, ctx *ReleaseExecutionContext) (*plugin.Response, error) {
	if ctx == nil {
		return commandErrorResponse(command, "EXECUTION_CONTEXT_FAILED", "release execution context is missing"), nil
	}
	plan := BuildReleasePlan(ctx)
	materializer, err := ResolveVersionMaterializer(ctx.Executor)
	if err != nil {
		return commandErrorResponse(command, "MATERIALIZATION_FAILED", err.Error()), nil
	}
	materializationPlan, err := materializer.Plan(ctx)
	if err != nil {
		return commandErrorResponse(command, "MATERIALIZATION_FAILED", err.Error()), nil
	}
	if validationErr := materializer.Validate(materializationPlan); validationErr != nil {
		return commandErrorResponse(command, "MATERIALIZATION_FAILED", validationErr.Error()), nil
	}
	knownFiles, err := NewKnownReleaseFiles(ctx, materializationPlan)
	if err != nil {
		return commandErrorResponse(command, "GIT_COORDINATION_FAILED", err.Error()), nil
	}
	commitMessage := ReleaseCommitMessage(ctx)
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
				{"property": "Unit", "value": ctx.Unit.ID},
				{"property": "Current Version", "value": ctx.CurrentVersion},
				{"property": "New Version", "value": ctx.NextVersion},
				{"property": "Tag", "value": ctx.Tag},
				{"property": "Executor", "value": ctx.Executor},
				{"property": "Delivery", "value": ctx.Delivery},
				{"property": "Working Directory", "value": ctx.Unit.WorkingDirectory},
				{"property": "Unit Root", "value": ctx.UnitRoot},
				{"property": "State Change", "value": plan.StateChange},
				{"property": "Materialized Files", "value": materializedFilesValue(materializationPlan)},
				{"property": "Known Release Files", "value": strings.Join(knownFiles.RelativePaths(), ", ")},
				{"property": "Planned Release Commit", "value": commitMessage},
				{"property": "Planned Tag", "value": ctx.Tag},
				{"property": "Planned Push Order", "value": "1. release commit, 2. unit tag"},
				{"property": "Tool Ownership", "value": plan.OwnershipSummary},
				{"property": "V2 Git Ownership", "value": plan.V2GitOwnership},
				{"property": "State Commit Guarantee", "value": plan.StateGuarantee},
				{"property": "Executor Start", "value": "no"},
				{"property": "Dry Run", "value": "yes"},
				{"property": "Status", "value": "V2 preview - no changes made"},
			},
		},
		RendererHint: "table",
	}, nil
}

func materializedFilesValue(plan *MaterializationPlan) string {
	if plan == nil || len(plan.Changes) == 0 {
		if plan != nil && plan.BlockedReason != "" {
			return "blocked: " + plan.BlockedReason
		}
		return "none"
	}
	values := make([]string, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		values = append(values, change.RepositoryRelativePath)
	}
	return strings.Join(values, ", ")
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

// V2ExecutionUnavailableResponse documents the temporary Milestone-4A boundary:
// V2 can prepare local delivery and executor context, but mutating execution is
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
			Message: "release schema v2 has Local Delivery and ExecutorContext prepared, but actual V2 release execution is not available until the next milestone",
		},
	}
}
