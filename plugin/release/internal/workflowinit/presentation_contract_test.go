package workflowinit

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestWorkflowInitPreviewSeparatesDefaultAndDescribeAndPreservesJSON(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	response, err := HandleGitHubWorkflowInitAt(root, plugin.Request{
		Command: githubWorkflowInitCommandName,
		Flags:   map[string]any{"dry-run": true},
	})
	if err != nil || response.Status != "success" {
		t.Fatalf("preview response=%#v err=%v", response, err)
	}
	generated, ok := response.Data["generated_content"].(string)
	if !ok || generated == "" {
		t.Fatal("preview generated content is missing")
	}

	plain := renderWorkflowInitResponse(t, response, false)
	described := renderWorkflowInitResponse(t, response, true)
	for _, want := range []string{
		"GitHub Actions workflow scaffolding preview", "Target: .github/workflows/release.yml",
		"Status: create", "Action: would-create", "Generated workflow", generated,
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("workflow preview default omitted %q:\n%s", want, plain)
		}
	}
	for _, hidden := range []string{"Workflow Identity", "Target Comparison", "Validation Facts", "Write Plan", "Limitations"} {
		if strings.Contains(plain, hidden) || !strings.Contains(described, hidden) {
			t.Fatalf("workflow describe visibility for %q is incorrect:\nplain:\n%s\ndescribed:\n%s", hidden, plain, described)
		}
	}
	if strings.Count(described, generated) != 1 {
		t.Fatalf("workflow describe duplicated generated YAML:\n%s", described)
	}
	if strings.Contains(plain, root.Path()) || strings.Contains(described, root.Path()) {
		t.Fatalf("workflow preview exposed absolute fixture root:\nplain:\n%s\ndescribed:\n%s", plain, described)
	}
	assertWorkflowInitJSONPresentationInvariant(t, response)
}

func TestWorkflowInitCreateAndIdenticalDefaultsAreConcise(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	target := filepath.Join(root.Path(), ".github", "workflows", "release.yml")

	created, err := HandleGitHubWorkflowInitAt(root, plugin.Request{Command: githubWorkflowInitCommandName})
	if err != nil || created.Status != "success" {
		t.Fatalf("create response=%#v err=%v", created, err)
	}
	createOutput := renderWorkflowInitResponse(t, created, false)
	for _, want := range []string{
		"GitHub Workflow Initialization", "Workflow created", ".github/workflows/release.yml",
		"Canonical workflow", "Contract version", "Next action",
	} {
		if !strings.Contains(createOutput, want) {
			t.Fatalf("workflow create default omitted %q:\n%s", want, createOutput)
		}
	}
	for _, hidden := range []string{"Workflow Identity", "Target Comparison", "Validation Facts", "Write Plan", "Limitations"} {
		if strings.Contains(createOutput, hidden) {
			t.Fatalf("workflow create default exposed %q:\n%s", hidden, createOutput)
		}
	}
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read created workflow: %v", err)
	}

	identical, err := HandleGitHubWorkflowInitAt(root, plugin.Request{Command: githubWorkflowInitCommandName})
	if err != nil || identical.Status != "success" {
		t.Fatalf("identical response=%#v err=%v", identical, err)
	}
	identicalOutput := renderWorkflowInitResponse(t, identical, false)
	for _, want := range []string{"GitHub Workflow Initialization", "Workflow already current", "No write required"} {
		if !strings.Contains(identicalOutput, want) {
			t.Fatalf("workflow identical default omitted %q:\n%s", want, identicalOutput)
		}
	}
	after, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("identical workflow was rewritten: err=%v", err)
	}
}

func TestWorkflowInitConflictIsActionableSafeAndNeverOverwrites(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	target := filepath.Join(root.Path(), ".github", "workflows", "release.yml")
	existing := []byte("name: consumer-owned\n")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("create workflow directory: %v", err)
	}
	if err := os.WriteFile(target, existing, 0o600); err != nil {
		t.Fatalf("write conflicting workflow: %v", err)
	}

	response, err := HandleGitHubWorkflowInitAt(root, plugin.Request{Command: githubWorkflowInitCommandName})
	if err != nil || response.Status != "error" || response.ExitCode != 1 {
		t.Fatalf("conflict response=%#v err=%v", response, err)
	}
	output := renderWorkflowInitResponse(t, response, false)
	for _, want := range []string{
		"Workflow Initialization Blocked", "WORKFLOW_TARGET_CONFLICT", ".github/workflows/release.yml",
		"Different content", "Overwrite", "Refused", "--dry-run", "resolve the file manually",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("workflow conflict omitted %q:\n%s", want, output)
		}
	}
	for _, forbidden := range []string{root.Path(), "\x1b[", "Authorization", "Bearer"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("workflow conflict output contains %q:\n%s", forbidden, output)
		}
	}
	got, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(got, existing) {
		t.Fatalf("workflow conflict overwrote target: content=%q err=%v", got, err)
	}
}

