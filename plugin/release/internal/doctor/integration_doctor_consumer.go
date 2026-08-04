package doctor

import (
	"fmt"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"gopkg.in/yaml.v3"
)

func inspectIntegrationDoctorConsumerReleaseStructure(
	repositoryRoot string,
	workflowPath string,
	units []releaseconfig.ReleaseUnit,
	root *yaml.Node,
	jobs []integrationDoctorWorkflowJob,
	files integrationDoctorRepositoryFileReader,
) ([]integrationDoctorVerification, []integrationDoctorDiagnostic) {
	verifications := make([]integrationDoctorVerification, 0, 2)
	diagnostics := make([]integrationDoctorDiagnostic, 0)
	add := func(unit, code, message, remediation string) {
		diagnostics = append(diagnostics, newIntegrationDoctorWorkflowDiagnostic(
			integrationDoctorError, workflowPath, unit, code, message, remediation,
		))
	}

	goreleaser := inspectIntegrationDoctorGoReleaser(
		repositoryRoot, workflowPath, units, root, jobs, files,
	)
	diagnostics = append(diagnostics, goreleaser.Diagnostics...)
	if !goreleaser.Supported {
		unsupported := newIntegrationDoctorVerification(
			"goreleaser_configuration",
			integrationDoctorUnsupported,
			"No supported GoReleaser action invocation was available for local configuration and artifact inspection.",
			workflowPath,
			"",
			workflowPath,
		)
		unsupported.LimitationClass = integrationDoctorRuntimeLimitation
		verifications = append(verifications, unsupported)
		diagnostics = append(diagnostics, newIntegrationDoctorWorkflowDiagnostic(
			integrationDoctorNotVerifiable,
			workflowPath,
			"",
			"CONSUMER_BUILD_NOT_VERIFIABLE",
			"The local workflow uses an unsupported consumer build or publication shape, so its structure could not be fully verified.",
			"Use the supported GoReleaser action and explicit publication forms or review the custom consumer commands manually.",
		))
		return verifications, diagnostics
	}

	if !integrationDoctorGoReleaserHasCommand(goreleaser.Invocations, "check", false) {
		add("", "GORELEASER_CHECK_MISSING", "The workflow does not run GoReleaser configuration validation.", "Run `goreleaser check` against the same repository-confined configuration used for build and publication.")
	}
	if !integrationDoctorGoReleaserHasCommand(goreleaser.Invocations, "build", true) {
		add("", "CONSUMER_BUILD_MISSING", "The workflow does not run a supported GoReleaser snapshot build after context validation.", "Add a snapshot-only GoReleaser build after tests and before publication.")
	}
	validatorJob, validatorIndex, validatorOK := integrationDoctorValidatorLocation(jobs)
	if validatorOK && !integrationDoctorJobTestsAfter(validatorJob, validatorIndex) {
		add("", "CONSUMER_TESTS_MISSING", "The validated consumer job has no recognized test command after context validation.", "Run the repository test suite after canonical context validation and before snapshot build.")
	}

	publicationJobID, publicationStepIndex, publicationOK := integrationDoctorPublicationLocation(jobs, goreleaser.Invocations)
	if !publicationOK {
		code := "PUBLICATION_MISSING"
		message := "The workflow has no supported real publication operation."
		if integrationDoctorHasPublicationNoOp(jobs) {
			code = "PUBLICATION_NO_OP"
			message = "A publication-shaped step is only a successful no-op and does not publish artifacts."
		}
		add("", code, message, "Use a non-snapshot GoReleaser release or an explicit GitHub release create/upload operation.")
	} else if validatorOK && !integrationDoctorJobNeeds(jobs, publicationJobID, validatorJob.id) {
		add("", "PUBLICATION_VALIDATION_ORDER_INVALID", "The publication job does not depend on the canonical validation job.", "Make publication depend on the job that exports the validated unit, version, tag, and release SHA.")
	}

	hasPluginUnit := false
	hasNormalUnit := false
	for _, unit := range units {
		if unit.IsPlugin {
			hasPluginUnit = true
			if !integrationDoctorManifestValidationExists(jobs, unit.PluginManifestPath) {
				add(unit.ID, "PLUGIN_MANIFEST_VALIDATION_MISSING", fmt.Sprintf("Plugin unit %q does not validate materialized manifest %q against the release version.", unit.ID, unit.PluginManifestPath), "Validate the configured plugin manifest version after context validation.")
			}
		} else {
			hasNormalUnit = true
		}
	}
	generateJob, generateIndex, generatesRegistry := integrationDoctorScriptLocation(jobs, ".github/scripts/generate-plugin-index.sh")
	publishRegistryJob, publishRegistryIndex, publishesRegistry := integrationDoctorScriptLocation(jobs, ".github/scripts/publish-plugin-index.sh")
	if hasPluginUnit {
		if !generatesRegistry || !publishesRegistry {
			add("", "PLUGIN_REGISTRY_PUBLICATION_MISSING", "A plugin release workflow does not generate and publish the plugin registry index.", "Generate the plugin index after release creation and publish it through the focused registry script.")
		} else if publicationJobID != generateJob || generateJob != publishRegistryJob ||
			publicationStepIndex >= generateIndex || generateIndex >= publishRegistryIndex {
			add("", "PLUGIN_REGISTRY_ORDER_INVALID", "Plugin registry generation/publication does not follow successful plugin release creation in one ordered job.", "Create the plugin release first, then generate and publish the registry index.")
		}
	}
	if hasNormalUnit && (generatesRegistry || publishesRegistry) {
		add("", "NORMAL_UNIT_PLUGIN_REGISTRY_MUTATION", "A normal release unit enters the plugin registry publication path.", "Keep plugin-index generation and publication exclusive to plugin units.")
	}

	references := append([]string{workflowPath}, goreleaser.References...)
	if goreleaser.Verified {
		verifications = append(verifications, newIntegrationDoctorVerification(
			"goreleaser_configuration",
			integrationDoctorVerified,
			"Referenced GoReleaser version-2 build, binary, archive, checksum where required, and release identities are locally consistent.",
			workflowPath,
			"",
			references...,
		))
	}
	if !integrationDoctorDiagnosticsContainErrors(diagnostics) && goreleaser.Verified {
		verifications = append(verifications, newIntegrationDoctorVerification(
			"consumer_structure",
			integrationDoctorVerified,
			"Context validation, tests, GoReleaser check, snapshot build, real publication, and unit-specific registry behavior are locally ordered and structurally consistent.",
			workflowPath,
			"",
			references...,
		))
		diagnostics = append(diagnostics, newIntegrationDoctorWorkflowDiagnostic(
			integrationDoctorNotVerifiable,
			workflowPath,
			"",
			"CONSUMER_BUILD_NOT_VERIFIABLE",
			"Consumer validation, tests, build, and publication structure were locally verified; future runner execution, test success, produced binary behavior, and publication success remain runtime-dependent.",
			"Run the workflow for the validated release context and retain its runtime build and publication evidence.",
		))
	}
	return verifications, diagnostics
}

