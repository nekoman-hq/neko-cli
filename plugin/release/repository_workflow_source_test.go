package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/localaction"
	"gopkg.in/yaml.v3"
)

func repositoryRootForWorkflowCharacterization() string {
	return filepath.Clean(filepath.Join("..", ".."))
}

// repositoryEffectiveWorkflowSource renders the ordered effective steps of one
// repository workflow, expanding the repository-local composite actions it
// invokes. Ordering characterizations follow the steps a run really performs
// instead of the text of a single file.
func repositoryEffectiveWorkflowSource(t *testing.T, workflowPath string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repositoryRootForWorkflowCharacterization(), filepath.FromSlash(workflowPath)))
	if err != nil {
		t.Fatalf("read %s: %v", workflowPath, err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatalf("parse %s: %v", workflowPath, err)
	}
	root := &document
	if root.Kind == yaml.DocumentNode && len(root.Content) == 1 {
		root = root.Content[0]
	}
	expander := localaction.NewRepositoryActions(repositoryRootForWorkflowCharacterization())
	effective := make([]string, 0)
	for _, job := range workflowCharacterizationJobSteps(t, root) {
		for _, step := range expander.Expand(job) {
			effective = append(effective, workflowCharacterizationNodeText(t, step.Node))
		}
	}
	return strings.Join(effective, "")
}

func workflowCharacterizationJobSteps(t *testing.T, root *yaml.Node) [][]*yaml.Node {
	t.Helper()
	jobs := workflowCharacterizationValue(root, "jobs")
	if jobs == nil || jobs.Kind != yaml.MappingNode {
		t.Fatal("workflow has no jobs mapping")
	}
	ordered := make([][]*yaml.Node, 0, len(jobs.Content)/2)
	for index := 0; index+1 < len(jobs.Content); index += 2 {
		steps := workflowCharacterizationValue(jobs.Content[index+1], "steps")
		if steps == nil || steps.Kind != yaml.SequenceNode {
			continue
		}
		ordered = append(ordered, steps.Content)
	}
	return ordered
}

func workflowCharacterizationValue(node *yaml.Node, key string) *yaml.Node {
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

func workflowCharacterizationNodeText(t *testing.T, node *yaml.Node) string {
	t.Helper()
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(node); err != nil {
		t.Fatalf("encode workflow step: %v", err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatalf("close workflow step encoder: %v", err)
	}
	return output.String()
}
