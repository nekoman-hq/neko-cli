package doctor

import (
	"reflect"
	"testing"
)

func TestIntegrationDoctorBoundaryLimitationsArePredicateDriven(t *testing.T) {
	root := parseIntegrationDoctorWorkflowBytes(t, canonicalIntegrationDoctorWorkflow(t))
	facts, diagnostics := inspectIntegrationDoctorBoundaryLimitations(".github/workflows/release.yml", root)
	if len(facts) != 3 || len(diagnostics) != 3 {
		t.Fatalf("facts=%#v diagnostics=%#v", facts, diagnostics)
	}
	if got, want := integrationDoctorNotVerifiableCodes(diagnostics), []string{
		"REMOTE_WORKFLOW_NOT_VERIFIABLE",
		"REPOSITORY_VARIABLES_NOT_VERIFIABLE",
		"REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("codes = %v, want %v", got, want)
	}
	if facts[0].LimitationClass != integrationDoctorRemoteLimitation ||
		facts[1].LimitationClass != integrationDoctorRemoteLimitation ||
		facts[2].LimitationClass != integrationDoctorMutationRequiredLimitation {
		t.Fatalf("limitation classes = %#v", facts)
	}

	withoutVariables := parseIntegrationDoctorWorkflowBytes(t, []byte(`
on:
  workflow_dispatch:
    inputs:
      unit: {}
      version: {}
      tag: {}
      release_sha: {}
jobs: {}
`))
	facts, diagnostics = inspectIntegrationDoctorBoundaryLimitations(".github/workflows/release.yml", withoutVariables)
	if len(facts) != 2 || len(diagnostics) != 2 {
		t.Fatalf("variable-free facts=%#v diagnostics=%#v", facts, diagnostics)
	}

	withoutDispatch := parseIntegrationDoctorWorkflowBytes(t, []byte("name: local-only\njobs: {}\n"))
	facts, diagnostics = inspectIntegrationDoctorBoundaryLimitations(".github/workflows/release.yml", withoutDispatch)
	if len(facts) != 1 || len(diagnostics) != 1 || diagnostics[0].Code != "REMOTE_WORKFLOW_NOT_VERIFIABLE" {
		t.Fatalf("dispatch-free facts=%#v diagnostics=%#v", facts, diagnostics)
	}
}

func TestRepositoryDoctorRetainsEveryResidualUncertaintyWithoutBroadMessages(t *testing.T) {
	result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, repositoryInspectionRoot(t), nil))
	if result.Summary.NotVerifiable != 21 {
		t.Fatalf("not-verifiable = %d, want 21", result.Summary.NotVerifiable)
	}
	wantCodes := map[string]int{
		"CONSUMER_BUILD_NOT_VERIFIABLE":                3,
		"INSTALLATION_ARTIFACTS_NOT_VERIFIABLE":        3,
		"PUBLICATION_CREDENTIALS_NOT_VERIFIABLE":       3,
		"PUBLICATION_TARGET_NOT_VERIFIABLE":            3,
		"REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE": 3,
		"REMOTE_WORKFLOW_NOT_VERIFIABLE":               3,
		"REPOSITORY_VARIABLES_NOT_VERIFIABLE":          3,
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity != integrationDoctorNotVerifiable {
			continue
		}
		wantCodes[diagnostic.Code]--
	}
	for code, remaining := range wantCodes {
		if remaining != 0 {
			t.Errorf("diagnostic %s remaining count = %d", code, remaining)
		}
	}
}