func integrationDoctorValidatorLocation(
	jobs []integrationDoctorWorkflowJob,
) (integrationDoctorWorkflowJob, int, bool) {
	for _, job := range jobs {
		for index, step := range job.steps {
			if strings.Contains(step.run, integrationDoctorContextValidatorCommand) {
				return job, index, true
			}
		}
	}
	return integrationDoctorWorkflowJob{}, -1, false
}

func integrationDoctorJobTestsAfter(job integrationDoctorWorkflowJob, validatorIndex int) bool {
	for _, step := range job.steps[validatorIndex+1:] {
		for _, line := range strings.Split(step.run, "\n") {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) >= 2 && fields[0] == "go" && fields[1] == "test" {
				return true
			}
		}
	}
	return false
}

func integrationDoctorGoReleaserHasCommand(
	invocations []integrationDoctorGoReleaserInvocation,
	command string,
	requireSnapshot bool,
) bool {
	for _, invocation := range invocations {
		if invocation.Command == command && (!requireSnapshot || invocation.Snapshot) {
			return true
		}
	}
	return false
}

func integrationDoctorPublicationLocation(
	jobs []integrationDoctorWorkflowJob,
	invocations []integrationDoctorGoReleaserInvocation,
) (string, int, bool) {
	for _, job := range jobs {
		for index, step := range job.steps {
			for _, invocation := range invocations {
				if invocation.JobID == job.id && invocation.StepName == step.name && invocation.Publishes {
					return job.id, index, true
				}
			}
			if integrationDoctorRunPublishesGitHubRelease(step.run) {
				return job.id, index, true
			}
		}
	}
	return "", -1, false
}

func integrationDoctorJobNeeds(
	jobs []integrationDoctorWorkflowJob,
	jobID string,
	neededJobID string,
) bool {
	for _, job := range jobs {
		if job.id != jobID {
			continue
		}
		needs := workflowMappingValue(job.node, "needs")
		if workflowScalar(needs) == neededJobID {
			return true
		}
		if needs != nil && needs.Kind == yaml.SequenceNode {
			for _, item := range needs.Content {
				if workflowScalar(item) == neededJobID {
					return true
				}
			}
		}
	}
	return false
}

func integrationDoctorHasPublicationNoOp(jobs []integrationDoctorWorkflowJob) bool {
	for _, job := range jobs {
		for _, step := range job.steps {
			name := strings.ToLower(step.name)
			if !strings.Contains(name, "publish") && !strings.Contains(name, "release") {
				continue
			}
			effective := make([]string, 0)
			for _, line := range strings.Split(step.run, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "set ") {
					continue
				}
				effective = append(effective, line)
			}
			if len(effective) > 0 {
				allNoOp := true
				for _, line := range effective {
					if !strings.HasPrefix(line, "echo ") && !strings.HasPrefix(line, "printf ") && line != "true" {
						allNoOp = false
					}
				}
				if allNoOp {
					return true
				}
			}
		}
	}
	return false
}

func integrationDoctorManifestValidationExists(
	jobs []integrationDoctorWorkflowJob,
	manifestPath string,
) bool {
	for _, job := range jobs {
		for _, step := range job.steps {
			if strings.Contains(step.run, manifestPath) && strings.Contains(step.run, "jq") && strings.Contains(step.run, "version") {
				return true
			}
		}
	}
	return false
}

func integrationDoctorScriptLocation(
	jobs []integrationDoctorWorkflowJob,
	script string,
) (string, int, bool) {
	for _, job := range jobs {
		for index, step := range job.steps {
			if strings.Contains(step.run, script) {
				return job.id, index, true
			}
		}
	}
	return "", -1, false
}
