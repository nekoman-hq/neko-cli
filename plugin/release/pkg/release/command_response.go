package release

import (
	"fmt"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

// MapCommandFailure maps an expected application failure using an explicitly
// supplied timestamp.
func MapCommandFailure(command string, failure *CommandFailure, timestamp time.Time) *plugin.Response {
	if failure == nil {
		return nil
	}
	return &plugin.Response{
		Status:   "error",
		Metadata: commandResponseMetadata(command, timestamp),
		Error: &plugin.ResponseError{
			Code:    failure.Code,
			Message: failure.responseMessage(),
			Details: cloneResponseDetails(failure.Details),
		},
	}
}

// MapReleaseCommandOutcome renders one typed release application outcome.
func MapReleaseCommandOutcome(command string, outcome ReleaseCommandOutcome, timestamp time.Time) (*plugin.Response, error) {
	switch result := outcome.(type) {
	case *LegacyReleasePreview:
		return successTableResponse(command, timestamp, []map[string]any{
			{"property": "Release Type", "value": string(result.ReleaseType)},
			{"property": "Current Version", "value": result.CurrentVersion},
			{"property": "New Version", "value": result.NextVersion},
			{"property": "Release System", "value": result.ReleaseSystem},
			{"property": "Dry Run", "value": "yes"},
			{"property": "Status", "value": "Preview - no changes made"},
		}), nil
	case *LegacyReleaseCompleted:
		return successTableResponse(command, timestamp, []map[string]any{
			{"property": "Release Type", "value": string(result.ReleaseType)},
			{"property": "Previous Version", "value": result.PreviousVersion},
			{"property": "New Version", "value": result.NextVersion},
			{"property": "Release System", "value": result.ReleaseSystem},
			{"property": "Status", "value": "Released successfully"},
		}), nil
	case *V2ReleasePreview:
		return mapV2ReleasePreview(command, result, timestamp), nil
	case *GitHubActionsReleaseResult:
		return mapGitHubActionsReleaseResult(command, result, timestamp), nil
	default:
		return nil, fmt.Errorf("unsupported release command outcome %T", outcome)
	}
}

// MapResumeCommandOutcome renders one typed resume application outcome.
func MapResumeCommandOutcome(outcome ResumeCommandOutcome, timestamp time.Time) (*plugin.Response, error) {
	switch result := outcome.(type) {
	case *ResumeAssessment:
		return successTableResponse("resume", timestamp, []map[string]any{
			{"property": "Unit", "value": result.UnitID},
			{"property": "Version", "value": result.NextVersion},
			{"property": "Tag", "value": result.Tag},
			{"property": "Execution Journal", "value": result.ExecutionJournalPath},
			{"property": "State", "value": string(result.State)},
			{"property": "Pending Action", "value": string(result.PendingAction)},
			{"property": "Recovery Status", "value": string(result.RecoveryStatus)},
			{"property": "Safe To Continue", "value": fmt.Sprintf("%t", result.SafeToContinue)},
			{"property": "Known Files", "value": strings.Join(result.KnownFilePaths, ", ")},
			{"property": "Next Step", "value": result.Guidance},
		}), nil
	case *GitHubActionsReleaseResult:
		return mapGitHubActionsReleaseResult("resume", result, timestamp), nil
	default:
		return nil, fmt.Errorf("unsupported resume command outcome %T", outcome)
	}
}

// MapReleasePlanInspection renders read-only release-plan inspection facts.
func MapReleasePlanInspection(result *ReleasePlanInspection, timestamp time.Time) *plugin.Response {
	if result == nil {
		return successTableResponse("plan", timestamp, nil)
	}
	return successTableResponse("plan", timestamp, []map[string]any{
		{"property": "Source", "value": string(result.Source)},
		{"property": "Unit", "value": result.Unit.ID},
		{"property": "Display Name", "value": emptyFallback(result.Unit.DisplayName, "not configured")},
		{"property": "Current Version", "value": result.CurrentVersion},
		{"property": "Requested Change", "value": string(result.RequestedChange)},
		{"property": "Next Version", "value": result.NextVersion},
		{"property": "Tag", "value": result.Tag},
		{"property": "Executor", "value": result.Executor},
		{"property": "Delivery", "value": result.Delivery},
		{"property": "Workflow", "value": workflowResponseValue(result.Workflow)},
		{"property": "Working Directory", "value": result.WorkingDirectory},
		{"property": "Unit Root", "value": result.UnitRoot},
		{"property": "Planned Materialized Files", "value": materializedOutputRows(result.MaterializedOutputs)},
		{"property": "Known Release Files", "value": inspectedKnownReleaseFileRows(result.KnownReleaseFiles)},
		{"property": "Local Readiness", "value": string(result.Readiness)},
		{"property": "Local Blockers", "value": localPlanBlockerRows(result.Blockers)},
		{"property": "Limitations", "value": releasePlanLimitationRows(result.Limitations)},
		{"property": "Status", "value": "Release plan inspected locally; no release execution was started"},
	})
}

func mapV2ReleasePreview(command string, result *V2ReleasePreview, timestamp time.Time) *plugin.Response {
	items := []map[string]any{
		{"property": "Release Type", "value": command},
		{"property": "Unit", "value": result.UnitID},
		{"property": "Current Version", "value": result.CurrentVersion},
		{"property": "New Version", "value": result.NextVersion},
		{"property": "Tag", "value": result.Tag},
		{"property": "Executor", "value": result.Executor},
		{"property": "Delivery", "value": result.Delivery},
		{"property": "Workflow", "value": workflowResponseValue(result.Workflow)},
		{"property": "Dispatch", "value": dispatchResponseValue(result.Delivery)},
		{"property": "Working Directory", "value": result.WorkingDirectory},
		{"property": "Unit Root", "value": result.UnitRoot},
		{"property": "State Change", "value": result.StateChange},
		{"property": "Materialized Files", "value": materializedFilesResponseValue(result)},
		{"property": "Known Release Files", "value": strings.Join(result.KnownReleaseFilePaths, ", ")},
		{"property": "Planned Release Commit", "value": result.CommitMessage},
		{"property": "Planned Tag", "value": result.Tag},
		{"property": "Planned Push Order", "value": "1. release commit, 2. unit tag"},
		{"property": "Tool Ownership", "value": result.OwnershipSummary},
		{"property": "V2 Git Ownership", "value": result.V2GitOwnership},
		{"property": "State Commit Guarantee", "value": result.StateGuarantee},
		{"property": "Executor Start", "value": "no"},
		{"property": "Dry Run", "value": "yes"},
		{"property": "Status", "value": "V2 preview - no changes made"},
	}
	if result.Dispatch != nil {
		items = append(items,
			map[string]any{"property": "Dispatch Ref", "value": result.Dispatch.Ref},
			map[string]any{"property": "Dispatch Inputs", "value": dispatchInputsValue(result.Dispatch.Inputs)},
			map[string]any{"property": "Journal Identity", "value": result.Dispatch.JournalIdentity},
			map[string]any{"property": "Journal Location", "value": result.Dispatch.JournalLocation},
			map[string]any{"property": "Dispatch Status", "value": result.Dispatch.Status},
		)
	}
	return successTableResponse(command, timestamp, items)
}

func mapGitHubActionsReleaseResult(command string, result *GitHubActionsReleaseResult, timestamp time.Time) *plugin.Response {
	return successTableResponse(command, timestamp, []map[string]any{
		{"property": "Unit", "value": result.Unit},
		{"property": "Version", "value": result.Version},
		{"property": "Tag", "value": result.Tag},
		{"property": "Release Commit", "value": result.CommitSHA},
		{"property": "Workflow", "value": result.Workflow},
		{"property": "Execution Journal", "value": result.ExecutionJournalPath},
		{"property": "Dispatch Journal", "value": result.DispatchJournalPath},
		{"property": "Execution State", "value": string(result.ExecutionState)},
		{"property": "Dispatch State", "value": string(result.DispatchState)},
		{"property": "Dispatch Run", "value": emptyFallback(result.DispatchRunURL, "not resolved")},
		{"property": "Status", "value": result.RecoveryGuidance},
	})
}

func successTableResponse(command string, timestamp time.Time, items []map[string]any) *plugin.Response {
	return &plugin.Response{
		Status:       "success",
		Metadata:     commandResponseMetadata(command, timestamp),
		Data:         map[string]any{"items": items},
		RendererHint: "table",
	}
}

func commandResponseMetadata(command string, timestamp time.Time) plugin.ResponseMetadata {
	return plugin.ResponseMetadata{
		Plugin:    metadata.PluginName,
		Version:   metadata.Version,
		Command:   command,
		Timestamp: timestamp,
	}
}

func cloneResponseDetails(details map[string]any) map[string]any {
	if details == nil {
		return nil
	}
	clone := make(map[string]any, len(details))
	for key, value := range details {
		clone[key] = value
	}
	return clone
}

func materializedFilesResponseValue(result *V2ReleasePreview) string {
	if len(result.MaterializedFilePaths) > 0 {
		return strings.Join(result.MaterializedFilePaths, ", ")
	}
	if result.MaterializationBlockedReason != "" {
		return "blocked: " + result.MaterializationBlockedReason
	}
	return "none"
}

func workflowResponseValue(workflow string) string {
	if workflow == "" {
		return "not applicable"
	}
	return workflow
}

func dispatchResponseValue(delivery string) string {
	if delivery == string(config.DeliveryGitHubActions) {
		return "planned after commit and tag push"
	}
	return "not applicable"
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
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
