package release

import (
	"fmt"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

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
