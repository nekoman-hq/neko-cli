package pipelineinspection

import (
	"bytes"
	"encoding/json"
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
	wantKeys := []string{"execution", "limitations", "progress_inspection", "release", "repository", "schema_version", "stages", "status", "unit", "workflow"}
	gotKeys := make([]string, 0, len(envelope.Data))
	for key := range envelope.Data {
		gotKeys = append(gotKeys, key)
	}
	slices.Sort(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("schema keys = %#v, want %#v", gotKeys, wantKeys)
	}
	text := first.String()
	for _, required := range []string{`"schema_version": 1`, `"stages": [`, `"limitations": [`, `"materialized_files": []`, `"consumer_operations": []`, `"execution_progress": "not_inspected"`, `"runtime_status": "not_observed"`, `"observations": []`} {
		if !strings.Contains(text, required) {
			t.Errorf("JSON omitted %s:\n%s", required, text)
		}
	}
	for _, forbidden := range []string{
		root.Path(), "human_table", "human_properties", "presentation", "\x1b[",
		"next_version", "next_tag", "proposed", "journal_state", "resume_eligible",
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
	response := mapPipelineResult(normalizePipelineArrays(result))
	encoded, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"materialized_files", "required_inputs", "consumer_operations", "stages", "limitations"} {
		if strings.Contains(string(encoded), `"`+field+`":null`) {
			t.Fatalf("%s encoded null: %s", field, encoded)
		}
	}
}
