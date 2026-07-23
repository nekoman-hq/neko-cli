package release

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

// MapReleasePlanInspection renders read-only release-plan inspection facts.
func MapReleasePlanInspection(result *ReleasePlanInspection, timestamp time.Time) *plugin.Response {
	if result == nil {
		return successTableResponse("plan", timestamp, nil)
	}
	response := successTableResponse("plan", timestamp, releasePlanMachineItems(result))
	response.PresentationProperties = &presentation.Properties{
		Title:      "Release Plan",
		Properties: releasePlanSummaryProperties(result),
	}
	response.PresentationTable = releasePlanPresentationTables(result)
	return response
}

type releasePlanResponseProperty struct {
	value any
	label string
}

func releasePlanSharedProperties(result *ReleasePlanInspection) []releasePlanResponseProperty {
	return []releasePlanResponseProperty{
		{label: "Source", value: string(result.Source)},
		{label: "Unit", value: result.Unit.ID},
		{label: "Display Name", value: emptyFallback(result.Unit.DisplayName, "not configured")},
		{label: "Current Version", value: result.CurrentVersion},
		{label: "Requested Change", value: string(result.RequestedChange)},
		{label: "Next Version", value: result.NextVersion},
		{label: "Tag", value: result.Tag},
		{label: "Executor", value: result.Executor},
		{label: "Delivery", value: result.Delivery},
		{label: "Workflow", value: workflowResponseValue(result.Workflow)},
		{label: "Working Directory", value: result.WorkingDirectory},
		{label: "Unit Root", value: result.UnitRoot},
		{label: "Planned Materialized Files", value: materializedOutputRows(result.MaterializedOutputs)},
		{label: "Known Release Files", value: inspectedKnownReleaseFileRows(result.KnownReleaseFiles)},
		{label: "Local Readiness", value: string(result.Readiness)},
		{label: "Local Blockers", value: localPlanBlockerRows(result.Blockers)},
	}
}

func releasePlanMachineItems(result *ReleasePlanInspection) []map[string]any {
	properties := append(releasePlanSharedProperties(result),
		releasePlanResponseProperty{label: "Limitations", value: releasePlanLimitationRows(result.Limitations)},
		releasePlanStatusProperty(),
	)
	items := make([]map[string]any, 0, len(properties))
	for _, property := range properties {
		items = append(items, map[string]any{"property": property.label, "value": property.value})
	}
	return items
}

func releasePlanSummaryProperties(result *ReleasePlanInspection) []presentation.Property {
	properties := []presentation.Property{
		{Label: "Unit", Value: result.Unit.ID, Emphasized: true},
		{Label: "Current version", Value: result.CurrentVersion},
		{Label: "Requested change", Value: string(result.RequestedChange)},
		{Label: "Next version", Value: result.NextVersion, Emphasized: true},
		{Label: "Tag", Value: result.Tag},
		{
			Label: "Local readiness", Value: releasePlanReadableLabel(string(result.Readiness)),
			Role: releasePlanReadinessRole(result.Readiness), Emphasized: true,
		},
	}
	if len(result.Blockers) > 0 {
		properties = append(properties, presentation.Property{
			Label: "Blockers", Value: len(result.Blockers), Role: presentation.StyleError,
		})
	}
	return append(properties, presentation.Property{
		Label: "Mutation boundary", Value: "Inspection only; release execution was not started",
		Role: presentation.StyleMuted,
	})
}

func releasePlanReadinessRole(readiness LocalPlanReadiness) presentation.StyleRole {
	switch readiness {
	case LocalPlanReady:
		return presentation.StyleSuccess
	case LocalPlanBlocked:
		return presentation.StyleError
	case LocalPlanUnsupported:
		return presentation.StyleWarning
	default:
		return presentation.StyleDefault
	}
}

