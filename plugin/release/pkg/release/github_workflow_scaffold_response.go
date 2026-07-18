package release

import (
	"fmt"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

var githubWorkflowScaffoldHumanProperties = []plugin.HumanProperty{
	{Key: "target", Label: "Target"},
	{Key: "classification", Label: "Status"},
	{Key: "action", Label: "Action"},
	{Key: "selected_unit", Label: "Selected unit"},
	{Key: "units_using_workflow", Label: "Units using workflow"},
	{Key: "contract_version", Label: "Contract version"},
	{Key: "written", Label: "Written"},
	{Key: "unchanged", Label: "Unchanged"},
	{Key: "guidance", Label: "Guidance"},
}

func mapGitHubWorkflowScaffoldResult(result *githubWorkflowScaffoldResult, timestamp time.Time) *plugin.Response {
	if result == nil {
		return workflowScaffoldFailureResponse(
			failureFromMessage("WORKFLOW_CREATE_FAILED", "workflow scaffolding did not produce a result"),
			timestamp,
		)
	}
	data := map[string]any{
		"target":               result.Plan.Target.RelativePath,
		"classification":       string(result.Plan.Classification),
		"action":               result.Action,
		"written":              result.Written,
		"unchanged":            result.Unchanged,
		"dry_run":              result.Preview,
		"contract_version":     result.Plan.ContractVersion,
		"selected_unit":        result.Plan.SelectedUnit,
		"units_using_workflow": append([]string(nil), result.Plan.UnitsUsingWorkflow...),
		"guidance":             result.Guidance,
	}
	response := &plugin.Response{
		Status:       "success",
		Metadata:     commandResponseMetadata(githubWorkflowInitCommandName, timestamp),
		Data:         data,
		RendererHint: "table",
		HumanProperties: &plugin.HumanProperties{
			Properties: append([]plugin.HumanProperty(nil), githubWorkflowScaffoldHumanProperties...),
		},
	}
	if result.Preview {
		data["generated_content"] = string(result.Plan.GeneratedContent)
		response.HumanProperties = nil
		response.HumanText = &plugin.HumanText{Content: githubWorkflowScaffoldPreviewText(result)}
	}
	return response
}

func githubWorkflowScaffoldPreviewText(result *githubWorkflowScaffoldResult) string {
	var output strings.Builder
	_, _ = fmt.Fprintf(&output, "GitHub Actions workflow scaffolding preview\n\n")
	_, _ = fmt.Fprintf(&output, "Target: %s\n", result.Plan.Target.RelativePath)
	_, _ = fmt.Fprintf(&output, "Status: %s\n", result.Plan.Classification)
	_, _ = fmt.Fprintf(&output, "Action: %s\n", result.Action)
	_, _ = fmt.Fprintf(&output, "Units: %s\n", strings.Join(result.Plan.UnitsUsingWorkflow, ", "))
	_, _ = fmt.Fprintf(&output, "Contract version: %d\n", result.Plan.ContractVersion)
	_, _ = fmt.Fprintf(&output, "Guidance: %s\n\n", result.Guidance)
	_, _ = fmt.Fprintf(&output, "Generated workflow\n\n%s", result.Plan.GeneratedContent)
	return output.String()
}
