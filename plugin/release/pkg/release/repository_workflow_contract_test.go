package release

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
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
			assertRepositoryWorkflowValidation(t, root, behavior.unit)
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
	if first.Summary.Errors != 0 || first.Summary.Warnings != 0 || first.Summary.Recommendations != 0 || first.Summary.NotVerifiable != 15 {
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
		"PUBLICATION_CREDENTIALS_NOT_VERIFIABLE",
		"PUBLICATION_TARGET_NOT_VERIFIABLE",
		"REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE",
		"REMOTE_WORKFLOW_NOT_VERIFIABLE",
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
				result.Summary.Recommendations != 0 || result.Summary.NotVerifiable != 5 {
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
	root := repositoryInspectionRoot(t)
	for _, behavior := range repositoryWorkflowBehaviors() {
		t.Run(behavior.unit, func(t *testing.T) {
			result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(
				t, root, map[string]any{"unit": behavior.unit},
			))
			assertIntegrationDoctorCodeAbsent(t, result.Diagnostics, "PERMISSIONS_BROAD")
			assertIntegrationDoctorCodeAbsent(t, result.Diagnostics, "PERMISSIONS_IMPLICIT")
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
	canonical := releaseworkflow.CanonicalDispatchInputContract()
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

func assertRepositoryWorkflowValidation(t *testing.T, root *yaml.Node, unit string) {
	t.Helper()
	jobs := integrationDoctorWorkflowJobs(root)
	validatorCount := 0
	for _, job := range jobs {
		for index, step := range job.steps {
			if !strings.Contains(step.run, "neko release ci-validate-context") {
				continue
			}
			validatorCount++
			if job.id != "validate" || step.action.CallerID != "release-context" {
				t.Errorf("validator location/invocation = %q/%q", job.id, step.action.CallerID)
			}
			if step.action.ActionPath != ".github/actions/validate-neko-release-context/action.yml" {
				t.Errorf("validator is not owned by the canonical local action: %q", step.action.ActionPath)
			}
			assertRepositorySelfReleaseValidationToolchain(t, job, unit)
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
			for name, input := range map[string]string{
				"RELEASE_UNIT":    "inputs.unit",
				"RELEASE_VERSION": "inputs.version",
				"RELEASE_TAG":     "inputs.tag",
				"RELEASE_SHA":     "inputs.release_sha",
			} {
				value := workflowScalar(workflowMappingValue(workflowMappingValue(step.node, "env"), name))
				if !strings.Contains(value, input) {
					t.Errorf("validator %s = %q, want the dispatched %s", name, value, input)
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
	for output, name := range map[string]string{
		"unit": "unit", "version": "version", "tag": "tag", "release_sha": "release-sha",
	} {
		got := workflowScalar(workflowMappingValue(outputs, output))
		if !strings.Contains(got, "steps.release-context.outputs") || !strings.Contains(got, name) {
			t.Errorf("validate output %s = %q, want the validated %q output", output, got, name)
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

// assertRepositorySelfReleaseValidationToolchain verifies that the inline guard
// rejects a mismatched release identity before any candidate code runs and that
// the exact-source toolchain the shared local action builds is wired into
// context validation.
func assertRepositorySelfReleaseValidationToolchain(t *testing.T, job integrationDoctorWorkflowJob, unit string) {
	t.Helper()
	identity := repositoryWorkflowStepIndex(job, func(step integrationDoctorWorkflowStep) bool {
		return step.name == "Validate immutable self-release identity"
	})
	setupGo := repositoryWorkflowStepIndex(job, func(step integrationDoctorWorkflowStep) bool {
		return strings.HasPrefix(step.uses, "actions/setup-go@")
	})
	toolchain := repositoryWorkflowStepIndex(job, func(step integrationDoctorWorkflowStep) bool {
		return step.name == "Build exact-source Neko validation toolchain"
	})
	validator := repositoryWorkflowStepIndex(job, func(step integrationDoctorWorkflowStep) bool {
		return strings.Contains(step.run, "neko release ci-validate-context")
	})
	if identity < 0 || setupGo <= identity || toolchain <= setupGo || validator <= toolchain {
		t.Fatalf("job %q identity/setup/toolchain/validator order is invalid", job.id)
	}
	if job.steps[identity].action.ActionPath != "" {
		t.Error("the immutable identity guard must stay inline in the workflow")
	}
	if job.steps[toolchain].action.ActionPath != ".github/actions/setup-source-neko-toolchain/action.yml" {
		t.Errorf("toolchain step source = %q", job.steps[toolchain].action.ActionPath)
	}

	tagPrefix := map[string]string{"cli": "v", "plugin-release": "plugin-release/v", "plugin-ui": "plugin-ui/v"}[unit]
	guard := repositoryWorkflowShellCommand(job.steps[identity].run)
	for _, required := range []string{
		`if [[ "$RELEASE_UNIT" != "` + unit + `" ]]`,
		`if [[ "$RELEASE_TAG" != "` + tagPrefix + `${RELEASE_VERSION}" ]]`,
		`head_sha="$(git rev-parse HEAD)"`,
		`tag_sha="$(git rev-list -n 1 "$RELEASE_TAG")"`,
		`'.units["` + unit + `"].version == $version'`,
	} {
		if !strings.Contains(guard, required) {
			t.Errorf("self-release identity guard is missing %q", required)
		}
	}
	if manifest := map[string]string{
		"plugin-release": "plugin/release/manifest.json", "plugin-ui": "plugin/ui/manifest.json",
	}[unit]; manifest != "" && !strings.Contains(guard, `'.version == $version' `+manifest) {
		t.Errorf("%s self-release identity omits %s version validation", unit, manifest)
	}

	build := repositoryWorkflowShellCommand(job.steps[toolchain].run)
	for _, required := range []string{
		`jq -er '.units.cli.version'`,
		`jq -er '.units["plugin-release"].version'`,
		"go build",
		"-trimpath",
		`-o "$NEKO_BIN_DIR/neko"`,
		`-o "$release_plugin_dir/plugin-release" ./plugin/release`,
		`cp plugin/release/manifest.json "$release_plugin_dir/manifest.json"`,
		`echo "$NEKO_BIN_DIR" >> "$GITHUB_PATH"`,
	} {
		if !strings.Contains(build, required) {
			t.Errorf("self-release validation toolchain is missing %q", required)
		}
	}
	if got := workflowScalar(workflowMappingValue(workflowMappingValue(job.steps[toolchain].node, "env"), "NEKO_PLUGIN_DIR")); got != "${{ runner.temp }}/neko/plugins" {
		t.Errorf("exact-source plugin directory = %q", got)
	}
	consumed := workflowScalar(workflowMappingValue(workflowMappingValue(job.steps[validator].node, "env"), "NEKO_PLUGIN_DIR"))
	if !strings.Contains(consumed, "steps."+job.steps[toolchain].action.CallerID+".outputs.neko-plugin-dir") {
		t.Errorf("context validation plugin directory = %q, want the exact-source toolchain output", consumed)
	}

	effective := integrationDoctorEffectiveJobText(job)
	for _, forbidden := range []string{
		"vars.NEKO_VERSION",
		"vars.NEKO_RELEASE_PLUGIN_VERSION",
		"install.sh",
		"neko plugin install release",
	} {
		if strings.Contains(effective, forbidden) {
			t.Errorf("self-release validation toolchain retains bootstrap dependency %q", forbidden)
		}
	}
}

func repositoryWorkflowStepIndex(
	job integrationDoctorWorkflowJob,
	matches func(integrationDoctorWorkflowStep) bool,
) int {
	for index, step := range job.steps {
		if matches(step) {
			return index
		}
	}
	return -1
}

// repositoryWorkflowShellCommand joins line continuations and collapses
// whitespace so contract fragments do not depend on shell formatting.
func repositoryWorkflowShellCommand(run string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(run, "\\\n", " ")), " ")
}

func repositoryInspectionRoot(t *testing.T) workspace.RepositoryRoot {
	t.Helper()
	root, err := workspace.ResolveInspectionRepositoryRoot(repositoryRootForSelfMigrationTest())
	if err != nil {
		t.Fatalf("resolve repository inspection root: %v", err)
	}
	return root
}
