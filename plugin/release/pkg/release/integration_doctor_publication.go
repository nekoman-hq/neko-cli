package release

import (
	"fmt"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"gopkg.in/yaml.v3"
)

func inspectIntegrationDoctorPublication(
	repositoryRoot string,
	workflowPath string,
	units []releaseconfig.ReleaseUnit,
	root *yaml.Node,
	jobs []integrationDoctorWorkflowJob,
	files integrationDoctorRepositoryFileReader,
	repositoryIdentity integrationDoctorRepositoryIdentity,
	repositoryIdentityErr error,
) (integrationDoctorVerification, []integrationDoctorDiagnostic) {
	fact := newIntegrationDoctorVerification(
		"publication_identity", integrationDoctorVerified, "", workflowPath, "", workflowPath,
	)
	diagnostics := make([]integrationDoctorDiagnostic, 0)
	add := func(unit, code, message, remediation string) {
		diagnostics = append(diagnostics, newIntegrationDoctorWorkflowDiagnostic(
			integrationDoctorError, workflowPath, unit, code, message, remediation,
		))
	}
	if repositoryIdentityErr != nil || repositoryIdentity.Name() == "" {
		fact.State = integrationDoctorUnverifiable
		fact.LimitationClass = integrationDoctorRemoteLimitation
		fact.Evidence = "Publication commands were identified, but the local origin repository identity could not be resolved."
		diagnostics = append(diagnostics, integrationDoctorPublicationLimitation(workflowPath, false))
		return fact, diagnostics
	}

	publishJobs := make([]integrationDoctorWorkflowJob, 0)
	for _, job := range jobs {
		if integrationDoctorJobPublishesRepositoryContents(job) {
			publishJobs = append(publishJobs, job)
		}
	}
	if len(publishJobs) == 0 {
		fact.State = integrationDoctorUnsupported
		fact.LimitationClass = integrationDoctorRuntimeLimitation
		fact.Evidence = "No supported GoReleaser or gh release publication command could be inspected."
		diagnostics = append(diagnostics, integrationDoctorPublicationLimitation(workflowPath, false))
		return fact, diagnostics
	}

	for _, unit := range units {
		config, configPath, configOK := integrationDoctorLoadUnitGoReleaserConfig(repositoryRoot, unit, files)
		if !configOK {
			add(unit.ID, "PUBLICATION_CONFIG_MISSING", "Publication artifact identity cannot be linked to a supported local GoReleaser configuration.", "Reference one repository-confined supported GoReleaser configuration from the workflow.")
			continue
		}
		fact.References = append(fact.References, configPath)
		if integrationDoctorDiagnosticsContainErrors(inspectIntegrationDoctorGoReleaserConfig(workflowPath, configPath, config, []releaseconfig.ReleaseUnit{unit})) {
			add(unit.ID, "PUBLICATION_ARTIFACT_IDENTITY_MISMATCH", "Published archives, binaries, checksums, or release IDs conflict with the configured unit identity.", "Align the supported GoReleaser build, archive, checksum, and release IDs with the release unit.")
		}
		if !integrationDoctorPublicationCommandForUnit(unit, publishJobs) {
			add(unit.ID, "PUBLICATION_COMMAND_UNSUPPORTED", "No supported real GoReleaser release or gh release create command publishes this unit.", "Use a real non-snapshot GoReleaser release or an explicit gh release create command with validated tag and target SHA.")
		}
		if unit.IsPlugin {
			registryReferences, registryDiagnostics := inspectIntegrationDoctorPluginRegistryPublication(
				repositoryRoot, workflowPath, unit, publishJobs, files,
			)
			fact.References = append(fact.References, registryReferences...)
			diagnostics = append(diagnostics, registryDiagnostics...)
		} else if integrationDoctorJobsContainPluginRegistryMutation(publishJobs) {
			add(unit.ID, "NORMAL_UNIT_REGISTRY_MUTATION", "A non-plugin unit enters the plugin registry publication path.", "Keep plugin-index generation and publication exclusive to configured plugin units.")
		}
	}
	for _, job := range publishJobs {
		if !integrationDoctorPublishCheckoutUsesValidatedSHA(job) {
			add("", "PUBLICATION_SHA_MISMATCH", fmt.Sprintf("Publication job %q does not checkout the validated release SHA.", job.id), "Checkout needs.<validation>.outputs.release_sha before publication.")
		}
		for _, step := range job.steps {
			if integrationDoctorRunPublishesGitHubRelease(step.run) && !integrationDoctorGHReleaseUsesValidatedIdentity(step) {
				add("", "PUBLICATION_TAG_TARGET_MISMATCH", fmt.Sprintf("Publication step %q does not use the validated tag and release SHA with tag verification.", step.name), "Bind gh release create to validated tag and release_sha outputs and pass --verify-tag.")
			}
		}
	}
	if integrationDoctorDiagnosticsContainErrors(diagnostics) {
		fact.State = integrationDoctorMismatch
		fact.Evidence = "One or more local provider, target, tag/SHA, artifact, or plugin-registry publication contracts conflict."
		return fact, diagnostics
	}
	fact.Evidence = fmt.Sprintf("GitHub publication targets local origin %q and locally aligns validated tag/SHA flow, archives, checksums, binaries, plugin assets, and plugin-registry ordering where applicable.", repositoryIdentity.Name())
	fact.References = newIntegrationDoctorVerification(
		fact.Category, fact.State, fact.Evidence, fact.Workflow, fact.Unit, fact.References...,
	).References
	diagnostics = append(diagnostics, integrationDoctorPublicationLimitation(workflowPath, true))
	return fact, diagnostics
}

