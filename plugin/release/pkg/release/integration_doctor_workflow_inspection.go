package release

import (
	"fmt"
	"sort"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"gopkg.in/yaml.v3"
)

const generatedConsumerPlaceholder = "Replace this generated step with consumer-owned build and publication commands."

type integrationDoctorWorkflowStep struct {
	node *yaml.Node
	name string
	id   string
	uses string
	run  string
}

//nolint:govet // Logical job order keeps YAML ownership readable.
type integrationDoctorWorkflowJob struct {
	id          string
	node        *yaml.Node
	steps       []integrationDoctorWorkflowStep
	permissions *yaml.Node
	env         *yaml.Node
}

func inspectIntegrationDoctorWorkflow(
	repositoryRoot string,
	path string,
	units []releaseconfig.ReleaseUnit,
	repositoryUnits []releaseconfig.ReleaseUnit,
	snapshot integrationDoctorWorkflowSnapshot,
	files integrationDoctorRepositoryFileReader,
	repositoryIdentity integrationDoctorRepositoryIdentity,
	repositoryIdentityErr error,
) (integrationDoctorWorkflow, []integrationDoctorVerification, []integrationDoctorDiagnostic) {
	unitIDs := make([]string, 0, len(units))
	for _, unit := range units {
		unitIDs = append(unitIDs, unit.ID)
	}
	fact := integrationDoctorWorkflow{Path: path, Units: unitIDs, Exists: snapshot.Exists}
	sort.Strings(fact.Units)
	verifications := make([]integrationDoctorVerification, 0)
	diagnostics := make([]integrationDoctorDiagnostic, 0)
	add := func(severity integrationDoctorSeverity, code, message, remediation string) {
		diagnostic := newIntegrationDoctorDiagnostic(severity, "workflow", code, message, remediation)
		diagnostic.Workflow = path
		diagnostics = append(diagnostics, diagnostic)
	}

	if snapshot.FailureCode != "" {
		code := "WORKFLOW_INSPECTION_FAILED"
		switch snapshot.FailureCode {
		case "WORKFLOW_YAML_INVALID":
			code = snapshot.FailureCode
		case "WORKFLOW_TARGET_SYMLINK_ESCAPE":
			code = "WORKFLOW_PATH_ESCAPE"
		case "WORKFLOW_TARGET_INVALID":
			code = "WORKFLOW_PATH_INVALID"
		}
		add(integrationDoctorError, code, "The configured workflow cannot be inspected safely: "+snapshot.FailureText+".", "Restore a readable regular YAML file at the configured canonical path without symlink escape.")
		fact.Classification = "malformed"
		return fact, verifications, diagnostics
	}
	if !snapshot.Exists {
		add(integrationDoctorError, "WORKFLOW_MISSING", "The configured GitHub Actions workflow file does not exist.", "Create it with `neko release github-workflow-init`, then replace the generated consumer placeholder.")
		fact.Classification = "missing"
		return fact, verifications, diagnostics
	}
	root := workflowDocumentRoot(snapshot.Document)
	if root == nil || root.Kind != yaml.MappingNode {
		add(integrationDoctorError, "WORKFLOW_YAML_INVALID", "The workflow document root is not a YAML mapping.", "Use a structurally valid GitHub Actions workflow mapping.")
		fact.Classification = "malformed"
		return fact, verifications, diagnostics
	}

	inspectIntegrationDoctorDispatch(root, add)
	inspectIntegrationDoctorPermissions(root, add)
	inspectIntegrationDoctorConcurrency(root, add)
	jobs := integrationDoctorWorkflowJobs(root)
	inspectIntegrationDoctorJob(jobs, add)
	if !integrationDoctorDiagnosticsContainErrors(diagnostics) {
		localFacts, localDiagnostics := inspectIntegrationDoctorConsumerReleaseStructure(
			repositoryRoot,
			path,
			units,
			root,
			jobs,
			files,
		)
		verifications = append(verifications, localFacts...)
		diagnostics = append(diagnostics, localDiagnostics...)
	}
	installationFact, installationDiagnostics := inspectIntegrationDoctorInstallation(
		repositoryRoot,
		path,
		repositoryUnits,
		jobs,
		files,
		repositoryIdentity,
		repositoryIdentityErr,
	)
	verifications = append(verifications, installationFact)
	diagnostics = append(diagnostics, installationDiagnostics...)
	credentialFact, credentialDiagnostics := inspectIntegrationDoctorCredentialWiring(path, root, jobs)
	verifications = append(verifications, credentialFact)
	diagnostics = append(diagnostics, credentialDiagnostics...)
	publicationFact, publicationDiagnostics := inspectIntegrationDoctorPublication(
		repositoryRoot,
		path,
		units,
		root,
		jobs,
		files,
		repositoryIdentity,
		repositoryIdentityErr,
	)
	verifications = append(verifications, publicationFact)
	diagnostics = append(diagnostics, publicationDiagnostics...)
	appendIntegrationDoctorRemoteLimits(add)

	fact.Classification = integrationDoctorWorkflowClassification(snapshot.Canonical, diagnostics)
	return fact, verifications, diagnostics
}

