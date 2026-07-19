package release

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
	"gopkg.in/yaml.v3"
)

func TestRepositoryDogfoodWorkflowCanonicalContract(t *testing.T) {
	for _, behavior := range repositoryWorkflowBehaviors() {
		t.Run(behavior.unit, func(t *testing.T) {
			content, root := readRepositoryWorkflow(t, behavior.path)
			assertRepositoryWorkflowDispatchContract(t, root)
			assertRepositoryWorkflowConcurrency(t, root)
			assertRepositoryWorkflowPermissions(t, root)
			assertRepositoryWorkflowCheckouts(t, root)
			assertRepositoryWorkflowValidation(t, root)
			if bytes.Contains(content, []byte(generatedConsumerPlaceholder)) {
				t.Fatal("consumer publication is still the generated failing placeholder")
			}
		})
	}
}

func TestRepositoryDogfoodDoctorAcceptance(t *testing.T) {
	root := repositoryInspectionRoot(t)
	t.Setenv("GITHUB_TOKEN", "repository-doctor-secret-sentinel")

	first := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil))
	second := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil))
	if first.Readiness != integrationDoctorReady {
		t.Fatalf("readiness = %q, want ready", first.Readiness)
	}
	if first.Summary.Errors != 0 || first.Summary.Warnings != 0 || first.Summary.Recommendations != 0 || first.Summary.NotVerifiable != 21 {
		t.Fatalf("Doctor summary = %#v, diagnostics=%#v", first.Summary, first.Diagnostics)
	}
	if !reflect.DeepEqual(first.Diagnostics, second.Diagnostics) {
		t.Fatal("repository Doctor diagnostics are not deterministic")
	}
	for _, diagnostic := range first.Diagnostics {
		if diagnostic.Severity != integrationDoctorNotVerifiable {
			t.Errorf("avoidable warning remained: %#v", diagnostic)
		}
	}
	for _, removed := range []string{"PERMISSIONS_BROAD", "PERMISSIONS_IMPLICIT", "CHECKOUT_TAGS_INCOMPLETE", "CONCURRENCY_IDENTITY_INCOMPLETE"} {
		assertIntegrationDoctorCodeAbsent(t, first.Diagnostics, removed)
	}
	for _, retained := range []string{
		"CONSUMER_BUILD_NOT_VERIFIABLE",
		"INSTALLATION_ARTIFACTS_NOT_VERIFIABLE",
		"PUBLICATION_CREDENTIALS_NOT_VERIFIABLE",
		"PUBLICATION_TARGET_NOT_VERIFIABLE",
		"REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE",
		"REMOTE_WORKFLOW_NOT_VERIFIABLE",
		"REPOSITORY_VARIABLES_NOT_VERIFIABLE",
	} {
		assertIntegrationDoctorCodes(t, first.Diagnostics, retained)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal Doctor result: %v", err)
	}
	if bytes.Contains(encoded, []byte("repository-doctor-secret-sentinel")) {
		t.Fatal("repository Doctor diagnostics leaked an ambient token")
	}
}

func TestRepositoryDogfoodDoctorUnitScopesHaveNoErrors(t *testing.T) {
	root := repositoryInspectionRoot(t)
	for _, behavior := range repositoryWorkflowBehaviors() {
		t.Run(behavior.unit, func(t *testing.T) {
			response := runIntegrationDoctor(t, root, map[string]any{"unit": behavior.unit})
			result := integrationDoctorResultFromResponse(t, response)
			if result.Readiness != integrationDoctorReady || response.ExitCode != 0 ||
				result.Summary.Errors != 0 || result.Summary.Warnings != 0 ||
				result.Summary.Recommendations != 0 || result.Summary.NotVerifiable != 7 {
				t.Fatalf("unit Doctor result = %#v", result)
			}
			if len(result.Units) != 1 || result.Units[0].ID != behavior.unit {
				t.Fatalf("inspected units = %#v", result.Units)
			}
			assertIntegrationDoctorCodeAbsent(t, result.Diagnostics, "PERMISSIONS_BROAD")
			assertIntegrationDoctorCodeAbsent(t, result.Diagnostics, "PERMISSIONS_IMPLICIT")
		})
	}
}