func releasePlanPresentationTables(result *ReleasePlanInspection) *presentation.Table {
	tables := []*presentation.Table{
		releasePlanOperationsTable(result),
		releasePlanBlockersTable(result.Blockers),
		releasePlanPrimaryMaterializedFilesTable(result.MaterializedOutputs),
		releasePlanDetailsTable(result),
		releasePlanMaterializedFileFactsTable(result.MaterializedOutputs),
		releasePlanKnownReleaseFilesTable(result.KnownReleaseFiles),
		releasePlanLimitationsTable(result.Limitations),
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

func releasePlanOperationsTable(result *ReleasePlanInspection) *presentation.Table {
	return &presentation.Table{
		Title: "Operations",
		Columns: []presentation.Column{
			{Key: "step", Label: "Step", Essential: true},
			{Key: "result", Label: "Result", Essential: true},
			{Key: "scope", Label: "Scope"},
		},
		Rows: []map[string]any{
			{
				"step": "Resolve release identity",
				"result": fmt.Sprintf(
					"%s \u2192 %s (%s)",
					result.CurrentVersion,
					result.NextVersion,
					result.RequestedChange,
				),
				"scope": result.Unit.ID,
			},
			{"step": "Prepare tag", "result": result.Tag, "scope": "local plan"},
			{
				"step":   "Materialize files",
				"result": fmt.Sprintf("%d planned", len(result.MaterializedOutputs)),
				"scope":  "release commit",
			},
			{
				"step":   "Release execution",
				"result": "Not started",
				"scope":  "inspection boundary",
			},
		},
	}
}

func releasePlanBlockersTable(blockers []LocalPlanBlocker) *presentation.Table {
	if len(blockers) == 0 {
		return nil
	}
	rows := make([]map[string]any, 0, len(blockers))
	for _, blocker := range blockers {
		rows = append(rows, map[string]any{
			"problem": releasePlanReadableLabel(blocker.Category),
			"status":  "Blocked",
			"reason":  blocker.Message,
		})
	}
	return &presentation.Table{
		Title: "Blockers",
		Columns: []presentation.Column{
			{Key: "problem", Label: "Problem", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "reason", Label: "Reason", Essential: true},
		},
		Rows: rows,
	}
}

func releasePlanPrimaryMaterializedFilesTable(outputs []PlannedMaterializedOutput) *presentation.Table {
	if len(outputs) == 0 {
		return nil
	}
	rows := make([]map[string]any, 0, len(outputs))
	for _, output := range outputs {
		rows = append(rows, map[string]any{
			"path":           releasePlanSafePath(output.Path),
			"action":         releasePlanMaterializedAction(output),
			"release_commit": releasePlanYesNo(output.RequiredForReleaseCommit),
			"reason":         output.Reason,
		})
	}
	return &presentation.Table{
		Title: "Primary Materialized Files",
		Columns: []presentation.Column{
			{Key: "path", Label: "Path", Essential: true},
			{Key: "action", Label: "Action", Essential: true},
			{Key: "release_commit", Label: "Release commit"},
			{Key: "reason", Label: "Reason"},
		},
		Rows: rows,
	}
}

func releasePlanDetailsTable(result *ReleasePlanInspection) *presentation.Table {
	return &presentation.Table{
		Title: "Plan Details", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "unit", Label: "Unit", Essential: true},
			{Key: "source", Label: "Source ownership", Essential: true},
			{Key: "readiness", Label: "Readiness", Essential: true},
			{Key: "executor", Label: "Executor"},
			{Key: "delivery", Label: "Delivery"},
			{Key: "workflow", Label: "Workflow"},
		},
		Rows: []map[string]any{
			{
				"unit": result.Unit.ID, "source": releasePlanSourceOwnership(string(result.Source)),
				"readiness": releasePlanReadableLabel(string(result.Readiness)),
				"executor":  result.Executor, "delivery": result.Delivery,
				"workflow": releasePlanSafePath(result.Workflow),
			},
		},
		Details: &presentation.Properties{
			SectionTitle: "Complete planning facts",
			Properties:   releasePlanDetailProperties(result),
		},
	}
}