func inspectIntegrationDoctorDispatch(
	root *yaml.Node,
	add func(integrationDoctorSeverity, string, string, string),
) {
	on := workflowMappingValue(root, "on")
	dispatch := workflowMappingValue(on, "workflow_dispatch")
	if dispatch == nil {
		add(integrationDoctorError, "WORKFLOW_DISPATCH_MISSING", "The workflow does not expose workflow_dispatch.", "Add workflow_dispatch with the canonical Release V2 inputs.")
	} else {
		inputs := workflowMappingValue(dispatch, "inputs")
		definitions := map[string]workflowDispatchInputDefinition{}
		for _, definition := range canonicalWorkflowDispatchInputContract() {
			definitions[definition.Name] = definition
			input := workflowMappingValue(inputs, definition.Name)
			if input == nil {
				add(integrationDoctorError, "DISPATCH_INPUT_MISSING", fmt.Sprintf("Required workflow_dispatch input %q is missing.", definition.Name), "Declare all four canonical Release V2 dispatch inputs.")
				continue
			}
			required, requiredOK := workflowBool(workflowMappingValue(input, "required"))
			if !requiredOK || !required {
				add(integrationDoctorError, "DISPATCH_INPUT_NOT_REQUIRED", fmt.Sprintf("workflow_dispatch input %q is not required.", definition.Name), "Set required: true for the canonical input.")
			}
			if workflowScalar(workflowMappingValue(input, "type")) != "string" {
				add(integrationDoctorError, "DISPATCH_INPUT_TYPE_INVALID", fmt.Sprintf("workflow_dispatch input %q is not type string.", definition.Name), "Set type: string for the canonical input.")
			}
		}
		for _, name := range workflowMappingKeys(inputs) {
			if _, canonical := definitions[name]; canonical {
				continue
			}
			required, _ := workflowBool(workflowMappingValue(workflowMappingValue(inputs, name), "required"))
			if required {
				add(integrationDoctorError, "DISPATCH_EXTRA_REQUIRED_INPUT", fmt.Sprintf("Additional workflow_dispatch input %q is required and cannot be supplied by Neko.", name), "Make the consumer-owned input optional or remove it.")
			}
		}
	}
	for _, trigger := range workflowMappingKeys(on) {
		competing := trigger == "release" || trigger == "workflow_run" || trigger == "schedule" || trigger == "repository_dispatch"
		if trigger == "push" {
			push := workflowMappingValue(on, trigger)
			competing = workflowMappingValue(push, "tags") != nil || workflowMappingValue(push, "tags-ignore") != nil
		}
		if competing {
			add(integrationDoctorError, "COMPETING_RELEASE_TRIGGER", fmt.Sprintf("Automatic trigger %q can compete with Neko-owned release dispatch.", trigger), "Remove the release-capable automatic trigger from this publication workflow.")
		}
	}
}

