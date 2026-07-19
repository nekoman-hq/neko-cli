package release

import (
	"path"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

const (
	integrationDoctorHumanTitle       = "Release Integration Doctor"
	integrationDoctorDiagnosticsTitle = "Diagnostics"
	integrationDoctorSemanticRoleKey  = "semantic_role"
)

func mapIntegrationDoctorResult(result *integrationDoctorResult, timestamp time.Time) *plugin.Response {
	data := map[string]any{
		"readiness":   result.Readiness,
		"summary":     result.Summary,
		"units":       append([]integrationDoctorUnit(nil), result.Units...),
		"workflows":   append([]integrationDoctorWorkflow(nil), result.Workflows...),
		"diagnostics": append([]integrationDoctorDiagnostic(nil), result.Diagnostics...),
	}
	response := &plugin.Response{
		Status:       "success",
		Metadata:     commandResponseMetadata(integrationDoctorCommandName, timestamp),
		Data:         data,
		RendererHint: "table",
		HumanProperties: &plugin.HumanProperties{
			Title:      integrationDoctorHumanTitle,
			Properties: integrationDoctorSummaryProperties(result),
		},
	}
	if len(result.Diagnostics) > 0 {
		response.HumanTable = &plugin.HumanTable{
			Title: integrationDoctorDiagnosticsTitle,
			Columns: []plugin.HumanColumn{
				{Key: "severity", Label: "Severity", RoleKey: integrationDoctorSemanticRoleKey, Essential: true},
				{Key: "code", Label: "Code", RoleKey: integrationDoctorSemanticRoleKey, Essential: true},
				{Key: "target", Label: "Target"},
				{Key: "scope", Label: "Scope"},
			},
			Rows: integrationDoctorDiagnosticRows(result.Diagnostics),
			Details: &plugin.HumanProperties{
				Properties: integrationDoctorDiagnosticDetailProperties(result.Diagnostics),
			},
		}
	}
	if result.Readiness == integrationDoctorNotReady {
		response.ExitCode = 1
	}
	return response
}

func integrationDoctorSummaryProperties(result *integrationDoctorResult) []plugin.HumanProperty {
	return []plugin.HumanProperty{
		{
			Label:      "Readiness",
			Value:      string(result.Readiness),
			Role:       integrationDoctorReadinessRole(result.Readiness),
			Emphasized: true,
		},
		integrationDoctorCountProperty("Errors", result.Summary.Errors, plugin.HumanStyleError),
		integrationDoctorCountProperty("Warnings", result.Summary.Warnings, plugin.HumanStyleWarning),
		integrationDoctorCountProperty("Recommendations", result.Summary.Recommendations, plugin.HumanStyleInfo),
		integrationDoctorCountProperty("Not verifiable", result.Summary.NotVerifiable, plugin.HumanStyleMuted),
		{Label: "Inspected units", Value: len(result.Units)},
		{Label: "Inspected workflows", Value: len(result.Workflows)},
	}
}

func integrationDoctorReadinessRole(readiness integrationDoctorReadiness) plugin.HumanStyleRole {
	switch readiness {
	case integrationDoctorNotReady:
		return plugin.HumanStyleError
	case integrationDoctorReadyWithWarnings:
		return plugin.HumanStyleWarning
	case integrationDoctorReady:
		return plugin.HumanStyleSuccess
	default:
		return plugin.HumanStyleDefault
	}
}

func integrationDoctorCountProperty(
	label string,
	value int,
	positiveRole plugin.HumanStyleRole,
) plugin.HumanProperty {
	property := plugin.HumanProperty{Label: label, Value: value}
	if value > 0 {
		property.Role = positiveRole
	}
	return property
}

func integrationDoctorDiagnosticRows(diagnostics []integrationDoctorDiagnostic) []map[string]any {
	collidingBasenames := integrationDoctorCollidingWorkflowBasenames(diagnostics)
	rows := make([]map[string]any, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		role := integrationDoctorSeverityRole(diagnostic.Severity)
		rows = append(rows, map[string]any{
			"severity":                       strings.ToUpper(string(diagnostic.Severity)),
			"code":                           diagnostic.Code,
			"target":                         integrationDoctorDiagnosticTarget(diagnostic, collidingBasenames),
			"scope":                          diagnostic.Scope,
			integrationDoctorSemanticRoleKey: string(role),
		})
	}
	return rows
}

func integrationDoctorCollidingWorkflowBasenames(diagnostics []integrationDoctorDiagnostic) map[string]bool {
	pathsByBasename := make(map[string]map[string]struct{})
	for _, diagnostic := range diagnostics {
		if diagnostic.Workflow == "" {
			continue
		}
		basename := path.Base(diagnostic.Workflow)
		if pathsByBasename[basename] == nil {
			pathsByBasename[basename] = make(map[string]struct{})
		}
		pathsByBasename[basename][diagnostic.Workflow] = struct{}{}
	}
	collisions := make(map[string]bool)
	for basename, workflowPaths := range pathsByBasename {
		collisions[basename] = len(workflowPaths) > 1
	}
	return collisions
}

func integrationDoctorDiagnosticTarget(
	diagnostic integrationDoctorDiagnostic,
	collidingBasenames map[string]bool,
) string {
	if diagnostic.Workflow != "" {
		workflow := path.Base(diagnostic.Workflow)
		if collidingBasenames[workflow] {
			workflow = diagnostic.Workflow
		}
		if diagnostic.Unit != "" {
			return diagnostic.Unit + " · " + workflow
		}
		return workflow
	}
	if diagnostic.Unit != "" {
		return diagnostic.Unit
	}
	return diagnostic.Scope
}

func integrationDoctorDiagnosticDetailProperties(diagnostics []integrationDoctorDiagnostic) []plugin.HumanProperty {
	properties := make([]plugin.HumanProperty, 0, len(diagnostics)*6)
	for _, diagnostic := range diagnostics {
		properties = append(properties, plugin.HumanProperty{
			Label:      strings.ToUpper(string(diagnostic.Severity)) + " · " + diagnostic.Code,
			Role:       integrationDoctorSeverityRole(diagnostic.Severity),
			Emphasized: true,
			Heading:    true,
		})
		properties = append(properties, plugin.HumanProperty{Label: "Scope", Value: diagnostic.Scope})
		if diagnostic.Unit != "" {
			properties = append(properties, plugin.HumanProperty{Label: "Unit", Value: diagnostic.Unit})
		}
		if diagnostic.Workflow != "" {
			properties = append(properties, plugin.HumanProperty{Label: "Workflow", Value: diagnostic.Workflow})
		}
		properties = append(properties,
			plugin.HumanProperty{Label: "Message", Value: diagnostic.Message},
			plugin.HumanProperty{Label: "Remediation", Value: diagnostic.Remediation},
		)
	}
	return properties
}

func integrationDoctorSeverityRole(severity integrationDoctorSeverity) plugin.HumanStyleRole {
	switch severity {
	case integrationDoctorError:
		return plugin.HumanStyleError
	case integrationDoctorWarning:
		return plugin.HumanStyleWarning
	case integrationDoctorRecommendation:
		return plugin.HumanStyleInfo
	case integrationDoctorNotVerifiable:
		return plugin.HumanStyleMuted
	default:
		return plugin.HumanStyleDefault
	}
}