func releasePlanDetailProperties(result *ReleasePlanInspection) []presentation.Property {
	return []presentation.Property{
		{Label: "Selected source", Value: string(result.Source)},
		{Label: "Source ownership", Value: releasePlanSourceOwnership(string(result.Source))},
		{Label: "Unit", Value: result.Unit.ID},
		{Label: "Display name", Value: emptyFallback(result.Unit.DisplayName, "not configured")},
		{Label: "Resolved release identity", Value: result.Unit.ID + "@" + result.NextVersion},
		{Label: "Current version", Value: result.CurrentVersion},
		{Label: "Requested change", Value: string(result.RequestedChange)},
		{Label: "Next version", Value: result.NextVersion},
		{Label: "Tag", Value: result.Tag},
		{Label: "Executor", Value: result.Executor},
		{Label: "Delivery", Value: result.Delivery},
		{Label: "Workflow", Value: releasePlanSafePath(result.Workflow)},
		{Label: "Working directory", Value: releasePlanSafePath(result.WorkingDirectory)},
		{Label: "Unit root", Value: releasePlanSafeUnitRoot(result)},
		{Label: "Preflight readiness", Value: releasePlanReadableLabel(string(result.Readiness))},
		{Label: "Preflight blockers", Value: len(result.Blockers)},
		{Label: "Materialized files", Value: len(result.MaterializedOutputs)},
		{Label: "Known release files", Value: len(result.KnownReleaseFiles)},
		{Label: "Mutation boundary", Value: "Inspection only; release execution was not started"},
	}
}

func releasePlanMaterializedFileFactsTable(outputs []PlannedMaterializedOutput) *presentation.Table {
	if len(outputs) == 0 {
		return nil
	}
	rows := make([]map[string]any, 0, len(outputs))
	for _, output := range outputs {
		rows = append(rows, map[string]any{
			"path": releasePlanSafePath(output.Path), "action": releasePlanMaterializedAction(output),
			"exists":         releasePlanYesNo(output.Exists),
			"release_commit": releasePlanYesNo(output.RequiredForReleaseCommit),
			"reason":         output.Reason,
		})
	}
	return &presentation.Table{
		Title: "Materialized File Facts", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "path", Label: "Path", Essential: true},
			{Key: "action", Label: "Action", Essential: true},
			{Key: "exists", Label: "Exists", Essential: true},
			{Key: "release_commit", Label: "Release commit"},
			{Key: "reason", Label: "Reason"},
		},
		Rows: rows,
	}
}

func releasePlanKnownReleaseFilesTable(files []InspectedKnownReleaseFile) *presentation.Table {
	if len(files) == 0 {
		return nil
	}
	rows := make([]map[string]any, 0, len(files))
	for _, file := range files {
		rows = append(rows, map[string]any{
			"path":           releasePlanSafePath(file.Path),
			"release_commit": releasePlanYesNo(file.RequiredForReleaseCommit),
			"reason":         file.Reason,
		})
	}
	return &presentation.Table{
		Title: "Known Release Files", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "path", Label: "Path", Essential: true},
			{Key: "release_commit", Label: "Release commit", Essential: true},
			{Key: "reason", Label: "Reason"},
		},
		Rows: rows,
	}
}

func releasePlanLimitationsTable(limitations []ReleasePlanLimitation) *presentation.Table {
	if len(limitations) == 0 {
		return nil
	}
	rows := make([]map[string]any, 0, len(limitations))
	for _, limitation := range limitations {
		rows = append(rows, map[string]any{
			"assumption": releasePlanLimitationLabel(limitation.Category),
			"statement":  limitation.Message,
		})
	}
	return &presentation.Table{
		Title: "Assumptions and Limitations", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "assumption", Label: "Assumption", Essential: true},
			{Key: "statement", Label: "Statement", Essential: true},
		},
		Rows: rows,
	}
}

