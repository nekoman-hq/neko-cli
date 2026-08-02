package release

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/pipelineinspection"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestPipelineStagesReflectConfiguredRepositoryConsumers(t *testing.T) {
	root, err := workspace.ValidateRepositoryRoot("../../../..")
	if err != nil {
		t.Fatalf("ValidateRepositoryRoot: %v", err)
	}
	//nolint:govet // Field order keeps the expected stage contract readable.
	tests := []struct {
		wantConsumer     []string
		unit             string
		wantRegistry     string
		wantMaterialized int
	}{
		{
			unit: "cli",
			wantConsumer: []string{
				"canonical-context-validation", "consumer-tests",
				"release-tool-configuration-validation", "snapshot-build",
				"consumer-worktree-validation", "release-publication",
			},
			wantRegistry: "not_applicable",
		},
		{
			unit: "plugin-release", wantMaterialized: 1, wantRegistry: "configured",
			wantConsumer: []string{
				"canonical-context-validation", "plugin-manifest-validation", "consumer-tests",
				"release-tool-configuration-validation", "snapshot-build",
				"consumer-worktree-validation", "release-artifact-packaging", "release-publication",
				"plugin-index-generation", "plugin-index-publication",
			},
		},
		{
			unit: "plugin-ui", wantMaterialized: 1, wantRegistry: "configured",
			wantConsumer: []string{
				"canonical-context-validation", "plugin-manifest-validation", "consumer-tests",
				"release-tool-configuration-validation", "snapshot-build",
				"consumer-worktree-validation", "release-artifact-packaging", "release-publication",
				"plugin-index-generation", "plugin-index-publication",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.unit, func(t *testing.T) {
			response, err := HandlePipelineAt(root, plugin.Request{Command: "pipeline", Flags: map[string]any{"unit": test.unit}})
			if err != nil {
				t.Fatalf("HandlePipelineAt: %v", err)
			}
			if response.Status != "success" {
				t.Fatalf("response = %#v", response)
			}
			stages, ok := response.Data["stages"].([]pipelineinspection.LifecycleStage)
			if !ok {
				t.Fatalf("stages type = %T", response.Data["stages"])
			}
			rootCount := len(configuredReleaseLifecycleStages())
			gotConsumer := make([]string, 0, len(stages)-rootCount)
			for _, stage := range stages[rootCount:] {
				gotConsumer = append(gotConsumer, stage.ID)
			}
			if !reflect.DeepEqual(gotConsumer, test.wantConsumer) {
				t.Fatalf("consumer stages = %#v, want %#v", gotConsumer, test.wantConsumer)
			}
			workflow := response.Data["workflow"]
			if got := pipelineWorkflowRegistry(t, workflow); got != test.wantRegistry {
				t.Fatalf("plugin registry = %q, want %q", got, test.wantRegistry)
			}
			if got := pipelineMaterializedFileCount(t, response.Data["release"]); got != test.wantMaterialized {
				t.Fatalf("materialized files = %d, want %d", got, test.wantMaterialized)
			}
		})
	}
}

