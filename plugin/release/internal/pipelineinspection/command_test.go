package pipelineinspection

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestPipelineCommandProjectsConfiguredIdentityWithoutRuntimeClaims(t *testing.T) {
	root := writePipelineRepository(t, []pipelineFixtureUnit{{ID: "service", Version: "1.2.3"}})
	response, err := HandlePipelineAt(root, plugin.Request{Command: pipelineCommandName}, nil)
	if err != nil {
		t.Fatalf("HandlePipelineAt: %v", err)
	}
	if response.Status != "success" || response.ExitCode != 0 || response.Metadata.Command != pipelineCommandName {
		t.Fatalf("response envelope = %#v", response)
	}
	if response.Data["schema_version"] != 1 || response.Data["status"] != pipelineReady {
		t.Fatalf("schema/status = %#v", response.Data)
	}
	unit := response.Data["unit"].(pipelineUnit)
	if unit.ID != "service" || unit.ConfiguredVersion != "1.2.3" || unit.WorkingDirectory != "." || unit.Kind != "release" {
		t.Fatalf("unit = %#v", unit)
	}
	release := response.Data["release"].(pipelineRelease)
	if release.ConfiguredTag != "service/v1.2.3" || release.MaterializedFiles == nil {
		t.Fatalf("release = %#v", release)
	}
	workflow := response.Data["workflow"].(pipelineWorkflow)
	if !reflect.DeepEqual(workflow.RequiredInputs, []string{"unit", "version", "tag", "release_sha"}) || workflow.ConsumerOperations == nil {
		t.Fatalf("workflow = %#v", workflow)
	}
	progress := response.Data["progress_inspection"].(pipelineProgressInspection)
	if progress.ExecutionProgress != "not_inspected" || progress.JournalsInspected || progress.ResumeEligibilityEvaluated || progress.RemoteStateInspected {
		t.Fatalf("progress = %#v", progress)
	}
	if response.PresentationProperties == nil || response.PresentationProperties.Title != "Release Pipeline Inspection" || response.PresentationTable == nil {
		t.Fatalf("presentation = %#v / %#v", response.PresentationProperties, response.PresentationTable)
	}
}

func TestPipelineCommandUsesCanonicalUnitSelection(t *testing.T) {
	root := writePipelineRepository(t, []pipelineFixtureUnit{{ID: "service", Version: "1.2.3"}, {ID: "worker", Version: "2.0.0"}})

	omitted := runPipelineAt(t, root, plugin.Request{Command: pipelineCommandName})
	assertPipelineFailure(t, omitted, "PIPELINE_UNIT_INVALID")
	unknown := runPipelineAt(t, root, plugin.Request{Command: pipelineCommandName, Flags: map[string]any{"unit": "missing"}})
	assertPipelineFailure(t, unknown, "PIPELINE_UNIT_INVALID")
	selected := runPipelineAt(t, root, plugin.Request{Command: pipelineCommandName, Flags: map[string]any{"unit": "worker"}})
	if selected.Status != "success" || selected.Data["unit"].(pipelineUnit).ID != "worker" {
		t.Fatalf("selected response = %#v", selected)
	}
}

func TestPipelineCommandRejectsMalformedAndUnsupportedRequests(t *testing.T) {
	root := writePipelineRepository(t, []pipelineFixtureUnit{{ID: "service", Version: "1.2.3"}})
	for _, request := range []plugin.Request{
		{Command: pipelineCommandName, Args: []string{"service"}},
		{Command: pipelineCommandName, Flags: map[string]any{"unit": true}},
		{Command: pipelineCommandName, Flags: map[string]any{"unit": " service"}},
		{Command: pipelineCommandName, Flags: map[string]any{"all": true}},
		{Command: pipelineCommandName, Flags: map[string]any{"verify-remote": true}},
		{Command: pipelineCommandName, Flags: map[string]any{"journal": true}},
	} {
		assertPipelineFailure(t, runPipelineAt(t, root, request), "INVALID_PIPELINE_REQUEST")
	}
}

