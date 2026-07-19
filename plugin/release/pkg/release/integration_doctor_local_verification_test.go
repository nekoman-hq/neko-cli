package release

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestRepositoryDoctorRepresentsFocusedLocalVerification(t *testing.T) {
	result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, repositoryInspectionRoot(t), nil))
	if result.Summary.Verified != 9 {
		t.Fatalf("verified facts = %d, want 9", result.Summary.Verified)
	}
	if len(result.Verifications) != 9 {
		t.Fatalf("verifications = %d, want 9: %#v", len(result.Verifications), result.Verifications)
	}

	byWorkflow := make(map[string][]string)
	for _, fact := range result.Verifications {
		if fact.State != integrationDoctorVerified {
			t.Errorf("repository fact is not verified: %#v", fact)
		}
		if fact.Evidence == "" || len(fact.References) == 0 {
			t.Errorf("repository fact lacks evidence references: %#v", fact)
		}
		for _, reference := range fact.References {
			if strings.HasPrefix(reference, "/") || strings.Contains(reference, "\\") {
				t.Errorf("fact exposes non-portable reference %q", reference)
			}
		}
		byWorkflow[fact.Workflow] = append(byWorkflow[fact.Workflow], fact.Category)
	}
	wantCategories := []string{
		"consumer_structure",
		"goreleaser_configuration",
		"installation_wiring",
	}
	for _, behavior := range repositoryWorkflowBehaviors() {
		got := append([]string(nil), byWorkflow[behavior.path]...)
		sort.Strings(got)
		if !reflect.DeepEqual(got, wantCategories) {
			t.Errorf("%s verification categories = %v, want %v", behavior.path, got, wantCategories)
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
