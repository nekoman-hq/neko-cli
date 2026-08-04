package localaction

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// definition is one parsed repository-local action file.
type definition struct {
	path      string
	inputs    map[string]string
	steps     []*yaml.Node
	composite bool
}

// actionDefinitionFiles lists the supported repository-local action file names
// in the order GitHub resolves them.
var actionDefinitionFiles = []string{"action.yml", "action.yaml"}

// readActionDefinition reads one repository-local action definition without
// leaving the repository root. It returns a failure code instead of an error
// so workflow inspection can report an explicit diagnostic.
func readActionDefinition(repositoryRoot, directory string) (definition, string) {
	absoluteRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return definition{}, FailureReferenceInvalid
	}
	resolvedRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return definition{}, FailureReferenceInvalid
	}
	for _, name := range actionDefinitionFiles {
		relativePath := path.Join(directory, name)
		absolutePath := filepath.Join(absoluteRoot, filepath.FromSlash(relativePath))
		info, statErr := os.Stat(absolutePath)
		if statErr != nil || !info.Mode().IsRegular() {
			continue
		}
		resolvedPath, resolveErr := filepath.EvalSymlinks(absolutePath)
		if resolveErr != nil || !pathWithinRoot(resolvedRoot, resolvedPath) {
			return definition{}, FailurePathEscape
		}
		content, readErr := os.ReadFile(absolutePath)
		if readErr != nil {
			return definition{}, FailureDefinitionInvalid
		}
		return parseActionDefinition(relativePath, content)
	}
	return definition{}, FailureMissing
}

func parseActionDefinition(relativePath string, content []byte) (definition, string) {
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return definition{}, FailureDefinitionInvalid
	}
	root := documentRoot(&document)
	runs := mappingValue(root, "runs")
	if root == nil || root.Kind != yaml.MappingNode || runs == nil || runs.Kind != yaml.MappingNode {
		return definition{}, FailureDefinitionInvalid
	}
	parsed := definition{path: relativePath, inputs: declaredInputDefaults(mappingValue(root, "inputs"))}
	if scalarValue(mappingValue(runs, "using")) != "composite" {
		return parsed, ""
	}
	steps := mappingValue(runs, "steps")
	if steps == nil || steps.Kind != yaml.SequenceNode {
		return definition{}, FailureDefinitionInvalid
	}
	parsed.composite = true
	parsed.steps = steps.Content
	return parsed, ""
}

func declaredInputDefaults(inputs *yaml.Node) map[string]string {
	defaults := make(map[string]string)
	if inputs == nil || inputs.Kind != yaml.MappingNode {
		return defaults
	}
	for index := 0; index+1 < len(inputs.Content); index += 2 {
		defaults[inputs.Content[index].Value] = scalarValue(mappingValue(inputs.Content[index+1], "default"))
	}
	return defaults
}

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}
