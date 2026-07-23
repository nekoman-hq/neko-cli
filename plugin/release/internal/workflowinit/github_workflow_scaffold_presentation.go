package workflowinit

import (
	"fmt"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
)

func githubWorkflowScaffoldSummaryPresentation(result *githubWorkflowScaffoldResult) *presentation.Properties {
	return &presentation.Properties{
		Title: "GitHub Workflow Initialization",
		Properties: []presentation.Property{
			{Label: "Result", Value: workflowScaffoldResultLabel(result), Role: workflowScaffoldResultRole(result), Emphasized: true},
			{Label: "Target", Value: result.Plan.Target.RelativePath},
			{Label: "Canonical workflow", Value: "Release selected unit"},
			{Label: "Contract version", Value: result.Plan.ContractVersion},
			{Label: "Write outcome", Value: workflowScaffoldWriteOutcome(result)},
			{Label: "Next action", Value: result.Guidance},
		},
	}
}

func workflowScaffoldResultLabel(result *githubWorkflowScaffoldResult) string {
	switch {
	case result == nil:
		return "Workflow result unavailable"
	case result.Preview && result.Plan.Classification == githubWorkflowTargetCreate:
		return "Workflow would be created"
	case result.Preview && result.Plan.Classification == githubWorkflowTargetUnchanged:
		return "Workflow already current"
	case result.Preview:
		return "Workflow conflict previewed"
	case result.Written:
		return "Workflow created"
	case result.Unchanged:
		return "Workflow already current"
	default:
		return "Workflow initialization completed"
	}
}

func workflowScaffoldResultRole(result *githubWorkflowScaffoldResult) presentation.StyleRole {
	if result != nil && result.Plan.Classification == githubWorkflowTargetConflict {
		return presentation.StyleWarning
	}
	if result != nil && result.Preview {
		return presentation.StyleInfo
	}
	return presentation.StyleSuccess
}

func workflowScaffoldWriteOutcome(result *githubWorkflowScaffoldResult) string {
	switch {
	case result == nil:
		return "Unavailable"
	case result.Preview:
		return "No write performed"
	case result.Written:
		return "Created"
	case result.Unchanged:
		return "No write required"
	default:
		return "No write performed"
	}
}

func githubWorkflowScaffoldDetailPresentation(result *githubWorkflowScaffoldResult) *presentation.Table {
	identity := &presentation.Table{
		Title: "Workflow Identity", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "identity", Label: "Canonical Workflow", Essential: true},
			{Key: "target", Label: "Target", Essential: true},
			{Key: "classification", Label: "Decision", Essential: true},
			{Key: "contract", Label: "Contract", Essential: true},
		},
		Rows: []map[string]any{{
			"identity": "Release selected unit", "target": result.Plan.Target.RelativePath,
			"classification": string(result.Plan.Classification), "contract": result.Plan.ContractVersion,
		}},
		Details: &presentation.Properties{
			SectionTitle: "Configured consumers",
			Properties: []presentation.Property{
				{Label: "Selected unit", Value: workflowScaffoldOptionalValue(result.Plan.SelectedUnit)},
				{Label: "Units using workflow", Value: strings.Join(result.Plan.UnitsUsingWorkflow, ", ")},
				{Label: "Canonical content", Value: "Validated deterministic workflow contract"},
			},
		},
	}
	identity.Following = githubWorkflowTargetComparisonPresentation(result)
	identity.Following.Following = githubWorkflowValidationPresentation(result)
	identity.Following.Following.Following = githubWorkflowInputPresentation()
	identity.Following.Following.Following.Following = githubWorkflowWritePlanPresentation(result)
	identity.Following.Following.Following.Following.Following = githubWorkflowLimitationsPresentation()
	return identity
}

func workflowScaffoldOptionalValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "not explicitly selected"
	}
	return value
}

