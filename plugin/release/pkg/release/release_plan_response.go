package release

import (
	"fmt"
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
	response.PresentationProperties = &presentation.Properties{Properties: releasePlanPresentationProperties(result)}
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

func releasePlanPresentationProperties(result *ReleasePlanInspection) []presentation.Property {
	properties := releasePlanSharedProperties(result)
	for _, limitation := range result.Limitations {
		properties = append(properties, releasePlanResponseProperty{
			label: releasePlanLimitationLabel(limitation.Category),
			value: limitation.Message,
		})
	}
	properties = append(properties, releasePlanStatusProperty())

	declarations := make([]presentation.Property, 0, len(properties))
	for _, property := range properties {
		declarations = append(declarations, presentation.Property{Label: property.label, Value: property.value})
	}
	return declarations
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
