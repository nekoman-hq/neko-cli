package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestHandleGitHubWorkflowInitCreatesConfiguredWorkflowIdempotently(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	t.Setenv("GITHUB_TOKEN", "workflow-scaffold-secret-sentinel")

	response, err := HandleGitHubWorkflowInitAt(root, plugin.Request{Command: githubWorkflowInitCommandName})
	if err != nil || response.Status != "success" {
		t.Fatalf("first workflow init failed: response=%#v err=%v", response, err)
	}
	if response.Data["classification"] != "create" || response.Data["action"] != "created" || response.Data["written"] != true {
		t.Fatalf("unexpected create response: %#v", response.Data)
	}
	target := filepath.Join(root.Path(), ".github", "workflows", "release.yml")
	want, err := RenderCanonicalGitHubActionsReleaseWorkflow()
	if err != nil {
		t.Fatalf("render canonical workflow: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read generated workflow: %v", err)
	}
	if string(got) != string(want) {
		t.Fatal("generated workflow bytes differ from canonical renderer")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat generated workflow: %v", err)
	}
	if info.Mode().Perm() != githubWorkflowFileMode {
		t.Fatalf("generated workflow mode = %o, want %o", info.Mode().Perm(), githubWorkflowFileMode)
	}

	preservedTime := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(target, preservedTime, preservedTime); err != nil {
		t.Fatalf("set workflow modification time: %v", err)
	}
	response, err = HandleGitHubWorkflowInitAt(root, plugin.Request{Command: githubWorkflowInitCommandName})
	if err != nil || response.Status != "success" {
		t.Fatalf("second workflow init failed: response=%#v err=%v", response, err)
	}
	if response.Data["classification"] != "unchanged" || response.Data["written"] != false || response.Data["unchanged"] != true {
		t.Fatalf("unexpected unchanged response: %#v", response.Data)
	}
	info, err = os.Stat(target)
	if err != nil {
		t.Fatalf("stat unchanged workflow: %v", err)
	}
	if !info.ModTime().Equal(preservedTime) {
		t.Fatalf("unchanged workflow was rewritten: modtime=%s", info.ModTime())
	}
	assertWorkflowScaffoldSecretAbsent(t, response)
}

func TestHandleGitHubWorkflowInitFailsClosedForDifferentExistingContent(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	target := filepath.Join(root.Path(), ".github", "workflows", "release.yml")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatalf("create workflows directory: %v", err)
	}
	existing := []byte("name: Consumer workflow\n")
	if err := os.WriteFile(target, existing, 0600); err != nil {
		t.Fatalf("write existing workflow: %v", err)
	}

	response, err := HandleGitHubWorkflowInitAt(root, plugin.Request{Command: githubWorkflowInitCommandName})
	if err != nil {
		t.Fatalf("conflict returned Go error: %v", err)
	}
	if response.Status != "error" || response.Error.Code != "WORKFLOW_TARGET_CONFLICT" || response.ExitCode != 1 {
		t.Fatalf("unexpected conflict response: %#v", response)
	}
	if got, readErr := os.ReadFile(target); readErr != nil || string(got) != string(existing) {
		t.Fatalf("conflicting target changed: content=%q err=%v", got, readErr)
	}

	preview, err := HandleGitHubWorkflowInitAt(root, plugin.Request{
		Command: githubWorkflowInitCommandName,
		Flags:   map[string]any{"dry-run": true},
	})
	if err != nil || preview.Status != "success" {
		t.Fatalf("conflict preview failed: response=%#v err=%v", preview, err)
	}
	if preview.Data["classification"] != "conflict" || preview.Data["action"] != "blocked" || preview.Data["dry_run"] != true {
		t.Fatalf("unexpected conflict preview: %#v", preview.Data)
	}
	if preview.Data["generated_content"] == "" || preview.HumanText == nil {
		t.Fatalf("preview omitted generated workflow: %#v", preview)
	}
	if got, readErr := os.ReadFile(target); readErr != nil || string(got) != string(existing) {
		t.Fatalf("preview changed conflicting target: content=%q err=%v", got, readErr)
	}
}

func TestGitHubWorkflowSelectionRequiresExactChoiceForMultiplePaths(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{
		"api": ".github/workflows/release-api.yml",
		"web": ".github/workflows/release-web.yml",
	})

	response, err := HandleGitHubWorkflowInitAt(root, plugin.Request{Command: githubWorkflowInitCommandName})
	if err != nil {
		t.Fatalf("ambiguous selection returned Go error: %v", err)
	}
	if response.Status != "error" || response.Error.Code != "AMBIGUOUS_WORKFLOW_TARGET" {
		t.Fatalf("unexpected ambiguous response: %#v", response)
	}

	preview, err := HandleGitHubWorkflowInitAt(root, plugin.Request{
		Command: githubWorkflowInitCommandName,
		Flags: map[string]any{
			"unit":    "api",
			"dry-run": true,
		},
	})
	if err != nil || preview.Status != "success" {
		t.Fatalf("unit selection failed: response=%#v err=%v", preview, err)
	}
	if preview.Data["target"] != ".github/workflows/release-api.yml" || preview.Data["selected_unit"] != "api" {
		t.Fatalf("unexpected unit selection: %#v", preview.Data)
	}
	if !reflect.DeepEqual(preview.Data["units_using_workflow"], []string{"api"}) {
		t.Fatalf("units using workflow = %#v", preview.Data["units_using_workflow"])
	}

	mismatch, err := HandleGitHubWorkflowInitAt(root, plugin.Request{
		Command: githubWorkflowInitCommandName,
		Flags: map[string]any{
			"unit": "api",
			"path": ".github/workflows/release-web.yml",
		},
	})
	if err != nil || mismatch.Status != "error" || mismatch.Error.Code != "WORKFLOW_TARGET_NOT_CONFIGURED" {
		t.Fatalf("unexpected unit/path mismatch: response=%#v err=%v", mismatch, err)
	}
}

