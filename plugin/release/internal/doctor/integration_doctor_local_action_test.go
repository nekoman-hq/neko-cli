package doctor

import (
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/localaction"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// TestIntegrationDoctorFindsCanonicalValidatorInsideLocalAction proves the
// canonical context validator is recognized exactly once per first-party
// workflow even though the command lives in a repository-local action.
func TestIntegrationDoctorFindsCanonicalValidatorInsideLocalAction(t *testing.T) {
	for _, behavior := range repositoryWorkflowBehaviors() {
		t.Run(behavior.unit, func(t *testing.T) {
			validators := integrationDoctorMatchingSteps(
				repositoryWorkflowJobsForTest(t, behavior.path),
				func(step integrationDoctorWorkflowStep) bool {
					return strings.Contains(step.run, "neko release ci-validate-context")
				},
			)
			if len(validators) != 1 {
				t.Fatalf("context validator count = %d, want 1", len(validators))
			}
			validator := validators[0]
			if validator.action.ActionPath != ".github/actions/validate-neko-release-context/action.yml" {
				t.Fatalf("validator source = %q", validator.action.ActionPath)
			}
			if validator.referenceID() != "release-context" {
				t.Fatalf("validator reference id = %q", validator.referenceID())
			}
			for _, flag := range []struct{ name, environment, input string }{
				{"--unit", "RELEASE_UNIT", "unit"},
				{"--version", "RELEASE_VERSION", "version"},
				{"--tag", "RELEASE_TAG", "tag"},
				{"--release-sha", "RELEASE_SHA", "release_sha"},
			} {
				if !integrationDoctorCommandFlagMatches(validator.run, flag.name, flag.environment, "") {
					t.Errorf("validator does not pass %s", flag.name)
				}
				if got := integrationDoctorWorkflowStepEnvironment(validator, flag.environment); !strings.Contains(got, "inputs."+flag.input) {
					t.Errorf("validator %s = %q, want the dispatched inputs.%s", flag.environment, got, flag.input)
				}
			}
		})
	}
}

// TestIntegrationDoctorVerifiesExactSourceToolchainInsideLocalAction proves the
// exact-source toolchain contract is verified from the shared local action.
func TestIntegrationDoctorVerifiesExactSourceToolchainInsideLocalAction(t *testing.T) {
	root := repositoryInspectionRoot(t)
	repository, err := releaseconfig.LoadReleaseRepository(root.Path())
	if err != nil {
		t.Fatalf("load release repository: %v", err)
	}
	for _, behavior := range repositoryWorkflowBehaviors() {
		t.Run(behavior.unit, func(t *testing.T) {
			jobs := repositoryWorkflowJobsForTest(t, behavior.path)
			toolchain, _, ok := integrationDoctorSourceToolchainBuildStep(jobs)
			if !ok || toolchain.action.ActionPath != ".github/actions/setup-source-neko-toolchain/action.yml" {
				t.Fatalf("exact-source toolchain step = %#v", toolchain.action)
			}
			fact, diagnostics := inspectIntegrationDoctorSourceValidationToolchain(
				root.Path(), behavior.path,
				[]releaseconfig.ReleaseUnit{integrationDoctorUnitForTest(t, repository, behavior.unit)},
				repository.Units, jobs, filesystemIntegrationDoctorRepositoryFileReader{},
			)
			if fact.State != integrationDoctorVerified || len(diagnostics) != 0 {
				t.Fatalf("exact-source fact = %#v diagnostics = %#v", fact, diagnostics)
			}
		})
	}
}

// TestIntegrationDoctorRecognizesPluginIndexOrderingAcrossLocalAction proves
// registry ordering is derived from the effective steps even though generation
// and publication live inside the publish-plugin-index action.
func TestIntegrationDoctorRecognizesPluginIndexOrderingAcrossLocalAction(t *testing.T) {
	root := repositoryInspectionRoot(t)
	repository, err := releaseconfig.LoadReleaseRepository(root.Path())
	if err != nil {
		t.Fatalf("load release repository: %v", err)
	}
	for _, behavior := range repositoryWorkflowBehaviors() {
		if !behavior.pluginRegistry {
			continue
		}
		t.Run(behavior.unit, func(t *testing.T) {
			jobs := repositoryWorkflowJobsForTest(t, behavior.path)
			releaseIndex := integrationDoctorEffectiveStepPosition(jobs, func(step integrationDoctorWorkflowStep) bool {
				return integrationDoctorRunContainsGHReleaseCreate(step.run)
			})
			generateIndex := integrationDoctorEffectiveStepPosition(jobs, func(step integrationDoctorWorkflowStep) bool {
				return strings.Contains(step.run, ".github/scripts/generate-plugin-index.sh")
			})
			publishIndex := integrationDoctorEffectiveStepPosition(jobs, func(step integrationDoctorWorkflowStep) bool {
				return strings.Contains(step.run, ".github/scripts/publish-plugin-index.sh")
			})
			if releaseIndex < 0 || generateIndex <= releaseIndex || publishIndex <= generateIndex {
				t.Fatalf("registry order release=%d generate=%d publish=%d", releaseIndex, generateIndex, publishIndex)
			}
			_, diagnostics := inspectIntegrationDoctorPluginRegistryPublication(
				root.Path(), behavior.path,
				integrationDoctorUnitForTest(t, repository, behavior.unit),
				jobs, filesystemIntegrationDoctorRepositoryFileReader{},
			)
			if len(diagnostics) != 0 {
				t.Fatalf("plugin registry diagnostics = %#v", diagnostics)
			}
		})
	}
}

// TestIntegrationDoctorScopesLocalActionCredentialToItsPublication proves the
// credential written on the publish-plugin-index invocation is classified as
// publication-scoped and is not reported once per expanded inner step.
func TestIntegrationDoctorScopesLocalActionCredentialToItsPublication(t *testing.T) {
	for _, behavior := range repositoryWorkflowBehaviors() {
		if !behavior.pluginRegistry {
			continue
		}
		t.Run(behavior.unit, func(t *testing.T) {
			content := readIntegrationDoctorRepositoryFile(t, behavior.path)
			workflowRoot := parseIntegrationDoctorWorkflowBytes(t, content)
			jobs := integrationDoctorWorkflowJobs(
				workflowRoot, localaction.NewRepositoryActions(repositoryRootForSelfMigrationTest()),
			)
			fact, diagnostics := inspectIntegrationDoctorCredentialWiring(behavior.path, workflowRoot, jobs)
			if fact.State != integrationDoctorVerified {
				t.Fatalf("credential fact = %#v diagnostics = %#v", fact, diagnostics)
			}
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == integrationDoctorError {
					t.Fatalf("credential diagnostic = %#v", diagnostic)
				}
			}
			scoped := 0
			for _, reference := range integrationDoctorCredentialReferences(workflowRoot, jobs) {
				if reference.Name != "GITHUB_TOKEN" {
					continue
				}
				if !reference.Publication {
					t.Fatalf("credential %q in step %q is not publication-scoped", reference.Name, reference.StepName)
				}
				scoped++
			}
			if scoped != 2 {
				t.Fatalf("built-in credential references = %d, want the release and plugin-index publications", scoped)
			}
		})
	}
}

