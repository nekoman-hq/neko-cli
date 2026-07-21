package release

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
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
