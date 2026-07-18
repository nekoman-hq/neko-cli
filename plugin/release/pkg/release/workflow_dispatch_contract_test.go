package release

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestCanonicalWorkflowDispatchInputContract(t *testing.T) {
	want := []workflowDispatchInputDefinition{
		{Name: "unit", Description: "Neko Release V2 unit id"},
		{Name: "version", Description: "Neko-authoritative release version"},
		{Name: "tag", Description: "Neko-created unit tag"},
		{Name: "release_sha", Description: "Neko-created release commit SHA"},
	}
	if got := canonicalWorkflowDispatchInputContract(); !reflect.DeepEqual(got, want) {
		t.Fatalf("contract = %#v, want %#v", got, want)
	}

	values := canonicalWorkflowDispatchInputValues("api", "1.2.3", "api/v1.2.3", "release-commit")
	wantValues := []string{"api", "1.2.3", "api/v1.2.3", "release-commit"}
	for index, definition := range want {
		wantValue := wantValues[index]
		if values[definition.Name] != wantValue {
			t.Fatalf("value %q = %q, want %q", definition.Name, values[definition.Name], wantValue)
		}
	}
	if len(values) != len(want) {
		t.Fatalf("value count = %d, want %d", len(values), len(want))
	}
}

func TestWorkflowDispatchConsumersUseCanonicalContractOwner(t *testing.T) {
	consumers := map[string][]string{
		"release_dispatch_request.go":       {"canonicalWorkflowDispatchInputValues"},
		"github_actions_dispatch_client.go": {"canonicalWorkflowDispatchInputContract"},
		"github_actions_workflow_spec.go":   {"canonicalWorkflowDispatchInputContract"},
		"release_context_validation.go":     {"workflowDispatchInputUnit", "workflowDispatchInputVersion", "workflowDispatchInputTag", "workflowDispatchInputReleaseSHA"},
		"release_context_response.go":       {"workflowDispatchInputUnit", "workflowDispatchInputVersion", "workflowDispatchInputTag", "workflowDispatchInputReleaseSHA"},
	}
	for path, required := range consumers {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, symbol := range required {
			if !strings.Contains(string(source), symbol) {
				t.Errorf("%s does not use canonical workflow dispatch contract symbol %q", path, symbol)
			}
		}
	}
}