func githubWorkflowTargetComparisonPresentation(result *githubWorkflowScaffoldResult) *presentation.Table {
	existing, comparison := "Missing", "No existing content"
	switch result.Plan.Classification {
	case githubWorkflowTargetUnchanged:
		existing, comparison = "Present", "Byte-identical to canonical content"
	case githubWorkflowTargetConflict:
		existing, comparison = "Present", "Different from canonical content"
	}
	return &presentation.Table{
		Title: "Target Comparison", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "target", Label: "Target", Essential: true},
			{Key: "existing", Label: "Existing", Essential: true},
			{Key: "comparison", Label: "Comparison", Essential: true},
			{Key: "overwrite", Label: "Overwrite", Essential: true},
		},
		Rows: []map[string]any{{
			"target": result.Plan.Target.RelativePath, "existing": existing,
			"comparison": comparison, "overwrite": "Never",
		}},
	}
}

func githubWorkflowValidationPresentation(result *githubWorkflowScaffoldResult) *presentation.Table {
	return &presentation.Table{
		Title: "Validation Facts", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "check", Label: "Check", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "details", Label: "Details"},
		},
		Rows: []map[string]any{
			{"check": "Release source", "status": "Valid", "details": "Release V2 config/state pair"},
			{"check": "Target path", "status": "Valid", "details": "Canonical repository-relative workflow path"},
			{"check": "Generated content", "status": "Valid", "details": "One YAML mapping document ending with a newline"},
			{"check": "Target decision", "status": workflowClassificationLabel(result.Plan.Classification), "details": "Create-only comparison policy"},
		},
	}
}

func workflowClassificationLabel(classification githubWorkflowTargetClassification) string {
	switch classification {
	case githubWorkflowTargetCreate:
		return "Create"
	case githubWorkflowTargetUnchanged:
		return "Unchanged"
	case githubWorkflowTargetConflict:
		return "Conflict"
	default:
		return "Unknown"
	}
}

func githubWorkflowInputPresentation() *presentation.Table {
	definitions := releaseworkflow.CanonicalDispatchInputContract()
	rows := make([]map[string]any, 0, len(definitions))
	for _, definition := range definitions {
		rows = append(rows, map[string]any{
			"input": definition.Name, "required": "Yes", "description": definition.Description,
		})
	}
	return &presentation.Table{
		Title: "Required Workflow Inputs", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "input", Label: "Input", Essential: true},
			{Key: "required", Label: "Required", Essential: true},
			{Key: "description", Label: "Description", Essential: true},
		},
		Rows: rows,
	}
}

func githubWorkflowWritePlanPresentation(result *githubWorkflowScaffoldResult) *presentation.Table {
	return &presentation.Table{
		Title: "Write Plan", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "target", Label: "Target", Essential: true},
			{Key: "decision", Label: "Decision", Essential: true},
			{Key: "dry_run", Label: "Dry Run", Essential: true},
			{Key: "outcome", Label: "Outcome", Essential: true},
		},
		Rows: []map[string]any{{
			"target": result.Plan.Target.RelativePath, "decision": result.Action,
			"dry_run": workflowScaffoldYesNo(result.Preview), "outcome": workflowScaffoldWriteOutcome(result),
		}},
	}
}

func workflowScaffoldYesNo(value bool) string {
	if value {
		return "Yes"
	}
	return "No"
}

func githubWorkflowLimitationsPresentation() *presentation.Table {
	return &presentation.Table{
		Title: "Limitations", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "scope", Label: "Scope", Essential: true},
			{Key: "details", Label: "Details", Essential: true},
		},
		Rows: []map[string]any{
			{"scope": "Overwrite", "details": "Differing existing workflows are never overwritten."},
			{"scope": "Consumer steps", "details": "The generated failing placeholder must be replaced deliberately."},
			{"scope": "Remote actions", "details": "No dispatch, upload, publication, release, or remote request is performed."},
		},
	}
}

