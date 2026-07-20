package doctor

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
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
	content, exists, failureCode, failureText := inspectIntegrationDoctorWorkflowTarget(root, relativePath)
	if failureCode != "" {
		return integrationDoctorWorkflowSnapshot{
			FailureCode: failureCode,
			FailureText: failureText,
		}
	}
	snapshot := integrationDoctorWorkflowSnapshot{Content: content, Exists: exists}
	if !exists {
		return snapshot
	}
	canonical, err := releaseworkflow.RenderCanonicalGitHubActionsReleaseWorkflow()
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

func inspectIntegrationDoctorWorkflowTarget(root, relativePath string) ([]byte, bool, string, string) {
	if err := releaseconfig.ValidateV2WorkflowPath(relativePath); err != nil {
		return nil, false, "WORKFLOW_TARGET_INVALID", "workflow target must be a canonical .github/workflows/*.yml or .yaml path"
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, false, "WORKFLOW_TARGET_INVALID", "repository root could not be resolved"
	}
	absolutePath := filepath.Join(absoluteRoot, filepath.FromSlash(relativePath))
	if code, message := validateIntegrationDoctorWorkflowParent(absoluteRoot, absolutePath); code != "" {
		return nil, false, code, message
	}
	info, err := os.Lstat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, "", ""
		}
		return nil, false, "WORKFLOW_TARGET_INVALID", "workflow target could not be inspected"
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, "WORKFLOW_TARGET_SYMLINK_ESCAPE", "workflow target must not be a symlink"
	}
	if !info.Mode().IsRegular() {
		return nil, false, "WORKFLOW_TARGET_INVALID", "workflow target must be a regular file"
	}
	content, err := os.ReadFile(absolutePath)
	if err != nil {
		return nil, false, "WORKFLOW_PREVIEW_FAILED", "existing workflow target could not be read safely"
	}
	return content, true, "", ""
}

func validateIntegrationDoctorWorkflowParent(root, target string) (string, string) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "WORKFLOW_TARGET_INVALID", "repository root could not be resolved physically"
	}
	existingParent, err := nearestExistingIntegrationDoctorWorkflowParent(filepath.Dir(target))
	if err != nil {
		return "WORKFLOW_TARGET_INVALID", "workflow target parent could not be inspected"
	}
	resolvedParent, err := filepath.EvalSymlinks(existingParent)
	if err != nil {
		return "WORKFLOW_TARGET_INVALID", "workflow target parent could not be resolved physically"
	}
	if !integrationDoctorPathWithinRoot(resolvedRoot, resolvedParent) {
		return "WORKFLOW_TARGET_SYMLINK_ESCAPE", "workflow target parent resolves outside the repository root"
	}
	return "", ""
}

func nearestExistingIntegrationDoctorWorkflowParent(start string) (string, error) {
	for current := start; ; current = filepath.Dir(current) {
		info, err := os.Stat(current)
		if err == nil {
			if !info.IsDir() {
				return "", fmt.Errorf("workflow output parent %s is not a directory", current)
			}
			return current, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		next := filepath.Dir(current)
		if next == current {
			return "", fmt.Errorf("workflow output parent does not exist")
		}
	}
}

func integrationDoctorPathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative))
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
