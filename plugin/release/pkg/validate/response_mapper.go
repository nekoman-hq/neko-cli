package validate

import (
	"fmt"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

type validationResponseClock interface {
	Now() time.Time
}

type systemValidationResponseClock struct{}

func (systemValidationResponseClock) Now() time.Time {
	return time.Now()
}

func mapValidationQueryResponse(result validationQueryResult, failure *validationQueryFailure, timestamp time.Time) *plugin.Response {
	metadataValue := plugin.ResponseMetadata{
		Plugin:    metadata.PluginName,
		Version:   metadata.Version,
		Command:   "validate",
		Timestamp: timestamp,
	}
	if failure != nil {
		response := &plugin.Response{
			Status:   "error",
			Metadata: metadataValue,
			Error: &plugin.ResponseError{
				Code:    failure.Code,
				Message: failure.Message,
			},
		}
		if failure.Hint != "" {
			response.Error.Details = map[string]any{"hint": failure.Hint}
		}
		return response
	}

	response := &plugin.Response{
		Status:       "success",
		Metadata:     metadataValue,
		Data:         map[string]any{"items": validationResponseItems(result)},
		RendererHint: "table",
	}
	response.PresentationProperties = validationSummaryPresentation(result)
	response.PresentationTable = validationUnitPresentation(result)
	return response
}

func validationResponseItems(result validationQueryResult) []map[string]any {
	if result.SourceFormat == config.SourceFormatV1 {
		return legacyValidationResponseItems(result)
	}
	return v2ValidationResponseItems(result)
}

func legacyValidationResponseItems(result validationQueryResult) []map[string]any {
	if !result.Show {
		return []map[string]any{
			{"property": "Configuration", "value": ".release.neko.json"},
			{"property": "Status", "value": "✓ Valid"},
			{"property": "Unit", "value": result.Legacy.UnitID},
		}
	}
	return []map[string]any{
		{"property": "Project Name", "value": result.Legacy.ProjectName},
		{"property": "Project Owner", "value": result.Legacy.ProjectOwner},
		{"property": "Project Type", "value": result.Legacy.ProjectType},
		{"property": "Release System", "value": result.Legacy.ReleaseSystem},
		{"property": "Version", "value": result.Legacy.Version},
		{"property": "Status", "value": "✓ Valid"},
	}
}

func v2ValidationResponseItems(result validationQueryResult) []map[string]any {
	if !result.Show {
		return []map[string]any{
			{"property": "Configuration", "value": ".neko/release.config.json"},
			{"property": "Schema", "value": "v2"},
			{"property": "Status", "value": "✓ Valid"},
		}
	}
	items := []map[string]any{{"property": "Schema", "value": "v2"}}
	for _, unit := range result.Units {
		items = append(items, validationUnitShowRow(unit))
	}
	return items
}

func validationUnitShowRow(unit config.ReleaseUnit) map[string]any {
	parts := []string{
		fmt.Sprintf("version=%s", unit.Version),
		fmt.Sprintf("workingDirectory=%s", unit.WorkingDirectory),
		fmt.Sprintf("tagPrefix=%s", unit.TagPrefix),
		fmt.Sprintf("executor=%s", unit.ExecutorType),
		fmt.Sprintf("delivery=%s", unit.Delivery),
		fmt.Sprintf("workflow=%s", validationWorkflowShowValue(unit)),
		fmt.Sprintf("paths=%v", unit.Paths),
	}
	if unit.Kind != "" {
		parts = append(parts, fmt.Sprintf("kind=%s", unit.Kind))
	}
	if unit.IsPlugin {
		parts = append(parts,
			fmt.Sprintf("plugin=%s", unit.PluginName),
			fmt.Sprintf("pluginManifest=%s", unit.PluginManifestPath),
			fmt.Sprintf("pluginAssetPrefix=%s", unit.PluginAssetPrefix),
			fmt.Sprintf("pluginBinary=%s", unit.PluginBinaryName),
		)
	}
	return map[string]any{
		"property": fmt.Sprintf("Unit %s", unit.ID),
		"value":    strings.Join(parts, " "),
	}
}

func validationWorkflowShowValue(unit config.ReleaseUnit) string {
	if unit.Workflow == "" {
		return "not applicable"
	}
	return unit.Workflow
}
