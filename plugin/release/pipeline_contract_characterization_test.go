package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestPipelineInspectionCommandConventionsCharacterization(t *testing.T) {
	commands := loadManifestCommands(t)
	for _, name := range []string{"plan", "doctor", "units"} {
		command := commands[name]
		if !reflect.DeepEqual(command.Outputs, []string{"table", "json"}) {
			t.Fatalf("%s outputs = %#v, want table and json", name, command.Outputs)
		}
		if _, present := flagDescriptions(command)["output"]; present {
			t.Fatalf("%s declares a command-specific output flag", name)
		}
	}
	if got := flagDescriptions(commands["plan"])["unit"]; !strings.Contains(got, "Required when a V2 repository defines multiple units") {
		t.Fatalf("plan unit-selection help = %q", got)
	}
}

func TestPipelineInspectionUnitSelectionPolicyCharacterization(t *testing.T) {
	single := &releaseconfig.ReleaseRepository{Units: []releaseconfig.ReleaseUnit{{ID: "service"}}}
	unit, err := releaseconfig.ResolveReleaseUnit(single, "", releaseconfig.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil || unit.ID != "service" {
		t.Fatalf("single-unit omission = %#v, %v", unit, err)
	}

	multiple := &releaseconfig.ReleaseRepository{Units: []releaseconfig.ReleaseUnit{{ID: "service"}, {ID: "plugin"}}}
	unit, err = releaseconfig.ResolveReleaseUnit(multiple, "plugin", releaseconfig.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil || unit.ID != "plugin" {
		t.Fatalf("explicit selection = %#v, %v", unit, err)
	}
	if _, err := releaseconfig.ResolveReleaseUnit(multiple, "", releaseconfig.UnitResolutionOptions{RequireExplicitForMulti: true}); err == nil || !strings.Contains(err.Error(), "release unit is required") {
		t.Fatalf("multi-unit omission error = %v", err)
	}
	if _, err := releaseconfig.ResolveReleaseUnit(multiple, "unknown", releaseconfig.UnitResolutionOptions{RequireExplicitForMulti: true}); err == nil || !strings.Contains(err.Error(), "unknown release unit") {
		t.Fatalf("unknown-unit error = %v", err)
	}
}

func TestPipelineInspectionV1SourceCharacterization(t *testing.T) {
	//nolint:staticcheck // This characterization intentionally locks the legacy V1 boundary.
	repository := releaseconfig.NormalizeV1Repository("/repository", &releaseconfig.V1ReleaseConfig{
		ProjectName: "legacy", Version: "1.2.3",
	})
	if repository.SourceFormat != releaseconfig.SourceFormatV1 || len(repository.Units) != 1 || repository.Units[0].ID != "default" {
		t.Fatalf("normalized V1 source = %#v", repository)
	}
}

func TestPipelineInspectionRootLifecycleOrderCharacterization(t *testing.T) {
	assertOrderedSourceFragments(t, "pkg/release/release_start_v2.go",
		"config.ResolveReleaseUnit(",
		"BuildV2ReleaseExecutionContext(",
		"startV2Release(",
	)
	assertOrderedSourceFragments(t, "pkg/release/github_actions_release_use_case.go",
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

func TestPipelineInspectionConsumerWorkflowOrderCharacterization(t *testing.T) {
	assertOrderedFragments(t, ".github/workflows/release-neko-cli.yml",
		repositoryEffectiveWorkflowSource(t, ".github/workflows/release-neko-cli.yml"),
		"neko release ci-validate-context",
		"go test ./...",
		"args: check --config",
		"args: build --config",
		"args: release --config",
	)
	for _, path := range []string{
		".github/workflows/release-plugin-release.yml",
		".github/workflows/release-plugin-ui.yml",
	} {
		assertOrderedFragments(t, path, repositoryEffectiveWorkflowSource(t, path),
			".version == $version",
			"neko release ci-validate-context",
			"go test ./...",
			"args: check --config",
			"args: build --config",
			"args: release --config",
			"gh release create \"$RELEASE_TAG\"",
			".github/scripts/generate-plugin-index.sh",
			".github/scripts/publish-plugin-index.sh",
		)
	}
}

func TestPipelineInspectionPluginRegistryConditionCharacterization(t *testing.T) {
	repository, err := releaseconfig.LoadReleaseRepository(filepath.Clean("../.."))
	if err != nil {
		t.Fatalf("load repository release source: %v", err)
	}
	for _, unit := range repository.Units {
		effective := repositoryEffectiveWorkflowSource(t, unit.Workflow)
		hasRegistry := strings.Contains(effective, ".github/scripts/generate-plugin-index.sh") &&
			strings.Contains(effective, ".github/scripts/publish-plugin-index.sh")
		if hasRegistry != unit.IsPlugin {
			t.Errorf("unit %s plugin=%t registry=%t", unit.ID, unit.IsPlugin, hasRegistry)
		}
	}
}

func TestPipelineInspectionOutputEnvelopeCharacterization(t *testing.T) {
	response := &plugin.Response{
		Status: "success",
		Data: map[string]any{
			"schema_version": 1,
			"stages":         []string{},
			"limitations":    []string{},
		},
		RendererHint: "table",
		PresentationProperties: &presentation.Properties{
			Title: "Summary", Properties: []presentation.Property{{Label: "Status", Value: "ready"}},
		},
		PresentationTable: &presentation.Table{
			Title: "Stages", Columns: []presentation.Column{{Key: "stage", Label: "Stage", Essential: true}},
			Rows: []map[string]any{},
		},
	}
	var output bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatJSON, &output); err != nil {
		t.Fatalf("render public JSON: %v", err)
	}
	text := output.String()
	for _, required := range []string{`"schema_version": 1`, `"stages": []`, `"limitations": []`} {
		if !strings.Contains(text, required) {
			t.Errorf("response omitted %s: %s", required, text)
		}
	}
	for _, forbidden := range []string{"Summary", "Stages", "human_table", "human_properties", "presentation"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("machine response contains presentation value %q: %s", forbidden, text)
		}
	}
}

func assertOrderedSourceFragments(t *testing.T, path string, fragments ...string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	assertOrderedFragments(t, path, string(content), fragments...)
}

func assertOrderedFragments(t *testing.T, subject, content string, fragments ...string) {
	t.Helper()
	previous := -1
	for _, fragment := range fragments {
		index := strings.Index(content, fragment)
		if index < 0 {
			t.Fatalf("%s is missing %q", subject, fragment)
		}
		if index <= previous {
			t.Fatalf("%s fragment %q is out of order", subject, fragment)
		}
		previous = index
	}
}
