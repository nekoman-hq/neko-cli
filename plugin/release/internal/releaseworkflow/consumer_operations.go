package releaseworkflow

import (
	"fmt"
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool/goreleaser"
	"gopkg.in/yaml.v3"
)

// ConsumerOperationID identifies one immutable, locally configured consumer
// workflow operation. These values describe workflow facts and execute nothing.
type ConsumerOperationID string

const (
	ConsumerContextValidation        ConsumerOperationID = "canonical-context-validation"
	ConsumerPluginManifestValidation ConsumerOperationID = "plugin-manifest-validation"
	ConsumerTests                    ConsumerOperationID = "consumer-tests"
	ConsumerToolConfigurationCheck   ConsumerOperationID = "release-tool-configuration-validation"
	ConsumerSnapshotBuild            ConsumerOperationID = "snapshot-build"
	ConsumerWorktreeValidation       ConsumerOperationID = "consumer-worktree-validation"
	ConsumerArtifactPackaging        ConsumerOperationID = "release-artifact-packaging"
	ConsumerReleasePublication       ConsumerOperationID = "release-publication"
	ConsumerPluginIndexGeneration    ConsumerOperationID = "plugin-index-generation"
	ConsumerPluginIndexPublication   ConsumerOperationID = "plugin-index-publication"
)

// ConsumerOperation is a non-executable fact derived from one configured
// workflow step.
//
//nolint:govet // Field order keeps user-facing stage metadata together.
type ConsumerOperation struct {
	ID              ConsumerOperationID
	Label           string
	Owner           string
	Location        string
	Mutation        string
	JobID           string
	StepName        string
	ToolCommand     string
	ConfigReference string
	Snapshot        bool
	SkipPublication bool
	Publishes       bool
	PluginOnly      bool
}

// ConsumerWorkflowFacts contains ordered local workflow operations and no raw
// YAML, diagnostics, credentials, or executable handlers.
type ConsumerWorkflowFacts struct {
	Operations []ConsumerOperation
}

// InspectConsumerWorkflow parses one local workflow into ordered neutral
// consumer-operation facts.
func InspectConsumerWorkflow(content []byte, pluginUnit bool) (ConsumerWorkflowFacts, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return ConsumerWorkflowFacts{}, fmt.Errorf("parse consumer workflow YAML: %w", err)
	}
	return InspectConsumerWorkflowDocument(&document, pluginUnit)
}

// InspectConsumerWorkflowDocument derives ordered operation facts from an
// already parsed workflow document. Doctor and read-only inspection commands
// share this classifier instead of maintaining separate tool parsers.
func InspectConsumerWorkflowDocument(document *yaml.Node, pluginUnit bool) (ConsumerWorkflowFacts, error) {
	root := workflowRoot(document)
	if root == nil || root.Kind != yaml.MappingNode {
		return ConsumerWorkflowFacts{}, fmt.Errorf("consumer workflow root must be a mapping")
	}
	facts := ConsumerWorkflowFacts{Operations: make([]ConsumerOperation, 0)}
	jobs := workflowValue(root, "jobs")
	if jobs == nil || jobs.Kind != yaml.MappingNode {
		return facts, nil
	}
	for jobIndex := 0; jobIndex+1 < len(jobs.Content); jobIndex += 2 {
		jobID := jobs.Content[jobIndex].Value
		job := jobs.Content[jobIndex+1]
		steps := workflowValue(job, "steps")
		if steps == nil || steps.Kind != yaml.SequenceNode {
			continue
		}
		for _, step := range steps.Content {
			facts.Operations = append(facts.Operations, classifyConsumerStep(root, jobID, job, step, pluginUnit)...)
		}
	}
	return facts, nil
}

// HasConsumerOperation reports whether ordered facts contain id.
func HasConsumerOperation(facts ConsumerWorkflowFacts, id ConsumerOperationID) bool {
	for _, operation := range facts.Operations {
		if operation.ID == id {
			return true
		}
	}
	return false
}

