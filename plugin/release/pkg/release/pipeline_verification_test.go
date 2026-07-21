package release

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/doctor"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/pipelineinspection"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestPipelineDefaultIncludesLocalDoctorVerificationWithoutTokenExposure(t *testing.T) {
	root, err := workspace.ValidateRepositoryRoot("../../../..")
	if err != nil {
		t.Fatalf("ValidateRepositoryRoot: %v", err)
	}
	t.Setenv("GITHUB_TOKEN", "pipeline-local-verification-must-not-read-this-token")
	response, err := HandlePipelineAt(root, plugin.Request{
		Command: "pipeline", Flags: map[string]any{"unit": "plugin-release"},
	})
	if err != nil {
		t.Fatalf("HandlePipelineAt: %v", err)
	}
	verification := pipelineJSONView(t, response.Data["verification"])
	summary, ok := verification["summary"].(map[string]any)
	if !ok {
		t.Fatalf("verification summary = %#v", verification["summary"])
	}
	if summary["remote_requested"] != false || summary["remote_attempted"] != false || summary["remote_status"] != "not_requested" {
		t.Fatalf("default remote summary = %#v", summary)
	}
	facts, ok := verification["facts"].([]any)
	if !ok || len(facts) == 0 {
		t.Fatalf("local facts = %#v", verification["facts"])
	}
	encoded, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "pipeline-local-verification-must-not-read-this-token") {
		t.Fatalf("pipeline data exposed token: %s", encoded)
	}
}

func TestPipelineMapsEveryDoctorVerificationStateExactlyOnce(t *testing.T) {
	tests := []struct {
		state           doctor.VerificationState
		class           pipelineinspection.VerificationClass
		remoteRequested bool
		want            pipelineinspection.VerificationStatus
	}{
		{doctor.VerificationVerified, pipelineinspection.VerificationLocal, false, pipelineinspection.VerificationVerified},
		{doctor.VerificationMissing, pipelineinspection.VerificationRemote, true, pipelineinspection.VerificationFailed},
		{doctor.VerificationMismatch, pipelineinspection.VerificationRemote, true, pipelineinspection.VerificationFailed},
		{doctor.VerificationUnavailable, pipelineinspection.VerificationRemote, true, pipelineinspection.VerificationUnavailable},
		{doctor.VerificationUnauthorized, pipelineinspection.VerificationRemote, true, pipelineinspection.VerificationUnauthorized},
		{doctor.VerificationRateLimited, pipelineinspection.VerificationRemote, true, pipelineinspection.VerificationRateLimited},
		{doctor.VerificationNotAttempted, pipelineinspection.VerificationRemote, true, pipelineinspection.VerificationNotChecked},
		{doctor.VerificationUnsupported, pipelineinspection.VerificationRemote, true, pipelineinspection.VerificationUnresolved},
		{doctor.VerificationUnverifiable, pipelineinspection.VerificationRemote, false, pipelineinspection.VerificationNotChecked},
		{doctor.VerificationUnverifiable, pipelineinspection.VerificationMutationRequired, false, pipelineinspection.VerificationUnresolved},
	}
	for _, test := range tests {
		if got := pipelineVerificationStatus(test.state, test.class, test.remoteRequested); got != test.want {
			t.Errorf("state=%q class=%q requested=%t: got %q, want %q", test.state, test.class, test.remoteRequested, got, test.want)
		}
	}

	classes := map[doctor.VerificationLimitation]pipelineinspection.VerificationClass{
		"":                                   pipelineinspection.VerificationLocal,
		doctor.VerificationRemoteLimitation:  pipelineinspection.VerificationRemote,
		doctor.VerificationRuntimeLimitation: pipelineinspection.VerificationRuntimeRequired,
		doctor.VerificationMutationRequiredLimitation: pipelineinspection.VerificationMutationRequired,
	}
	for limitation, want := range classes {
		if got := pipelineVerificationClass(limitation); got != want {
			t.Errorf("limitation %q = %q, want %q", limitation, got, want)
		}
	}
}

func TestPipelineVerificationMappingDoesNotMutateDoctorFacts(t *testing.T) {
	fact := doctor.VerificationFact{
		Subject: "workflow", Category: "consumer_structure", State: doctor.VerificationVerified,
		Evidence: "verified", References: []string{"workflow"}, Workflow: "workflow",
	}
	want := append([]string(nil), fact.References...)
	mapped := pipelineVerificationFact(fact, false)
	mapped.References[0] = "mutated"
	if !reflect.DeepEqual(fact.References, want) {
		t.Fatalf("mapping mutated Doctor fact: %#v", fact)
	}
}