func inspectIntegrationDoctorConcurrency(
	root *yaml.Node,
	add func(integrationDoctorSeverity, string, string, string),
) {
	concurrency := workflowMappingValue(root, "concurrency")
	if concurrency == nil {
		add(integrationDoctorWarning, "CONCURRENCY_MISSING", "The workflow has no explicit release concurrency policy.", "Add a group containing unit and tag identity with cancel-in-progress: false.")
		return
	}
	group := workflowScalar(workflowMappingValue(concurrency, "group"))
	if !strings.Contains(group, ".unit") || !strings.Contains(group, ".tag") {
		add(integrationDoctorWarning, "CONCURRENCY_IDENTITY_INCOMPLETE", "The concurrency group does not include both release unit and tag identity.", "Include the dispatched unit and tag in the concurrency group.")
	}
	cancel, ok := workflowBool(workflowMappingValue(concurrency, "cancel-in-progress"))
	if !ok || cancel {
		add(integrationDoctorWarning, "CONCURRENCY_CANCELLATION_UNSAFE", "The concurrency policy may cancel an in-flight release.", "Set cancel-in-progress: false.")
	}
}

func inspectIntegrationDoctorJob(
	jobs []integrationDoctorWorkflowJob,
	add func(integrationDoctorSeverity, string, string, string),
) {
	if len(jobs) == 0 {
		add(integrationDoctorError, "RELEASE_JOB_MISSING", "The workflow does not define a job with steps.", "Add a release job following the canonical checkout, install, validation, and consumer sequence.")
		return
	}
	validatorJobs := make([]int, 0)
	validatorSteps := make([]int, 0)
	for jobIndex, job := range jobs {
		for stepIndex, step := range job.steps {
			if strings.Contains(step.run, "neko release ci-validate-context") {
				validatorJobs = append(validatorJobs, jobIndex)
				validatorSteps = append(validatorSteps, stepIndex)
			}
		}
	}
	if len(validatorJobs) == 0 {
		add(integrationDoctorError, "CONTEXT_VALIDATOR_MISSING", "No step runs neko release ci-validate-context.", "Install the Release plugin and run the canonical context validator before consumer build commands.")
		inspectIntegrationDoctorReleaseSteps(jobs[0], -1, add)
		return
	}
	if len(validatorJobs) > 1 {
		add(integrationDoctorError, "CONTEXT_VALIDATOR_AMBIGUOUS", "The workflow contains multiple context-validator steps.", "Keep one authoritative context-validation step in the release job.")
	}
	inspectIntegrationDoctorReleaseSteps(jobs[validatorJobs[0]], validatorSteps[0], add)
}

func inspectIntegrationDoctorReleaseSteps(
	job integrationDoctorWorkflowJob,
	validatorIndex int,
	add func(integrationDoctorSeverity, string, string, string),
) {
	checkoutIndex := -1
	cliInstallIndex := -1
	pluginInstallIndex := -1
	for index, step := range job.steps {
		switch {
		case strings.HasPrefix(step.uses, "actions/checkout@") && checkoutIndex < 0:
			checkoutIndex = index
		case strings.Contains(step.run, "install.sh") && cliInstallIndex < 0:
			cliInstallIndex = index
		case strings.Contains(step.run, "neko plugin install release") && pluginInstallIndex < 0:
			pluginInstallIndex = index
		}
	}
	if checkoutIndex < 0 {
		add(integrationDoctorError, "CHECKOUT_MISSING", "The release job has no actions/checkout step.", "Checkout the exact dispatched release_sha before validation.")
	} else {
		inspectIntegrationDoctorCheckout(job.steps[checkoutIndex], add)
	}
	if cliInstallIndex < 0 {
		add(integrationDoctorError, "NEKO_INSTALL_MISSING", "The release job does not install the Neko CLI.", "Install a pinned Neko CLI version before context validation.")
	} else if !strings.Contains(job.steps[cliInstallIndex].run, "NEKO_VERSION") || strings.Contains(strings.ToLower(job.steps[cliInstallIndex].run), "latest") {
		add(integrationDoctorWarning, "NEKO_VERSION_UNPINNED", "The Neko CLI installation is not visibly version-pinned.", "Pin the Neko CLI with an explicit version or repository variable.")
	}
	if pluginInstallIndex < 0 {
		add(integrationDoctorError, "RELEASE_PLUGIN_INSTALL_MISSING", "The release job does not install the Neko Release plugin.", "Install a pinned Release plugin version before context validation.")
	} else if !strings.Contains(job.steps[pluginInstallIndex].run, "--version") || strings.Contains(strings.ToLower(job.steps[pluginInstallIndex].run), "latest") {
		add(integrationDoctorWarning, "RELEASE_PLUGIN_VERSION_UNPINNED", "The Release plugin installation is not visibly version-pinned.", "Install the Release plugin with --version and a fixed value or repository variable.")
	}
	if validatorIndex < 0 {
		return
	}
	if cliInstallIndex > validatorIndex || pluginInstallIndex > validatorIndex {
		add(integrationDoctorError, "INSTALL_ORDER_INVALID", "Required Neko installations occur after context validation.", "Install the Neko CLI and Release plugin before the validator step.")
	}
	validator := job.steps[validatorIndex]
	inspectIntegrationDoctorValidator(validator, job.steps[validatorIndex+1:], add)
}