func TestPipelinePresentationUsesActualStageCountsGroupsAndResponsiveWidths(t *testing.T) {
	root, err := workspace.ValidateRepositoryRoot("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		unit       string
		groups     []string
		stageCount int
	}{
		{unit: "cli", stageCount: 22, groups: []string{"Local release preparation", "Git and provider handoff", "Consumer workflow"}},
		{unit: "plugin-release", stageCount: 26, groups: []string{"Local release preparation", "Git and provider handoff", "Consumer workflow", "Plugin registry"}},
	}
	for _, test := range tests {
		t.Run(test.unit, func(t *testing.T) {
			request := plugin.Request{Command: "pipeline", Flags: map[string]any{"unit": test.unit}}
			verification, err := inspectPipelineVerification(root, request)
			if err != nil {
				t.Fatal(err)
			}
			response, err := pipelineinspection.HandlePipelineRuntimeVerificationAt(
				root, request, configuredReleaseLifecycleStages(), pipelineinspection.RuntimeSnapshot{}, verification,
			)
			if err != nil {
				t.Fatal(err)
			}
			stageTable := pipelinePresentationTable(t, response, "Configured Pipeline")
			if stageTable == nil || stageTable.Title != "Configured Pipeline" || len(stageTable.Rows) != test.stageCount {
				t.Fatalf("stage presentation = %#v", stageTable)
			}
			groups := make([]string, 0, len(test.groups))
			for index, row := range stageTable.Rows {
				if row["number"] != index+1 {
					t.Fatalf("row %d number = %#v", index, row["number"])
				}
				group, _ := row["group"].(string)
				if len(groups) == 0 || groups[len(groups)-1] != group {
					groups = append(groups, group)
				}
			}
			if !reflect.DeepEqual(groups, test.groups) {
				t.Fatalf("groups = %#v, want %#v", groups, test.groups)
			}

			normal := renderPipelineResponse(t, response, pipelineOutputWidth{width: 120, available: true})
			for _, want := range append([]string{"Configured Pipeline", "Stage", "Runtime", "Owner"}, test.groups...) {
				if !strings.Contains(normal, want) {
					t.Errorf("normal output omitted %q:\n%s", want, normal)
				}
			}
			for _, forbidden := range []string{"Record ", "Stage:", "not_observed", "consumer_structure", "mutation_required", "Source: doctor", "Runtime and Limitations"} {
				if strings.Contains(normal, forbidden) {
					t.Errorf("normal output exposed %q:\n%s", forbidden, normal)
				}
			}
			if strings.Count(normal, "runtime stages have not been observed") != 1 {
				t.Errorf("untouched runtime note count changed:\n%s", normal)
			}
			assertPipelineOutputWidth(t, normal, 120)

			narrow := renderPipelineResponse(t, response, pipelineOutputWidth{width: 72, available: true})
			for _, essential := range []string{"Check", "Status", "Scope", "Stage", "Runtime", "Owner"} {
				if !strings.Contains(narrow, essential) {
					t.Errorf("narrow output omitted essential field %q:\n%s", essential, narrow)
				}
			}
			assertPipelineOutputWidth(t, narrow, 72)

			firstUnknown := renderPipelineResponse(t, response, pipelineOutputWidth{})
			secondUnknown := renderPipelineResponse(t, response, pipelineOutputWidth{})
			if firstUnknown != secondUnknown || !strings.Contains(firstUnknown, "Stage:") || !strings.Contains(firstUnknown, "Runtime: —") {
				t.Fatalf("unknown-width output is not deterministic vertical output:\n%s", firstUnknown)
			}
			if test.unit == "cli" && strings.Contains(normal, "Plugin registry") {
				t.Fatalf("normal release unit exposed plugin-only group:\n%s", normal)
			}
		})
	}
}

func pipelinePresentationTable(t *testing.T, response *plugin.Response, title string) *presentation.Table {
	t.Helper()
	for table := response.PresentationTable; table != nil; table = table.Following {
		if table.Title == title {
			return table
		}
	}
	t.Fatalf("pipeline presentation table %q is missing", title)
	return nil
}

type pipelineOutputWidth struct {
	width     int
	available bool
}

func (width pipelineOutputWidth) Width(io.Writer) (int, bool) {
	return width.width, width.available
}

func renderPipelineResponse(t *testing.T, response *plugin.Response, width renderer.OutputWidthProvider) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true, WidthProvider: width,
	}, &output); err != nil {
		t.Fatal(err)
	}
	return ansi.Strip(output.String())
}

func assertPipelineOutputWidth(t *testing.T, output string, width int) {
	t.Helper()
	for lineNumber, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if got := ansi.StringWidth(line); got > width {
			t.Fatalf("line %d width = %d, want <= %d: %q", lineNumber+1, got, width, line)
		}
	}
}

func pipelineWorkflowRegistry(t *testing.T, value any) string {
	t.Helper()
	view := pipelineJSONView(t, value)
	registry, _ := view["plugin_registry"].(string)
	return registry
}

func pipelineMaterializedFileCount(t *testing.T, value any) int {
	t.Helper()
	view := pipelineJSONView(t, value)
	files, _ := view["materialized_files"].([]any)
	return len(files)
}

func pipelineJSONView(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err := json.Unmarshal(encoded, &view); err != nil {
		t.Fatal(err)
	}
	return view
}
