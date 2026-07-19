package release

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
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
		Status:          "success",
		Metadata:        commandResponseMetadata(integrationDoctorCommandName, timestamp),
		Data:            data,
		RendererHint:    "table",
		HumanProperties: &plugin.HumanProperties{Properties: integrationDoctorSummaryProperties(result)},
	}
	if len(result.Diagnostics) > 0 {
		response.HumanTable = &plugin.HumanTable{
			Columns: []plugin.HumanColumn{
				{Key: "severity", Label: "Severity", Essential: true},
				{Key: "code", Label: "Code", Essential: true},
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
		{Label: "Readiness", Value: string(result.Readiness)},
		{Label: "Errors", Value: result.Summary.Errors},
		{Label: "Warnings", Value: result.Summary.Warnings},
		{Label: "Recommendations", Value: result.Summary.Recommendations},
		{Label: "Not verifiable", Value: result.Summary.NotVerifiable},
		{Label: "Inspected units", Value: len(result.Units)},
		{Label: "Inspected workflows", Value: len(result.Workflows)},
	}
}

func integrationDoctorDiagnosticRows(diagnostics []integrationDoctorDiagnostic) []map[string]any {
	collidingBasenames := integrationDoctorCollidingWorkflowBasenames(diagnostics)
	rows := make([]map[string]any, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		rows = append(rows, map[string]any{
			"severity": strings.ToUpper(string(diagnostic.Severity)),
			"code":     diagnostic.Code,
			"target":   integrationDoctorDiagnosticTarget(diagnostic, collidingBasenames),
			"scope":    diagnostic.Scope,
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
	properties := make([]plugin.HumanProperty, 0, len(diagnostics)*8)
	for index, diagnostic := range diagnostics {
		properties = append(properties,
			plugin.HumanProperty{Label: "Diagnostic", Value: fmt.Sprintf("%d of %d", index+1, len(diagnostics))},
			plugin.HumanProperty{Label: "Severity", Value: strings.ToUpper(string(diagnostic.Severity))},
			plugin.HumanProperty{Label: "Code", Value: diagnostic.Code},
			plugin.HumanProperty{Label: "Scope", Value: diagnostic.Scope},
		)
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
