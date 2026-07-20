package jreleaser

import (
	"fmt"
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
	"gopkg.in/yaml.v3"
)

// RewriteProjectVersion updates only the project.version line while preserving
// the surrounding JReleaser YAML bytes.
func RewriteProjectVersion(content []byte, nextVersion string) ([]byte, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("parse %s: %w", releasetool.JReleaserConfigFile, err)
	}
	versionNode, err := findYAMLPath(&document, "project", "version")
	if err != nil {
		return nil, fmt.Errorf("locate project.version in %s: %w", releasetool.JReleaserConfigFile, err)
	}
	lines := strings.SplitAfter(string(content), "\n")
	if versionNode.Line < 1 || versionNode.Line > len(lines) {
		return nil, fmt.Errorf("project.version line %d is outside %s", versionNode.Line, releasetool.JReleaserConfigFile)
	}
	line := lines[versionNode.Line-1]
	prefixIndex := strings.Index(line, "version:")
	if prefixIndex < 0 {
		return nil, fmt.Errorf("project.version line does not contain version key")
	}
	lineEnding := ""
	if strings.HasSuffix(line, "\n") {
		lineEnding = "\n"
		line = strings.TrimSuffix(line, "\n")
	}
	if strings.HasSuffix(line, "\r") {
		lineEnding = "\r" + lineEnding
		line = strings.TrimSuffix(line, "\r")
	}
	prefix := line[:prefixIndex+len("version:")]
	lines[versionNode.Line-1] = prefix + " " + nextVersion + lineEnding
	return []byte(strings.Join(lines, "")), nil
}

func findYAMLPath(document *yaml.Node, path ...string) (*yaml.Node, error) {
	if document.Kind != yaml.DocumentNode || len(document.Content) == 0 {
		return nil, fmt.Errorf("document is empty")
	}
	node := document.Content[0]
	for _, part := range path {
		if node.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s is not a mapping", part)
		}
		var next *yaml.Node
		for index := 0; index+1 < len(node.Content); index += 2 {
			if node.Content[index].Value == part {
				next = node.Content[index+1]
				break
			}
		}
		if next == nil {
			return nil, fmt.Errorf("%s not found", part)
		}
		node = next
	}
	return node, nil
}
