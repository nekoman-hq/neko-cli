package release

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestConfiguredReleaseLifecycleStagesMatchDirectProductionOrder(t *testing.T) {
	stages := configuredReleaseLifecycleStages()
	wantIDs := []string{
		"source-unit-resolution",
		"release-context-planning",
		"dispatch-token-resolution",
		"release-file-planning",
		"release-preflight",
		"execution-journal-preparation",
		"release-file-materialization",
		"selected-unit-state-write",
		"known-release-file-staging",
		"release-commit-creation",
		"unit-tag-creation",
		"workflow-request-preparation",
		"release-commit-push",
		"unit-tag-push",
		"workflow-request-submission",
		"handoff-confirmation",
	}
	gotIDs := make([]string, 0, len(stages))
	for _, stage := range stages {
		gotIDs = append(gotIDs, stage.ID)
		if stage.ConfigurationStatus != "configured" || stage.Owner == "" || stage.Location == "" || stage.Mutation == "" || stage.Source == "" {
			t.Errorf("incomplete stage metadata: %#v", stage)
		}
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("stage IDs = %#v, want %#v", gotIDs, wantIDs)
	}
	assertLifecycleSourceOrder(t, "release_start_v2.go",
		"config.ResolveReleaseUnit(",
		"BuildV2ReleaseExecutionContext(",
	)
	assertLifecycleSourceOrder(t, "github_actions_release_use_case.go",
		"tokenResolver.ResolveGitHubActionsDispatchToken(",
		"planner.Plan(",
		"preflightValidator.Validate(",
		"executionPreparer.Prepare(",
		"materialization.Apply(",
		"stateWriter.Write(",
		"fileStager.Stage(",
		"commitCreator.Create(",
		"tagCreator.Create(",
		"dispatchPreparer.Prepare(",
		"commitPusher.Push(",
		"tagPusher.Push(",
		"workflowDispatcher.Dispatch(",
		"handoffConfirmer.Confirm(",
	)
}

func TestConfiguredReleaseLifecycleStagesReturnFreshMetadata(t *testing.T) {
	first := configuredReleaseLifecycleStages()
	first[0].ID = "changed"
	if configuredReleaseLifecycleStages()[0].ID == "changed" {
		t.Fatal("lifecycle stage metadata is mutable across callers")
	}
}

func assertLifecycleSourceOrder(t *testing.T, path string, fragments ...string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	previous := -1
	for _, fragment := range fragments {
		index := strings.Index(string(content), fragment)
		if index <= previous {
			t.Fatalf("%s fragment %q missing or out of order", path, fragment)
		}
		previous = index
	}
}
