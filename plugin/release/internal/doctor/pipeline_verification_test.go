package doctor

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestInspectLocalVerificationReturnsOnlyImmutableNeutralFacts(t *testing.T) {
	root := repositoryInspectionRoot(t)
	t.Setenv("GITHUB_TOKEN", "local-verification-must-not-read-this-token")

	first := InspectLocalVerification(context.Background(), root, "")
	if len(first.Facts) != 24 || first.Remote.Requested || first.Remote.Status != "not_requested" {
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