// TestIntegrationDoctorReportsUnresolvableLocalActionReferences proves a broken
// repository-local action reference becomes an explicit workflow diagnostic
// instead of silently disappearing from the inspected steps.
func TestIntegrationDoctorReportsUnresolvableLocalActionReferences(t *testing.T) {
	root := t.TempDir()
	workflow := parseIntegrationDoctorWorkflowBytes(t, []byte(
		"jobs:\n  release:\n    steps:\n      - name: Invoke\n        uses: ./.github/actions/absent\n",
	))
	jobs := integrationDoctorWorkflowJobs(workflow, localaction.NewRepositoryActions(root))
	codes := make([]string, 0, 1)
	inspectIntegrationDoctorLocalActions(jobs, func(_ integrationDoctorSeverity, code, message, _ string) {
		codes = append(codes, code)
		if !strings.Contains(message, "./.github/actions/absent") || !strings.Contains(message, localaction.FailureMissing) {
			t.Fatalf("diagnostic message = %q", message)
		}
	})
	if len(codes) != 1 || codes[0] != "LOCAL_ACTION_UNRESOLVED" {
		t.Fatalf("local action diagnostics = %v", codes)
	}
}

func repositoryWorkflowJobsForTest(t *testing.T, workflowPath string) []integrationDoctorWorkflowJob {
	t.Helper()
	return integrationDoctorWorkflowJobs(
		parseIntegrationDoctorWorkflowBytes(t, readIntegrationDoctorRepositoryFile(t, workflowPath)),
		localaction.NewRepositoryActions(repositoryRootForSelfMigrationTest()),
	)
}

func integrationDoctorUnitForTest(
	t *testing.T,
	repository *releaseconfig.ReleaseRepository,
	unitID string,
) releaseconfig.ReleaseUnit {
	t.Helper()
	for _, unit := range repository.Units {
		if unit.ID == unitID {
			return unit
		}
	}
	t.Fatalf("release unit %q is missing", unitID)
	return releaseconfig.ReleaseUnit{}
}

func integrationDoctorMatchingSteps(
	jobs []integrationDoctorWorkflowJob,
	matches func(integrationDoctorWorkflowStep) bool,
) []integrationDoctorWorkflowStep {
	found := make([]integrationDoctorWorkflowStep, 0, 1)
	for _, job := range jobs {
		for _, step := range job.steps {
			if matches(step) {
				found = append(found, step)
			}
		}
	}
	return found
}

func integrationDoctorEffectiveStepPosition(
	jobs []integrationDoctorWorkflowJob,
	matches func(integrationDoctorWorkflowStep) bool,
) int {
	position := 0
	for _, job := range jobs {
		for _, step := range job.steps {
			if matches(step) {
				return position
			}
			position++
		}
	}
	return -1
}
