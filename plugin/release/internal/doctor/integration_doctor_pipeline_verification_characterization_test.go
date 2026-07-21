package doctor

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestIntegrationDoctorCharacterizesNeutralPipelineVerificationFacts(t *testing.T) {
	result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, repositoryInspectionRoot(t), nil))

	type factSignature struct {
		Category   string
		State      integrationDoctorVerificationState
		Limitation integrationDoctorLimitationClass
	}
	signatures := make([]factSignature, 0, len(result.Verifications))
	for _, fact := range result.Verifications {
		if strings.TrimSpace(fact.Subject) == "" || strings.TrimSpace(fact.Evidence) == "" {
			t.Fatalf("verification fact is not self-contained: %#v", fact)
		}
		encoded, err := json.Marshal(fact)
		if err != nil {
			t.Fatalf("marshal verification fact: %v", err)
		}
		for _, forbidden := range []string{"diagnostic", "readiness", "remediation", "presentation"} {
			if strings.Contains(strings.ToLower(string(encoded)), forbidden) {
				t.Errorf("neutral verification fact leaked %q: %s", forbidden, encoded)
			}
		}
		signatures = append(signatures, factSignature{
			Category: fact.Category, State: fact.State, Limitation: fact.LimitationClass,
		})
	}
	sort.Slice(signatures, func(left, right int) bool {
		if signatures[left].Category != signatures[right].Category {
			return signatures[left].Category < signatures[right].Category
		}
		if signatures[left].State != signatures[right].State {
			return signatures[left].State < signatures[right].State
		}
		return signatures[left].Limitation < signatures[right].Limitation
	})

	wantPerWorkflow := []factSignature{
		{Category: "consumer_structure", State: integrationDoctorVerified},
		{Category: "credential_wiring", State: integrationDoctorVerified},
		{Category: "dispatch_authorization", State: integrationDoctorUnverifiable, Limitation: integrationDoctorMutationRequiredLimitation},
		{Category: "goreleaser_configuration", State: integrationDoctorVerified},
		{Category: "installation_wiring", State: integrationDoctorVerified},
		{Category: "publication_identity", State: integrationDoctorVerified},
		{Category: "remote_workflow_identity", State: integrationDoctorUnverifiable, Limitation: integrationDoctorRemoteLimitation},
		{Category: "repository_variable_values", State: integrationDoctorUnverifiable, Limitation: integrationDoctorRemoteLimitation},
	}
	want := make([]factSignature, 0, len(wantPerWorkflow)*len(repositoryWorkflowBehaviors()))
	for range repositoryWorkflowBehaviors() {
		want = append(want, wantPerWorkflow...)
	}
	sort.Slice(want, func(left, right int) bool {
		if want[left].Category != want[right].Category {
			return want[left].Category < want[right].Category
		}
		if want[left].State != want[right].State {
			return want[left].State < want[right].State
		}
		return want[left].Limitation < want[right].Limitation
	})
	if !reflect.DeepEqual(signatures, want) {
		t.Fatalf("verification signatures changed\nwant: %#v\n got: %#v", want, signatures)
	}
}
