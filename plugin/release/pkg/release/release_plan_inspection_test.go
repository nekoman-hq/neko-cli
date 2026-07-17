package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestInspectReleasePlanV2ReturnsCanonicalPlanningFacts(t *testing.T) {
	root := newPluginReleaseMaterializationRepository(t)
	gitCmd(t, root, "init")
	gitCmd(t, root, "config", "user.email", "test@example.com")
	gitCmd(t, root, "config", "user.name", "Test User")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "initial")
	t.Setenv("GITHUB_TOKEN", releaseSecretSentinel)

	stateBefore := mustReadString(t, releaseconfig.V2StatePath(root))
	manifestBefore := mustReadString(t, filepath.Join(root, pluginReleaseManifestPath))
	headBefore := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	tagsBefore := strings.TrimSpace(gitOutput(t, root, "tag", "--list"))
	useCase := newReleasePlanInspectionUseCase(root)

	inspection, failure := useCase.Inspect(t.Context(), ReleasePlanInspectionRequest{
		ReleaseType: Patch,
		UnitID:      "plugin-release",
	})

	if failure != nil {
		t.Fatalf("Inspect failure: %#v", failure)
	}
	if inspection.Source != releaseconfig.SourceFormatV2 || inspection.Unit.ID != "plugin-release" {
		t.Fatalf("unexpected source/unit: %#v", inspection)
	}
	if inspection.CurrentVersion != "3.0.0" || inspection.NextVersion != "3.0.1" || inspection.Tag != "plugin-release/v3.0.1" {
		t.Fatalf("unexpected version plan: %#v", inspection)
	}
	if inspection.Executor != "goreleaser" || inspection.Delivery != "github-actions" ||
		inspection.Workflow != ".github/workflows/release-plugin-release.yml" {
		t.Fatalf("unexpected executor/delivery facts: %#v", inspection)
	}
	if got := materializedOutputPaths(inspection.MaterializedOutputs); strings.Join(got, ", ") != pluginReleaseManifestPath {
		t.Fatalf("materialized outputs = %#v", got)
	}
	if got := inspectedKnownFilePaths(inspection.KnownReleaseFiles); strings.Join(got, ", ") != ".neko/release.state.json, plugin/release/manifest.json" {
		t.Fatalf("known files = %#v", got)
	}
	if inspection.Readiness != LocalPlanReady || len(inspection.Blockers) != 0 || len(inspection.Limitations) == 0 {
		t.Fatalf("unexpected local readiness/blockers/limitations: %#v", inspection)
	}
	if got := mustReadString(t, releaseconfig.V2StatePath(root)); got != stateBefore {
		t.Fatalf("inspection rewrote state:\n%s", got)
	}
	if got := mustReadString(t, filepath.Join(root, pluginReleaseManifestPath)); got != manifestBefore {
		t.Fatalf("inspection rewrote manifest:\n%s", got)
	}
	if headAfter := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD")); headAfter != headBefore {
		t.Fatalf("inspection created a commit: before=%s after=%s", headBefore, headAfter)
	}
	if tagsAfter := strings.TrimSpace(gitOutput(t, root, "tag", "--list")); tagsAfter != tagsBefore {
		t.Fatalf("inspection changed tags: before=%q after=%q", tagsBefore, tagsAfter)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".git", "neko")); !os.IsNotExist(statErr) {
		t.Fatalf("inspection created release journals: %v", statErr)
	}
	assertSecretAbsentFromInspection(t, inspection)
}

func TestInspectReleasePlanV2ReportsLocalMaterializationBlocker(t *testing.T) {
	root := newReleaseItPlanInspectionRepository(t)
	useCase := newReleasePlanInspectionUseCase(root)

	inspection, failure := useCase.Inspect(t.Context(), ReleasePlanInspectionRequest{ReleaseType: Patch, UnitID: "web"})

	if failure != nil {
		t.Fatalf("Inspect failure: %#v", failure)
	}
	if inspection.Readiness != LocalPlanBlocked || len(inspection.Blockers) != 1 {
		t.Fatalf("expected blocked local plan, got %#v", inspection)
	}
	if blocker := inspection.Blockers[0]; blocker.Category != "materialization-blocked" ||
		!strings.Contains(blocker.Message, "V2 local release-it is blocked") {
		t.Fatalf("unexpected blocker: %#v", blocker)
	}
}