func releasePlanMaterializedAction(output PlannedMaterializedOutput) string {
	if output.Exists {
		return "Update"
	}
	return "Create"
}

func releasePlanYesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func releasePlanSourceOwnership(source string) string {
	switch source {
	case "v1":
		return "Legacy release configuration"
	case "v2":
		return "V2 config and state"
	default:
		return "Unresolved release source"
	}
}

func releasePlanReadableLabel(value string) string {
	value = strings.NewReplacer("_", " ", "-", " ").Replace(strings.TrimSpace(value))
	if value == "" {
		return "Not configured"
	}
	words := strings.Fields(value)
	for index := range words {
		words[index] = strings.ToUpper(words[index][:1]) + words[index][1:]
	}
	return strings.Join(words, " ")
}

func releasePlanSafePath(value string) string {
	value = strings.TrimSpace(value)
	switch {
	case value == "":
		return "not configured"
	case value == ".":
		return "repository root"
	case filepath.IsAbs(value):
		return "repository-local path"
	default:
		return filepath.ToSlash(filepath.Clean(value))
	}
}

func releasePlanSafeUnitRoot(result *ReleasePlanInspection) string {
	if !filepath.IsAbs(result.UnitRoot) {
		return releasePlanSafePath(result.UnitRoot)
	}
	if strings.TrimSpace(result.WorkingDirectory) == "." {
		return "repository root"
	}
	return releasePlanSafePath(result.WorkingDirectory)
}

func releasePlanLimitationLabel(category string) string {
	switch category {
	case "local-only":
		return "Local Inspection Only"
	case "no-evidence-inspection":
		return "Evidence Not Inspected"
	case "no-remote-checks":
		return "Remote Checks Not Performed"
	case "token-free":
		return "Token Free"
	case "v1-latest-tag-evidence":
		return "V1 Tag Evidence"
	case "v1-known-release-files":
		return "V1 Known Release Files"
	default:
		return "Inspection Limitation"
	}
}

func releasePlanStatusProperty() releasePlanResponseProperty {
	return releasePlanResponseProperty{
		label: "Status",
		value: "Release plan inspected locally; no release execution was started",
	}
}

func materializedOutputRows(outputs []PlannedMaterializedOutput) string {
	return formatPlanRecords(outputs, func(output PlannedMaterializedOutput) string {
		if strings.TrimSpace(output.Reason) == "" {
			return output.Path
		}
		return fmt.Sprintf("%s (%s)", output.Path, output.Reason)
	})
}

func inspectedKnownReleaseFileRows(files []InspectedKnownReleaseFile) string {
	return formatPlanRecords(files, func(file InspectedKnownReleaseFile) string {
		if strings.TrimSpace(file.Reason) == "" {
			return file.Path
		}
		return fmt.Sprintf("%s (%s)", file.Path, file.Reason)
	})
}

func localPlanBlockerRows(blockers []LocalPlanBlocker) string {
	return formatPlanCategoryMessages(blockers, func(blocker LocalPlanBlocker) (string, string) {
		return blocker.Category, blocker.Message
	})
}

func releasePlanLimitationRows(limitations []ReleasePlanLimitation) string {
	return formatPlanCategoryMessages(limitations, func(limitation ReleasePlanLimitation) (string, string) {
		return limitation.Category, limitation.Message
	})
}

func formatPlanRecords[T any](records []T, value func(T) string) string {
	if len(records) == 0 {
		return "none"
	}
	values := make([]string, 0, len(records))
	for _, record := range records {
		values = append(values, value(record))
	}
	return strings.Join(values, ", ")
}

func formatPlanCategoryMessages[T any](records []T, value func(T) (string, string)) string {
	if len(records) == 0 {
		return "none"
	}
	values := make([]string, 0, len(records))
	for _, record := range records {
		category, message := value(record)
		values = append(values, fmt.Sprintf("%s: %s", category, message))
	}
	return strings.Join(values, " | ")
}
