package doctor

import "sort"

func newIntegrationDoctorVerification(
	category string,
	state integrationDoctorVerificationState,
	evidence string,
	workflow string,
	unit string,
	references ...string,
) integrationDoctorVerification {
	unique := make(map[string]struct{}, len(references))
	ordered := make([]string, 0, len(references))
	for _, reference := range references {
		if reference == "" {
			continue
		}
		if _, exists := unique[reference]; exists {
			continue
		}
		unique[reference] = struct{}{}
		ordered = append(ordered, reference)
	}
	sort.Strings(ordered)
	return integrationDoctorVerification{
		Subject:    workflow,
		Category:   category,
		State:      state,
		Evidence:   evidence,
		References: ordered,
		Unit:       unit,
		Workflow:   workflow,
	}
}

func newIntegrationDoctorWorkflowDiagnostic(
	severity integrationDoctorSeverity,
	workflow string,
	unit string,
	code string,
	message string,
	remediation string,
) integrationDoctorDiagnostic {
	diagnostic := newIntegrationDoctorDiagnostic(severity, "workflow", code, message, remediation)
	diagnostic.Workflow = workflow
	diagnostic.Unit = unit
	return diagnostic
}
