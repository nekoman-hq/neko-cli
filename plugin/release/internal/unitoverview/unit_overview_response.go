package unitoverview

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

var unitOverviewPresentationColumns = []presentation.Column{
	{Key: "unit", Label: "Unit", Essential: true},
	{Key: "version", Label: "Version", Essential: true},
	{Key: "status", Label: "Status", Essential: true},
	{Key: "kind", Label: "Kind"},
	{Key: "executor", Label: "Executor"},
	{Key: "delivery", Label: "Delivery"},
	{Key: "issues", Label: "Issues"},
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
		Metadata:     unitOverviewResponseMetadata(unitOverviewCommandName, timestamp),
		Data:         data,
		RendererHint: "table",
		PresentationTable: &presentation.Table{
			Title:   "Release Units",
			Columns: append([]presentation.Column(nil), unitOverviewPresentationColumns...),
			Rows:    unitOverviewDefaultRows(result.Units),
		},
	}
	response.PresentationTable.Following = unitOverviewFollowingTables(result)
	if len(result.Units) == 0 && result.SourceIssue != nil {
		response.PresentationProperties = &presentation.Properties{Properties: []presentation.Property{
			{Label: "Status", Value: result.Status},
			{Label: "Issue", Value: result.SourceIssue.Code},
			{Label: "Message", Value: result.SourceIssue.Message},
			{Label: "Remediation", Value: result.SourceIssue.Remediation},
		}}
		response.PresentationTable = unitOverviewLimitationsTable()
	}
	if result.Status != unitOverviewValid {
		response.ExitCode = 1
	}
	return response
}

func unitOverviewDefaultRows(units []unitOverviewRow) []map[string]any {
	rows := make([]map[string]any, 0, len(units))
	for _, unit := range units {
		rows = append(rows, map[string]any{
			"unit": unit.ID, "version": unitOverviewVersionValue(unit),
			"status": unitOverviewReadinessValue(unit), "kind": unitOverviewKindValue(unit),
			"executor": unit.Executor, "delivery": unit.Delivery,
			"issues": strings.Join(unitOverviewReadableIssueCodes(unit.Issues), ", "),
		})
	}
	return rows
}

func unitOverviewVersionValue(unit unitOverviewRow) string {
	if unit.ConfiguredVersion != "" {
		return unit.ConfiguredVersion
	}
	return "missing"
}

func unitOverviewReadinessValue(unit unitOverviewRow) string {
	if len(unit.Issues) == 0 && unit.Alignment == unitOverviewAligned {
		return "ready"
	}
	return "has issues"
}

func unitOverviewKindValue(unit unitOverviewRow) string {
	if strings.TrimSpace(unit.Kind) == "" {
		return "release"
	}
	return unitOverviewReadableLabel(unit.Kind)
}

func unitOverviewReadableIssueCodes(issues []unitOverviewIssue) []string {
	values := make([]string, 0, len(issues))
	for _, issue := range issues {
		values = append(values, unitOverviewReadableLabel(strings.TrimPrefix(issue.Code, "UNIT_")))
	}
	return values
}

func unitOverviewFollowingTables(result *unitOverviewResult) *presentation.Table {
	tables := []*presentation.Table{
		unitOverviewIssuesTable(result.Units),
		unitOverviewDetailsTable(result.Units),
		unitOverviewLimitationsTable(),
	}
	var first *presentation.Table
	var tail *presentation.Table
	for _, table := range tables {
		if table == nil {
			continue
		}
		if first == nil {
			first = table
			tail = table
			continue
		}
		tail.Following = table
		tail = table
	}
	return first
}

func unitOverviewIssuesTable(units []unitOverviewRow) *presentation.Table {
	rows := make([]map[string]any, 0)
	for _, unit := range units {
		for _, issue := range unit.Issues {
			rows = append(rows, map[string]any{
				"unit": unit.ID, "status": unitOverviewReadableLabel(string(issue.Severity)),
				"problem": unitOverviewReadableLabel(strings.TrimPrefix(issue.Code, "UNIT_")),
				"reason":  issue.Message, "guidance": issue.Remediation,
			})
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return &presentation.Table{
		Title: "Issues",
		Columns: []presentation.Column{
			{Key: "unit", Label: "Unit", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "problem", Label: "Problem", Essential: true},
			{Key: "reason", Label: "Reason"},
			{Key: "guidance", Label: "Guidance"},
		},
		Rows: rows,
	}
}

func unitOverviewDetailsTable(units []unitOverviewRow) *presentation.Table {
	if len(units) == 0 {
		return nil
	}
	rows := make([]map[string]any, 0, len(units))
	for _, unit := range units {
		rows = append(rows, map[string]any{
			"unit": unit.ID, "source": unitOverviewSourceOwnership(unit),
			"status": unitOverviewReadableLabel(string(unit.Alignment)),
			"kind":   unitOverviewKindValue(unit), "version": unitOverviewVersionValue(unit),
			"tag":      unit.TagShape,
			"executor": unit.Executor, "delivery": unit.Delivery,
			"workflow": unitOverviewSafePath(unit.WorkflowPath),
		})
	}
	return &presentation.Table{
		Title: "Unit Details", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "unit", Label: "Unit", Essential: true},
			{Key: "source", Label: "Source ownership", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "kind", Label: "Kind"},
			{Key: "version", Label: "Version"},
			{Key: "tag", Label: "Tag shape"},
			{Key: "executor", Label: "Executor"},
			{Key: "delivery", Label: "Delivery"},
			{Key: "workflow", Label: "Workflow"},
		},
		Rows: rows,
		Details: &presentation.Properties{
			Properties: unitOverviewDetailProperties(units),
		},
	}
}

