package contextvalidation

import (
	"path/filepath"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
)

var releaseContextCompleteProperties = []presentation.Property{
	{Key: "valid", Label: "Release context valid"},
	{Key: releaseworkflow.DispatchInputUnit, Label: "Unit"},
	{Key: "display_name", Label: "Display name"},
	{Key: releaseworkflow.DispatchInputVersion, Label: "Version"},
	{Key: "tag_prefix", Label: "Tag prefix"},
	{Key: releaseworkflow.DispatchInputTag, Label: "Tag"},
	{Key: releaseworkflow.DispatchInputReleaseSHA, Label: "Release commit"},
	{Key: "working_directory", Label: "Working directory"},
	{Key: "executor", Label: "Executor"},
	{Key: "delivery", Label: "Delivery"},
	{Key: "workflow", Label: "Workflow"},
	{Key: "git_object_format", Label: "Git object format"},
	{Key: "head_matches", Label: "HEAD matches"},
	{Key: "tag_target_matches", Label: "Tag target matches"},
}

var releaseContextGitHubOutputFields = []plugin.GitHubOutputField{
	{Name: releaseworkflow.DispatchInputUnit, DataKey: releaseworkflow.DispatchInputUnit},
	{Name: "display_name", DataKey: "display_name"},
	{Name: releaseworkflow.DispatchInputVersion, DataKey: releaseworkflow.DispatchInputVersion},
	{Name: "tag_prefix", DataKey: "tag_prefix"},
	{Name: releaseworkflow.DispatchInputTag, DataKey: releaseworkflow.DispatchInputTag},
	{Name: releaseworkflow.DispatchInputReleaseSHA, DataKey: releaseworkflow.DispatchInputReleaseSHA},
	{Name: "working_directory", DataKey: "working_directory"},
	{Name: "executor", DataKey: "executor"},
	{Name: "delivery", DataKey: "delivery"},
	{Name: "workflow", DataKey: "workflow"},
}

// MapValidatedReleaseContext maps the typed application result at the command
// boundary into stable machine data and transport-only presentation metadata.
func MapValidatedReleaseContext(result *ValidatedReleaseContext, timestamp time.Time) *plugin.Response {
	if result == nil {
		response := mapCommandFailure(
			releaseContextValidationCommandName,
			failureFromMessage("CONTEXT_VALIDATION_RESULT_INVALID", "release context validation did not produce a validated context"),
			timestamp,
		)
		return response
	}
	response := &plugin.Response{
		Status:   "success",
		Metadata: contextValidationResponseMetadata(releaseContextValidationCommandName, timestamp),
		Data: map[string]any{
			"valid":                                 true,
			releaseworkflow.DispatchInputUnit:       result.UnitID,
			"display_name":                          result.DisplayName,
			releaseworkflow.DispatchInputVersion:    result.Version,
			"tag_prefix":                            result.TagPrefix,
			releaseworkflow.DispatchInputTag:        result.Tag,
			releaseworkflow.DispatchInputReleaseSHA: result.ReleaseSHA,
			"working_directory":                     result.WorkingDirectory,
			"executor":                              result.Executor,
			"delivery":                              result.Delivery,
			"workflow":                              result.Workflow,
			"git_object_format":                     string(result.GitObjectFormat),
			"head_matches":                          result.HeadMatches,
			"tag_target_matches":                    result.TagTargetMatches,
		},
		RendererHint: "table",
		PresentationProperties: &presentation.Properties{
			Title: "Validated Release Context",
			Properties: []presentation.Property{
				{Label: "Release context", Value: "Valid", Role: presentation.StyleSuccess, Emphasized: true},
				{Label: "Unit", Value: result.UnitID},
				{Label: "Version", Value: result.Version},
				{Label: "Tag", Value: result.Tag},
				{Label: "Release commit", Value: result.ReleaseSHA},
				{Label: "Git consistency", Value: "HEAD and tag target match", Role: presentation.StyleSuccess},
			},
		},
		GitHubOutput: &plugin.GitHubOutput{
			Fields: append([]plugin.GitHubOutputField(nil), releaseContextGitHubOutputFields...),
		},
	}
	response.PresentationTable = releaseContextCompleteChecksTable(result.Checks)
	response.PresentationTable.Following = releaseContextDetailsTable(result)
	response.PresentationTable.Following.Following = releaseContextGitHubMappingTable()
	response.PresentationTable.Following.Following.Following = releaseContextLimitationsTable()
	response.SetExitCode(0)
	return response
}