func TestWorkflowInitVerbosePhasesDistinguishDryRunCreateIdenticalAndConflict(t *testing.T) {
	dryRunRoot := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	dryRunOutput := captureWorkflowInitLogs(t, func() {
		response, err := HandleGitHubWorkflowInitAt(dryRunRoot, plugin.Request{
			Command: githubWorkflowInitCommandName,
			Flags:   map[string]any{"dry-run": true},
		})
		if err != nil || response.Status != "success" {
			t.Fatalf("dry-run response=%#v err=%v", response, err)
		}
	})
	assertWorkflowInitLogOrder(t, dryRunOutput, []string{
		"Validating workflow initialization request",
		"Selecting workflow initialization mode",
		"Reading Release V2 workflow configuration",
		"Resolving configured workflow target",
		"Validating workflow target path policy",
		"Reading existing workflow target",
		"Rendering canonical workflow content",
		"Validating canonical workflow content",
		"Comparing canonical and existing workflow content",
		"Workflow target classified as create",
		"Dry-run selected; no workflow file written",
		"Workflow preview completed",
	})
	for _, forbidden := range []string{"Writing missing workflow file", "Workflow file created", dryRunRoot.Path(), "\x1b["} {
		if strings.Contains(dryRunOutput, forbidden) {
			t.Fatalf("workflow dry-run log contains %q:\n%s", forbidden, dryRunOutput)
		}
	}

	createRoot := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	createOutput := captureWorkflowInitLogs(t, func() {
		response, err := HandleGitHubWorkflowInitAt(createRoot, plugin.Request{Command: githubWorkflowInitCommandName})
		if err != nil || response.Status != "success" {
			t.Fatalf("create response=%#v err=%v", response, err)
		}
	})
	assertWorkflowInitLogOrder(t, createOutput, []string{
		"Workflow target classified as create",
		"Writing missing workflow file",
		"Workflow file created",
		"Workflow initialization completed",
	})

	identicalOutput := captureWorkflowInitLogs(t, func() {
		response, err := HandleGitHubWorkflowInitAt(createRoot, plugin.Request{Command: githubWorkflowInitCommandName})
		if err != nil || response.Status != "success" {
			t.Fatalf("identical response=%#v err=%v", response, err)
		}
	})
	for _, want := range []string{"Workflow target classified as unchanged", "Existing canonical workflow accepted; no write required", "Workflow initialization completed"} {
		if !strings.Contains(identicalOutput, want) {
			t.Fatalf("workflow identical log omitted %q:\n%s", want, identicalOutput)
		}
	}
	if strings.Contains(identicalOutput, "Writing missing workflow file") {
		t.Fatalf("workflow identical log claimed a write:\n%s", identicalOutput)
	}

	conflictRoot := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	conflictTarget := filepath.Join(conflictRoot.Path(), ".github", "workflows", "release.yml")
	if err := os.MkdirAll(filepath.Dir(conflictTarget), 0o755); err != nil {
		t.Fatalf("create conflict directory: %v", err)
	}
	if err := os.WriteFile(conflictTarget, []byte("name: consumer-owned\n"), 0o600); err != nil {
		t.Fatalf("write conflict target: %v", err)
	}
	conflictOutput := captureWorkflowInitLogs(t, func() {
		response, err := HandleGitHubWorkflowInitAt(conflictRoot, plugin.Request{Command: githubWorkflowInitCommandName})
		if err != nil || response.Status != "error" {
			t.Fatalf("conflict response=%#v err=%v", response, err)
		}
	})
	for _, want := range []string{"Workflow target classified as conflict", "Workflow content conflict detected; overwrite refused"} {
		if !strings.Contains(conflictOutput, want) {
			t.Fatalf("workflow conflict log omitted %q:\n%s", want, conflictOutput)
		}
	}
	for _, output := range []string{createOutput, identicalOutput, conflictOutput} {
		for _, forbidden := range []string{createRoot.Path(), conflictRoot.Path(), "\x1b[", "GITHUB_TOKEN", "Authorization", "Bearer"} {
			if strings.Contains(output, forbidden) {
				t.Fatalf("workflow verbose output contains %q:\n%s", forbidden, output)
			}
		}
	}
}

