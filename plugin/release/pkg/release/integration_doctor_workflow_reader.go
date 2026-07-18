package release

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

//nolint:govet // Logical reader result order keeps parse facts together.
type integrationDoctorWorkflowSnapshot struct {
	Document    *yaml.Node
	Content     []byte
	FailureCode string
	FailureText string
	Exists      bool
	Canonical   bool
}

type integrationDoctorWorkflowReader interface {
	Read(string, string) integrationDoctorWorkflowSnapshot
}

type filesystemIntegrationDoctorWorkflowReader struct{}

func (filesystemIntegrationDoctorWorkflowReader) Read(root, relativePath string) integrationDoctorWorkflowSnapshot {
	_, content, exists, failure := inspectGitHubWorkflowOutputTarget(root, relativePath)
	if failure != nil {
		return integrationDoctorWorkflowSnapshot{
			FailureCode: failure.Code,
			FailureText: failure.responseMessage(),
		}
	}
	snapshot := integrationDoctorWorkflowSnapshot{Content: content, Exists: exists}
	if !exists {
		return snapshot
	}
	canonical, err := RenderCanonicalGitHubActionsReleaseWorkflow()
	if err == nil {
		snapshot.Canonical = bytes.Equal(content, canonical)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		snapshot.FailureCode = "WORKFLOW_YAML_INVALID"
		snapshot.FailureText = "workflow YAML could not be parsed structurally"
		return snapshot
	}
	snapshot.Document = &document
	return snapshot
}

func workflowDocumentRoot(document *yaml.Node) *yaml.Node {
	if document == nil {
		return nil
	}
	if document.Kind == yaml.DocumentNode && len(document.Content) == 1 {
		return document.Content[0]
	}
	return document
}

func workflowMappingValue(node *yaml.Node, key string) *yaml.Node {
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

func workflowMappingKeys(node *yaml.Node) []string {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	keys := make([]string, 0, len(node.Content)/2)
	for index := 0; index+1 < len(node.Content); index += 2 {
		keys = append(keys, node.Content[index].Value)
	}
	return keys
}

func workflowScalar(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.ScalarNode {
		return ""
	}
	return node.Value
}

func workflowBool(node *yaml.Node) (bool, bool) {
	if node == nil || node.Kind != yaml.ScalarNode || node.Tag != "!!bool" {
		return false, false
	}
	return node.Value == "true", true
}

func workflowNodeText(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(node); err != nil {
		return ""
	}
	_ = encoder.Close()
	return output.String()
}

var _ integrationDoctorWorkflowReader = filesystemIntegrationDoctorWorkflowReader{}
