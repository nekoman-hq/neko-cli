package validate

import (
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const (
	validationPresentationTitle = "Release Configuration Validation"
	validationSummaryRoleKey    = "value_role"
	validationUnitRoleKey       = "unit_role"
	validationVersionRoleKey    = "version_role"
	validationKindRoleKey       = "kind_role"
)

var validationSummaryColumns = []presentation.Column{
	{Key: "property", Label: "PROPERTY", Essential: true},
	{Key: "value", Label: "VALUE", RoleKey: validationSummaryRoleKey, Essential: true},
}

var v2ValidationPresentationColumns = []presentation.Column{
	{Key: "unit", Label: "Unit", RoleKey: validationUnitRoleKey, Essential: true},
	{Key: "version", Label: "Version", RoleKey: validationVersionRoleKey, Essential: true},
	{Key: "kind", Label: "Kind", RoleKey: validationKindRoleKey, Essential: true},
	{Key: "executor", Label: "Executor"},
	{Key: "delivery", Label: "Delivery"},
	{Key: "workflow", Label: "Workflow"},
}

var v1ValidationPresentationColumns = []presentation.Column{
	{Key: "unit", Label: "Unit", RoleKey: validationUnitRoleKey, Essential: true},
	{Key: "version", Label: "Version", RoleKey: validationVersionRoleKey, Essential: true},
	{Key: "project_type", Label: "Project type", Essential: true},
	{Key: "release_system", Label: "Release system"},
}

func validationSummaryPresentation(result validationQueryResult) *presentation.Properties {
	return &presentation.Properties{
		Title:      validationPresentationTitle,
		Properties: validationSummaryProperties(result),
	}
}

func validationSummaryTable(result validationQueryResult) *presentation.Table {
	properties := validationSummaryProperties(result)
	rows := make([]map[string]any, 0, len(properties))
	for _, property := range properties {
		role := property.Role
		if role == "" {
			role = presentation.StyleDefault
		}
		rows = append(rows, map[string]any{
			"property":               property.Label,
			"value":                  property.Value,
			validationSummaryRoleKey: string(role),
		})
	}
	return &presentation.Table{
		Title:   validationPresentationTitle,
		Columns: append([]presentation.Column(nil), validationSummaryColumns...),
		Rows:    rows,
	}
}

func validationSummaryProperties(result validationQueryResult) []presentation.Property {
	properties := []presentation.Property{
		{Label: "Status", Value: "✓ Valid", Role: presentation.StyleSuccess, Emphasized: true},
	}
	if result.SourceFormat == config.SourceFormatV1 {
		properties = append(properties,
			presentation.Property{Label: "Source", Value: "Legacy V1 config"},
			presentation.Property{Label: "Schema", Value: "v1"},
			presentation.Property{Label: "Configuration", Value: ".release.neko.json"},
		)
	} else {
		properties = append(properties,
			presentation.Property{Label: "Source", Value: "V2 config and state"},
			presentation.Property{Label: "Schema", Value: "v2"},
			presentation.Property{Label: "Configuration", Value: ".neko/release.config.json"},
			presentation.Property{Label: "State", Value: ".neko/release.state.json"},
		)
	}
	if result.SelectedUnit != "" {
		properties = append(properties, presentation.Property{Label: "Selected unit", Value: result.SelectedUnit})
	}
	if result.SourceFormat == config.SourceFormatV2 {
		properties = append(properties, presentation.Property{Label: "Configured units", Value: result.ConfiguredUnitCount})
	}
	return properties
}

func validationUnitPresentation(result validationQueryResult) *presentation.Table {
	if !result.Show {
		return nil
	}
	if result.SourceFormat == config.SourceFormatV1 {
		return legacyValidationUnitPresentation(result.Legacy)
	}
	return v2ValidationUnitPresentation(result.Units)
}

func v2ValidationUnitPresentation(units []config.ReleaseUnit) *presentation.Table {
	rows := make([]map[string]any, 0, len(units))
	for _, unit := range units {
		rows = append(rows, map[string]any{
			"unit":                   unit.ID,
			"version":                unit.Version,
			"kind":                   validationUnitKind(unit),
			"executor":               unit.ExecutorType,
			"delivery":               unit.Delivery,
			"workflow":               validationWorkflowShowValue(unit),
			validationUnitRoleKey:    string(presentation.StyleEmphasis),
			validationVersionRoleKey: string(presentation.StyleInfo),
			validationKindRoleKey:    string(validationUnitKindRole(unit)),
		})
	}
	return &presentation.Table{
		Columns: append([]presentation.Column(nil), v2ValidationPresentationColumns...),
		Rows:    rows,
		Details: &presentation.Properties{Properties: v2ValidationUnitDetails(units)},
	}
}

func v2ValidationUnitDetails(units []config.ReleaseUnit) []presentation.Property {
	properties := make([]presentation.Property, 0, len(units)*10)
	for _, unit := range units {
		properties = append(properties, presentation.Property{
			Label: "Unit " + unit.ID, Role: presentation.StyleInfo, Heading: true, Emphasized: true,
		})
		if unit.DisplayName != "" {
			properties = append(properties, presentation.Property{Label: "Display name", Value: unit.DisplayName})
		}
		properties = append(properties,
			presentation.Property{Label: "Version", Value: unit.Version, Role: presentation.StyleInfo},
			presentation.Property{Label: "Kind", Value: validationUnitKind(unit), Role: validationUnitKindRole(unit)},
			presentation.Property{Label: "Working directory", Value: unit.WorkingDirectory},
			presentation.Property{Label: "Tag prefix", Value: unit.TagPrefix},
			presentation.Property{Label: "Executor", Value: unit.ExecutorType},
			presentation.Property{Label: "Delivery", Value: unit.Delivery},
			presentation.Property{Label: "Workflow", Value: validationWorkflowShowValue(unit)},
			presentation.Property{Label: "Paths", Value: validationPathsValue(unit.Paths)},
		)
		if unit.IsPlugin {
			properties = append(properties,
				presentation.Property{Label: "Plugin name", Value: unit.PluginName},
				presentation.Property{Label: "Plugin manifest", Value: unit.PluginManifestPath},
				presentation.Property{Label: "Plugin asset prefix", Value: unit.PluginAssetPrefix},
				presentation.Property{Label: "Plugin binary", Value: unit.PluginBinaryName},
			)
		}
	}
	return properties
}

func legacyValidationUnitPresentation(legacy legacyValidationDetails) *presentation.Table {
	return &presentation.Table{
		Columns: append([]presentation.Column(nil), v1ValidationPresentationColumns...),
		Rows: []map[string]any{{
			"unit":                   legacy.UnitID,
			"version":                legacy.Version,
			"project_type":           legacy.ProjectType,
			"release_system":         legacy.ReleaseSystem,
			validationUnitRoleKey:    string(presentation.StyleEmphasis),
			validationVersionRoleKey: string(presentation.StyleInfo),
		}},
		Details: &presentation.Properties{Properties: []presentation.Property{
			{Label: "Unit " + legacy.UnitID, Role: presentation.StyleInfo, Heading: true, Emphasized: true},
			{Label: "Project name", Value: legacy.ProjectName},
			{Label: "Project owner", Value: legacy.ProjectOwner},
			{Label: "Project type", Value: legacy.ProjectType},
			{Label: "Release system", Value: legacy.ReleaseSystem},
			{Label: "Version", Value: legacy.Version, Role: presentation.StyleInfo},
		}},
	}
}

func validationUnitKind(unit config.ReleaseUnit) string {
	if unit.Kind == "" {
		return "release"
	}
	return unit.Kind
}

func validationUnitKindRole(unit config.ReleaseUnit) presentation.StyleRole {
	if unit.IsPlugin {
		return presentation.StyleInfo
	}
	return presentation.StyleDefault
}

func validationPathsValue(paths []string) string {
	if len(paths) == 0 {
		return "none configured"
	}
	return strings.Join(paths, "\n")
}
