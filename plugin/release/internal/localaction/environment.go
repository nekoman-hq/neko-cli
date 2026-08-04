package localaction

import "gopkg.in/yaml.v3"

// environmentEntry is one ordered `env` binding of a step.
type environmentEntry struct {
	value *yaml.Node
	name  string
}

func environmentEntries(step *yaml.Node) []environmentEntry {
	environment := mappingValue(step, "env")
	if environment == nil || environment.Kind != yaml.MappingNode {
		return nil
	}
	entries := make([]environmentEntry, 0, len(environment.Content)/2)
	for index := 0; index+1 < len(environment.Content); index += 2 {
		entries = append(entries, environmentEntry{
			value: environment.Content[index+1],
			name:  environment.Content[index].Value,
		})
	}
	return entries
}

// mergeEnvironment applies GitHub's composite-action environment inheritance:
// the invoking step's bindings are available to every inner step, and an inner
// binding of the same name wins. Inherited order is preserved so expansion
// stays deterministic.
func mergeEnvironment(inherited, declared []environmentEntry) []environmentEntry {
	merged := make([]environmentEntry, 0, len(inherited)+len(declared))
	merged = append(merged, inherited...)
	for _, entry := range declared {
		overridden := false
		for index := range merged {
			if merged[index].name == entry.name {
				merged[index] = entry
				overridden = true
				break
			}
		}
		if !overridden {
			merged = append(merged, entry)
		}
	}
	return merged
}

func stepWithEnvironment(step *yaml.Node, environment []environmentEntry) *yaml.Node {
	effective := cloneNode(step)
	if len(environment) == 0 {
		return effective
	}
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, entry := range environment {
		node.Content = append(
			node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: entry.name},
			cloneNode(entry.value),
		)
	}
	replaceMappingValue(effective, "env", node)
	return effective
}