func integrationDoctorPublicationCommandForUnit(
	unit releaseconfig.ReleaseUnit,
	jobs []integrationDoctorWorkflowJob,
) bool {
	for _, job := range jobs {
		for _, step := range job.steps {
			if integrationDoctorGoReleaserStepPublishes(step) {
				return !unit.IsPlugin
			}
			if integrationDoctorRunContainsGHReleaseCreate(step.run) {
				return true
			}
		}
	}
	return false
}

func integrationDoctorRunContainsGHReleaseCreate(command string) bool {
	for _, line := range strings.Split(command, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 3 && fields[0] == "gh" && fields[1] == "release" && fields[2] == "create" {
			return true
		}
	}
	return false
}

func integrationDoctorPublishCheckoutUsesValidatedSHA(job integrationDoctorWorkflowJob) bool {
	for _, step := range job.steps {
		if !strings.HasPrefix(step.uses, "actions/checkout@") {
			continue
		}
		ref := workflowScalar(workflowMappingValue(workflowMappingValue(step.node, "with"), "ref"))
		return strings.Contains(ref, "needs.") && strings.Contains(ref, ".outputs.release_sha")
	}
	return false
}

func integrationDoctorGHReleaseUsesValidatedIdentity(step integrationDoctorWorkflowStep) bool {
	env := workflowMappingValue(step.node, "env")
	tag := workflowScalar(workflowMappingValue(env, "RELEASE_TAG"))
	sha := workflowScalar(workflowMappingValue(env, "RELEASE_SHA"))
	return strings.Contains(tag, "needs.") && strings.Contains(tag, ".outputs.tag") &&
		strings.Contains(sha, "needs.") && strings.Contains(sha, ".outputs.release_sha") &&
		strings.Contains(step.run, `gh release create "$RELEASE_TAG"`) &&
		strings.Contains(step.run, `--target "$RELEASE_SHA"`) &&
		strings.Contains(step.run, "--verify-tag") &&
		strings.Contains(step.run, "dist/*.tar.gz") &&
		strings.Contains(step.run, "dist/*.zip") &&
		strings.Contains(step.run, "dist/*_checksums.txt")
}

