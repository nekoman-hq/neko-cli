package release

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestGitHubWorkflowScaffoldRequestErrorsUseStableResponses(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	tests := map[string]map[string]any{
		"unit type":       {"unit": true},
		"path whitespace": {"path": " .github/workflows/release.yml"},
		"dry run type":    {"dry-run": "true"},
	}
	for name, flags := range tests {
		t.Run(name, func(t *testing.T) {
			response, err := HandleGitHubWorkflowInitAt(root, plugin.Request{
				Command: githubWorkflowInitCommandName,
				Flags:   flags,
			})
			if err != nil {
				t.Fatalf("invalid request returned Go error: %v", err)
			}
			if response.Status != "error" || response.Error.Code != "INVALID_WORKFLOW_SCAFFOLD_REQUEST" || response.ExitCode != 1 {
				t.Fatalf("unexpected invalid request response: %#v", response)
			}
		})
	}
}

func TestGitHubWorkflowScaffoldPreviewHasReadableOutputAndStableJSON(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	response, err := HandleGitHubWorkflowInitAt(root, plugin.Request{
		Command: githubWorkflowInitCommandName,
		Flags:   map[string]any{"dry-run": true},
	})
	if err != nil || response.Status != "success" {
		t.Fatalf("preview failed: response=%#v err=%v", response, err)
	}
	for key, want := range map[string]any{
		"target":           ".github/workflows/release.yml",
		"classification":   "create",
		"action":           "would-create",
		"written":          false,
		"unchanged":        false,
		"dry_run":          true,
		"contract_version": GitHubActionsReleaseWorkflowContractVersion,
		"selected_unit":    "",
	} {
		if got := response.Data[key]; got != want {
			t.Errorf("response data %s = %#v, want %#v", key, got, want)
		}
	}
	if !reflect.DeepEqual(response.Data["units_using_workflow"], []string{"api"}) {
		t.Fatalf("units_using_workflow = %#v", response.Data["units_using_workflow"])
	}
	generated, ok := response.Data["generated_content"].(string)
	if !ok || generated == "" {
		t.Fatal("preview generated_content is missing")
	}

	var human bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatTable, &human); err != nil {
		t.Fatalf("render human preview: %v", err)
	}
	for _, fragment := range []string{"GitHub Actions workflow scaffolding preview", "Status: create", generated} {
		if !strings.Contains(human.String(), fragment) {
			t.Errorf("human preview is missing %q", fragment)
		}
	}

	var machine bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatJSON, &machine); err != nil {
		t.Fatalf("render JSON preview: %v", err)
	}
	var public map[string]any
	if err := json.Unmarshal(machine.Bytes(), &public); err != nil {
		t.Fatalf("decode JSON preview: %v", err)
	}
	if _, present := public["human_text"]; present {
		t.Fatal("human presentation metadata leaked into public JSON")
	}
	data, ok := public["data"].(map[string]any)
	if !ok || data["written"] != false || data["generated_content"] != generated {
		t.Fatalf("public JSON data drifted: %#v", public)
	}
}

func TestGitHubWorkflowScaffoldPreviewClassifiesIdenticalTargetWithoutWriting(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	target := filepath.Join(root.Path(), ".github", "workflows", "release.yml")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatalf("create workflow directory: %v", err)
	}
	content, err := RenderCanonicalGitHubActionsReleaseWorkflow()
	if err != nil {
		t.Fatalf("render workflow: %v", err)
	}
	if writeErr := os.WriteFile(target, content, 0644); writeErr != nil {
		t.Fatalf("write identical target: %v", writeErr)
	}
	before, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	response, err := HandleGitHubWorkflowInitAt(root, plugin.Request{
		Command: githubWorkflowInitCommandName,
		Flags:   map[string]any{"dry-run": true},
	})
	if err != nil || response.Status != "success" || response.Data["classification"] != "unchanged" {
		t.Fatalf("unexpected identical preview: response=%#v err=%v", response, err)
	}
	after, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target after preview: %v", err)
	}
	if !before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() {
		t.Fatal("preview changed an identical target")
	}
}

