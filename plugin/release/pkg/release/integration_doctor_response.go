package release

import (
	"path"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

const (
	integrationDoctorPresentationTitle = "Release Integration Doctor"
	integrationDoctorDiagnosticsTitle  = "Diagnostics"
	integrationDoctorSemanticRoleKey   = "semantic_role"
	integrationDoctorDefaultRoleKey    = "default_role"
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
		PresentationProperties: &presentation.Properties{
			Title:      integrationDoctorPresentationTitle,
			Properties: integrationDoctorSummaryProperties(result),
		},
	}
	if len(result.Diagnostics) > 0 {
		response.PresentationTable = &presentation.Table{
			Title: integrationDoctorDiagnosticsTitle,
			Columns: []presentation.Column{
				{Key: "severity", Label: "Severity", RoleKey: integrationDoctorSemanticRoleKey, Essential: true},
				{Key: "code", Label: "Code", RoleKey: integrationDoctorSemanticRoleKey, Essential: true},
				{Key: "target", Label: "Target", RoleKey: integrationDoctorDefaultRoleKey},
				{Key: "scope", Label: "Scope", RoleKey: integrationDoctorDefaultRoleKey},
			},
			Rows: integrationDoctorDiagnosticRows(result.Diagnostics),
			Details: &presentation.Properties{
				Properties: integrationDoctorDiagnosticDetailProperties(result.Diagnostics),
			},
		}
	}
	if result.Readiness == integrationDoctorNotReady {
		response.ExitCode = 1
	}
	return response
}

func integrationDoctorSummaryProperties(result *integrationDoctorResult) []presentation.Property {
	return []presentation.Property{
		{
			Label:      "Readiness",
			Value:      string(result.Readiness),
			Role:       integrationDoctorReadinessRole(result.Readiness),
			Emphasized: true,
		},
		integrationDoctorCountProperty("Errors", result.Summary.Errors, presentation.StyleError),
		integrationDoctorCountProperty("Warnings", result.Summary.Warnings, presentation.StyleWarning),
		integrationDoctorCountProperty("Recommendations", result.Summary.Recommendations, presentation.StyleInfo),
		integrationDoctorCountProperty("Not verifiable", result.Summary.NotVerifiable, presentation.StyleMuted),
		{Label: "Inspected units", Value: len(result.Units)},
		{Label: "Inspected workflows", Value: len(result.Workflows)},
	}
}

func integrationDoctorReadinessRole(readiness integrationDoctorReadiness) presentation.StyleRole {
	switch readiness {
	case integrationDoctorNotReady:
		return presentation.StyleError
	case integrationDoctorReadyWithWarnings:
		return presentation.StyleWarning
	case integrationDoctorReady:
		return presentation.StyleSuccess
	default:
		return presentation.StyleDefault
	}
}

func integrationDoctorCountProperty(
	label string,
	value int,
	positiveRole presentation.StyleRole,
) presentation.Property {
	property := presentation.Property{Label: label, Value: value}
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
			integrationDoctorDefaultRoleKey:  string(presentation.StyleDefault),
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

func integrationDoctorDiagnosticDetailProperties(diagnostics []integrationDoctorDiagnostic) []presentation.Property {
	properties := make([]presentation.Property, 0, len(diagnostics)*6)
	for _, diagnostic := range diagnostics {
		properties = append(properties, presentation.Property{
			Label:      strings.ToUpper(string(diagnostic.Severity)) + " · " + diagnostic.Code,
			Role:       integrationDoctorSeverityRole(diagnostic.Severity),
			Emphasized: true,
			Heading:    true,
		})
		properties = append(properties, presentation.Property{
			Label: "Scope", Value: diagnostic.Scope, Role: presentation.StyleDefault,
		})
		if diagnostic.Unit != "" {
			properties = append(properties, presentation.Property{
				Label: "Unit", Value: diagnostic.Unit, Role: presentation.StyleDefault,
			})
		}
		if diagnostic.Workflow != "" {
			properties = append(properties, presentation.Property{
				Label: "Workflow", Value: diagnostic.Workflow, Role: presentation.StyleDefault,
			})
		}
		properties = append(properties,
			presentation.Property{Label: "Message", Value: diagnostic.Message, Role: presentation.StyleDefault},
			presentation.Property{Label: "Remediation", Value: diagnostic.Remediation, Role: presentation.StyleDefault},
		)
	}
	return properties
}

func integrationDoctorSeverityRole(severity integrationDoctorSeverity) presentation.StyleRole {
	switch severity {
	case integrationDoctorError:
		return presentation.StyleError
	case integrationDoctorWarning:
		return presentation.StyleWarning
	case integrationDoctorRecommendation:
		return presentation.StyleInfo
	case integrationDoctorNotVerifiable:
		return presentation.StyleMuted
	default:
		return presentation.StyleDefault
	}
}
