package doctor

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestInspectLocalVerificationReturnsOnlyImmutableNeutralFacts(t *testing.T) {
	root := repositoryInspectionRoot(t)
	t.Setenv("GITHUB_TOKEN", "local-verification-must-not-read-this-token")

	first := InspectLocalVerification(context.Background(), root, "")
	if len(first.Facts) != 23 || first.Remote.Requested || first.Remote.Status != "not_requested" {
		t.Fatalf("local snapshot = %#v", first)
	}
	for _, fact := range first.Facts {
		if strings.Contains(fact.Evidence, "local-verification-must-not-read-this-token") {
			t.Fatalf("local fact contains token value: %#v", fact)
		}
	}
	first.Facts[0].References[0] = "mutated"
	second := InspectLocalVerification(context.Background(), root, "")
	if reflect.DeepEqual(first.Facts, second.Facts) {
		t.Fatal("local snapshot shares caller-mutated reference storage")
	}
	for _, fact := range second.Facts {
		for _, reference := range fact.References {
			if reference == "mutated" {
				t.Fatal("caller mutation entered a later local snapshot")
			}
		}
	}
}

func TestLocalVerificationBoundaryDoesNotConstructRemoteOrPresentationAdapters(t *testing.T) {
	content, err := os.ReadFile("pipeline_verification.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(content)
	for _, forbidden := range []string{
		"newIntegrationDoctorGitHubReadClient",
		"environmentGitHubReadTokenResolver",
		"integrationDoctorGitHubRemoteInspector",
		"integrationDoctorCommandHandler",
		"mapIntegrationDoctorResult",
		"plugin.Response",
		"github.com/nekoman-hq/neko-cli/pkg/presentation",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("local verification boundary contains %q", forbidden)
		}
	}
}

func TestInspectRemoteVerificationReusesExistingGETOnlyDoctorBoundary(t *testing.T) {
	root := repositoryInspectionRoot(t)
	server, requests := newSuccessfulIntegrationDoctorGitHubServer(t, root.Path(), false)
	defer server.Close()
	client := newIntegrationDoctorGitHubReadClientForTest(t, server.URL)
	snapshot := inspectRemoteVerification(
		context.Background(),
		root,
		"cli",
		integrationDoctorGitHubRemoteInspector{
			reader: client,
			tokens: integrationDoctorRecordingTokenResolver{value: "pipeline-read-token"},
		},
	)
	if !snapshot.Remote.Requested || snapshot.Remote.Status != "complete" || snapshot.Remote.Verified == 0 ||
		snapshot.Remote.Unresolved != 0 || snapshot.Remote.Failed != 0 {
		t.Fatalf("remote snapshot summary = %#v", snapshot.Remote)
	}
	remoteFacts := 0
	for _, fact := range snapshot.Facts {
		if fact.Remote {
			remoteFacts++
		}
		if strings.Contains(fact.Evidence, "pipeline-read-token") {
			t.Fatalf("remote fact exposed token: %#v", fact)
		}
	}
	if remoteFacts == 0 {
		t.Fatalf("remote snapshot did not identify GET-derived facts: %#v", snapshot.Facts)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"pipeline-read-token", "Authorization", "Bearer", root.Path(), "\x1b["} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("remote snapshot exposed %q: %s", forbidden, encoded)
		}
	}
	for _, request := range requests.snapshot() {
		if request.method != http.MethodGet {
			t.Fatalf("remote verification emitted %s %s", request.method, request.uri)
		}
	}
}

func TestPipelineRemoteProvenanceDoesNotChangeDoctorJSON(t *testing.T) {
	fact := integrationDoctorRemoteFact(
		"owner/repository", "remote_workflow_identity", integrationDoctorVerified,
		"verified", "", "", ".git/config",
	)
	encoded, err := json.Marshal(fact)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"remote"`) || !strings.Contains(string(encoded), `"state":"verified"`) {
		t.Fatalf("Doctor verification JSON changed: %s", encoded)
	}
}
