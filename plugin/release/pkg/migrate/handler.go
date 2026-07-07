package migrate

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

// HandleMigrate handles the V1-to-V2 migration command.
func HandleMigrate(req plugin.Request) (*plugin.Response, error) {
	dryRun := getFlagBool(req.Flags, "dry-run")
	plan, err := Run(req.Context.WorkingDir, dryRun)
	if err != nil {
		return &plugin.Response{
			Status: "error",
			Metadata: plugin.ResponseMetadata{
				Plugin:    metadata.PluginName,
				Version:   metadata.Version,
				Command:   "migrate",
				Timestamp: time.Now(),
			},
			Error: &plugin.ResponseError{
				Code:    "MIGRATION_FAILED",
				Message: err.Error(),
			},
		}, nil
	}

	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "migrate",
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"items":       planItems(plan, dryRun),
			"actions":     plan.Actions,
			"config_json": plan.ConfigJSON,
			"state_json":  plan.StateJSON,
		},
		RendererHint: "table",
	}, nil
}

func planItems(plan *Plan, dryRun bool) []map[string]any {
	status := "migrated"
	if dryRun {
		status = "dry-run"
	}
	if plan.AlreadyDone {
		status = "already migrated"
	}
	if plan.Recovery {
		status = "recovery"
		if dryRun {
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

func getFlagBool(flags map[string]any, key string) bool {
	if value, ok := flags[key]; ok {
		if b, ok := value.(bool); ok {
			return b
		}
	}
	return false
}