func attachFailedReleaseContextPresentation(response *plugin.Response, result *ValidatedReleaseContext) {
	if response == nil || result == nil || len(result.Checks) == 0 {
		return
	}
	findings := releaseContextCheckRows(result.Checks, true)
	if len(findings) == 0 {
		return
	}
	response.PresentationTable = &presentation.Table{
		Title:   "Failed Checks",
		Columns: releaseContextCheckColumns(),
		Rows:    findings,
	}
	response.PresentationTable.Following = releaseContextCompleteChecksTable(result.Checks)
	response.PresentationTable.Following.Following = releaseContextDetailsTable(result)
	response.PresentationTable.Following.Following.Following = releaseContextLimitationsTable()
}

func releaseContextCompleteChecksTable(checks []ReleaseContextCheck) *presentation.Table {
	return &presentation.Table{
		Title: "Context Checks", DescribeOnly: true,
		Columns: releaseContextCheckColumns(),
		Rows:    releaseContextCheckRows(checks, false),
	}
}

func releaseContextCheckColumns() []presentation.Column {
	return []presentation.Column{
		{Key: "check", Label: "Check", Essential: true},
		{Key: "status", Label: "Status", Essential: true},
		{Key: "subject", Label: "Subject", Essential: true},
		{Key: "expected", Label: "Expected"},
		{Key: "actual", Label: "Actual"},
		{Key: "guidance", Label: "Guidance"},
	}
}

func releaseContextCheckRows(checks []ReleaseContextCheck, failedOnly bool) []map[string]any {
	rows := make([]map[string]any, 0, len(checks))
	for _, check := range checks {
		if failedOnly && check.Status != "failed" {
			continue
		}
		rows = append(rows, map[string]any{
			"check": check.Name, "status": check.Status, "subject": check.Subject,
			"expected": check.Expected, "actual": check.Actual, "guidance": check.Guidance,
		})
	}
	return rows
}

func releaseContextDetailsTable(result *ValidatedReleaseContext) *presentation.Table {
	properties := make([]presentation.Property, 0, len(releaseContextCompleteProperties))
	for _, property := range releaseContextCompleteProperties {
		value := resultPresentationValue(result, property.Key)
		properties = append(properties, presentation.Property{Label: property.Label, Value: value})
	}
	return &presentation.Table{
		Title: "Resolved Context", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "unit", Label: "Unit", Essential: true},
			{Key: "version", Label: "Version", Essential: true},
			{Key: "tag", Label: "Tag", Essential: true},
			{Key: "release_sha", Label: "Release commit"},
			{Key: "source", Label: "Source"},
		},
		Rows: []map[string]any{{
			"unit": result.UnitID, "version": result.Version, "tag": result.Tag,
			"release_sha": result.ReleaseSHA, "source": "V2 config, state, and local Git",
		}},
		Details: &presentation.Properties{Properties: properties},
	}
}

func resultPresentationValue(result *ValidatedReleaseContext, key string) any {
	switch key {
	case "valid":
		return result.HeadMatches && result.TagTargetMatches
	case "unit":
		return result.UnitID
	case "display_name":
		return result.DisplayName
	case "version":
		return result.Version
	case "tag_prefix":
		return result.TagPrefix
	case "tag":
		return result.Tag
	case "release_sha":
		return result.ReleaseSHA
	case "working_directory":
		if filepath.IsAbs(result.WorkingDirectory) {
			return "repository-local path"
		}
		return result.WorkingDirectory
	case "executor":
		return result.Executor
	case "delivery":
		return result.Delivery
	case "workflow":
		return result.Workflow
	case "git_object_format":
		return string(result.GitObjectFormat)
	case "head_matches":
		return result.HeadMatches
	case "tag_target_matches":
		return result.TagTargetMatches
	default:
		return ""
	}
}

func releaseContextGitHubMappingTable() *presentation.Table {
	rows := make([]map[string]any, 0, len(releaseContextGitHubOutputFields))
	for _, field := range releaseContextGitHubOutputFields {
		rows = append(rows, map[string]any{"key": field.Name, "source": field.DataKey})
	}
	return &presentation.Table{
		Title: "GitHub Output Mapping", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "key", Label: "Output key", Essential: true},
			{Key: "source", Label: "Validated fact", Essential: true},
		},
		Rows: rows,
	}
}

func releaseContextLimitationsTable() *presentation.Table {
	return &presentation.Table{
		Title: "Limitations", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "area", Label: "Area", Essential: true},
			{Key: "scope", Label: "Scope", Essential: true},
			{Key: "details", Label: "Details"},
		},
		Rows: []map[string]any{
			{"area": "Repository", "scope": "Local checkout", "details": "No remote provider state is inspected"},
			{"area": "Credentials", "scope": "Token free", "details": "No GitHub token or credential is read"},
			{"area": "Mutation", "scope": "Read only", "details": "No Git, workflow, release, upload, or publication mutation is performed"},
		},
	}
}