func TestWorkflowInitPresentationRemainsReadableAtNarrowAndUnknownWidth(t *testing.T) {
	root := newWorkflowScaffoldRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	response, err := HandleGitHubWorkflowInitAt(root, plugin.Request{Command: githubWorkflowInitCommandName})
	if err != nil || response.Status != "success" {
		t.Fatalf("create response=%#v err=%v", response, err)
	}
	for _, width := range []workflowInitPresentationWidthState{{width: 34, available: true}, {available: false}} {
		var output bytes.Buffer
		if renderErr := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
			Format: renderer.FormatTable, Describe: true, WidthProvider: width,
		}, &output); renderErr != nil {
			t.Fatalf("render workflow width %#v: %v", width, renderErr)
		}
		for _, want := range []string{"GitHub Workflow Initialization", "Workflow Identity", ".github/workflows/rel", "Limitations"} {
			if !strings.Contains(output.String(), want) {
				t.Fatalf("workflow width %#v omitted %q:\n%s", width, want, output.String())
			}
		}
		if strings.Contains(output.String(), root.Path()) || strings.Contains(output.String(), "\x1b[") {
			t.Fatalf("workflow width %#v exposed unsafe output:\n%s", width, output.String())
		}
	}
}

func renderWorkflowInitResponse(t *testing.T, response *plugin.Response, describe bool) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: describe, WidthProvider: workflowInitPresentationWidth(120),
	}, &output); err != nil {
		t.Fatalf("render workflow init response: %v", err)
	}
	return output.String()
}

func assertWorkflowInitJSONPresentationInvariant(t *testing.T, response *plugin.Response) {
	t.Helper()
	var plain bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{Format: renderer.FormatJSON}, &plain); err != nil {
		t.Fatalf("render workflow JSON: %v", err)
	}
	var described bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
		Format: renderer.FormatJSON, Describe: true,
	}, &described); err != nil {
		t.Fatalf("render described workflow JSON: %v", err)
	}
	if !reflect.DeepEqual(plain.Bytes(), described.Bytes()) {
		t.Fatalf("describe changed workflow JSON:\nplain=%s\ndescribed=%s", plain.String(), described.String())
	}
	for _, forbidden := range []string{"human_table", "human_properties", "human_text", "describe_only", "\x1b["} {
		if strings.Contains(plain.String(), forbidden) {
			t.Fatalf("workflow JSON contains %q:\n%s", forbidden, plain.String())
		}
	}
}

func captureWorkflowInitLogs(t *testing.T, run func()) string {
	t.Helper()
	previousVerbose := log.Verbose
	log.Verbose = true
	t.Cleanup(func() { log.Verbose = previousVerbose })
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	previousStderr := os.Stderr
	os.Stderr = writer
	run()
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close stderr writer: %v", closeErr)
	}
	os.Stderr = previousStderr
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if closeErr := reader.Close(); closeErr != nil {
		t.Fatalf("close stderr reader: %v", closeErr)
	}
	return ansi.Strip(string(content))
}

func assertWorkflowInitLogOrder(t *testing.T, output string, phases []string) {
	t.Helper()
	previous := -1
	for _, phase := range phases {
		index := strings.Index(output, phase)
		if index < 0 {
			t.Fatalf("workflow verbose output omitted %q:\n%s", phase, output)
		}
		if index <= previous {
			t.Fatalf("workflow verbose phase %q is out of order:\n%s", phase, output)
		}
		previous = index
	}
}

type workflowInitPresentationWidth int

func (width workflowInitPresentationWidth) Width(io.Writer) (int, bool) {
	return int(width), true
}

type workflowInitPresentationWidthState struct {
	width     int
	available bool
}

func (width workflowInitPresentationWidthState) Width(io.Writer) (int, bool) {
	return width.width, width.available
}
