package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestV2LocalDeliveryCurrentPlanDryRunAndExecutionContract(t *testing.T) {
	root := newCurrentLocalDeliveryRepository(t, "goreleaser")
	withWorkingDirectoryRoot(t, root)

	planResp, planErr := HandlePlan(plugin.Request{
		Command: "plan",
		Flags: map[string]any{
			"change": "patch",
			"unit":   "api",
		},
	})
	if planErr != nil || planResp.Status != "success" {
		t.Fatalf("plan currently fails for local delivery: response=%#v err=%v", planResp, planErr)
	}
	if got := responseValueForProperty(t, planResp.Data["items"], "Delivery"); got != "local" {
		t.Fatalf("plan delivery = %q, want local", got)
	}

	dryRunResp, dryRunErr := HandleRelease(plugin.Request{
		Command: "patch",
		Flags: map[string]any{
			"dry-run": true,
			"unit":    "api",
		},
	}, Patch)
	if dryRunErr != nil || dryRunResp.Status != "success" {
		t.Fatalf("dry-run currently fails for local delivery: response=%#v err=%v", dryRunResp, dryRunErr)
	}
	if got := responseValueForProperty(t, dryRunResp.Data["items"], "Delivery"); got != "local" {
		t.Fatalf("dry-run delivery = %q, want local", got)
	}

	releaseResp, releaseErr := HandleRelease(plugin.Request{
		Command: "patch",
		Flags:   map[string]any{"unit": "api"},
	}, Patch)
	assertReleaseCommandError(t, releaseResp, releaseErr, "V2_LOCAL_DELIVERY_BLOCKED", "not available yet")
}

func TestV2LocalDeliveryCurrentJReleaserFactsRemainPlanningOnly(t *testing.T) {
	root := newCurrentLocalDeliveryRepository(t, "jreleaser")
	beforeState := mustReadString(t, releaseconfig.V2StatePath(root))
	beforeJReleaser := mustReadString(t, filepath.Join(root, "jreleaser.yml"))
	useCase := newReleasePlanInspectionUseCase(root)

	inspection, failure := useCase.Inspect(t.Context(), ReleasePlanInspectionRequest{
		ReleaseType: Minor,
		UnitID:      "api",
	})

	if failure != nil {
		t.Fatalf("Inspect failure: %#v", failure)
	}
	if inspection.Delivery != "local" || inspection.Executor != "jreleaser" || inspection.Readiness != LocalPlanReady {
		t.Fatalf("unexpected current jreleaser local planning facts: %#v", inspection)
	}
	if got := materializedOutputPaths(inspection.MaterializedOutputs); strings.Join(got, ", ") != "jreleaser.yml" {
		t.Fatalf("expected jreleaser materialization plan, got %#v", got)
	}
	if got := mustReadString(t, releaseconfig.V2StatePath(root)); got != beforeState {
		t.Fatalf("plan inspection rewrote state:\n%s", got)
	}
	if got := mustReadString(t, filepath.Join(root, "jreleaser.yml")); got != beforeJReleaser {
		t.Fatalf("plan inspection rewrote jreleaser.yml:\n%s", got)
	}
}

func newCurrentLocalDeliveryRepository(t *testing.T, executor string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".neko"), 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	switch executor {
	case "jreleaser":
		if err := os.WriteFile(filepath.Join(root, "jreleaser.yml"), []byte("project:\n  name: api\n  version: 1.0.0\n"), 0644); err != nil {
			t.Fatalf("write jreleaser config: %v", err)
		}
	default:
		if err := os.WriteFile(filepath.Join(root, ".goreleaser.yml"), []byte("{}"), 0644); err != nil {
			t.Fatalf("write goreleaser config: %v", err)
		}
	}
	config := `{"schemaVersion":2,"units":[{"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"` + executor + `","delivery":"local"}}]}`
	state := `{"schemaVersion":2,"units":{"api":{"version":"1.0.0"}}}`
	if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(config), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(releaseconfig.V2StatePath(root), []byte(state), 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	return root
}
