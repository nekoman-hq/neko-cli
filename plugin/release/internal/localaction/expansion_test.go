package localaction

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRepositoryActionsExpandsCompositeActionSteps(t *testing.T) {
	root := t.TempDir()
	writeLocalActionFixture(t, root, ".github/actions/publish/action.yml", `
name: Publish
description: Publish one artifact.
inputs:
  target:
    description: Publication target.
    required: true
  channel:
    description: Publication channel.
    required: false
    default: stable
runs:
  using: composite
  steps:
    - name: Prepare
      shell: bash
      run: echo prepare
    - name: Publish
      shell: bash
      env:
        TARGET: ${{ inputs.target }}
        CHANNEL: ${{ inputs.channel }}
      run: echo publish
`)

	steps := expandWorkflowSteps(t, root, `
- name: Checkout
  uses: actions/checkout@v4
- name: Publish artifact
  id: publication
  uses: ./.github/actions/publish
  env:
    GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  with:
    target: ${{ needs.validate.outputs.release_sha }}
`)

	if names := stepNames(steps); !reflect.DeepEqual(names, []string{"Checkout", "Prepare", "Publish"}) {
		t.Fatalf("effective step order = %v", names)
	}
	if steps[0].Origin.Expanded() || steps[0].Failure != "" {
		t.Fatalf("declared workflow step = %#v", steps[0].Origin)
	}
	for _, step := range steps[1:] {
		if step.Origin.ActionPath != ".github/actions/publish/action.yml" ||
			step.Origin.CallerName != "Publish artifact" || step.Origin.CallerID != "publication" {
			t.Fatalf("expanded step origin = %#v", step.Origin)
		}
		if !reflect.DeepEqual(step.Origin.Inputs, []string{"target"}) {
			t.Fatalf("recorded invocation inputs = %#v", step.Origin.Inputs)
		}
		if got := stepEnvironment(step.Node, "GH_TOKEN"); got != "${{ secrets.GITHUB_TOKEN }}" {
			t.Fatalf("inherited environment = %q", got)
		}
	}
	if got := stepEnvironment(steps[2].Node, "TARGET"); got != "${{ needs.validate.outputs.release_sha }}" {
		t.Fatalf("supplied input = %q", got)
	}
	if got := stepEnvironment(steps[2].Node, "CHANNEL"); got != "stable" {
		t.Fatalf("declared input default = %q", got)
	}
	if got := stepEnvironment(steps[2].Declared, "GH_TOKEN"); got != "" {
		t.Fatalf("declared action step must not own the invocation credential: %q", got)
	}
}

func TestRepositoryActionsSupportsBothActionFileNames(t *testing.T) {
	for _, name := range []string{"action.yml", "action.yaml"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeLocalActionFixture(t, root, ".github/actions/setup/"+name, `
name: Setup
description: Set one value.
runs:
  using: composite
  steps:
    - name: Setup step
      shell: bash
      run: echo setup
`)
			steps := expandWorkflowSteps(t, root, "- uses: ./.github/actions/setup\n")
			if names := stepNames(steps); !reflect.DeepEqual(names, []string{"Setup step"}) {
				t.Fatalf("effective steps = %v", names)
			}
		})
	}
}

