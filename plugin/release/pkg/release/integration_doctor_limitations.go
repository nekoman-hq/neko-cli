package release

import (
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var integrationDoctorRepositoryVariablePattern = regexp.MustCompile(`\$\{\{\s*vars\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

func inspectIntegrationDoctorBoundaryLimitations(
	workflowPath string,
	root *yaml.Node,
) ([]integrationDoctorVerification, []integrationDoctorDiagnostic) {
	facts := make([]integrationDoctorVerification, 0, 3)
	diagnostics := make([]integrationDoctorDiagnostic, 0, 3)
	if workflowPath != "" && root != nil && root.Kind == yaml.MappingNode {
		fact := newIntegrationDoctorVerification(
			"remote_workflow_identity",
			integrationDoctorUnverifiable,
			"The repository-confined local workflow exists and was structurally inspected; remote default-branch presence, content identity, and enabled state require remote access.",
			workflowPath,
			"",
			workflowPath,
		)
		fact.LimitationClass = integrationDoctorRemoteLimitation
		facts = append(facts, fact)
		diagnostics = append(diagnostics, newIntegrationDoctorWorkflowDiagnostic(
			integrationDoctorNotVerifiable,
			workflowPath,
			"",
			"REMOTE_WORKFLOW_NOT_VERIFIABLE",
			"The local workflow was inspected; its presence, content identity, and enabled state on the remote default branch were not checked offline.",
			"Compare the committed remote default-branch workflow during a separately authorized remote verification.",
		))
	}

	variables := integrationDoctorRepositoryVariables(root)
	if len(variables) > 0 {
		fact := newIntegrationDoctorVerification(
			"repository_variable_values",
			integrationDoctorUnverifiable,
			"Recognized repository-variable references and their local pinning semantics were identified: "+strings.Join(variables, ", ")+"; remote existence and values were not queried.",
			workflowPath,
			"",
			workflowPath,
		)
		fact.LimitationClass = integrationDoctorRemoteLimitation
		facts = append(facts, fact)
		diagnostics = append(diagnostics, newIntegrationDoctorWorkflowDiagnostic(
			integrationDoctorNotVerifiable,
			workflowPath,
			"",
			"REPOSITORY_VARIABLES_NOT_VERIFIABLE",
			"Recognized version-pin variable references and semantic requirements were locally identified; their remote existence and actual values remain unknown offline.",
			"Confirm the identified repository variables exist remotely and resolve to supported pinned versions.",
		))
	}

	if integrationDoctorHasDispatchContract(root) {
		fact := newIntegrationDoctorVerification(
			"dispatch_authorization",
			integrationDoctorUnverifiable,
			"The local workflow_dispatch trigger and canonical input structure were inspected; exact authorization requires a real remote authorization decision.",
			workflowPath,
			"",
			workflowPath,
		)
		fact.LimitationClass = integrationDoctorMutationRequiredLimitation
		facts = append(facts, fact)
		diagnostics = append(diagnostics, newIntegrationDoctorWorkflowDiagnostic(
			integrationDoctorNotVerifiable,
			workflowPath,
			"",
			"REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE",
			"Local dispatch prerequisites were structurally inspected; only a real remote authorization decision can prove the dispatch identity's exact permission.",
			"Verify authorization in a separately approved remote operation; the local Doctor does not dispatch workflows.",
		))
	}
	return facts, diagnostics
}

func integrationDoctorRepositoryVariables(root *yaml.Node) []string {
	if root == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var walk func(*yaml.Node)
	walk = func(node *yaml.Node) {
		if node == nil {
			return
		}
		if node.Kind == yaml.ScalarNode {
			for _, match := range integrationDoctorRepositoryVariablePattern.FindAllStringSubmatch(node.Value, -1) {
				seen[match[1]] = struct{}{}
			}
		}
		for _, child := range node.Content {
			walk(child)
		}
	}
	walk(root)
	variables := make([]string, 0, len(seen))
	for variable := range seen {
		variables = append(variables, variable)
	}
	sort.Strings(variables)
	return variables
}

func integrationDoctorHasDispatchContract(root *yaml.Node) bool {
	on := workflowMappingValue(root, "on")
	dispatch := workflowMappingValue(on, "workflow_dispatch")
	inputs := workflowMappingValue(dispatch, "inputs")
	if dispatch == nil || inputs == nil {
		return false
	}
	for _, definition := range canonicalWorkflowDispatchInputContract() {
		if workflowMappingValue(inputs, definition.Name) == nil {
			return false
		}
	}
	return true
}