func TestRepositoryDogfoodPermissionDiagnosticsAcceptPublishingJobs(t *testing.T) {
	for _, behavior := range repositoryWorkflowBehaviors() {
		t.Run(behavior.unit, func(t *testing.T) {
			_, root := readRepositoryWorkflow(t, behavior.path)
			diagnostics := make([]string, 0)
			inspectIntegrationDoctorPermissions(root, func(_ integrationDoctorSeverity, code, _, _ string) {
				diagnostics = append(diagnostics, code)
			})
			if len(diagnostics) != 0 {
				t.Fatalf("%s permission diagnostics = %v, want none", behavior.path, diagnostics)
			}
		})
	}
}

func assertRepositoryWorkflowDispatchContract(t *testing.T, root *yaml.Node) {
	t.Helper()
	on := workflowMappingValue(root, "on")
	if keys := workflowMappingKeys(on); len(keys) != 1 || keys[0] != "workflow_dispatch" {
		t.Fatalf("triggers = %v, want only workflow_dispatch", keys)
	}
	inputs := workflowMappingValue(workflowMappingValue(on, "workflow_dispatch"), "inputs")
	canonical := canonicalWorkflowDispatchInputContract()
	if len(workflowMappingKeys(inputs)) != len(canonical) {
		t.Fatalf("dispatch inputs = %v", workflowMappingKeys(inputs))
	}
	for _, definition := range canonical {
		input := workflowMappingValue(inputs, definition.Name)
		required, ok := workflowBool(workflowMappingValue(input, "required"))
		if input == nil || !ok || !required || workflowScalar(workflowMappingValue(input, "type")) != "string" {
			t.Errorf("dispatch input %q is not a required string", definition.Name)
		}
	}
}

func assertRepositoryWorkflowConcurrency(t *testing.T, root *yaml.Node) {
	t.Helper()
	concurrency := workflowMappingValue(root, "concurrency")
	if got := workflowScalar(workflowMappingValue(concurrency, "group")); got != "release-${{ inputs.unit }}-${{ inputs.tag }}" {
		t.Errorf("concurrency group = %q", got)
	}
	cancel, ok := workflowBool(workflowMappingValue(concurrency, "cancel-in-progress"))
	if !ok || cancel {
		t.Errorf("cancel-in-progress = %q, want false", workflowScalar(workflowMappingValue(concurrency, "cancel-in-progress")))
	}
}

func assertRepositoryWorkflowPermissions(t *testing.T, root *yaml.Node) {
	t.Helper()
	permissions := workflowMappingValue(root, "permissions")
	if len(workflowMappingKeys(permissions)) != 1 || workflowScalar(workflowMappingValue(permissions, "contents")) != "read" {
		t.Errorf("workflow permissions must be only contents: read")
	}
	jobs := integrationDoctorWorkflowJobs(root)
	if len(jobs) != 2 {
		t.Fatalf("jobs = %d, want validate and publish", len(jobs))
	}
	for _, job := range jobs {
		text := workflowNodeText(job.permissions)
		switch job.id {
		case "validate":
			if job.permissions != nil {
				t.Errorf("validation job must inherit read-only permissions: %s", text)
			}
		case "publish":
			if len(workflowMappingKeys(job.permissions)) != 1 || workflowScalar(workflowMappingValue(job.permissions, "contents")) != "write" {
				t.Errorf("publication permissions must be only contents: write: %s", text)
			}
		default:
			t.Errorf("unexpected job %q", job.id)
		}
		if strings.Contains(text, "write-all") || strings.Contains(text, "id-token") {
			t.Errorf("job %q has unjustified permissions: %s", job.id, text)
		}
	}
}

func assertRepositoryWorkflowCheckouts(t *testing.T, root *yaml.Node) {
	t.Helper()
	for _, job := range integrationDoctorWorkflowJobs(root) {
		checkoutCount := 0
		for _, step := range job.steps {
			if !strings.HasPrefix(step.uses, "actions/checkout@") {
				continue
			}
			checkoutCount++
			if step.uses != "actions/checkout@v4" {
				t.Errorf("job %q checkout action = %q", job.id, step.uses)
			}
			with := workflowMappingValue(step.node, "with")
			ref := workflowScalar(workflowMappingValue(with, "ref"))
			if ref != "${{ inputs.release_sha }}" && ref != "${{ needs.validate.outputs.release_sha }}" {
				t.Errorf("job %q checkout ref = %q", job.id, ref)
			}
			if workflowScalar(workflowMappingValue(with, "fetch-depth")) != "0" {
				t.Errorf("job %q checkout is shallow", job.id)
			}
			fetchTags, fetchTagsOK := workflowBool(workflowMappingValue(with, "fetch-tags"))
			persist, persistOK := workflowBool(workflowMappingValue(with, "persist-credentials"))
			if !fetchTagsOK || !fetchTags || !persistOK || persist {
				t.Errorf("job %q checkout tags/credentials are unsafe", job.id)
			}
		}
		if checkoutCount != 1 {
			t.Errorf("job %q checkout count = %d, want 1", job.id, checkoutCount)
		}
	}
}

