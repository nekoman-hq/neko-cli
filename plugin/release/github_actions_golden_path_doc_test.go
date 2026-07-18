package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	goldenPathWorkflowStart = "<!-- golden-path-workflow:start -->"
	goldenPathWorkflowEnd   = "<!-- golden-path-workflow:end -->"
)

type documentedWorkflow struct {
	On struct {
		WorkflowDispatch struct {
			Inputs map[string]documentedWorkflowInput `yaml:"inputs"`
		} `yaml:"workflow_dispatch"`
	} `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Jobs        map[string]struct {
		Steps []documentedWorkflowStep `yaml:"steps"`
	} `yaml:"jobs"`
	Concurrency struct {
		Group            string `yaml:"group"`
		CancelInProgress bool   `yaml:"cancel-in-progress"`
	} `yaml:"concurrency"`
}

type documentedWorkflowInput struct {
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
	Required    bool   `yaml:"required"`
}

type documentedWorkflowStep struct {
	Name string            `yaml:"name"`
	ID   string            `yaml:"id"`
	Uses string            `yaml:"uses"`
	With map[string]any    `yaml:"with"`
	Env  map[string]string `yaml:"env"`
	Run  string            `yaml:"run"`
}

func TestGitHubActionsGoldenPathWorkflowContract(t *testing.T) {
	document := readGoldenPathDocument(t)
	workflowYAML := extractGoldenPathWorkflow(t, document)

	var workflow documentedWorkflow
	if err := yaml.Unmarshal([]byte(workflowYAML), &workflow); err != nil {
		t.Fatalf("parse documented workflow YAML: %v", err)
	}

	expectedInputs := []string{"release_sha", "tag", "unit", "version"}
	actualInputs := make([]string, 0, len(workflow.On.WorkflowDispatch.Inputs))
	for name, input := range workflow.On.WorkflowDispatch.Inputs {
		actualInputs = append(actualInputs, name)
		if !input.Required {
			t.Errorf("workflow input %q must be required", name)
		}
		if input.Type != "string" {
			t.Errorf("workflow input %q type = %q, want string", name, input.Type)
		}
		if input.Description == "" {
			t.Errorf("workflow input %q must explain its authority", name)
		}
	}
	slices.Sort(actualInputs)
	if !slices.Equal(actualInputs, expectedInputs) {
		t.Fatalf("workflow inputs = %v, want exactly %v", actualInputs, expectedInputs)
	}

	if len(workflow.Permissions) != 1 || workflow.Permissions["contents"] != "read" {
		t.Errorf("workflow permissions = %v, want only contents: read", workflow.Permissions)
	}
	if workflow.Concurrency.Group != "release-${{ inputs.unit }}-${{ inputs.tag }}" {
		t.Errorf("concurrency group = %q, want unit and tag identity", workflow.Concurrency.Group)
	}
	if workflow.Concurrency.CancelInProgress {
		t.Error("release workflow must not cancel an in-progress release")
	}

	releaseJob, ok := workflow.Jobs["release"]
	if !ok {
		t.Fatal("documented workflow must define the release job")
	}
	checkout := findDocumentedStep(t, releaseJob.Steps, "Checkout the exact release commit with tags")
	if checkout.Uses != "actions/checkout@v4" {
		t.Errorf("checkout action = %q, want actions/checkout@v4", checkout.Uses)
	}
	assertWorkflowValue(t, checkout.With, "ref", "${{ inputs.release_sha }}")
	assertWorkflowValue(t, checkout.With, "fetch-depth", "0")
	assertWorkflowValue(t, checkout.With, "fetch-tags", "true")
	assertWorkflowValue(t, checkout.With, "persist-credentials", "false")

	validation := findDocumentedStep(t, releaseJob.Steps, "Validate Neko release context")
	for _, fragment := range []string{
		"neko release ci-validate-context",
		"--unit \"$RELEASE_UNIT\"",
		"--version \"$RELEASE_VERSION\"",
		"--tag \"$RELEASE_TAG\"",
		"--release-sha \"$RELEASE_SHA\"",
		"--output github",
		"--github-output-file \"$GITHUB_OUTPUT\"",
	} {
		if !strings.Contains(validation.Run, fragment) {
			t.Errorf("context validation is missing %q", fragment)
		}
	}
	if strings.Contains(validation.Run, "${{") {
		t.Error("context validation must receive dispatch values through environment variables")
	}
	if validation.ID != "release-context" {
		t.Errorf("context validation step id = %q, want release-context", validation.ID)
	}
	for name, value := range map[string]string{
		"RELEASE_UNIT":    "${{ inputs.unit }}",
		"RELEASE_VERSION": "${{ inputs.version }}",
		"RELEASE_TAG":     "${{ inputs.tag }}",
		"RELEASE_SHA":     "${{ inputs.release_sha }}",
	} {
		if validation.Env[name] != value {
			t.Errorf("validation env %s = %q, want %q", name, validation.Env[name], value)
		}
	}
	for _, forbidden := range []string{"jq ", "fail()", "git rev-parse", "git show-ref"} {
		if strings.Contains(validation.Run, forbidden) {
			t.Errorf("context validation retained manual implementation %q", forbidden)
		}
	}

	consumer := findDocumentedStep(t, releaseJob.Steps, "Build and publish selected unit")
	for name, value := range map[string]string{
		"RELEASE_UNIT":    "${{ steps.release-context.outputs.unit }}",
		"RELEASE_VERSION": "${{ steps.release-context.outputs.version }}",
		"RELEASE_TAG":     "${{ steps.release-context.outputs.tag }}",
		"RELEASE_SHA":     "${{ steps.release-context.outputs.release_sha }}",
	} {
		if consumer.Env[name] != value {
			t.Errorf("consumer env %s = %q, want %q", name, consumer.Env[name], value)
		}
	}
	for _, fragment := range []string{
		"./tooling/publish-release",
		"--unit \"$RELEASE_UNIT\"",
		"--version \"$RELEASE_VERSION\"",
		"--tag \"$RELEASE_TAG\"",
		"--release-sha \"$RELEASE_SHA\"",
	} {
		if !strings.Contains(consumer.Run, fragment) {
			t.Errorf("consumer extension point is missing %q", fragment)
		}
	}

	allCommands := validation.Run + "\n" + consumer.Run
	for _, forbidden := range []string{
		".release.neko.json",
		"neko release execute",
		"neko release patch",
		"neko release minor",
		"neko release major",
		"git commit",
		"git tag",
		"git push",
		"gh workflow run",
	} {
		if strings.Contains(allCommands, forbidden) {
			t.Errorf("documented workflow contains forbidden release-owner command %q", forbidden)
		}
	}
}

func TestGitHubActionsGoldenPathNavigation(t *testing.T) {
	root := repositoryRoot(t)
	paths := []string{
		"docs/release/overview.md",
		"docs/release/github-actions-delivery.md",
		"docs/release/cli-reference.md",
		"docs/release/examples.md",
		"docs/plugins/release.md",
	}
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read navigation file %s: %v", path, err)
		}
		if !strings.Contains(string(data), "github-actions-golden-path.md") {
			t.Errorf("%s does not link to the canonical golden path", path)
		}
	}
}

func readGoldenPathDocument(t *testing.T) string {
	t.Helper()
	path := filepath.Join(repositoryRoot(t), "docs", "release", "github-actions-golden-path.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden path document: %v", err)
	}
	return string(data)
}

func extractGoldenPathWorkflow(t *testing.T, document string) string {
	t.Helper()
	_, marked, found := strings.Cut(document, goldenPathWorkflowStart)
	if !found {
		t.Fatal("golden path workflow start marker is missing")
	}
	marked, _, found = strings.Cut(marked, goldenPathWorkflowEnd)
	if !found {
		t.Fatal("golden path workflow end marker is missing")
	}
	_, fenced, found := strings.Cut(marked, "```yaml\n")
	if !found {
		t.Fatal("golden path workflow YAML fence is missing")
	}
	workflowYAML, remainder, found := strings.Cut(fenced, "\n```")
	if !found || strings.TrimSpace(remainder) != "" {
		t.Fatal("golden path markers must contain exactly one YAML fence")
	}
	return workflowYAML
}

func findDocumentedStep(t *testing.T, steps []documentedWorkflowStep, name string) documentedWorkflowStep {
	t.Helper()
	for _, step := range steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("documented workflow step %q is missing", name)
	return documentedWorkflowStep{}
}

func assertWorkflowValue(t *testing.T, values map[string]any, name string, want string) {
	t.Helper()
	if got := fmt.Sprint(values[name]); got != want {
		t.Errorf("workflow value %s = %q, want %q", name, got, want)
	}
}
