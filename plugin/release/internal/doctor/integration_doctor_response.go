package doctor

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

const (
	integrationDoctorPresentationTitle = "Release Integration Doctor"
	integrationDoctorFindingsTitle     = "Findings"
	integrationDoctorDiagnosticsTitle  = "Complete Diagnostics"
	integrationDoctorSemanticRoleKey   = "semantic_role"
	integrationDoctorDefaultRoleKey    = "default_role"
)

var integrationDoctorFactColumns = []presentation.Column{
	{Key: "check", Label: "Check", RoleKey: integrationDoctorSemanticRoleKey, Essential: true},
	{Key: "status", Label: "Status", RoleKey: integrationDoctorSemanticRoleKey, Essential: true},
	{Key: "scope", Label: "Scope", RoleKey: integrationDoctorDefaultRoleKey, Essential: true},
	{Key: "subject", Label: "Subject", RoleKey: integrationDoctorDefaultRoleKey},
	{Key: "evidence", Label: "Evidence", RoleKey: integrationDoctorDefaultRoleKey},
	{Key: "guidance", Label: "Guidance", RoleKey: integrationDoctorDefaultRoleKey},
}

func mapIntegrationDoctorResult(result *integrationDoctorResult, timestamp time.Time) *plugin.Response {
	data := map[string]any{
		"readiness":           result.Readiness,
		"summary":             result.Summary,
		"remote_verification": result.RemoteVerification,
		"units":               append([]integrationDoctorUnit(nil), result.Units...),
		"workflows":           append([]integrationDoctorWorkflow(nil), result.Workflows...),
		"verifications":       append([]integrationDoctorVerification{}, result.Verifications...),
		"diagnostics":         append([]integrationDoctorDiagnostic(nil), result.Diagnostics...),
	}
	response := &plugin.Response{
		Status:       "success",
		Metadata:     integrationDoctorResponseMetadata(timestamp),
		Data:         data,
		RendererHint: "table",
		PresentationProperties: &presentation.Properties{
			Title:      integrationDoctorPresentationTitle,
			Properties: integrationDoctorSummaryProperties(result),
		},
	}
	response.PresentationTable = integrationDoctorPresentationTables(result)
	if result.Readiness == integrationDoctorNotReady {
		response.ExitCode = 1
	}
	return response
}

func integrationDoctorSummaryProperties(result *integrationDoctorResult) []presentation.Property {
	verifiedLabel := "Locally verified"
	if result.RemoteVerification.Requested {
		verifiedLabel = "Verified facts"
	}
	properties := []presentation.Property{
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
		integrationDoctorCountProperty(verifiedLabel, result.Summary.Verified, presentation.StyleSuccess),
		{Label: "Inspected units", Value: len(result.Units)},
		{Label: "Inspected workflows", Value: len(result.Workflows)},
	}
	if len(result.Units) == 1 {
		properties = append(properties, presentation.Property{Label: "Selected unit", Value: result.Units[0].ID})
	}
	properties = append(properties,
		presentation.Property{Label: "Inspection scope", Value: integrationDoctorInspectionScope(result)},
		presentation.Property{Label: "Local verification", Value: integrationDoctorVerificationSummary(result.Verifications, false)},
	)
	if result.RemoteVerification.Requested {
		properties = append(properties,
			presentation.Property{Label: "Remote verification", Value: string(result.RemoteVerification.Status), Emphasized: true},
			integrationDoctorCountProperty("Remote verified", result.RemoteVerification.Verified, presentation.StyleSuccess),
			integrationDoctorCountProperty("Remote unresolved", result.RemoteVerification.Unresolved, presentation.StyleMuted),
			integrationDoctorCountProperty("Remote failed", result.RemoteVerification.Failed, presentation.StyleError),
		)
	}
	return properties
}

func integrationDoctorInspectionScope(result *integrationDoctorResult) string {
	if result.RemoteVerification.Requested {
		return "Local and explicit remote verification"
	}
	return "Local verification only"
}

