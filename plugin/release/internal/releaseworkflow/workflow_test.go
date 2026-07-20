package releaseworkflow

import (
	"bytes"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCanonicalGitHubActionsReleaseWorkflowRendersDeterministicYAML(t *testing.T) {
	first, err := RenderCanonicalGitHubActionsReleaseWorkflow()
	if err != nil {
		t.Fatalf("RenderCanonicalGitHubActionsReleaseWorkflow: %v", err)
	}
	second, err := RenderCanonicalGitHubActionsReleaseWorkflow()
	if err != nil {
		t.Fatalf("RenderCanonicalGitHubActionsReleaseWorkflow second call: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("canonical GitHub Actions release workflow bytes are not deterministic")
	}
	if len(first) == 0 || first[len(first)-1] != '\n' {
		t.Fatal("canonical GitHub Actions release workflow must end with one newline")
	}
	var parsed yaml.Node
	if err := yaml.Unmarshal(first, &parsed); err != nil {
		t.Fatalf("parse canonical GitHub Actions release workflow: %v", err)
	}
}

func TestCanonicalGitHubActionsReleaseWorkflowSpecUsesDispatchContractOrder(t *testing.T) {
	spec := canonicalGitHubActionsReleaseWorkflowSpec()
	want := []string{"unit", "version", "tag", "release_sha"}
	if spec.ContractVersion != GitHubActionsReleaseWorkflowContractVersion {
		t.Fatalf("contract version = %d", spec.ContractVersion)
	}
	if len(spec.Inputs) != len(want) {
		t.Fatalf("inputs = %#v", spec.Inputs)
	}
	for index, name := range want {
		if spec.Inputs[index].Name != name {
			t.Fatalf("input %d = %q, want %q", index, spec.Inputs[index].Name, name)
		}
	}
}
