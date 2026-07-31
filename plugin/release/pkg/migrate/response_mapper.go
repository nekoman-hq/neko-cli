package migrate

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

func mapMigrationCommandResponse(
	result migrationCommandResult,
	failure *migrationFailure,
	timestamp time.Time,
) *plugin.Response {
	responseMetadata := plugin.ResponseMetadata{
		Plugin:    metadata.PluginName,
		Version:   metadata.Version,
		Command:   "migrate",
		Timestamp: timestamp,
	}
	if failure != nil {
		response := &plugin.Response{
			Status:   "error",
			Metadata: responseMetadata,
			Error: &plugin.ResponseError{
				Code:    "MIGRATION_FAILED",
				Message: failure.Error(),
			},
		}
		response.PresentationProperties = migrationFailureProperties()
		response.PresentationTable = migrationFailurePresentation(failure)
		response.SetExitCode(1)
		return response
	}

	plan := result.plan.compatibilityPlan()
	if result.outcome == migrationCompleted {
		plan.Actions = append(plan.Actions, "migration completed")
	}
	response := &plugin.Response{
		Status:   "success",
		Metadata: responseMetadata,
		Data: map[string]any{
			"items":       planItems(plan, result.outcome),
			"actions":     plan.Actions,
			"config_json": plan.ConfigJSON,
			"state_json":  plan.StateJSON,
		},
		RendererHint: "table",
	}
	response.PresentationProperties = migrationSummaryPresentation(result)
	response.PresentationTable = migrationDetailPresentation(result)
	response.SetExitCode(0)
	return response
}

func planItems(plan *Plan, outcome migrationCommandOutcome) []map[string]any {
	status := "migrated"
	if outcome == migrationPreviewed {
		status = "dry-run"
	}
	if plan.AlreadyDone {
		status = "already migrated"
	}
	if plan.Recovery {
		status = "recovery"
		if outcome == migrationPreviewed {
			status = "dry-run recovery"
		}
	}

	return []map[string]any{
		{"property": "Status", "value": status},
		{"property": "Source Type", "value": plan.SourceType},
		{"property": "Source Path", "value": plan.SourcePath},
		{"property": "Config Path", "value": plan.ConfigPath},
		{"property": "State Path", "value": plan.StatePath},
		{"property": "Backup Path", "value": plan.BackupPath},
		{"property": "Journal Path", "value": plan.JournalPath},
		{"property": "Unit ID", "value": plan.UnitID},
		{"property": "Version", "value": plan.Version},
		{"property": "Tag Prefix", "value": plan.TagPrefix},
		{"property": "Executor", "value": plan.Executor},
		{"property": "Delivery", "value": plan.Delivery},
	}
}
