package release

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestIntegrationDoctorReusesCanonicalDispatchAndWorkflowContracts(t *testing.T) {
	contract := canonicalWorkflowDispatchInputContract()
	wantInputs := []string{"unit", "version", "tag", "release_sha"}
	if len(contract) != len(wantInputs) {
		t.Fatalf("dispatch input count = %d, want %d", len(contract), len(wantInputs))
	}
	for index, want := range wantInputs {
		if contract[index].Name != want {
			t.Fatalf("dispatch input %d = %q, want %q", index, contract[index].Name, want)
		}
	}

	spec := canonicalGitHubActionsReleaseWorkflowSpec()
	if !reflect.DeepEqual(spec.Inputs, contract) {
		t.Fatalf("workflow inputs = %#v, want canonical contract %#v", spec.Inputs, contract)
	}
	if spec.CancelReleaseInFlight {
		t.Fatal("canonical release workflow must preserve in-flight releases")
	}
	if spec.ValidationStepID != "release-context" {
		t.Fatalf("validation step id = %q, want release-context", spec.ValidationStepID)
	}
}

func TestIntegrationDoctorCanInspectCanonicalWorkflowWithoutMutation(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	target := filepath.Join(root.Path(), ".github", "workflows", "release.yml")
	canonical, err := RenderCanonicalGitHubActionsReleaseWorkflow()
	if err != nil {
		t.Fatalf("render canonical workflow: %v", err)
	}
	if mkdirErr := os.MkdirAll(filepath.Dir(target), 0755); mkdirErr != nil {
		t.Fatalf("create workflow directory: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(target, canonical, 0600); writeErr != nil {
		t.Fatalf("write canonical workflow: %v", writeErr)
	}
	preservedTime := time.Unix(1_700_000_000, 0)
	if timeErr := os.Chtimes(target, preservedTime, preservedTime); timeErr != nil {
		t.Fatalf("set workflow modification time: %v", timeErr)
	}

	_, inspected, exists, failure := inspectGitHubWorkflowOutputTarget(root.Path(), ".github/workflows/release.yml")
	if failure != nil || !exists {
		t.Fatalf("inspect workflow: exists=%t failure=%#v", exists, failure)
	}
	if !bytes.Equal(inspected, canonical) {
		t.Fatal("inspected workflow differs from canonical bytes")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat workflow: %v", err)
	}
	if !info.ModTime().Equal(preservedTime) || info.Mode().Perm() != 0600 {
		t.Fatalf("inspection mutated workflow metadata: mode=%o modtime=%s", info.Mode().Perm(), info.ModTime())
	}
}
