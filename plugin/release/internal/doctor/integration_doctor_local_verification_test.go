package doctor

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestRepositoryDoctorRepresentsFocusedLocalVerification(t *testing.T) {
	result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, repositoryInspectionRoot(t), nil))
	if result.Summary.Verified != 15 {
		t.Fatalf("verified facts = %d, want 15", result.Summary.Verified)
	}
	if len(result.Verifications) != 24 {
		t.Fatalf("verifications = %d, want 24: %#v", len(result.Verifications), result.Verifications)
	}

	byWorkflow := make(map[string][]string)
	boundariesByWorkflow := make(map[string][]string)
	for _, fact := range result.Verifications {
		if fact.Evidence == "" || len(fact.References) == 0 {
			t.Errorf("repository fact lacks evidence references: %#v", fact)
		}
		for _, reference := range fact.References {
			if strings.HasPrefix(reference, "/") || strings.Contains(reference, "\\") {
				t.Errorf("fact exposes non-portable reference %q", reference)
			}
		}
		if fact.State == integrationDoctorVerified {
			byWorkflow[fact.Workflow] = append(byWorkflow[fact.Workflow], fact.Category)
			continue
		}
		if fact.State != integrationDoctorUnverifiable || fact.LimitationClass == "" {
			t.Errorf("repository boundary fact has invalid state: %#v", fact)
		}
		boundariesByWorkflow[fact.Workflow] = append(boundariesByWorkflow[fact.Workflow], fact.Category)
	}
	wantCategories := []string{
		"consumer_structure",
		"credential_wiring",
		"goreleaser_configuration",
		"installation_wiring",
		"publication_identity",
	}
	for _, behavior := range repositoryWorkflowBehaviors() {
		got := append([]string(nil), byWorkflow[behavior.path]...)
		sort.Strings(got)
		if !reflect.DeepEqual(got, wantCategories) {
			t.Errorf("%s verification categories = %v, want %v", behavior.path, got, wantCategories)
		}
		gotBoundaries := append([]string(nil), boundariesByWorkflow[behavior.path]...)
		sort.Strings(gotBoundaries)
		wantBoundaries := []string{"dispatch_authorization", "remote_workflow_identity", "repository_variable_values"}
		if !reflect.DeepEqual(gotBoundaries, wantBoundaries) {
			t.Errorf("%s boundary categories = %v, want %v", behavior.path, gotBoundaries, wantBoundaries)
		}
	}
}

func TestRepositoryDoctorLimitationsDescribeOnlyResidualUncertainty(t *testing.T) {
	result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, repositoryInspectionRoot(t), nil))
	if result.Summary.NotVerifiable != 21 {
		t.Fatalf("not-verifiable count = %d, want 21", result.Summary.NotVerifiable)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity != integrationDoctorNotVerifiable || diagnostic.Code != "CONSUMER_BUILD_NOT_VERIFIABLE" {
			continue
		}
		for _, broad := range []string{"cannot be proven correct by local structural inspection"} {
			if strings.Contains(strings.ToLower(diagnostic.Message), strings.ToLower(broad)) {
				t.Errorf("diagnostic remains broader than local evidence: %#v", diagnostic)
			}
		}
		if !strings.Contains(diagnostic.Message, "locally verified") {
			t.Errorf("diagnostic does not identify its verified local basis: %#v", diagnostic)
		}
	}
}

func TestIntegrationDoctorVerificationOrderingIsDeterministic(t *testing.T) {
	first := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, repositoryInspectionRoot(t), nil))
	second := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, repositoryInspectionRoot(t), nil))
	if !reflect.DeepEqual(first.Verifications, second.Verifications) {
		t.Fatalf("verification order changed:\nfirst=%#v\nsecond=%#v", first.Verifications, second.Verifications)
	}
}
