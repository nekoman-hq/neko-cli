package release

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

var unitOverviewHumanColumns = []plugin.HumanColumn{
	{Key: "id", Label: "Unit", Essential: true},
	{Key: "configured_version", Label: "Version", Essential: true},
	{Key: "alignment", Label: "Status", Essential: true},
	{Key: "display_name", Label: "Name"},
	{Key: "tag_prefix", Label: "Tag prefix"},
	{Key: "executor", Label: "Executor"},
	{Key: "delivery", Label: "Delivery"},
	{Key: "workflow_path", Label: "Workflow"},
	{Key: "working_directory", Label: "Working directory"},
	{Key: "issue_codes", Label: "Issues"},
}

func mapUnitOverviewResult(result *unitOverviewResult, timestamp time.Time) *plugin.Response {
	units := make([]map[string]any, 0, len(result.Units))
	for _, row := range result.Units {
		units = append(units, unitOverviewMachineRow(row))
	}
	data := map[string]any{
		"status":         result.Status,
		"summary":        result.Summary,
		"units":          units,
		"workflow_paths": append(make([]string, 0, len(result.WorkflowPaths)), result.WorkflowPaths...),
	}
	if result.SourceIssue != nil {
		data["source_issue"] = *result.SourceIssue
	}
	response := &plugin.Response{
		Status:       "success",
		Metadata:     commandResponseMetadata(unitOverviewCommandName, timestamp),
		Data:         data,
		RendererHint: "table",
		HumanTable: &plugin.HumanTable{
			Columns: append([]plugin.HumanColumn(nil), unitOverviewHumanColumns...),
		},
	}
	if len(result.Units) == 0 && result.SourceIssue != nil {
		response.HumanTable = nil
		response.HumanProperties = &plugin.HumanProperties{Properties: []plugin.HumanProperty{
			{Label: "Status", Value: result.Status},
			{Label: "Issue", Value: result.SourceIssue.Code},
			{Label: "Message", Value: result.SourceIssue.Message},
			{Label: "Remediation", Value: result.SourceIssue.Remediation},
		}}
	}
	if result.Status != unitOverviewValid {
		response.ExitCode = 1
	}
	return response
}

func unitOverviewMachineRow(row unitOverviewRow) map[string]any {
	value := map[string]any{
		"id":          row.ID,
		"alignment":   row.Alignment,
		"issues":      append(make([]unitOverviewIssue, 0, len(row.Issues)), row.Issues...),
		"issue_codes": unitOverviewIssueCodes(row.Issues),
	}
	if row.DisplayName != "" {
		value["display_name"] = row.DisplayName
	}
	if row.Version != "" {
		value["version"] = row.Version
	}
	if row.StatePresent {
		value["configured_version"] = row.ConfiguredVersion
	}
	if row.ConfigPresent {
		value["tag_prefix"] = row.TagPrefix
		value["executor"] = row.Executor
		value["delivery"] = row.Delivery
		value["workflow_path"] = row.WorkflowPath
		value["working_directory"] = row.WorkingDirectory
	}
	if row.TagShape != "" {
		value["tag_shape"] = row.TagShape
	}
	if row.ConfiguredTag != "" {
		value["configured_tag"] = row.ConfiguredTag
	}
	return value
}