func integrationDoctorVerificationSummary(verifications []integrationDoctorVerification, remote bool) string {
	verified := 0
	attention := 0
	for _, verification := range verifications {
		if verification.Remote != remote {
			continue
		}
		if verification.State == integrationDoctorVerified {
			verified++
		} else if verification.State != integrationDoctorNotAttempted {
			attention++
		}
	}
	return fmt.Sprintf("%d verified, %d require attention", verified, attention)
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

func integrationDoctorPresentationTables(result *integrationDoctorResult) *presentation.Table {
	findings := integrationDoctorFindingRows(result)
	var first *presentation.Table
	if len(findings) > 0 {
		first = &presentation.Table{
			Title: integrationDoctorFindingsTitle, Columns: integrationDoctorColumns(),
			Rows: findings,
		}
	}
	tables := []*presentation.Table{
		integrationDoctorCompleteDiagnosticTable(result.Diagnostics),
		integrationDoctorVerificationTable(result.Verifications),
		integrationDoctorUnitTable(result.Units),
		integrationDoctorWorkflowTable(result.Workflows),
		integrationDoctorLimitationTable(result.Verifications),
	}
	for _, table := range tables {
		if table == nil {
			continue
		}
		if first == nil {
			first = table
			continue
		}
		tail := first
		for tail.Following != nil {
			tail = tail.Following
		}
		tail.Following = table
	}
	return first
}

func integrationDoctorColumns() []presentation.Column {
	return append([]presentation.Column(nil), integrationDoctorFactColumns...)
}

func integrationDoctorFindingRows(result *integrationDoctorResult) []map[string]any {
	rows := integrationDoctorDiagnosticRows(result.Diagnostics)
	for _, verification := range result.Verifications {
		if !integrationDoctorVerificationActionable(verification, result.RemoteVerification.Requested) {
			continue
		}
		rows = append(rows, integrationDoctorVerificationRow(verification))
	}
	return rows
}

func integrationDoctorVerificationActionable(
	verification integrationDoctorVerification,
	remoteRequested bool,
) bool {
	switch verification.State {
	case integrationDoctorMissing, integrationDoctorMismatch,
		integrationDoctorUnauthorized, integrationDoctorRateLimited:
		return true
	case integrationDoctorUnavailable:
		return !verification.Remote || remoteRequested
	case integrationDoctorUnsupported, integrationDoctorUnverifiable:
		return verification.LimitationClass != ""
	case integrationDoctorVerified, integrationDoctorNotAttempted:
		return false
	default:
		return false
	}
}

func integrationDoctorCompleteDiagnosticTable(diagnostics []integrationDoctorDiagnostic) *presentation.Table {
	if len(diagnostics) == 0 {
		return nil
	}
	return &presentation.Table{
		Title: integrationDoctorDiagnosticsTitle, DescribeOnly: true,
		Columns: integrationDoctorColumns(),
		Rows:    integrationDoctorDiagnosticRows(diagnostics),
		Details: &presentation.Properties{
			Properties: integrationDoctorDiagnosticDetailProperties(diagnostics),
		},
	}
}

func integrationDoctorDiagnosticRows(diagnostics []integrationDoctorDiagnostic) []map[string]any {
	collidingBasenames := integrationDoctorCollidingWorkflowBasenames(diagnostics)
	rows := make([]map[string]any, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		role := integrationDoctorSeverityRole(diagnostic.Severity)
		rows = append(rows, map[string]any{
			"check":                          integrationDoctorReadableLabel(diagnostic.Code),
			"status":                         integrationDoctorReadableLabel(string(diagnostic.Severity)),
			"scope":                          integrationDoctorReadableLabel(diagnostic.Scope),
			"subject":                        integrationDoctorDiagnosticTarget(diagnostic, collidingBasenames),
			"evidence":                       diagnostic.Message,
			"guidance":                       diagnostic.Remediation,
			integrationDoctorSemanticRoleKey: string(role),
			integrationDoctorDefaultRoleKey:  string(presentation.StyleDefault),
		})
	}
	return rows
}

func integrationDoctorVerificationTable(verifications []integrationDoctorVerification) *presentation.Table {
	if len(verifications) == 0 {
		return nil
	}
	rows := make([]map[string]any, 0, len(verifications))
	for _, verification := range verifications {
		rows = append(rows, integrationDoctorVerificationRow(verification))
	}
	return &presentation.Table{
		Title: "Verification Facts", DescribeOnly: true,
		Columns: integrationDoctorColumns(),
		Rows:    rows,
		Details: &presentation.Properties{
			Properties: integrationDoctorVerificationDetailProperties(verifications),
		},
	}
}

func integrationDoctorVerificationRow(verification integrationDoctorVerification) map[string]any {
	scope := "Local"
	if verification.Remote {
		scope = "Remote"
	}
	return map[string]any{
		"check":                          integrationDoctorReadableLabel(verification.Category),
		"status":                         integrationDoctorReadableLabel(string(verification.State)),
		"scope":                          scope,
		"subject":                        integrationDoctorSafeValue(verification.Subject),
		"evidence":                       integrationDoctorSafeValue(verification.Evidence),
		"guidance":                       "",
		integrationDoctorSemanticRoleKey: string(integrationDoctorVerificationStateRole(verification.State)),
		integrationDoctorDefaultRoleKey:  string(presentation.StyleDefault),
	}
}

func integrationDoctorVerificationStateRole(state integrationDoctorVerificationState) presentation.StyleRole {
	switch state {
	case integrationDoctorVerified:
		return presentation.StyleSuccess
	case integrationDoctorMissing, integrationDoctorMismatch, integrationDoctorUnauthorized:
		return presentation.StyleError
	case integrationDoctorRateLimited, integrationDoctorUnavailable,
		integrationDoctorUnsupported, integrationDoctorUnverifiable:
		return presentation.StyleWarning
	default:
		return presentation.StyleMuted
	}
}

func integrationDoctorVerificationDetailProperties(
	verifications []integrationDoctorVerification,
) []presentation.Property {
	properties := make([]presentation.Property, 0, len(verifications)*6)
	for _, verification := range verifications {
		properties = append(properties, presentation.Property{
			Label: integrationDoctorReadableLabel(verification.Category) + " · " +
				integrationDoctorReadableLabel(string(verification.State)),
			Role:    integrationDoctorVerificationStateRole(verification.State),
			Heading: true, Emphasized: true,
		})
		properties = append(properties,
			presentation.Property{Label: "Scope", Value: integrationDoctorVerificationScope(verification)},
			presentation.Property{Label: "Subject", Value: integrationDoctorSafeValue(verification.Subject)},
			presentation.Property{Label: "Evidence", Value: integrationDoctorSafeValue(verification.Evidence)},
		)
		if len(verification.References) > 0 {
			properties = append(properties, presentation.Property{
				Label: "References", Value: strings.Join(verification.References, "\n"),
			})
		}
		if verification.LimitationClass != "" {
			properties = append(properties, presentation.Property{
				Label: "Limitation", Value: integrationDoctorReadableLabel(string(verification.LimitationClass)),
			})
		}
	}
	return properties
}

func integrationDoctorVerificationScope(verification integrationDoctorVerification) string {
	if verification.Remote {
		return "Remote"
	}
	return "Local"
}

func integrationDoctorUnitTable(units []integrationDoctorUnit) *presentation.Table {
	if len(units) == 0 {
		return nil
	}
	rows := make([]map[string]any, 0, len(units))
	for _, unit := range units {
		rows = append(rows, map[string]any{
			"unit": unit.ID, "version": unit.Version, "executor": unit.Executor,
			"delivery": unit.Delivery, "tag": unit.TagPrefix + "<version>",
			"workflow":          integrationDoctorSafePath(unit.Workflow),
			"working_directory": integrationDoctorSafePath(unit.WorkingDirectory),
		})
	}
	return &presentation.Table{
		Title: "Configured Units", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "unit", Label: "Unit", Essential: true},
			{Key: "version", Label: "Version", Essential: true},
			{Key: "executor", Label: "Executor", Essential: true},
			{Key: "delivery", Label: "Delivery"},
			{Key: "tag", Label: "Tag shape"},
			{Key: "workflow", Label: "Workflow"},
			{Key: "working_directory", Label: "Working directory"},
		},
		Rows: rows,
	}
}