func TestGitHubWorkflowScaffoldRejectsSymlinksAndUnsafeNames(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root.Path(), ".github")); err != nil {
		t.Fatalf("create parent symlink: %v", err)
	}
	response, err := HandleGitHubWorkflowInitAt(root, plugin.Request{Command: githubWorkflowInitCommandName})
	if err != nil || response.Status != "error" || response.Error.Code != "WORKFLOW_TARGET_SYMLINK_ESCAPE" {
		t.Fatalf("unexpected parent symlink response: response=%#v err=%v", response, err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "workflows", "release.yml")); !os.IsNotExist(statErr) {
		t.Fatalf("symlink escape created an outside target: %v", statErr)
	}

	root = newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	directory := filepath.Join(root.Path(), ".github", "workflows")
	if directoryErr := os.MkdirAll(directory, 0755); directoryErr != nil {
		t.Fatalf("create workflow directory: %v", directoryErr)
	}
	outsideTarget := filepath.Join(t.TempDir(), "outside.yml")
	if outsideWriteErr := os.WriteFile(outsideTarget, []byte("name: outside\n"), 0644); outsideWriteErr != nil {
		t.Fatalf("write outside target: %v", outsideWriteErr)
	}
	if symlinkErr := os.Symlink(outsideTarget, filepath.Join(directory, "release.yml")); symlinkErr != nil {
		t.Fatalf("create target symlink: %v", symlinkErr)
	}
	response, err = HandleGitHubWorkflowInitAt(root, plugin.Request{Command: githubWorkflowInitCommandName})
	if err != nil || response.Status != "error" || response.Error.Code != "WORKFLOW_TARGET_SYMLINK_ESCAPE" {
		t.Fatalf("unexpected target symlink response: response=%#v err=%v", response, err)
	}

	root = newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	for _, unsafe := range []string{
		".github/workflows/release name.yml",
		".github/workflows/rélease.yml",
		".github/workflows/nested/release.yml",
		".github/workflows/release.YML",
	} {
		response, err = HandleGitHubWorkflowInitAt(root, plugin.Request{
			Command: githubWorkflowInitCommandName,
			Flags:   map[string]any{"path": unsafe},
		})
		if err != nil || response.Status != "error" || response.Error.Code != "WORKFLOW_TARGET_INVALID" {
			t.Fatalf("unsafe path %q: response=%#v err=%v", unsafe, response, err)
		}
	}
}

func TestGitHubWorkflowScaffoldUsesExplicitRootsAcrossNestedInvocations(t *testing.T) {
	first := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	second := newWorkflowScaffoldRepository(t, map[string]string{"web": ".github/workflows/release-web.yml"})
	nested := filepath.Join(first.Path(), "services", "api")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	cwdBefore, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	preview, err := HandleGitHubWorkflowInit(plugin.Request{
		Command: githubWorkflowInitCommandName,
		Flags:   map[string]any{"dry-run": true},
		Context: plugin.Context{WorkingDir: nested},
	})
	if err != nil || preview.Status != "success" || preview.Data["target"] != ".github/workflows/release.yml" {
		t.Fatalf("nested preview failed: response=%#v err=%v", preview, err)
	}
	created, err := HandleGitHubWorkflowInitAt(second, plugin.Request{Command: githubWorkflowInitCommandName})
	if err != nil || created.Status != "success" || created.Data["target"] != ".github/workflows/release-web.yml" {
		t.Fatalf("second-root creation failed: response=%#v err=%v", created, err)
	}
	if _, firstStatErr := os.Stat(filepath.Join(first.Path(), ".github", "workflows", "release.yml")); !os.IsNotExist(firstStatErr) {
		t.Fatalf("nested preview wrote into first repository: %v", firstStatErr)
	}
	if _, secondStatErr := os.Stat(filepath.Join(second.Path(), ".github", "workflows", "release-web.yml")); secondStatErr != nil {
		t.Fatalf("second repository target missing: %v", secondStatErr)
	}
	cwdAfter, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd after commands: %v", err)
	}
	if cwdAfter != cwdBefore {
		t.Fatalf("workflow scaffolding changed process cwd: before=%q after=%q", cwdBefore, cwdAfter)
	}
}

