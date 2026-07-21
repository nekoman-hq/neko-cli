package pipelineinspection

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
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
	unit, ok := response.Data["unit"].(pipelineUnit)
	if !ok {
		t.Fatalf("unit type = %T", response.Data["unit"])
	}
	if unit.ID != "service" || unit.ConfiguredVersion != "1.2.3" || unit.WorkingDirectory != "." || unit.Kind != "release" {
		t.Fatalf("unit = %#v", unit)
	}
	release, ok := response.Data["release"].(pipelineRelease)
	if !ok {
		t.Fatalf("release type = %T", response.Data["release"])
	}
	if release.ConfiguredTag != "service/v1.2.3" || release.MaterializedFiles == nil {
		t.Fatalf("release = %#v", release)
	}
	workflow, ok := response.Data["workflow"].(pipelineWorkflow)
	if !ok {
		t.Fatalf("workflow type = %T", response.Data["workflow"])
	}
	if !reflect.DeepEqual(workflow.RequiredInputs, []string{"unit", "version", "tag", "release_sha"}) || workflow.ConsumerOperations == nil {
		t.Fatalf("workflow = %#v", workflow)
	}
	progress, ok := response.Data["progress_inspection"].(pipelineProgressInspection)
	if !ok {
		t.Fatalf("progress type = %T", response.Data["progress_inspection"])
	}
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
	if selected.Status != "success" || pipelineResponseUnit(t, selected).ID != "worker" {
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
	//nolint:staticcheck // The command must preserve a typed V1 unsupported contract.
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
	if pipelineResponseUnit(t, firstResponse).ID != "first" || pipelineResponseUnit(t, secondResponse).ID != "second" {
		t.Fatalf("root isolation failed: first=%#v second=%#v", firstResponse.Data, secondResponse.Data)
	}
	if current, err := os.Getwd(); err != nil || current != workingDirectory {
		t.Fatalf("cwd = %q, %v; want %q", current, err, workingDirectory)
	}
}

func TestPipelineCommandReturnsTypedExecutorDeliveryAndWorkflowFailures(t *testing.T) {
	t.Run("executor", func(t *testing.T) {
		root := writePipelineRepository(t, []pipelineFixtureUnit{{ID: "service", Version: "1.2.3"}})
		configPath := releaseconfig.V2ConfigPath(root.Path())
		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		writePipelineFile(t, configPath, strings.Replace(string(content), `"type":"goreleaser"`, `"type":"unknown"`, 1))
		assertPipelineFailure(t, runPipelineAt(t, root, plugin.Request{Command: pipelineCommandName}), "PIPELINE_EXECUTOR_UNSUPPORTED")
	})
	t.Run("delivery", func(t *testing.T) {
		root := writePipelineRepository(t, []pipelineFixtureUnit{{ID: "service", Version: "1.2.3"}})
		configPath := releaseconfig.V2ConfigPath(root.Path())
		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		writePipelineFile(t, configPath, strings.Replace(string(content), `"delivery":"github-actions"`, `"delivery":"local"`, 1))
		assertPipelineFailure(t, runPipelineAt(t, root, plugin.Request{Command: pipelineCommandName}), "PIPELINE_DELIVERY_UNSUPPORTED")
	})
	t.Run("workflow", func(t *testing.T) {
		root := writePipelineRepository(t, []pipelineFixtureUnit{{ID: "service", Version: "1.2.3"}})
		writePipelineFile(t, filepath.Join(root.Path(), ".github/workflows/release-service.yml"), "jobs: [")
		assertPipelineFailure(t, runPipelineAt(t, root, plugin.Request{Command: pipelineCommandName}), "PIPELINE_WORKFLOW_INVALID")
	})
}

func TestConfiguredMaterializedFilesFollowExecutorAndUnitKind(t *testing.T) {
	pluginUnit := releaseconfig.ReleaseUnit{
		IsPlugin: true, PluginManifestPath: "plugin/example/manifest.json", WorkingDirectory: "tools/release",
	}
	tests := []struct {
		identity releasetool.Identity
		want     []pipelineMaterializedFile
	}{
		{identity: releasetool.GoReleaser, want: []pipelineMaterializedFile{{Path: "plugin/example/manifest.json", Reason: "synchronize configured plugin manifest version during release execution"}}},
		{identity: releasetool.JReleaser, want: []pipelineMaterializedFile{{Path: "tools/release/jreleaser.yml", Reason: "synchronize JReleaser project version during release execution"}}},
		{identity: releasetool.ReleaseIt, want: []pipelineMaterializedFile{}},
	}
	for _, test := range tests {
		if got := configuredMaterializedFiles(pluginUnit, test.identity); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s materialized files = %#v, want %#v", test.identity, got, test.want)
		}
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

func pipelineResponseUnit(t *testing.T, response *plugin.Response) pipelineUnit {
	t.Helper()
	unit, ok := response.Data["unit"].(pipelineUnit)
	if !ok {
		t.Fatalf("unit type = %T", response.Data["unit"])
	}
	return unit
}
