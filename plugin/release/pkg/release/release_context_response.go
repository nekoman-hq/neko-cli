package release

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

var releaseContextPresentationProperties = []presentation.Property{
	{Key: "valid", Label: "Release context valid"},
	{Key: workflowDispatchInputUnit, Label: "Unit"},
	{Key: "display_name", Label: "Display name"},
	{Key: workflowDispatchInputVersion, Label: "Version"},
	{Key: "tag_prefix", Label: "Tag prefix"},
	{Key: workflowDispatchInputTag, Label: "Tag"},
	{Key: workflowDispatchInputReleaseSHA, Label: "Release commit"},
	{Key: "working_directory", Label: "Working directory"},
	{Key: "executor", Label: "Executor"},
	{Key: "delivery", Label: "Delivery"},
	{Key: "workflow", Label: "Workflow"},
	{Key: "git_object_format", Label: "Git object format"},
	{Key: "head_matches", Label: "HEAD matches"},
	{Key: "tag_target_matches", Label: "Tag target matches"},
}

var releaseContextGitHubOutputFields = []plugin.GitHubOutputField{
	{Name: workflowDispatchInputUnit, DataKey: workflowDispatchInputUnit},
	{Name: "display_name", DataKey: "display_name"},
	{Name: workflowDispatchInputVersion, DataKey: workflowDispatchInputVersion},
	{Name: "tag_prefix", DataKey: "tag_prefix"},
	{Name: workflowDispatchInputTag, DataKey: workflowDispatchInputTag},
	{Name: workflowDispatchInputReleaseSHA, DataKey: workflowDispatchInputReleaseSHA},
	{Name: "working_directory", DataKey: "working_directory"},
	{Name: "executor", DataKey: "executor"},
	{Name: "delivery", DataKey: "delivery"},
	{Name: "workflow", DataKey: "workflow"},
}

// MapValidatedReleaseContext maps the typed application result at the command
// boundary into stable machine data and transport-only presentation metadata.
func MapValidatedReleaseContext(result *ValidatedReleaseContext, timestamp time.Time) *plugin.Response {
	if result == nil {
		response := MapCommandFailure(
			releaseContextValidationCommandName,
			failureFromMessage("CONTEXT_VALIDATION_RESULT_INVALID", "release context validation did not produce a validated context"),
			timestamp,
		)
		response.ExitCode = 1
		return response
	}
	return &plugin.Response{
		Status:   "success",
		Metadata: commandResponseMetadata(releaseContextValidationCommandName, timestamp),
		Data: map[string]any{
			"valid":                         true,
			workflowDispatchInputUnit:       result.UnitID,
			"display_name":                  result.DisplayName,
			workflowDispatchInputVersion:    result.Version,
			"tag_prefix":                    result.TagPrefix,
			workflowDispatchInputTag:        result.Tag,
			workflowDispatchInputReleaseSHA: result.ReleaseSHA,
			"working_directory":             result.WorkingDirectory,
			"executor":                      result.Executor,
			"delivery":                      result.Delivery,
			"workflow":                      result.Workflow,
			"git_object_format":             string(result.GitObjectFormat),
			"head_matches":                  result.HeadMatches,
			"tag_target_matches":            result.TagTargetMatches,
		},
		RendererHint: "table",
		PresentationProperties: &presentation.Properties{
			Properties: append([]presentation.Property(nil), releaseContextPresentationProperties...),
		},
		GitHubOutput: &plugin.GitHubOutput{
			Fields: append([]plugin.GitHubOutputField(nil), releaseContextGitHubOutputFields...),
		},
	}
}