func TestGitHubWorkflowSelectionUsesOneSharedConfiguredPath(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{
		"api": ".github/workflows/release.yml",
		"web": ".github/workflows/release.yml",
	})
	response, err := HandleGitHubWorkflowInitAt(root, plugin.Request{
		Command: githubWorkflowInitCommandName,
		Flags:   map[string]any{"dry-run": true},
	})
	if err != nil || response.Status != "success" {
		t.Fatalf("shared workflow preview failed: response=%#v err=%v", response, err)
	}
	if !reflect.DeepEqual(response.Data["units_using_workflow"], []string{"api", "web"}) {
		t.Fatalf("shared workflow units = %#v", response.Data["units_using_workflow"])
	}
	if _, err := os.Stat(filepath.Join(root.Path(), ".github", "workflows", "release.yml")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created workflow: stat err=%v", err)
	}
}

func TestGitHubWorkflowInitRejectsUnsupportedSourceAndInvalidTarget(t *testing.T) {
	v1RootPath := t.TempDir()
	if err := os.Mkdir(filepath.Join(v1RootPath, ".git"), 0755); err != nil {
		t.Fatalf("create git marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(v1RootPath, releaseconfig.V1FileName), []byte("{}\n"), 0644); err != nil { //nolint:staticcheck
		t.Fatalf("write V1 source: %v", err)
	}
	v1Root, err := workspace.ValidateRepositoryRoot(v1RootPath)
	if err != nil {
		t.Fatalf("validate V1 root: %v", err)
	}
	response, err := HandleGitHubWorkflowInitAt(v1Root, plugin.Request{Command: githubWorkflowInitCommandName})
	if err != nil || response.Status != "error" || response.Error.Code != "UNSUPPORTED_RELEASE_SOURCE" {
		t.Fatalf("unexpected V1 response: response=%#v err=%v", response, err)
	}

	v2Root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	for _, invalidPath := range []string{"/tmp/release.yml", "../release.yml", ".neko/release.state.json"} {
		response, err := HandleGitHubWorkflowInitAt(v2Root, plugin.Request{
			Command: githubWorkflowInitCommandName,
			Flags:   map[string]any{"path": invalidPath},
		})
		if err != nil || response.Status != "error" || response.Error.Code != "WORKFLOW_TARGET_INVALID" {
			t.Fatalf("invalid path %q: response=%#v err=%v", invalidPath, response, err)
		}
	}
}

func newWorkflowScaffoldRepository(t *testing.T, units map[string]string) workspace.RepositoryRoot {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("create git marker: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, releaseconfig.V2Directory), 0755); err != nil {
		t.Fatalf("create V2 directory: %v", err)
	}
	unitIDs := make([]string, 0, len(units))
	for unitID := range units {
		unitIDs = append(unitIDs, unitID)
	}
	slices.Sort(unitIDs)
	config := releaseconfig.V2ReleaseConfig{SchemaVersion: 2}
	state := releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{}}
	for index, unitID := range unitIDs {
		config.Units = append(config.Units, releaseconfig.V2Unit{
			ID:               unitID,
			Paths:            []string{unitID + "/**"},
			WorkingDirectory: ".",
			TagPrefix:        unitID + "/v",
			Executor: releaseconfig.V2Executor{
				Type:     releaseconfig.ExecutorGoReleaser,
				Delivery: releaseconfig.DeliveryGitHubActions,
				Workflow: units[unitID],
			},
		})
		state.Units[unitID] = releaseconfig.V2UnitState{Version: fmt.Sprintf("0.%d.0", index+1)}
	}
	writeWorkflowScaffoldJSON(t, releaseconfig.V2ConfigPath(root), config)
	writeWorkflowScaffoldJSON(t, releaseconfig.V2StatePath(root), state)
	resolved, err := workspace.ValidateRepositoryRoot(root)
	if err != nil {
		t.Fatalf("validate repository root: %v", err)
	}
	return resolved
}

func writeWorkflowScaffoldJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertWorkflowScaffoldSecretAbsent(t *testing.T, response *plugin.Response) {
	t.Helper()
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if bytes.Contains(data, []byte("workflow-scaffold-secret-sentinel")) {
		t.Fatal("workflow scaffolding response leaked ambient token")
	}
}
