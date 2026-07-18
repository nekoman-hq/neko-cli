package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestV2LocalDeliveryCommandsRejectConfigurationBeforePlanning(t *testing.T) {
	root := newCurrentLocalDeliveryRepository(t, "goreleaser")
	withWorkingDirectoryRoot(t, root)

	planResp, planErr := HandlePlan(plugin.Request{
		Command: "plan",
		Flags: map[string]any{
			"change": "patch",
			"unit":   "api",
		},
	})
	assertPlanCommandError(t, planResp, planErr, "CONFIG_INVALID", "local delivery is not supported")

	dryRunResp, dryRunErr := HandleRelease(plugin.Request{
		Command: "patch",
		Flags: map[string]any{
			"dry-run": true,
			"unit":    "api",
		},
	}, Patch)
	assertReleaseCommandError(t, dryRunResp, dryRunErr, "CONFIG_INVALID", "local delivery is not supported")

	releaseResp, releaseErr := HandleRelease(plugin.Request{
		Command: "patch",
		Flags:   map[string]any{"unit": "api"},
	}, Patch)
	assertReleaseCommandError(t, releaseResp, releaseErr, "CONFIG_INVALID", "local delivery is not supported")
}

func TestV2LocalDeliveryJReleaserPlanInspectionRejectsConfigurationWithoutMutation(t *testing.T) {
	root := newCurrentLocalDeliveryRepository(t, "jreleaser")
	beforeState := mustReadString(t, releaseconfig.V2StatePath(root))
	beforeJReleaser := mustReadString(t, filepath.Join(root, "jreleaser.yml"))
	useCase := newReleasePlanInspectionUseCase(root)

	inspection, failure := useCase.Inspect(t.Context(), ReleasePlanInspectionRequest{
		ReleaseType: Minor,
		UnitID:      "api",
	})

	if failure == nil || failure.Code != "CONFIG_INVALID" || !strings.Contains(failure.responseMessage(), "local delivery is not supported") {
		t.Fatalf("expected local delivery inspection rejection, inspection=%#v failure=%#v", inspection, failure)
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

func assertPlanCommandError(t *testing.T, resp *plugin.Response, err error, code, messagePart string) {
	t.Helper()
	if err != nil {
		t.Fatalf("plan command returned a Go error: %v", err)
	}
	if resp == nil || resp.Status != "error" || resp.Error == nil {
		t.Fatalf("expected plugin error response, got %#v", resp)
	}
	if resp.Error.Code != code || !strings.Contains(resp.Error.Message, messagePart) {
		t.Fatalf("unexpected plan error contract: code=%q message=%q", resp.Error.Code, resp.Error.Message)
	}
	if resp.Metadata.Command != "plan" || resp.Metadata.Plugin == "" || resp.Metadata.Version == "" || resp.Metadata.Timestamp.IsZero() {
		t.Fatalf("plan error metadata is incomplete: %#v", resp.Metadata)
	}
}