func integrationDoctorWorkflowTable(workflows []integrationDoctorWorkflow) *presentation.Table {
	if len(workflows) == 0 {
		return nil
	}
	rows := make([]map[string]any, 0, len(workflows))
	for _, workflow := range workflows {
		rows = append(rows, map[string]any{
			"workflow": integrationDoctorSafePath(workflow.Path),
			"status":   integrationDoctorWorkflowExistence(workflow.Exists),
			"scope":    strings.Join(workflow.Units, ", "),
			"kind":     integrationDoctorReadableLabel(workflow.Classification),
		})
	}
	return &presentation.Table{
		Title: "Configured Workflows", DescribeOnly: true,
		Columns: []presentation.Column{
			{Key: "workflow", Label: "Workflow", Essential: true},
			{Key: "status", Label: "Status", Essential: true},
			{Key: "scope", Label: "Units", Essential: true},
			{Key: "kind", Label: "Classification"},
		},
		Rows: rows,
	}
}

func integrationDoctorWorkflowExistence(exists bool) string {
	if exists {
		return "Present"
	}
	return "Missing"
}

func integrationDoctorLimitationTable(
	verifications []integrationDoctorVerification,
) *presentation.Table {
	rows := make([]map[string]any, 0)
	for _, verification := range verifications {
		if verification.LimitationClass == "" {
			continue
		}
		rows = append(rows, map[string]any{
			"check":                          integrationDoctorReadableLabel(verification.Category),
			"status":                         integrationDoctorReadableLabel(string(verification.State)),
			"scope":                          integrationDoctorReadableLabel(string(verification.LimitationClass)),
			"subject":                        integrationDoctorSafeValue(verification.Subject),
			"evidence":                       integrationDoctorSafeValue(verification.Evidence),
			"guidance":                       "",
			integrationDoctorSemanticRoleKey: string(integrationDoctorVerificationStateRole(verification.State)),
			integrationDoctorDefaultRoleKey:  string(presentation.StyleDefault),
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return &presentation.Table{
		Title: "Limitations", DescribeOnly: true,
		Columns: integrationDoctorColumns(),
		Rows:    rows,
	}
}

func integrationDoctorReadableLabel(value string) string {
	fields := strings.Fields(strings.NewReplacer("_", " ", "-", " ").Replace(strings.TrimSpace(value)))
	for index, field := range fields {
		lower := strings.ToLower(field)
		fields[index] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(fields, " ")
}

func integrationDoctorSafeValue(value string) string {
	if filepath.IsAbs(strings.TrimSpace(value)) {
		return "Repository-local value"
	}
	return value
}

func integrationDoctorSafePath(value string) string {
	value = strings.TrimSpace(value)
	if filepath.IsAbs(value) {
		return "repository root"
	}
	return filepath.ToSlash(value)
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