func githubWorkflowFailurePresentation(failure *commandFailure) *presentation.Table {
	code := "WORKFLOW_CREATE_FAILED"
	if failure != nil && failure.Code != "" {
		code = failure.Code
	}
	area, reason, remediation := githubWorkflowFailureFacts(failure)
	target := workflowFailureTarget(failure)
	return &presentation.Table{
		Title: "Workflow Initialization Blocked",
		Columns: []presentation.Column{
			{Key: "code", Label: "Code", Essential: true},
			{Key: "area", Label: "Area", Essential: true},
			{Key: "target", Label: "Target", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "reason", Label: "Reason", Essential: true},
			{Key: "overwrite", Label: "Overwrite", Essential: true},
			{Key: "remediation", Label: "Remediation", Essential: true},
		},
		Rows: []map[string]any{{
			"code": code, "area": area, "target": target, "status": "Refused",
			"reason": reason, "overwrite": "Refused", "remediation": remediation,
		}},
	}
}

func workflowFailureTarget(failure *commandFailure) string {
	if failure != nil {
		if value, ok := failure.Details["target"].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "configured workflow target"
}

func githubWorkflowFailureFacts(failure *commandFailure) (area, reason, remediation string) {
	if failure == nil {
		return "Workflow initialization", "The workflow request could not be completed.", "Correct the repository state and retry."
	}
	switch failure.Code {
	case "WORKFLOW_TARGET_CONFLICT":
		return "Different content", "The existing workflow differs from the canonical generated workflow.", "Run with --dry-run, compare the preview, and resolve the file manually."
	case "WORKFLOW_TARGET_INVALID":
		return "Target path", "The requested target is not a safe canonical workflow path.", "Use a configured .github/workflows/*.yml or .yaml path."
	case "WORKFLOW_TARGET_SYMLINK_ESCAPE":
		return "Target path", "The workflow target or parent resolves through an unsafe symlink.", "Use a regular repository-owned .github/workflows target."
	case "AMBIGUOUS_WORKFLOW_TARGET":
		return "Workflow selection", "Multiple configured workflow targets require an explicit choice.", "Select one configured target with --unit or --path."
	case "WORKFLOW_TARGET_NOT_CONFIGURED", "WORKFLOW_NOT_CONFIGURED":
		return "Workflow selection", "The requested workflow is not configured by the Release V2 units.", "Select a configured unit/path or update the V2 configuration."
	case "RELEASE_UNIT_NOT_FOUND":
		return "Workflow selection", "The requested release unit does not exist.", "Choose an existing Release V2 unit."
	case "UNSUPPORTED_RELEASE_SOURCE":
		return "Release source", "Workflow initialization requires a Release V2 repository.", "Migrate or initialize Release V2 before scaffolding."
	case "V2_WORKFLOW_SOURCE_MISSING", "V2_WORKFLOW_SOURCE_CONFLICT", "V2_WORKFLOW_CONFIGURATION_INVALID",
		"V2_WORKFLOW_STATE_INVALID", "V2_WORKFLOW_CONFIG_STATE_MISMATCH", "V2_WORKFLOW_SOURCE_INVALID",
		"V2_WORKFLOW_RECOVERY_BLOCKED":
		return "Release V2 source", "The Release V2 config/state source is incomplete, conflicting, or invalid.", "Repair the V2 pair or recovery evidence before retrying."
	case "INVALID_WORKFLOW_SCAFFOLD_REQUEST":
		return "Request flags", "A workflow initialization flag has an invalid type or value.", "Correct the reported flag and retry."
	case "WORKFLOW_WRITE_FAILED", "WORKFLOW_ATOMIC_CREATE_FAILED":
		return "Workflow write", "The missing workflow could not be created atomically.", "Resolve the local filesystem failure and retry; no differing target was overwritten."
	case "WORKFLOW_CONTENT_INVALID":
		return "Canonical content", "The canonical workflow could not be rendered or validated.", "Do not write a workflow; report the canonical contract failure."
	default:
		return "Workflow initialization", fmt.Sprintf("Workflow initialization failed with %s.", failure.Code), "Correct the reported repository state and retry."
	}
}
