package doctor

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
	"gopkg.in/yaml.v3"
)

func TestIntegrationDoctorReusesCanonicalDispatchAndWorkflowContracts(t *testing.T) {
	contract := releaseworkflow.CanonicalDispatchInputContract()
	wantInputs := []string{"unit", "version", "tag", "release_sha"}
	if len(contract) != len(wantInputs) {
		t.Fatalf("dispatch input count = %d, want %d", len(contract), len(wantInputs))
	}
	for index, want := range wantInputs {
		if contract[index].Name != want {
			t.Fatalf("dispatch input %d = %q, want %q", index, contract[index].Name, want)
		}
	}

	canonical, err := releaseworkflow.RenderCanonicalGitHubActionsReleaseWorkflow()
	if err != nil {
		t.Fatalf("render canonical workflow: %v", err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(canonical, &document); err != nil {
		t.Fatalf("parse canonical workflow: %v", err)
	}
	root := workflowDocumentRoot(&document)
	dispatch := workflowMappingValue(workflowMappingValue(root, "on"), "workflow_dispatch")
	inputs := workflowMappingKeys(workflowMappingValue(dispatch, "inputs"))
	if !reflect.DeepEqual(inputs, wantInputs) {
		t.Fatalf("workflow inputs = %#v, want canonical contract %#v", inputs, wantInputs)
	}
	cancel, ok := workflowBool(workflowMappingValue(workflowMappingValue(root, "concurrency"), "cancel-in-progress"))
	if !ok || cancel {
		t.Fatal("canonical release workflow must preserve in-flight releases")
	}
	jobs := integrationDoctorWorkflowJobs(root)
	if len(jobs) != 1 {
		t.Fatalf("canonical workflow jobs = %d, want 1", len(jobs))
	}
	foundValidationStep := false
	for _, step := range jobs[0].steps {
		foundValidationStep = foundValidationStep || step.id == "release-context"
	}
	if !foundValidationStep {
		t.Fatal("canonical workflow validation step id changed")
	}
}

func TestIntegrationDoctorCanInspectCanonicalWorkflowWithoutMutation(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	target := filepath.Join(root.Path(), ".github", "workflows", "release.yml")
	canonical, err := releaseworkflow.RenderCanonicalGitHubActionsReleaseWorkflow()
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

	snapshot := (filesystemIntegrationDoctorWorkflowReader{}).Read(root.Path(), ".github/workflows/release.yml")
	if snapshot.FailureCode != "" || !snapshot.Exists {
		t.Fatalf("inspect workflow: exists=%t failure=%s", snapshot.Exists, snapshot.FailureCode)
	}
	if !bytes.Equal(snapshot.Content, canonical) {
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