func inspectIntegrationDoctorPluginRegistryPublication(
	repositoryRoot, workflowPath string,
	unit releaseconfig.ReleaseUnit,
	jobs []integrationDoctorWorkflowJob,
	files integrationDoctorRepositoryFileReader,
) ([]string, []integrationDoctorDiagnostic) {
	diagnostics := make([]integrationDoctorDiagnostic, 0)
	add := func(code, message, remediation string) {
		diagnostics = append(diagnostics, newIntegrationDoctorWorkflowDiagnostic(
			integrationDoctorError, workflowPath, unit.ID, code, message, remediation,
		))
	}
	releaseIndex, generateIndex, publishIndex := -1, -1, -1
	position := 0
	for _, job := range jobs {
		for _, step := range job.steps {
			switch {
			case integrationDoctorRunContainsGHReleaseCreate(step.run):
				releaseIndex = position
			case strings.Contains(step.run, ".github/scripts/generate-plugin-index.sh"):
				generateIndex = position
			case strings.Contains(step.run, ".github/scripts/publish-plugin-index.sh"):
				publishIndex = position
			}
			position++
		}
	}
	if releaseIndex < 0 || generateIndex <= releaseIndex || publishIndex <= generateIndex {
		add("PLUGIN_REGISTRY_ORDER_INVALID", "Plugin registry generation/publication does not follow successful plugin release creation.", "Create the plugin release, then generate the index, then publish the mutable registry asset.")
	}
	references := []string{".github/scripts/generate-plugin-index.sh", ".github/scripts/publish-plugin-index.sh"}
	generate, generateErr := files.ReadFile(repositoryRoot, references[0])
	publish, publishErr := files.ReadFile(repositoryRoot, references[1])
	if generateErr != nil || publishErr != nil || !integrationDoctorPluginRegistryScriptsSupported(generate, publish) {
		add("PLUGIN_REGISTRY_CONTRACT_MISMATCH", "Plugin registry scripts do not expose the supported repository, exact-tag, index-source, mutable-target, and target-SHA contract.", "Use the repository-confined plugin-index scripts with exact tags and the plugin-registry release; do not discover latest or tag-prefix fallbacks.")
	}
	return references, diagnostics
}

func integrationDoctorPluginRegistryScriptsSupported(generate, publish []byte) bool {
	generateText := string(generate)
	publishText := string(publish)
	for _, required := range []string{
		"release plugin-index", `--repository "$GITHUB_REPOSITORY"`, "RELEASE_UNIT", "RELEASE_VERSION", "RELEASE_TAG", ".tag == (.tagPrefix + .version)",
	} {
		if !strings.Contains(generateText, required) {
			return false
		}
	}
	for _, required := range []string{
		"plugin-registry", "plugin-index.json", `gh release upload "$registry_tag"`, `gh release create "$registry_tag"`, `--repo "$GITHUB_REPOSITORY"`, `--target "$target_sha"`,
	} {
		if !strings.Contains(publishText, required) {
			return false
		}
	}
	combined := generateText + publishText
	return !strings.Contains(combined, "/releases/latest") && !strings.Contains(combined, "git tag --list")
}

func integrationDoctorJobsContainPluginRegistryMutation(jobs []integrationDoctorWorkflowJob) bool {
	for _, job := range jobs {
		for _, step := range job.steps {
			if strings.Contains(step.run, "generate-plugin-index.sh") || strings.Contains(step.run, "publish-plugin-index.sh") {
				return true
			}
		}
	}
	return false
}

func integrationDoctorPublicationLimitation(
	workflowPath string,
	locallyVerified bool,
) integrationDoctorDiagnostic {
	message := "A supported local publication identity could not be fully established; remote target state and runtime acceptance were not checked."
	if locallyVerified {
		message = "Provider, local repository target, validated tag/SHA flow, artifact identities, and plugin-registry ordering where applicable were locally verified; remote target presence/enabled state, future version or upload acceptance, mutable overwrite authorization, service availability, and runtime artifact behavior remain unknown."
	}
	return newIntegrationDoctorWorkflowDiagnostic(
		integrationDoctorNotVerifiable,
		workflowPath,
		"",
		"PUBLICATION_TARGET_NOT_VERIFIABLE",
		message,
		"Confirm remote target state and acceptance during a controlled publication run without changing the local Doctor's offline boundary.",
	)
}