func assertRepositoryWorkflowValidation(t *testing.T, root *yaml.Node) {
	t.Helper()
	jobs := integrationDoctorWorkflowJobs(root)
	validatorCount := 0
	for _, job := range jobs {
		for index, step := range job.steps {
			if step.name == "Install pinned Neko CLI" {
				text := strings.ToLower(workflowNodeText(step.node))
				if !strings.Contains(text, "vars.neko_version") || !strings.Contains(step.run, "install.sh") || strings.Contains(text, "version: latest") {
					t.Error("Neko CLI installation is missing its repository-variable pin")
				}
			}
			if step.name == "Install pinned Neko Release Plugin" {
				text := strings.ToLower(workflowNodeText(step.node))
				if !strings.Contains(text, "vars.neko_release_plugin_version") || !strings.Contains(step.run, "--version") || strings.Contains(text, "version: latest") {
					t.Error("Release Plugin installation is missing its repository-variable pin")
				}
			}
			if !strings.Contains(step.run, "neko release ci-validate-context") {
				continue
			}
			validatorCount++
			if job.id != "validate" || step.id != "release-context" {
				t.Errorf("validator location/id = %q/%q", job.id, step.id)
			}
			before := workflowNodeText(job.node)
			installCLI := strings.Index(before, "Install pinned Neko CLI")
			installPlugin := strings.Index(before, "Install pinned Neko Release Plugin")
			validator := strings.Index(before, "Validate Neko release context")
			if installCLI < 0 || installPlugin <= installCLI || validator <= installPlugin {
				t.Errorf("job %q installation/validator order is invalid", job.id)
			}
			for _, fragment := range []string{
				"--unit \"$RELEASE_UNIT\"",
				"--version \"$RELEASE_VERSION\"",
				"--tag \"$RELEASE_TAG\"",
				"--release-sha \"$RELEASE_SHA\"",
				"--output github",
				"--github-output-file \"$GITHUB_OUTPUT\"",
			} {
				if !strings.Contains(step.run, fragment) {
					t.Errorf("validator is missing %q", fragment)
				}
			}
			if index+1 >= len(job.steps) || !strings.Contains(workflowNodeText(job.steps[index+1].node), "steps.release-context.outputs.") {
				t.Error("validated context is not consumed immediately after validation")
			}
		}
	}
	if validatorCount != 1 {
		t.Fatalf("context validator count = %d, want 1", validatorCount)
	}

	validate := workflowMappingValue(workflowMappingValue(root, "jobs"), "validate")
	outputs := workflowMappingValue(validate, "outputs")
	for _, name := range []string{"unit", "version", "tag", "release_sha"} {
		want := "${{ steps.release-context.outputs." + name + " }}"
		if got := workflowScalar(workflowMappingValue(outputs, name)); got != want {
			t.Errorf("validate output %s = %q, want %q", name, got, want)
		}
	}
	publish := workflowMappingValue(workflowMappingValue(root, "jobs"), "publish")
	if workflowScalar(workflowMappingValue(publish, "needs")) != "validate" {
		t.Errorf("publication job does not depend on validation")
	}
	publishText := workflowNodeText(publish)
	for _, name := range []string{"unit", "version", "tag", "release_sha"} {
		if !strings.Contains(publishText, "needs.validate.outputs."+name) {
			t.Errorf("publication does not consume validated %s output", name)
		}
	}
}

func repositoryInspectionRoot(t *testing.T) workspace.RepositoryRoot {
	t.Helper()
	root, err := workspace.ResolveInspectionRepositoryRoot(repositoryRootForSelfMigrationTest())
	if err != nil {
		t.Fatalf("resolve repository inspection root: %v", err)
	}
	return root
}