func classifyConsumerStep(root *yaml.Node, jobID string, job, step *yaml.Node, pluginUnit bool) []ConsumerOperation {
	name := workflowScalarValue(workflowValue(step, "name"))
	uses := strings.ToLower(workflowScalarValue(workflowValue(step, "uses")))
	run := workflowScalarValue(workflowValue(step, "run"))
	base := ConsumerOperation{JobID: jobID, StepName: name}
	operations := make([]ConsumerOperation, 0, 2)
	appendOperation := func(operation ConsumerOperation) {
		operation.JobID, operation.StepName = base.JobID, base.StepName
		operations = append(operations, operation)
	}

	switch {
	case strings.Contains(run, "neko release ci-validate-context"):
		appendOperation(consumerOperation(ConsumerContextValidation, "Validate canonical release context", "Neko CLI", "GitHub Actions runner", "none"))
	case pluginUnit && strings.Contains(strings.ToLower(name), "validate materialized plugin manifest") && strings.Contains(run, "jq"):
		operation := consumerOperation(ConsumerPluginManifestValidation, "Validate materialized plugin manifest", "consumer workflow", "GitHub Actions runner", "none")
		operation.PluginOnly = true
		appendOperation(operation)
	case containsCommand(run, "go", "test"):
		appendOperation(consumerOperation(ConsumerTests, "Run consumer tests", "consumer workflow", "GitHub Actions runner", "none"))
	case strings.Contains(run, "git diff --exit-code"):
		appendOperation(consumerOperation(ConsumerWorktreeValidation, "Verify tracked files are unchanged", "consumer workflow", "GitHub Actions runner", "none"))
	}

	if strings.HasPrefix(uses, "goreleaser/goreleaser-action@") {
		invocation := goreleaser.ClassifyArguments(workflowScalarValue(workflowValue(workflowValue(step, "with"), "args")))
		operation := ConsumerOperation{
			JobID: jobID, StepName: name, ToolCommand: invocation.Command,
			ConfigReference: resolveWorkflowValue(invocation.ConfigReference, root, job, step),
			Snapshot:        invocation.Snapshot, SkipPublication: invocation.SkipPublication,
			Publishes: invocation.RealPublication,
		}
		switch {
		case invocation.Command == "check":
			operation.ID, operation.Label = ConsumerToolConfigurationCheck, "Validate release-tool configuration"
			operation.Owner, operation.Location, operation.Mutation = "release tool", "GitHub Actions runner", "none"
		case invocation.Command == "build" && invocation.Snapshot:
			operation.ID, operation.Label = ConsumerSnapshotBuild, "Build snapshot artifacts"
			operation.Owner, operation.Location, operation.Mutation = "release tool", "GitHub Actions runner", "filesystem"
		case invocation.Command == "release" && invocation.Snapshot && invocation.SkipPublication:
			operation.ID, operation.Label = ConsumerArtifactPackaging, "Package release artifacts"
			operation.Owner, operation.Location, operation.Mutation = "release tool", "GitHub Actions runner", "filesystem"
		case invocation.RealPublication:
			operation.ID, operation.Label = ConsumerReleasePublication, "Publish release artifacts"
			operation.Owner, operation.Location, operation.Mutation = "release tool", "GitHub Actions runner", "publication"
		}
		if operation.ID != "" {
			operations = append(operations, operation)
		}
	}

	if containsCommand(run, "gh", "release", "create") {
		appendOperation(consumerOperation(ConsumerReleasePublication, "Create GitHub release", "GitHub API", "GitHub API", "publication"))
	}
	if pluginUnit && strings.Contains(run, ".github/scripts/generate-plugin-index.sh") {
		operation := consumerOperation(ConsumerPluginIndexGeneration, "Generate plugin registry index", "consumer workflow", "GitHub Actions runner", "filesystem")
		operation.PluginOnly = true
		appendOperation(operation)
	}
	if pluginUnit && strings.Contains(run, ".github/scripts/publish-plugin-index.sh") {
		operation := consumerOperation(ConsumerPluginIndexPublication, "Publish plugin registry index", "GitHub API", "GitHub API", "publication")
		operation.PluginOnly = true
		appendOperation(operation)
	}
	return operations
}

func consumerOperation(id ConsumerOperationID, label, owner, location, mutation string) ConsumerOperation {
	return ConsumerOperation{ID: id, Label: label, Owner: owner, Location: location, Mutation: mutation}
}

func containsCommand(command string, prefix ...string) bool {
	for _, line := range strings.Split(command, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < len(prefix) {
			continue
		}
		matches := true
		for index := range prefix {
			if fields[index] != prefix[index] {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func resolveWorkflowValue(value string, root, job, step *yaml.Node) string {
	value = strings.TrimSuffix(strings.TrimSpace(value), "\\")
	normalized := strings.NewReplacer("${{ ", "${{", " }}", "}}").Replace(value)
	name := ""
	switch {
	case strings.HasPrefix(normalized, "${{env.") && strings.HasSuffix(normalized, "}}"):
		name = strings.TrimSuffix(strings.TrimPrefix(normalized, "${{env."), "}}")
	case strings.HasPrefix(normalized, "${") && strings.HasSuffix(normalized, "}"):
		name = strings.TrimSuffix(strings.TrimPrefix(normalized, "${"), "}")
	case strings.HasPrefix(normalized, "$"):
		name = strings.TrimPrefix(normalized, "$")
	default:
		return normalized
	}
	for _, env := range []*yaml.Node{workflowValue(step, "env"), workflowValue(job, "env"), workflowValue(root, "env")} {
		if resolved := workflowScalarValue(workflowValue(env, name)); resolved != "" {
			return resolved
		}
	}
	return ""
}

func workflowRoot(document *yaml.Node) *yaml.Node {
	if document != nil && document.Kind == yaml.DocumentNode && len(document.Content) == 1 {
		return document.Content[0]
	}
	return document
}

func workflowValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1]
		}
	}
	return nil
}

func workflowScalarValue(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.ScalarNode {
		return ""
	}
	return node.Value
}
