package doctor

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var integrationDoctorSecretReferencePattern = regexp.MustCompile(`\$\{\{\s*secrets\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

type integrationDoctorCredentialKind string

const (
	integrationDoctorBuiltInCredential integrationDoctorCredentialKind = "github_token"
	integrationDoctorCustomCredential  integrationDoctorCredentialKind = "custom_secret"
)

//nolint:govet // Logical workflow location order keeps test evidence readable.
type integrationDoctorCredentialReference struct {
	JobID       string
	StepName    string
	Name        string
	Environment string
	Kind        integrationDoctorCredentialKind
	Publication bool
}

func inspectIntegrationDoctorCredentialWiring(
	workflowPath string,
	root *yaml.Node,
	jobs []integrationDoctorWorkflowJob,
) (integrationDoctorVerification, []integrationDoctorDiagnostic) {
	fact := newIntegrationDoctorVerification(
		"credential_wiring", integrationDoctorVerified, "", workflowPath, "", workflowPath,
	)
	references := integrationDoctorCredentialReferences(root, jobs)
	diagnostics := make([]integrationDoctorDiagnostic, 0)
	add := func(code, message, remediation string) {
		diagnostics = append(diagnostics, newIntegrationDoctorWorkflowDiagnostic(
			integrationDoctorError, workflowPath, "", code, message, remediation,
		))
	}
	if len(references) == 0 {
		if integrationDoctorJobsPublishRepositoryContents(jobs) {
			fact.State = integrationDoctorMissing
			fact.Evidence = "No credential reference was found for the recognized publication work."
			add("PUBLICATION_CREDENTIAL_MISSING", "Recognized publication work has no explicit workflow credential reference.", "Pass a credential only to the publication step and grant its job the matching least-privilege permission.")
			return fact, diagnostics
		}
		fact.State = integrationDoctorUnsupported
		fact.LimitationClass = integrationDoctorRuntimeLimitation
		fact.Evidence = "The local workflow uses an unsupported publication shape, so credential wiring could not be classified."
		diagnostics = append(diagnostics, integrationDoctorCredentialLimitation(workflowPath))
		return fact, diagnostics
	}

	builtIn := 0
	custom := 0
	jobsByID := make(map[string]integrationDoctorWorkflowJob, len(jobs))
	for _, job := range jobs {
		jobsByID[job.id] = job
	}
	for _, reference := range references {
		if reference.Kind == integrationDoctorBuiltInCredential {
			builtIn++
		} else {
			custom++
		}
		if !reference.Publication {
			add("CREDENTIAL_SCOPE_INVALID", fmt.Sprintf("Credential reference %q is available outside a recognized publication step in job %q.", reference.Name, reference.JobID), "Keep validation and build work credential-free and bind credentials only to their publication steps.")
			continue
		}
		job := jobsByID[reference.JobID]
		if !integrationDoctorContentsWriteEffective(root, job) {
			add("CREDENTIAL_PERMISSION_MISMATCH", fmt.Sprintf("Publication credential %q in job %q lacks an effective contents: write permission.", reference.Name, reference.JobID), "Grant contents: write only to the GitHub Release publication job.")
		}
		if integrationDoctorCredentialExposedInJob(job, reference) {
			add("CREDENTIAL_EXPOSURE_RISK", fmt.Sprintf("Credential reference %q in step %q can enter command output or a workflow output.", reference.Name, reference.StepName), "Do not echo credentials or write them to GITHUB_OUTPUT, summaries, diagnostics, or generated artifacts.")
		}
	}
	if integrationDoctorDiagnosticsContainErrors(diagnostics) {
		fact.State = integrationDoctorMismatch
		fact.Evidence = "One or more local credential scope, permission, or output-safety contracts conflict."
		return fact, diagnostics
	}
	fact.Evidence = fmt.Sprintf("Locally classified %d built-in GitHub token and %d custom secret reference(s); all are step-scoped to recognized publication work with compatible permissions and no visible output path.", builtIn, custom)
	diagnostics = append(diagnostics, integrationDoctorCredentialLimitation(workflowPath))
	return fact, diagnostics
}

func integrationDoctorJobsPublishRepositoryContents(jobs []integrationDoctorWorkflowJob) bool {
	for _, job := range jobs {
		if integrationDoctorJobPublishesRepositoryContents(job) {
			return true
		}
	}
	return false
}

func integrationDoctorCredentialReferences(
	root *yaml.Node,
	jobs []integrationDoctorWorkflowJob,
) []integrationDoctorCredentialReference {
	references := make([]integrationDoctorCredentialReference, 0)
	seen := make(map[string]struct{})
	for _, job := range jobs {
		jobPublication := integrationDoctorJobPublishesRepositoryContents(job)
		for _, reference := range integrationDoctorCredentialReferencesInNode(job.env) {
			reference.JobID = job.id
			reference.Publication = jobPublication && integrationDoctorEveryJobStepPublishes(job)
			integrationDoctorAppendCredentialReference(&references, seen, reference)
		}
		integrationDoctorAppendJobCredentialReferences(&references, seen, job)
	}
	for _, reference := range integrationDoctorCredentialReferencesInNode(workflowMappingValue(root, "env")) {
		integrationDoctorAppendCredentialReference(&references, seen, reference)
	}
	sort.Slice(references, func(left, right int) bool {
		leftKey := references[left].JobID + "\x00" + references[left].StepName + "\x00" + references[left].Name + "\x00" + references[left].Environment
		rightKey := references[right].JobID + "\x00" + references[right].StepName + "\x00" + references[right].Name + "\x00" + references[right].Environment
		return leftKey < rightKey
	})
	return references
}

// integrationDoctorAppendJobCredentialReferences classifies the credentials a
// job declares. A credential written on a repository-local action invocation
// belongs to that invocation, not to each expanded inner step, and is
// publication-scoped when the invoked action performs the publication.
func integrationDoctorAppendJobCredentialReferences(
	references *[]integrationDoctorCredentialReference,
	seen map[string]struct{},
	job integrationDoctorWorkflowJob,
) {
	inspected := make(map[*yaml.Node]struct{})
	for _, step := range job.steps {
		if invocation := step.action.CallerNode; invocation != nil {
			if _, done := inspected[invocation]; !done {
				inspected[invocation] = struct{}{}
				publication := integrationDoctorLocalActionPublishes(job, invocation)
				for _, reference := range integrationDoctorCredentialReferencesInNode(invocation) {
					reference.JobID = job.id
					reference.StepName = step.action.CallerName
					reference.Publication = publication
					integrationDoctorAppendCredentialReference(references, seen, reference)
				}
			}
		}
		publication := integrationDoctorStepPublishes(step)
		for _, reference := range integrationDoctorCredentialReferencesInNode(step.declared) {
			reference.JobID = job.id
			reference.StepName = step.name
			reference.Publication = publication
			integrationDoctorAppendCredentialReference(references, seen, reference)
		}
	}
}

// integrationDoctorLocalActionPublishes reports whether the effective steps of
// one repository-local action invocation perform publication work.
func integrationDoctorLocalActionPublishes(
	job integrationDoctorWorkflowJob,
	invocation *yaml.Node,
) bool {
	for _, step := range job.steps {
		if step.action.CallerNode == invocation && integrationDoctorStepPublishes(step) {
			return true
		}
	}
	return false
}

func integrationDoctorCredentialReferencesInNode(node *yaml.Node) []integrationDoctorCredentialReference {
	if node == nil {
		return nil
	}
	references := make([]integrationDoctorCredentialReference, 0)
	var walk func(*yaml.Node, string)
	walk = func(current *yaml.Node, environment string) {
		if current == nil {
			return
		}
		if current.Kind == yaml.ScalarNode {
			for _, match := range integrationDoctorSecretReferencePattern.FindAllStringSubmatch(current.Value, -1) {
				name := match[1]
				kind := integrationDoctorCustomCredential
				if name == "GITHUB_TOKEN" {
					kind = integrationDoctorBuiltInCredential
				}
				references = append(references, integrationDoctorCredentialReference{
					Name: name, Environment: environment, Kind: kind,
				})
			}
		}
		if current.Kind == yaml.MappingNode {
			for index := 0; index+1 < len(current.Content); index += 2 {
				key := current.Content[index].Value
				nextEnvironment := environment
				if current == node && workflowMappingValue(node, "env") == nil {
					nextEnvironment = key
				}
				walk(current.Content[index+1], nextEnvironment)
			}
			return
		}
		for _, child := range current.Content {
			walk(child, environment)
		}
	}
	if node.Kind == yaml.MappingNode {
		if env := workflowMappingValue(node, "env"); env != nil {
			for index := 0; index+1 < len(env.Content); index += 2 {
				walk(env.Content[index+1], env.Content[index].Value)
			}
		}
		for index := 0; index+1 < len(node.Content); index += 2 {
			if node.Content[index].Value != "env" {
				walk(node.Content[index+1], "")
			}
		}
	} else {
		walk(node, "")
	}
	return references
}

func integrationDoctorAppendCredentialReference(
	references *[]integrationDoctorCredentialReference,
	seen map[string]struct{},
	reference integrationDoctorCredentialReference,
) {
	key := reference.JobID + "\x00" + reference.StepName + "\x00" + reference.Name + "\x00" + reference.Environment
	if _, duplicate := seen[key]; duplicate {
		return
	}
	seen[key] = struct{}{}
	*references = append(*references, reference)
}

func integrationDoctorStepPublishes(step integrationDoctorWorkflowStep) bool {
	return integrationDoctorGoReleaserStepPublishes(step) ||
		integrationDoctorRunPublishesGitHubRelease(step.run) ||
		strings.Contains(step.run, ".github/scripts/publish-plugin-index.sh")
}

func integrationDoctorEveryJobStepPublishes(job integrationDoctorWorkflowJob) bool {
	return len(job.steps) > 0 && len(job.steps) == 1 && integrationDoctorStepPublishes(job.steps[0])
}

func integrationDoctorContentsWriteEffective(root *yaml.Node, job integrationDoctorWorkflowJob) bool {
	node := job.permissions
	if node == nil {
		node = workflowMappingValue(root, "permissions")
	}
	permissions := parseIntegrationDoctorPermissions(node)
	return permissions.understood && (permissions.writeAll || permissions.scopes["contents"] == "write")
}

// integrationDoctorCredentialExposedInJob reports whether any effective step
// that the credential reference reaches can print it or write it to a workflow
// output. A credential declared on a local action invocation reaches every
// step that action expands to.
func integrationDoctorCredentialExposedInJob(
	job integrationDoctorWorkflowJob,
	reference integrationDoctorCredentialReference,
) bool {
	for _, step := range job.steps {
		if step.name != reference.StepName && step.action.CallerName != reference.StepName {
			continue
		}
		if integrationDoctorCredentialExposed(step, reference) {
			return true
		}
	}
	return false
}

func integrationDoctorCredentialExposed(
	step integrationDoctorWorkflowStep,
	reference integrationDoctorCredentialReference,
) bool {
	for _, line := range strings.Split(step.run, "\n") {
		trimmed := strings.TrimSpace(line)
		writesOutput := strings.Contains(trimmed, "GITHUB_OUTPUT")
		prints := strings.HasPrefix(trimmed, "echo ") || strings.HasPrefix(trimmed, "printf ") || writesOutput
		if !prints {
			continue
		}
		if strings.Contains(trimmed, "secrets."+reference.Name) ||
			(reference.Environment != "" && (strings.Contains(trimmed, "$"+reference.Environment) || strings.Contains(trimmed, "${"+reference.Environment+"}"))) {
			return true
		}
	}
	return false
}

func integrationDoctorCredentialLimitation(workflowPath string) integrationDoctorDiagnostic {
	return newIntegrationDoctorWorkflowDiagnostic(
		integrationDoctorNotVerifiable,
		workflowPath,
		"",
		"PUBLICATION_CREDENTIALS_NOT_VERIFIABLE",
		"Credential names, step/job scope, publication-only use, permission compatibility, and visible output paths were locally verified; runtime issuance, value validity, expiry, remote authorization, and external service acceptance remain unknown.",
		"Confirm credential issuance and authorization during a controlled publication run without exposing credential values.",
	)
}
