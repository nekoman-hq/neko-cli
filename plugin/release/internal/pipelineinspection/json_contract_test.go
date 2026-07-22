package pipelineinspection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestPipelineJSONSchemaVersionOneIsStableAndPresentationFree(t *testing.T) {
	root := writePipelineRepository(t, []pipelineFixtureUnit{{ID: "service", Version: "1.2.3"}})
	response, err := HandlePipelineAt(root, plugin.Request{Command: pipelineCommandName}, []LifecycleStage{{
		ID: "source-unit-resolution", Label: "Resolve source", Owner: StageOwnerNekoCLI,
		Location: StageLocationLocalProcess, Mutation: MutationNone,
		ConfigurationStatus: StageConfigured, Source: "pkg/release/release_start_v2.go",
	}})
	if err != nil {
		t.Fatal(err)
	}
	var first bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatJSON, &first); err != nil {
		t.Fatal(err)
	}
	var second bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatJSON, &second); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Fatalf("JSON output is nondeterministic:\n%s\n%s", first.String(), second.String())
	}
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(first.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"dispatch", "execution", "limitations", "local_git", "manual_intervention", "progress_inspection", "recovery", "release", "repository", "schema_version", "stages", "status", "unit", "verification", "workflow"}
	gotKeys := make([]string, 0, len(envelope.Data))
	for key := range envelope.Data {
		gotKeys = append(gotKeys, key)
	}
	slices.Sort(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("schema keys = %#v, want %#v", gotKeys, wantKeys)
	}
	text := first.String()
	for _, required := range []string{`"schema_version": 1`, `"stages": [`, `"verification": {`, `"facts": []`, `"limitations": [`, `"materialized_files": []`, `"consumer_operations": []`, `"execution_progress": "not_inspected"`, `"runtime_status": "not_observed"`, `"observations": []`} {
		if !strings.Contains(text, required) {
			t.Errorf("JSON omitted %s:\n%s", required, text)
		}
	}
	for _, forbidden := range []string{
		root.Path(), "human_table", "human_properties", "presentation", "\x1b[",
		"next_version", "next_tag", "proposed", "journal_state",
		"terminal_width", "credential", "secret_value",
	} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Errorf("JSON contains forbidden %q:\n%s", forbidden, text)
		}
	}
}

func TestPipelineJSONArraysNeverEncodeAsNull(t *testing.T) {
	result := pipelinePresentationFixture()
	result.Release.MaterializedFiles = nil
	result.Workflow.RequiredInputs = nil
	result.Workflow.ConsumerOperations = nil
	result.Stages = nil
	result.Limitations = nil
	result.Execution.Observations = nil
	result.Dispatch.Observations = nil
	result.Recovery.Reasons = nil
	result.ManualIntervention.Reasons = nil
	result.Verification.Facts = []VerificationFact{{
		Category: "remote_workflow_identity", Class: VerificationRemote,
		Status: VerificationNotChecked, Subject: "workflow", Source: "doctor", Scope: "workflow",
		References: nil,
	}}
	response := mapPipelineResult(normalizePipelineArrays(result))
	encoded, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"materialized_files", "required_inputs", "consumer_operations", "stages", "limitations", "observations", "reasons", "facts", "references"} {
		if strings.Contains(string(encoded), `"`+field+`":null`) {
			t.Fatalf("%s encoded null: %s", field, encoded)
		}
	}
}

func TestPipelineRuntimeJSONSectionsHaveExactAdditiveKeys(t *testing.T) {
	response := mapPipelineResult(pipelinePresentationFixture())
	encoded, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &data); err != nil {
		t.Fatal(err)
	}
	assertPipelineJSONKeys(t, data["execution"], []string{
		"identity", "journal_count", "observations", "pending_action", "present", "state", "terminal", "unresolved_count", "validity",
	})
	assertPipelineJSONKeys(t, data["dispatch"], []string{
		"correlation", "identity", "journal_count", "observations", "present", "state", "unlinked_count",
	})
	assertPipelineJSONKeys(t, data["recovery"], []string{
		"classification", "evaluated", "manual_intervention_required", "reasons", "resume_eligible", "retry_safety", "safe_to_continue",
	})
	assertPipelineJSONKeys(t, data["manual_intervention"], []string{"reasons", "required"})
	assertPipelineJSONKeys(t, data["verification"], []string{"facts", "summary"})
	var verification map[string]json.RawMessage
	if err := json.Unmarshal(data["verification"], &verification); err != nil {
		t.Fatal(err)
	}
	assertPipelineJSONKeys(t, verification["summary"], []string{
		"failed", "local_status", "not_checked", "partial", "remote_attempted", "remote_requested",
		"remote_status", "status", "unresolved", "verified",
	})
	assertPipelineJSONKeys(t, data["local_git"], []string{
		"branch", "commit_content_verified", "commit_exists", "consistent", "expected_commit", "expected_tag", "head",
		"head_contains_expected_commit", "index_contains_recovery_evidence", "index_state", "remote_freshness", "scope",
		"tag_exists", "tag_matches_expected_commit", "worktree_contains_recovery_evidence", "worktree_state",
	})
}

func TestPipelineVerificationJSONHasExactAppendOnlyKeysAndVocabulary(t *testing.T) {
	result := pipelinePresentationFixture()
	result.Verification = projectPipelineVerification(VerificationSnapshot{
		RemoteStatus: "partial", RemoteRequested: true, RemoteAttempted: true,
		Facts: []VerificationFact{{
			Category: "consumer_structure", Class: VerificationLocal, Status: VerificationVerified,
			Subject: "workflow", Evidence: "verified", Source: "doctor", Scope: "unit",
			References: []string{"workflow"}, Unit: "service", Workflow: "workflow",
		}},
	})
	encoded, err := json.Marshal(mapPipelineResult(result).Data)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &data); err != nil {
		t.Fatal(err)
	}
	var verification struct {
		Summary json.RawMessage   `json:"summary"`
		Facts   []json.RawMessage `json:"facts"`
	}
	if err := json.Unmarshal(data["verification"], &verification); err != nil {
		t.Fatal(err)
	}
	assertPipelineJSONKeys(t, verification.Summary, []string{
		"failed", "local_status", "not_checked", "partial", "remote_attempted", "remote_requested",
		"remote_status", "status", "unresolved", "verified",
	})
	if len(verification.Facts) != 1 {
		t.Fatalf("verification facts = %d", len(verification.Facts))
	}
	assertPipelineJSONKeys(t, verification.Facts[0], []string{
		"category", "class", "evidence", "id", "references", "scope", "source", "status", "subject", "unit", "workflow",
	})

	statuses := []VerificationStatus{
		VerificationVerified, VerificationFailed, VerificationUnavailable, VerificationUnauthorized,
		VerificationRateLimited, VerificationNotChecked, VerificationUnresolved,
	}
	if got, want := fmt.Sprint(statuses), "[verified failed unavailable unauthorized rate_limited not_checked unresolved]"; got != want {
		t.Fatalf("verification statuses = %s, want %s", got, want)
	}
	classes := []VerificationClass{
		VerificationLocal, VerificationRemote, VerificationRuntimeRequired, VerificationMutationRequired,
	}
	if got, want := fmt.Sprint(classes), "[local remote runtime_required mutation_required]"; got != want {
		t.Fatalf("verification classes = %s, want %s", got, want)
	}
}

func assertPipelineJSONKeys(t *testing.T, raw json.RawMessage, want []string) {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(object))
	for key := range object {
		got = append(got, key)
	}
	slices.Sort(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON keys = %#v, want %#v", got, want)
	}
}