func TestPipelineCommandReturnsTypedV1UnsupportedFailure(t *testing.T) {
	directory := t.TempDir()
	writePipelineFile(t, filepath.Join(directory, releaseconfig.V1FileName), `{"project-name":"legacy","project-owner":"owner","project-type":"backend","release-system":"goreleaser","version":"1.2.3"}`)
	root, err := workspace.ValidateRepositoryRoot(directory)
	if err != nil {
		t.Fatalf("ValidateRepositoryRoot: %v", err)
	}
	response := runPipelineAt(t, root, plugin.Request{Command: pipelineCommandName})
	assertPipelineFailure(t, response, "PIPELINE_SOURCE_UNSUPPORTED")
	if !strings.Contains(response.Error.Message, "Migrate") && !strings.Contains(response.Error.Message, "migrate") {
		t.Fatalf("V1 guidance = %q", response.Error.Message)
	}
}

func TestPipelineCommandKeepsExplicitRootsAndProcessStateIsolated(t *testing.T) {
	first := writePipelineRepository(t, []pipelineFixtureUnit{{ID: "first", Version: "1.0.0"}})
	second := writePipelineRepository(t, []pipelineFixtureUnit{{ID: "second", Version: "2.0.0"}})
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	firstResponse := runPipelineAt(t, first, plugin.Request{Command: pipelineCommandName})
	secondResponse := runPipelineAt(t, second, plugin.Request{Command: pipelineCommandName})
	if firstResponse.Data["unit"].(pipelineUnit).ID != "first" || secondResponse.Data["unit"].(pipelineUnit).ID != "second" {
		t.Fatalf("root isolation failed: first=%#v second=%#v", firstResponse.Data, secondResponse.Data)
	}
	if current, err := os.Getwd(); err != nil || current != workingDirectory {
		t.Fatalf("cwd = %q, %v; want %q", current, err, workingDirectory)
	}
}

func TestPipelineJSONDataContainsNoAbsolutePathOrDeferredState(t *testing.T) {
	root := writePipelineRepository(t, []pipelineFixtureUnit{{ID: "service", Version: "1.2.3"}})
	response := runPipelineAt(t, root, plugin.Request{Command: pipelineCommandName})
	encoded, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{root.Path(), "token", "credential", "next_version", "completed", "journal_state", "human_table", "presentation"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Errorf("machine data contains %q: %s", forbidden, text)
		}
	}
}

type pipelineFixtureUnit struct {
	ID      string
	Version string
}

func writePipelineRepository(t *testing.T, units []pipelineFixtureUnit) workspace.RepositoryRoot {
	t.Helper()
	directory := t.TempDir()
	configUnits := make([]string, 0, len(units))
	stateUnits := make([]string, 0, len(units))
	for _, unit := range units {
		workflow := ".github/workflows/release-" + unit.ID + ".yml"
		writePipelineFile(t, filepath.Join(directory, workflow), "name: release\n")
		configUnits = append(configUnits, `{"id":"`+unit.ID+`","paths":["**"],"workingDirectory":".","tagPrefix":"`+unit.ID+`/v","executor":{"type":"goreleaser","delivery":"github-actions","workflow":"`+workflow+`"}}`)
		stateUnits = append(stateUnits, `"`+unit.ID+`":{"version":"`+unit.Version+`"}`)
	}
	writePipelineFile(t, releaseconfig.V2ConfigPath(directory), `{"schemaVersion":2,"units":[`+strings.Join(configUnits, ",")+`]}`)
	writePipelineFile(t, releaseconfig.V2StatePath(directory), `{"schemaVersion":2,"units":{`+strings.Join(stateUnits, ",")+`}}`)
	root, err := workspace.ValidateRepositoryRoot(directory)
	if err != nil {
		t.Fatalf("ValidateRepositoryRoot: %v", err)
	}
	return root
}

func writePipelineFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runPipelineAt(t *testing.T, root workspace.RepositoryRoot, request plugin.Request) *plugin.Response {
	t.Helper()
	response, err := HandlePipelineAt(root, request, nil)
	if err != nil {
		t.Fatalf("HandlePipelineAt: %v", err)
	}
	return response
}

func assertPipelineFailure(t *testing.T, response *plugin.Response, code string) {
	t.Helper()
	if response == nil || response.Status != "error" || response.ExitCode != 1 || response.Error == nil || response.Error.Code != code {
		t.Fatalf("failure = %#v, want %s", response, code)
	}
}