func inspectIntegrationDoctorCheckout(
	step integrationDoctorWorkflowStep,
	add func(integrationDoctorSeverity, string, string, string),
) {
	with := workflowMappingValue(step.node, "with")
	ref := workflowScalar(workflowMappingValue(with, "ref"))
	if !strings.Contains(ref, "inputs.release_sha") && !strings.Contains(ref, "github.event.inputs.release_sha") {
		add(integrationDoctorError, "CHECKOUT_REF_INVALID", "Checkout does not use the dispatched release_sha.", "Set checkout ref to inputs.release_sha or github.event.inputs.release_sha.")
	}
	if workflowScalar(workflowMappingValue(with, "fetch-depth")) != "0" {
		add(integrationDoctorWarning, "CHECKOUT_HISTORY_INCOMPLETE", "Checkout does not request full history.", "Set fetch-depth: 0 for release history and tags.")
	}
	fetchTags, fetchTagsOK := workflowBool(workflowMappingValue(with, "fetch-tags"))
	if !fetchTagsOK || !fetchTags {
		add(integrationDoctorWarning, "CHECKOUT_TAGS_INCOMPLETE", "Checkout does not explicitly fetch tags.", "Set fetch-tags: true.")
	}
	persist, persistOK := workflowBool(workflowMappingValue(with, "persist-credentials"))
	if !persistOK || persist {
		add(integrationDoctorWarning, "CHECKOUT_CREDENTIALS_PERSISTED", "Checkout may persist GitHub credentials.", "Set persist-credentials: false and provide credentials only to the consumer step that needs them.")
	}
}

func inspectIntegrationDoctorValidator(
	validator integrationDoctorWorkflowStep,
	later []integrationDoctorWorkflowStep,
	add func(integrationDoctorSeverity, string, string, string),
) {
	wants := []struct{ flag, environment, input string }{
		{"--unit", "RELEASE_UNIT", workflowDispatchInputUnit},
		{"--version", "RELEASE_VERSION", workflowDispatchInputVersion},
		{"--tag", "RELEASE_TAG", workflowDispatchInputTag},
		{"--release-sha", "RELEASE_SHA", workflowDispatchInputReleaseSHA},
	}
	for _, want := range wants {
		if !integrationDoctorCommandFlagMatches(validator.run, want.flag, want.environment, want.input) {
			add(integrationDoctorError, "CONTEXT_VALIDATOR_FLAG_INVALID", fmt.Sprintf("Context validation does not pass canonical %s input.", want.flag), "Pass each canonical dispatch value to its matching validator flag.")
		}
	}
	if !integrationDoctorCommandFlagMatches(validator.run, "--output", "", "github") {
		add(integrationDoctorError, "CONTEXT_OUTPUT_MODE_INVALID", "Context validation does not select GitHub output mode.", "Pass --output github to the validator.")
	}
	if !integrationDoctorCommandFlagMatches(validator.run, "--github-output-file", "GITHUB_OUTPUT", "") {
		add(integrationDoctorError, "CONTEXT_OUTPUT_FILE_INVALID", "Context validation does not target the GitHub output file.", "Pass --github-output-file \"$GITHUB_OUTPUT\".")
	}
	consumesOutputs := false
	for _, step := range later {
		text := workflowNodeText(step.node)
		if strings.Contains(text, "steps.") && strings.Contains(text, ".outputs.") {
			consumesOutputs = true
		}
	}
	if consumesOutputs && strings.TrimSpace(validator.id) == "" {
		add(integrationDoctorError, "CONTEXT_STEP_ID_MISSING", "Later steps consume validator outputs, but the validator has no stable step id.", "Assign a stable id to the validator and reference that id from consumer steps.")
	}
	if len(later) == 0 {
		add(integrationDoctorError, "CONSUMER_INTEGRATION_MISSING", "No consumer-owned build or publication step follows validation.", "Add unit-scoped build and publication steps after successful context validation.")
		return
	}
	placeholder := false
	for _, step := range later {
		if strings.Contains(step.run, generatedConsumerPlaceholder) {
			placeholder = true
			break
		}
	}
	if placeholder {
		add(integrationDoctorError, "CONSUMER_PLACEHOLDER_PRESENT", "The canonical deliberately failing consumer placeholder is still present.", "Replace the placeholder with consumer-owned unit build and publication commands.")
		return
	}
}

