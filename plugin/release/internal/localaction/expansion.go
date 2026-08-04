package localaction

import (
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

var actionInputPattern = regexp.MustCompile(`\$\{\{\s*inputs\.([A-Za-z_][A-Za-z0-9_-]*)\s*\}\}`)

// Origin records where one effective step was declared. Its zero value
// describes a step written directly in the workflow.
//
//nolint:govet // Logical provenance order keeps action evidence readable.
type Origin struct {
	// ActionPath is the repository-relative action file the step comes from.
	ActionPath string
	// ActionStep is the step name declared inside the action.
	ActionStep string
	// CallerName is the workflow step that invoked the action.
	CallerName string
	// CallerID is the workflow step id that invoked the action.
	CallerID string
	// CallerNode is the effective workflow step that invoked the action.
	CallerNode *yaml.Node
	// Inputs lists the sorted action input names supplied at the invocation.
	Inputs []string
}

// Expanded reports whether the step comes from a repository-local action.
func (origin Origin) Expanded() bool {
	return origin.ActionPath != ""
}

// Step is one effective workflow step in execution order.
//
//nolint:govet // Logical step order keeps effective and declared YAML together.
type Step struct {
	// Node is the effective step with inherited environment applied.
	Node *yaml.Node
	// Declared is the step exactly as written, before environment inheritance.
	Declared *yaml.Node
	// Origin describes the local action the step was expanded from.
	Origin Origin
	// Failure is set when a repository-local action reference could not be
	// expanded safely; the invoking step is then kept unexpanded.
	Failure string
}

// StepExpander turns declared workflow steps into effective steps.
type StepExpander interface {
	Expand(steps []*yaml.Node) []Step
	EffectiveSteps(steps []*yaml.Node) []*yaml.Node
}

// DeclaredSteps treats every declared workflow step as its own effective step.
type DeclaredSteps struct{}

// Expand returns the declared steps unchanged.
func (DeclaredSteps) Expand(steps []*yaml.Node) []Step {
	effective := make([]Step, 0, len(steps))
	for _, step := range steps {
		effective = append(effective, Step{Node: step, Declared: step})
	}
	return effective
}

// EffectiveSteps returns the declared step nodes unchanged.
func (DeclaredSteps) EffectiveSteps(steps []*yaml.Node) []*yaml.Node {
	return steps
}

// EffectiveSteps returns the effective step nodes of one workflow job for
// consumers that need the resolved YAML only.
func (actions RepositoryActions) EffectiveSteps(steps []*yaml.Node) []*yaml.Node {
	expanded := actions.Expand(steps)
	nodes := make([]*yaml.Node, 0, len(expanded))
	for _, step := range expanded {
		nodes = append(nodes, step.Node)
	}
	return nodes
}

// RepositoryActions expands repository-local composite actions found below one
// repository root.
type RepositoryActions struct {
	repositoryRoot string
}

// NewRepositoryActions returns an expander confined to repositoryRoot.
func NewRepositoryActions(repositoryRoot string) RepositoryActions {
	return RepositoryActions{repositoryRoot: repositoryRoot}
}

// Expand returns the effective steps of one workflow job in execution order.
func (actions RepositoryActions) Expand(steps []*yaml.Node) []Step {
	if actions.repositoryRoot == "" {
		return DeclaredSteps{}.Expand(steps)
	}
	expansion := &stepExpansion{repositoryRoot: actions.repositoryRoot, definitions: make(map[string]definitionLookup)}
	effective := make([]Step, 0, len(steps))
	for _, step := range steps {
		effective = append(effective, expansion.expand(step, Origin{}, nil, nil)...)
	}
	return effective
}

//nolint:govet // Logical lookup order keeps the definition before its failure.
type definitionLookup struct {
	definition definition
	failure    string
}

//nolint:govet // Logical expansion order keeps the root before its cache.
type stepExpansion struct {
	repositoryRoot string
	definitions    map[string]definitionLookup
}

func (expansion *stepExpansion) expand(
	declared *yaml.Node,
	origin Origin,
	inherited []environmentEntry,
	chain []string,
) []Step {
	environment := mergeEnvironment(inherited, environmentEntries(declared))
	effective := Step{Node: stepWithEnvironment(declared, environment), Declared: declared, Origin: origin}
	reference, local := localActionReference(scalarValue(mappingValue(declared, "uses")))
	if !local {
		return []Step{effective}
	}
	directory, valid := repositoryRelativeActionDirectory(reference)
	if !valid {
		effective.Failure = FailureReferenceInvalid
		return []Step{effective}
	}
	if expansionExceedsBounds(chain, directory) {
		effective.Failure = FailureRecursive
		return []Step{effective}
	}
	found := expansion.definition(directory)
	if found.failure != "" {
		effective.Failure = found.failure
		return []Step{effective}
	}
	if !found.definition.composite {
		return []Step{effective}
	}
	nested := make([]string, 0, len(chain)+1)
	nested = append(append(nested, chain...), directory)
	return expansion.expandComposite(found.definition, declared, effective, origin, environment, nested)
}

func (expansion *stepExpansion) expandComposite(
	action definition,
	declared *yaml.Node,
	invocation Step,
	origin Origin,
	environment []environmentEntry,
	chain []string,
) []Step {
	caller := origin
	if !origin.Expanded() {
		caller = Origin{
			CallerName: scalarValue(mappingValue(declared, "name")),
			CallerID:   scalarValue(mappingValue(declared, "id")),
			CallerNode: invocation.Node,
		}
	}
	inputs, supplied := resolveActionInputs(action, declared)
	steps := make([]Step, 0, len(action.steps))
	for _, inner := range action.steps {
		substituted := substituteActionInputs(inner, inputs)
		steps = append(steps, expansion.expand(substituted, Origin{
			ActionPath: action.path,
			ActionStep: scalarValue(mappingValue(substituted, "name")),
			CallerName: caller.CallerName,
			CallerID:   caller.CallerID,
			CallerNode: caller.CallerNode,
			Inputs:     supplied,
		}, environment, chain)...)
	}
	return steps
}

func (expansion *stepExpansion) definition(directory string) definitionLookup {
	if cached, found := expansion.definitions[directory]; found {
		return cached
	}
	parsed, failure := readActionDefinition(expansion.repositoryRoot, directory)
	lookup := definitionLookup{definition: parsed, failure: failure}
	expansion.definitions[directory] = lookup
	return lookup
}

func expansionExceedsBounds(chain []string, directory string) bool {
	if len(chain) >= MaxDepth {
		return true
	}
	for _, entry := range chain {
		if entry == directory {
			return true
		}
	}
	return false
}

func resolveActionInputs(action definition, invocation *yaml.Node) (map[string]string, []string) {
	inputs := make(map[string]string, len(action.inputs))
	for name, value := range action.inputs {
		inputs[name] = value
	}
	supplied := make([]string, 0, len(inputs))
	with := mappingValue(invocation, "with")
	if with != nil && with.Kind == yaml.MappingNode {
		for index := 0; index+1 < len(with.Content); index += 2 {
			name := with.Content[index].Value
			inputs[name] = scalarValue(with.Content[index+1])
			supplied = append(supplied, name)
		}
	}
	sort.Strings(supplied)
	return inputs, supplied
}

func substituteActionInputs(step *yaml.Node, inputs map[string]string) *yaml.Node {
	clone := cloneNode(step)
	var walk func(*yaml.Node)
	walk = func(current *yaml.Node) {
		if current == nil {
			return
		}
		if current.Kind == yaml.ScalarNode {
			current.Value = actionInputPattern.ReplaceAllStringFunc(current.Value, func(match string) string {
				value, declared := inputs[actionInputPattern.FindStringSubmatch(match)[1]]
				if !declared {
					return match
				}
				return value
			})
			return
		}
		for _, child := range current.Content {
			walk(child)
		}
	}
	walk(clone)
	return clone
}