func TestRepositoryActionsExpandsNestedCompositeActions(t *testing.T) {
	root := t.TempDir()
	writeLocalActionFixture(t, root, ".github/actions/outer/action.yml", `
name: Outer
description: Delegate to an inner action.
inputs:
  value:
    description: Forwarded value.
    required: true
runs:
  using: composite
  steps:
    - name: Outer step
      shell: bash
      run: echo outer
    - name: Inner invocation
      uses: ./.github/actions/inner
      with:
        forwarded: ${{ inputs.value }}
`)
	writeLocalActionFixture(t, root, ".github/actions/inner/action.yml", `
name: Inner
description: Consume a forwarded value.
inputs:
  forwarded:
    description: Forwarded value.
    required: true
runs:
  using: composite
  steps:
    - name: Inner step
      shell: bash
      env:
        FORWARDED: ${{ inputs.forwarded }}
      run: echo inner
`)

	steps := expandWorkflowSteps(t, root, `
- name: Outer invocation
  id: outer
  uses: ./.github/actions/outer
  env:
    SHARED: shared-value
  with:
    value: ${{ inputs.release_sha }}
`)
	if names := stepNames(steps); !reflect.DeepEqual(names, []string{"Outer step", "Inner step"}) {
		t.Fatalf("nested effective steps = %v", names)
	}
	inner := steps[1]
	if inner.Origin.ActionPath != ".github/actions/inner/action.yml" ||
		inner.Origin.CallerName != "Outer invocation" || inner.Origin.CallerID != "outer" {
		t.Fatalf("nested origin = %#v", inner.Origin)
	}
	if got := stepEnvironment(inner.Node, "FORWARDED"); got != "${{ inputs.release_sha }}" {
		t.Fatalf("forwarded input = %q", got)
	}
	if got := stepEnvironment(inner.Node, "SHARED"); got != "shared-value" {
		t.Fatalf("nested environment inheritance = %q", got)
	}
}

func TestRepositoryActionsRejectsUnsafeAndUnresolvableReferences(t *testing.T) {
	root := t.TempDir()
	writeLocalActionFixture(t, root, ".github/actions/plain/action.yml", `
name: Plain
description: A non-composite local action.
runs:
  using: node20
  main: index.js
`)
	writeLocalActionFixture(t, root, ".github/actions/broken/action.yml", "name: [unterminated\n")
	writeLocalActionFixture(t, root, ".github/actions/self/action.yml", `
name: Self
description: Invoke itself.
runs:
  using: composite
  steps:
    - name: Recurse
      uses: ./.github/actions/self
`)

	tests := []struct {
		name        string
		uses        string
		wantFailure string
	}{
		{name: "parent traversal", uses: "../outside/action", wantFailure: FailureReferenceInvalid},
		{name: "absolute path", uses: "/etc/actions/publish", wantFailure: FailureReferenceInvalid},
		{name: "windows separator", uses: `.\.github\actions\publish`, wantFailure: FailureReferenceInvalid},
		{name: "escaping traversal", uses: "./.github/actions/../../../outside", wantFailure: FailureReferenceInvalid},
		{name: "missing action", uses: "./.github/actions/absent", wantFailure: FailureMissing},
		{name: "invalid definition", uses: "./.github/actions/broken", wantFailure: FailureDefinitionInvalid},
		{name: "recursive action", uses: "./.github/actions/self", wantFailure: FailureRecursive},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			steps := expandWorkflowSteps(t, root, "- name: Invoke\n  uses: "+test.uses+"\n")
			if len(steps) != 1 || steps[0].Failure != test.wantFailure {
				t.Fatalf("steps = %#v", steps)
			}
		})
	}

	t.Run("non-composite action", func(t *testing.T) {
		steps := expandWorkflowSteps(t, root, "- name: Invoke\n  uses: ./.github/actions/plain\n")
		if len(steps) != 1 || steps[0].Failure != "" || steps[0].Origin.Expanded() {
			t.Fatalf("non-composite action was expanded: %#v", steps)
		}
	})

	t.Run("remote references", func(t *testing.T) {
		for _, uses := range []string{"actions/checkout@v4", "docker://alpine:3", "https://example.test/action"} {
			steps := expandWorkflowSteps(t, root, "- name: Invoke\n  uses: "+uses+"\n")
			if len(steps) != 1 || steps[0].Failure != "" || steps[0].Origin.Expanded() {
				t.Fatalf("remote reference %q was resolved: %#v", uses, steps)
			}
		}
	})
}

func TestRepositoryActionsRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink escape coverage requires POSIX symlinks")
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "action.yml"), []byte(
		"name: Outside\ndescription: Outside the repository.\nruns:\n  using: composite\n  steps:\n    - run: echo escaped\n",
	), 0644); err != nil {
		t.Fatalf("write outside action: %v", err)
	}
	root := t.TempDir()
	directory := filepath.Join(root, ".github", "actions", "escaping")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatalf("create action directory: %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, "action.yml"), filepath.Join(directory, "action.yml")); err != nil {
		t.Fatalf("create escaping symlink: %v", err)
	}
	steps := expandWorkflowSteps(t, root, "- name: Invoke\n  uses: ./.github/actions/escaping\n")
	if len(steps) != 1 || steps[0].Failure != FailurePathEscape {
		t.Fatalf("symlink escape was accepted: %#v", steps)
	}
}

func TestRepositoryActionsExpansionIsDeterministic(t *testing.T) {
	root := t.TempDir()
	writeLocalActionFixture(t, root, ".github/actions/ordered/action.yml", `
name: Ordered
description: Three ordered steps.
runs:
  using: composite
  steps:
    - name: First
      shell: bash
      run: echo first
    - name: Second
      shell: bash
      run: echo second
    - name: Third
      shell: bash
      run: echo third
`)
	workflow := "- name: Before\n  run: echo before\n- uses: ./.github/actions/ordered\n- name: After\n  run: echo after\n"
	want := []string{"Before", "First", "Second", "Third", "After"}
	for attempt := 0; attempt < 3; attempt++ {
		if names := stepNames(expandWorkflowSteps(t, root, workflow)); !reflect.DeepEqual(names, want) {
			t.Fatalf("attempt %d order = %v, want %v", attempt, names, want)
		}
	}
}

func TestDeclaredStepsPerformNoExpansion(t *testing.T) {
	root := t.TempDir()
	writeLocalActionFixture(t, root, ".github/actions/publish/action.yml", `
name: Publish
description: Publish one artifact.
runs:
  using: composite
  steps:
    - name: Publish
      shell: bash
      run: echo publish
`)
	declared := parseWorkflowSteps(t, "- name: Invoke\n  uses: ./.github/actions/publish\n")
	steps := DeclaredSteps{}.Expand(declared)
	if len(steps) != 1 || steps[0].Origin.Expanded() || steps[0].Node != declared[0] {
		t.Fatalf("declared expansion = %#v", steps)
	}
	nodes := DeclaredSteps{}.EffectiveSteps(declared)
	if !reflect.DeepEqual(nodes, declared) {
		t.Fatalf("declared effective steps = %#v", nodes)
	}
}

func writeLocalActionFixture(t *testing.T, root, relativePath, content string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatalf("create %s parent: %v", relativePath, err)
	}
	if err := os.WriteFile(target, []byte(strings.TrimPrefix(content, "\n")), 0644); err != nil {
		t.Fatalf("write %s: %v", relativePath, err)
	}
}

func expandWorkflowSteps(t *testing.T, root, steps string) []Step {
	t.Helper()
	return NewRepositoryActions(root).Expand(parseWorkflowSteps(t, steps))
}

func parseWorkflowSteps(t *testing.T, steps string) []*yaml.Node {
	t.Helper()
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(strings.TrimPrefix(steps, "\n")), &document); err != nil {
		t.Fatalf("parse workflow steps: %v", err)
	}
	sequence := documentRoot(&document)
	if sequence == nil || sequence.Kind != yaml.SequenceNode {
		t.Fatalf("workflow steps are not a YAML sequence")
	}
	return sequence.Content
}

func stepNames(steps []Step) []string {
	names := make([]string, 0, len(steps))
	for _, step := range steps {
		names = append(names, scalarValue(mappingValue(step.Node, "name")))
	}
	return names
}

func stepEnvironment(step *yaml.Node, name string) string {
	return scalarValue(mappingValue(mappingValue(step, "env"), name))
}
