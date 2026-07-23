package validate

import (
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

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

func validationUnitPresentation(result validationQueryResult) *presentation.Table {
	var table *presentation.Table
	if result.SourceFormat == config.SourceFormatV1 {
		table = legacyValidationUnitPresentation(result.Legacy)
	} else {
		table = v2ValidationUnitPresentation(result.Units)
	}
	table.Title = "Validated Units"
	table.DescribeOnly = !result.Show
	table.Following = validationScopePresentation(result)
	return table
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

func validationScopePresentation(result validationQueryResult) *presentation.Table {
	source := "V2 config and state"
	configPath := ".neko/release.config.json and .neko/release.state.json"
	if result.SourceFormat == config.SourceFormatV1 {
		source = "Legacy V1 config"
		configPath = ".release.neko.json"
	}
	unitScope := "All configured units"
	if result.SelectedUnit != "" {
		unitScope = "Selected unit " + result.SelectedUnit
	}
	return &presentation.Table{
		Title: "Validation Scope", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "check", Label: "Check", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "subject", Label: "Subject", Essential: true},
			{Key: "details", Label: "Details"},
		},
		Rows: []map[string]any{
			{"check": "Release source", "status": "Valid", "subject": source, "details": configPath},
			{"check": "Unit configuration", "status": "Valid", "subject": unitScope, "details": "Normalized release facts"},
			{
				"check": "Inspection boundary", "status": "Local configuration",
				"subject": "Read-only validation",
				"details": "Remote state, release execution, and publication are not inspected",
			},
		},
	}
}