func TestInspectReleasePlanV1ReturnsExplicitLegacyFactsAndLimitations(t *testing.T) {
	root := t.TempDir()
	configContent := `{
  "project-name": "neko-cli",
  "project-owner": "nekoman-hq",
  "project-type": "backend",
  "release-system": "goreleaser",
  "version": "1.2.3"
}`
	if err := os.WriteFile(filepath.Join(root, releaseconfig.V1FileName), []byte(configContent), 0644); err != nil {
		t.Fatalf("write v1 config: %v", err)
	}
	useCase := newReleasePlanInspectionUseCase(root)

	inspection, failure := useCase.Inspect(t.Context(), ReleasePlanInspectionRequest{ReleaseType: Minor})

	if failure != nil {
		t.Fatalf("Inspect failure: %#v", failure)
	}
	if inspection.Source != releaseconfig.SourceFormatV1 || inspection.Unit.ID != "default" {
		t.Fatalf("unexpected v1 source/unit: %#v", inspection)
	}
	if inspection.CurrentVersion != "1.2.3" || inspection.NextVersion != "1.3.0" || inspection.Tag != "v1.3.0" {
		t.Fatalf("unexpected v1 version plan: %#v", inspection)
	}
	if got := materializedOutputPaths(inspection.MaterializedOutputs); strings.Join(got, ", ") != releaseconfig.V1FileName {
		t.Fatalf("v1 materialized outputs = %#v", got)
	}
	if len(inspection.KnownReleaseFiles) != 0 || !inspectionHasLimitation(inspection, "v1-known-release-files") ||
		!inspectionHasLimitation(inspection, "v1-latest-tag-evidence") {
		t.Fatalf("v1 limitations not explicit: %#v", inspection)
	}
	if got := mustReadString(t, filepath.Join(root, releaseconfig.V1FileName)); got != configContent {
		t.Fatalf("inspection rewrote v1 config:\n%s", got)
	}
}

func TestHandlePlanAtUsesExplicitRootWithoutProcessWorkingDirectory(t *testing.T) {
	rootPath := newGitHubActionsDispatchRepository(t)
	otherRoot := t.TempDir()
	root, err := workspace.ValidateRepositoryRoot(rootPath)
	if err != nil {
		t.Fatalf("ValidateRepositoryRoot: %v", err)
	}
	withWorkingDirectoryRoot(t, otherRoot)
	t.Setenv("GITHUB_TOKEN", releaseSecretSentinel)

	resp, err := HandlePlanAt(root, plugin.Request{
		Command: "plan",
		Flags: map[string]any{
			"change": "major",
			"unit":   "api",
		},
	})

	if err != nil || resp.Status != "success" {
		t.Fatalf("HandlePlanAt failed: response=%#v err=%v", resp, err)
	}
	if got := responseValueForProperty(t, resp.Data["items"], "Next Version"); got != "1.0.0" {
		t.Fatalf("next version = %q, want 1.0.0", got)
	}
	if got := responseValueForProperty(t, resp.Data["items"], "Tag"); got != "api/v1.0.0" {
		t.Fatalf("tag = %q, want api/v1.0.0", got)
	}
	if _, err := os.Stat(releaseconfig.V2ConfigPath(otherRoot)); !os.IsNotExist(err) {
		t.Fatalf("HandlePlanAt touched process cwd; stat err=%v", err)
	}
	assertSecretAbsentFromResponse(t, resp)
}

func TestHandlePlanMapsInvalidChangeWithoutGoError(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	withWorkingDirectoryRoot(t, root)

	resp, err := HandlePlan(plugin.Request{Command: "plan", Flags: map[string]any{"change": "build", "unit": "api"}})

	if err != nil {
		t.Fatalf("HandlePlan returned Go error: %v", err)
	}
	if resp.Status != "error" || resp.Error.Code != "INVALID_RELEASE_CHANGE" || resp.Metadata.Command != "plan" {
		t.Fatalf("unexpected invalid-change response: %#v", resp)
	}
}

func materializedOutputPaths(outputs []PlannedMaterializedOutput) []string {
	paths := make([]string, 0, len(outputs))
	for _, output := range outputs {
		paths = append(paths, output.Path)
	}
	return paths
}

func inspectedKnownFilePaths(files []InspectedKnownReleaseFile) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	return paths
}

func inspectionHasLimitation(inspection *ReleasePlanInspection, category string) bool {
	for _, limitation := range inspection.Limitations {
		if limitation.Category == category {
			return true
		}
	}
	return false
}

func assertSecretAbsentFromInspection(t *testing.T, inspection *ReleasePlanInspection) {
	t.Helper()
	data, err := json.Marshal(inspection)
	if err != nil {
		t.Fatalf("marshal inspection: %v", err)
	}
	if strings.Contains(string(data), releaseSecretSentinel) {
		t.Fatal("secret sentinel appeared in release plan inspection")
	}
}

func newReleaseItPlanInspectionRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".neko"), 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".release-it.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write release-it config: %v", err)
	}
	config := `{"schemaVersion":2,"units":[{"id":"web","paths":["web/**"],"workingDirectory":".","tagPrefix":"web/v","executor":{"type":"release-it","delivery":"local"}}]}`
	state := `{"schemaVersion":2,"units":{"web":{"version":"0.4.0"}}}`
	if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(config), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(releaseconfig.V2StatePath(root), []byte(state), 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	return root
}