func integrationDoctorCommandFlagMatches(command, flag, environment, input string) bool {
	normalized := strings.NewReplacer("${{ ", "${{", " }}", "}}", "'", "", "\"", "").Replace(command)
	fields := strings.Fields(normalized)
	for index, field := range fields {
		if field != flag || index+1 >= len(fields) {
			continue
		}
		value := strings.TrimSuffix(fields[index+1], "\\")
		if environment != "" && value == "$"+environment {
			return true
		}
		if input != "" && (value == input || value == "${{inputs."+input+"}}" || value == "${{github.event.inputs."+input+"}}") {
			return true
		}
	}
	return false
}

func integrationDoctorWorkflowJobs(root *yaml.Node) []integrationDoctorWorkflowJob {
	jobsNode := workflowMappingValue(root, "jobs")
	jobs := make([]integrationDoctorWorkflowJob, 0)
	if jobsNode == nil || jobsNode.Kind != yaml.MappingNode {
		return jobs
	}
	for index := 0; index+1 < len(jobsNode.Content); index += 2 {
		jobNode := jobsNode.Content[index+1]
		stepsNode := workflowMappingValue(jobNode, "steps")
		if stepsNode == nil || stepsNode.Kind != yaml.SequenceNode {
			continue
		}
		job := integrationDoctorWorkflowJob{
			id: jobsNode.Content[index].Value, node: jobNode,
			permissions: workflowMappingValue(jobNode, "permissions"),
			env:         workflowMappingValue(jobNode, "env"),
		}
		for _, stepNode := range stepsNode.Content {
			job.steps = append(job.steps, integrationDoctorWorkflowStep{
				node: stepNode,
				name: workflowScalar(workflowMappingValue(stepNode, "name")),
				id:   workflowScalar(workflowMappingValue(stepNode, "id")),
				uses: workflowScalar(workflowMappingValue(stepNode, "uses")),
				run:  workflowScalar(workflowMappingValue(stepNode, "run")),
			})
		}
		jobs = append(jobs, job)
	}
	return jobs
}

func appendIntegrationDoctorRemoteLimits(add func(integrationDoctorSeverity, string, string, string)) {
	limits := []struct{ code, message, remediation string }{
		{"REMOTE_WORKFLOW_NOT_VERIFIABLE", "The workflow's presence and content on the remote default branch are not locally verifiable.", "Confirm the inspected workflow is committed and available on the remote default branch."},
		{"REPOSITORY_VARIABLES_NOT_VERIFIABLE", "Repository variable values used for version pins are not locally verifiable.", "Confirm Neko CLI and Release plugin version variables exist and resolve to supported pinned versions."},
		{"REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE", "Remote workflow-dispatch permissions are not locally verifiable.", "Confirm the dispatch identity can run the configured workflow on the remote repository."},
	}
	for _, limit := range limits {
		add(integrationDoctorNotVerifiable, limit.code, limit.message, limit.remediation)
	}
}

func integrationDoctorWorkflowClassification(canonical bool, diagnostics []integrationDoctorDiagnostic) string {
	if canonical {
		return "canonical_scaffold"
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == integrationDoctorError && diagnostic.Code == "COMPETING_RELEASE_TRIGGER" {
			return "conflicting"
		}
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == integrationDoctorError {
			return "incomplete"
		}
	}
	return "configured_custom"
}
