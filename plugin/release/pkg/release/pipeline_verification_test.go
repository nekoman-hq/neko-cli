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
	seenIDs := make(map[string]bool, len(facts))
	for _, value := range facts {
		fact, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("fact = %#v", value)
		}
		id, _ := fact["id"].(string)
		if id == "" || seenIDs[id] {
			t.Fatalf("local fact ID is empty or duplicated: %#v", fact)
		}
		seenIDs[id] = true
		if fact["source"] != "doctor" {
			t.Fatalf("fact source = %#v", fact)
		}
	}
	encoded, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		root.Path(), "pipeline-local-verification-must-not-read-this-token", "Authorization", "Bearer", "\x1b[",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("pipeline data exposed %q: %s", forbidden, encoded)
		}
	}
}

func TestPipelineMapsEveryDoctorVerificationStateExactlyOnce(t *testing.T) {
	tests := []struct {
		state           doctor.VerificationState
		class           pipelineinspection.VerificationClass
		want            pipelineinspection.VerificationStatus
		remoteRequested bool
	}{
		{doctor.VerificationVerified, pipelineinspection.VerificationLocal, pipelineinspection.VerificationVerified, false},
		{doctor.VerificationMissing, pipelineinspection.VerificationRemote, pipelineinspection.VerificationFailed, true},
		{doctor.VerificationMismatch, pipelineinspection.VerificationRemote, pipelineinspection.VerificationFailed, true},
		{doctor.VerificationUnavailable, pipelineinspection.VerificationRemote, pipelineinspection.VerificationUnavailable, true},
		{doctor.VerificationUnauthorized, pipelineinspection.VerificationRemote, pipelineinspection.VerificationUnauthorized, true},
		{doctor.VerificationRateLimited, pipelineinspection.VerificationRemote, pipelineinspection.VerificationRateLimited, true},
		{doctor.VerificationNotAttempted, pipelineinspection.VerificationRemote, pipelineinspection.VerificationNotChecked, true},
		{doctor.VerificationUnsupported, pipelineinspection.VerificationRemote, pipelineinspection.VerificationUnresolved, true},
		{doctor.VerificationUnverifiable, pipelineinspection.VerificationRemote, pipelineinspection.VerificationNotChecked, false},
		{doctor.VerificationUnverifiable, pipelineinspection.VerificationMutationRequired, pipelineinspection.VerificationUnresolved, false},
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

func TestPipelineRemoteDoctorFactsKeepRemoteProvenance(t *testing.T) {
	fact := doctor.VerificationFact{
		Subject: "owner/repository", Category: "remote_workflow_identity",
		State: doctor.VerificationVerified, Evidence: "verified through GET",
		References: []string{".git/config"}, Remote: true,
	}
	mapped := pipelineVerificationFact(fact, true)
	if mapped.Class != pipelineinspection.VerificationRemote || mapped.Status != pipelineinspection.VerificationVerified {
		t.Fatalf("remote mapping = %#v", mapped)
	}
	snapshot := doctor.VerificationSnapshot{
		Facts:  []doctor.VerificationFact{fact},
		Remote: doctor.VerificationRemoteSummary{Requested: true, Status: "complete"},
	}
	if !pipelineRemoteVerificationAttempted(snapshot) {
		t.Fatal("GET-derived fact did not mark remote verification attempted")
	}
}