func TestAtomicGitHubWorkflowCreatorNeverClobbersAppearingTarget(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	target, _, exists, failure := inspectGitHubWorkflowOutputTarget(root.Path(), ".github/workflows/release.yml")
	if failure != nil || exists {
		t.Fatalf("inspect missing target: target=%#v exists=%v failure=%#v", target, exists, failure)
	}
	if err := os.MkdirAll(filepath.Dir(target.AbsolutePath), 0755); err != nil {
		t.Fatalf("create target directory: %v", err)
	}
	appeared := []byte("name: consumer-owned\n")
	if err := os.WriteFile(target.AbsolutePath, appeared, 0600); err != nil {
		t.Fatalf("create appearing target: %v", err)
	}
	generated, err := RenderCanonicalGitHubActionsReleaseWorkflow()
	if err != nil {
		t.Fatalf("render generated workflow: %v", err)
	}
	failure = (atomicGitHubWorkflowOutputCreator{}).Create(target, generated)
	if failure == nil || failure.Code != "WORKFLOW_TARGET_CONFLICT" {
		t.Fatalf("unexpected atomic conflict: %#v", failure)
	}
	got, err := os.ReadFile(target.AbsolutePath)
	if err != nil || !bytes.Equal(got, appeared) {
		t.Fatalf("appearing target was clobbered: content=%q err=%v", got, err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(target.AbsolutePath), ".neko-release-workflow-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("atomic creator left partial candidates: matches=%v err=%v", matches, err)
	}
}

func TestGitHubWorkflowScaffoldPlanningIsReadOnlyAndWritingIsNarrow(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	request := githubWorkflowScaffoldRequest{RepositoryRoot: root.Path()}
	planner := newGitHubWorkflowGenerationPlanner()
	plan, failure := planner.Plan(context.Background(), request)
	if failure != nil || plan.Classification != githubWorkflowTargetCreate {
		t.Fatalf("plan missing target: plan=%#v failure=%#v", plan, failure)
	}
	if _, err := os.Stat(plan.Target.AbsolutePath); !os.IsNotExist(err) {
		t.Fatalf("read-only planning created target: %v", err)
	}

	creator := &recordingWorkflowOutputCreator{
		failure: failureFromMessage("WORKFLOW_WRITE_FAILED", "injected generated workflow write failure"),
	}
	result, failure := (githubWorkflowScaffoldCreateUseCase{planner: planner, writer: creator}).Create(context.Background(), request)
	if result != nil || failure == nil || failure.Code != "WORKFLOW_WRITE_FAILED" || creator.calls != 1 {
		t.Fatalf("narrow write failure: result=%#v failure=%#v calls=%d", result, failure, creator.calls)
	}
	if _, err := os.Stat(plan.Target.AbsolutePath); !os.IsNotExist(err) {
		t.Fatalf("failed narrow writer left a target: %v", err)
	}
}

func TestGitHubWorkflowScaffoldMissingConfigurationAndUnitAreTyped(t *testing.T) {
	missingRootPath := t.TempDir()
	if err := os.Mkdir(filepath.Join(missingRootPath, ".git"), 0755); err != nil {
		t.Fatalf("create git marker: %v", err)
	}
	missingRoot, err := workspace.ValidateRepositoryRoot(missingRootPath)
	if err != nil {
		t.Fatalf("validate missing-config root: %v", err)
	}
	response, err := HandleGitHubWorkflowInitAt(missingRoot, plugin.Request{Command: githubWorkflowInitCommandName})
	if err != nil || response.Status != "error" || response.Error.Code != "V2_WORKFLOW_SOURCE_MISSING" {
		t.Fatalf("unexpected missing source response: response=%#v err=%v", response, err)
	}

	unconfigured := newWorkflowScaffoldRepository(t, map[string]string{"api": ""})
	response, err = HandleGitHubWorkflowInitAt(unconfigured, plugin.Request{Command: githubWorkflowInitCommandName})
	if err != nil || response.Status != "error" || response.Error.Code != "WORKFLOW_NOT_CONFIGURED" {
		t.Fatalf("unexpected unconfigured workflow response: response=%#v err=%v", response, err)
	}

	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	response, err = HandleGitHubWorkflowInitAt(root, plugin.Request{
		Command: githubWorkflowInitCommandName,
		Flags:   map[string]any{"unit": "missing"},
	})
	if err != nil || response.Status != "error" || response.Error.Code != "RELEASE_UNIT_NOT_FOUND" {
		t.Fatalf("unexpected missing unit response: response=%#v err=%v", response, err)
	}
	response, err = HandleGitHubWorkflowInitAt(root, plugin.Request{
		Command: githubWorkflowInitCommandName,
		Flags:   map[string]any{"path": ".github/workflows/not-configured.yml"},
	})
	if err != nil || response.Status != "error" || response.Error.Code != "WORKFLOW_TARGET_NOT_CONFIGURED" {
		t.Fatalf("unexpected unconfigured path response: response=%#v err=%v", response, err)
	}
}

type recordingWorkflowOutputCreator struct {
	failure *CommandFailure
	calls   int
}

func (creator *recordingWorkflowOutputCreator) Create(_ githubWorkflowOutputTarget, _ []byte) *CommandFailure {
	creator.calls++
	return creator.failure
}