func unitOverviewSourceOwnership(unit unitOverviewRow) string {
	switch {
	case unit.ConfigPresent && unit.StatePresent:
		return "Config and state"
	case unit.ConfigPresent:
		return "Config only"
	case unit.StatePresent:
		return "State only"
	default:
		return "Unresolved"
	}
}

func unitOverviewDetailProperties(units []unitOverviewRow) []presentation.Property {
	properties := make([]presentation.Property, 0, len(units)*14)
	for _, unit := range units {
		properties = append(properties, presentation.Property{
			Label: "Unit " + unit.ID, Heading: true, Emphasized: true,
		})
		if unit.DisplayName != "" {
			properties = append(properties, presentation.Property{Label: "Display name", Value: unit.DisplayName})
		}
		properties = append(properties,
			presentation.Property{Label: "Source ownership", Value: unitOverviewSourceOwnership(unit)},
			presentation.Property{Label: "Alignment", Value: unitOverviewReadableLabel(string(unit.Alignment))},
			presentation.Property{Label: "Configured version", Value: unitOverviewVersionValue(unit)},
			presentation.Property{Label: "Tag prefix", Value: unit.TagPrefix},
			presentation.Property{Label: "Tag shape", Value: unit.TagShape},
			presentation.Property{Label: "Configured tag", Value: unit.ConfiguredTag},
			presentation.Property{Label: "Executor", Value: unit.Executor},
			presentation.Property{Label: "Delivery", Value: unit.Delivery},
			presentation.Property{Label: "Workflow", Value: unitOverviewSafePath(unit.WorkflowPath)},
			presentation.Property{Label: "Working directory", Value: unitOverviewSafePath(unit.WorkingDirectory)},
			presentation.Property{Label: "Declared paths", Value: unitOverviewStringList(unit.DeclaredPaths)},
		)
		if unit.PluginName != "" {
			properties = append(properties,
				presentation.Property{Label: "Plugin name", Value: unit.PluginName},
				presentation.Property{Label: "Plugin manifest", Value: unitOverviewSafePath(unit.PluginManifest)},
				presentation.Property{Label: "Plugin asset prefix", Value: unit.PluginAssetPrefix},
				presentation.Property{Label: "Plugin binary", Value: unit.PluginBinaryName},
			)
		}
		for _, issue := range unit.Issues {
			properties = append(properties,
				presentation.Property{
					Label: unitOverviewReadableLabel(strings.TrimPrefix(issue.Code, "UNIT_")),
					Role:  unitOverviewIssueRole(issue.Severity), Heading: true, Emphasized: true,
				},
				presentation.Property{Label: "Reason", Value: issue.Message},
				presentation.Property{Label: "Guidance", Value: issue.Remediation},
			)
		}
	}
	return properties
}

func unitOverviewIssueRole(severity unitOverviewIssueSeverity) presentation.StyleRole {
	if severity == unitOverviewIssueError {
		return presentation.StyleError
	}
	return presentation.StyleWarning
}

func unitOverviewLimitationsTable() *presentation.Table {
	return &presentation.Table{
		Title: "Limitations", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "area", Label: "Area", Essential: true},
			{Key: "scope", Label: "Scope", Essential: true},
			{Key: "details", Label: "Details"},
		},
		Rows: []map[string]any{
			{"area": "Workflow", "scope": "Configuration only", "details": "Workflow contents are not inspected."},
			{"area": "Repository", "scope": "Local source only", "details": "Git and remote provider state are not inspected."},
			{"area": "Planning", "scope": "Current state only", "details": "No next version or release operation is planned."},
		},
	}
}

func unitOverviewReadableLabel(value string) string {
	fields := strings.Fields(strings.NewReplacer("_", " ", "-", " ").Replace(strings.TrimSpace(value)))
	for index, field := range fields {
		lower := strings.ToLower(field)
		fields[index] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(fields, " ")
}

func unitOverviewSafePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "not configured"
	}
	if filepath.IsAbs(value) {
		return "repository-local path"
	}
	return filepath.ToSlash(value)
}

func unitOverviewStringList(values []string) string {
	if len(values) == 0 {
		return "none configured"
	}
	safe := make([]string, 0, len(values))
	for _, value := range values {
		safe = append(safe, unitOverviewSafePath(value))
	}
	return strings.Join(safe, "\n")
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
