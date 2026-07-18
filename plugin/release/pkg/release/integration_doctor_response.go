package release

import (
	"fmt"
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
		HumanProperties: &plugin.HumanProperties{Properties: integrationDoctorHumanProperties(result)},
	}
	if result.Readiness == integrationDoctorNotReady {
		response.ExitCode = 1
	}
	return response
}

func integrationDoctorHumanProperties(result *integrationDoctorResult) []plugin.HumanProperty {
	properties := []plugin.HumanProperty{
		{Label: "Readiness", Value: string(result.Readiness)},
		{Label: "Errors", Value: result.Summary.Errors},
		{Label: "Warnings", Value: result.Summary.Warnings},
		{Label: "Recommendations", Value: result.Summary.Recommendations},
		{Label: "Not verifiable", Value: result.Summary.NotVerifiable},
	}
	for _, unit := range result.Units {
		properties = append(properties, plugin.HumanProperty{
			Label: "Unit " + unit.ID,
			Value: fmt.Sprintf(
				"version %s; tag prefix %s; executor %s; delivery %s; workflow %s",
				unit.Version, unit.TagPrefix, unit.Executor, unit.Delivery, unit.Workflow,
			),
		})
	}
	for _, workflow := range result.Workflows {
		properties = append(properties, plugin.HumanProperty{
			Label: "Workflow",
			Value: fmt.Sprintf(
				"%s — %s; units %s",
				workflow.Path, workflow.Classification, strings.Join(workflow.Units, ", "),
			),
		})
	}
	for _, diagnostic := range result.Diagnostics {
		location := diagnostic.Scope
		if diagnostic.Unit != "" {
			location += " " + diagnostic.Unit
		}
		if diagnostic.Workflow != "" {
			location += " " + diagnostic.Workflow
		}
		properties = append(properties, plugin.HumanProperty{
			Label: strings.ToUpper(string(diagnostic.Severity)) + " " + diagnostic.Code,
			Value: fmt.Sprintf(
				"%s: %s Remediation: %s",
				location, diagnostic.Message, diagnostic.Remediation,
			),
		})
	}
	return properties
}
