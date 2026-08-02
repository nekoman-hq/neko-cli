package doctor

import (
	"encoding/json"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const (
	integrationDoctorSourceValidationToolchainStepName = "Build exact-commit Neko validation toolchain"
	integrationDoctorSelfReleaseIdentityStepName       = "Validate immutable self-release identity"
)

func inspectIntegrationDoctorSourceValidationToolchain(
	repositoryRoot string,
	workflowPath string,
	workflowUnits []releaseconfig.ReleaseUnit,
	jobs []integrationDoctorWorkflowJob,
	files integrationDoctorRepositoryFileReader,
) (integrationDoctorVerification, []integrationDoctorDiagnostic) {
	fact := newIntegrationDoctorVerification(
		"installation_wiring",
		integrationDoctorVerified,
		"",
		workflowPath,
		"plugin-release",
		workflowPath,
	)
	diagnostics := make([]integrationDoctorDiagnostic, 0)
	add := func(code, message, remediation string) {
		diagnostics = append(diagnostics, newIntegrationDoctorWorkflowDiagnostic(
			integrationDoctorError, workflowPath, "plugin-release", code, message, remediation,
		))
	}

	unit, scopeOK := integrationDoctorSourceValidationToolchainUnit(workflowUnits)
	if !scopeOK {
		add(
			"SOURCE_TOOLCHAIN_SCOPE_INVALID",
			"Exact-source validation toolchains are allowed only for the dedicated Release Plugin self-release workflow.",
			"Use pinned published Neko and Release Plugin versions for every other workflow.",
		)
	}

	step, stepOK := integrationDoctorSourceValidationToolchainStep(jobs)
	identity, identityOK := integrationDoctorInstallationStep(jobs, func(candidate integrationDoctorWorkflowStep) bool {
		return candidate.name == integrationDoctorSelfReleaseIdentityStepName
	})
	validator, validatorOK := integrationDoctorInstallationStep(jobs, func(candidate integrationDoctorWorkflowStep) bool {
		return strings.Contains(candidate.run, "neko release ci-validate-context")
	})
	if !identityOK || !integrationDoctorSelfReleaseIdentityContract(identity) ||
		!stepOK || !validatorOK || !integrationDoctorSourceValidationToolchainContract(step, validator) {
		add(
			"SOURCE_TOOLCHAIN_CONTRACT_INVALID",
			"The self-release validation toolchain does not build and wire both Neko executables from the exact checkout.",
			"Build the CLI and Release Plugin into runner-temporary directories, copy the manifest, expose the CLI through GITHUB_PATH, and pass the same NEKO_PLUGIN_DIR to context validation.",
		)
	}

	manifestPath := "plugin/release/manifest.json"
	manifestContent, manifestErr := files.ReadFile(repositoryRoot, manifestPath)
	_, moduleErr := files.ReadFile(repositoryRoot, "go.mod")
	_, pluginMainErr := files.ReadFile(repositoryRoot, "plugin/release/main.go")
	manifest := integrationDoctorPluginManifestIdentity{}
	if manifestErr != nil || moduleErr != nil || pluginMainErr != nil || json.Unmarshal(manifestContent, &manifest) != nil ||
		(scopeOK && (manifest.Name != unit.PluginName || manifest.Version != unit.Version)) {
		add(
			"SOURCE_TOOLCHAIN_FILES_INVALID",
			"The exact-source validation toolchain inputs are missing or disagree with Release Plugin metadata.",
			"Align go.mod, plugin/release/main.go, the Release Plugin manifest, and authoritative release state.",
		)
	}

	if integrationDoctorDiagnosticsContainErrors(diagnostics) {
		fact.State = integrationDoctorMismatch
		fact.Evidence = "The Release Plugin self-release source toolchain is incomplete or outside its bounded scope."
		return fact, diagnostics
	}
	fact.Evidence = "The dedicated Release Plugin workflow validates immutable identity, then builds and wires the CLI and candidate plugin from the exact checked-out commit without published installer dependencies."
	fact.References = newIntegrationDoctorVerification(
		fact.Category,
		fact.State,
		fact.Evidence,
		fact.Workflow,
		fact.Unit,
		workflowPath,
		"go.mod",
		"plugin/release/main.go",
		manifestPath,
	).References
	return fact, diagnostics
}

func integrationDoctorSelfReleaseIdentityContract(step integrationDoctorWorkflowStep) bool {
	for name, input := range map[string]string{
		"RELEASE_UNIT":    "inputs.unit",
		"RELEASE_VERSION": "inputs.version",
		"RELEASE_TAG":     "inputs.tag",
		"RELEASE_SHA":     "inputs.release_sha",
	} {
		if !strings.Contains(integrationDoctorWorkflowStepEnvironment(step, name), input) {
			return false
		}
	}
	for _, required := range []string{
		`if [[ "$RELEASE_UNIT" != "plugin-release" ]]`,
		`if [[ "$RELEASE_TAG" != "plugin-release/v${RELEASE_VERSION}" ]]`,
		`head_sha="$(git rev-parse HEAD)"`,
		`tag_sha="$(git rev-list -n 1 "$RELEASE_TAG")"`,
		`'.units["plugin-release"].version == $version'`,
		`'.version == $version' plugin/release/manifest.json`,
	} {
		if !strings.Contains(step.run, required) {
			return false
		}
	}
	return true
}

func integrationDoctorSourceValidationToolchainUnit(units []releaseconfig.ReleaseUnit) (releaseconfig.ReleaseUnit, bool) {
	if len(units) != 1 {
		return releaseconfig.ReleaseUnit{}, false
	}
	unit := units[0]
	return unit, unit.ID == "plugin-release" && unit.IsPlugin && unit.PluginName == "release"
}

func integrationDoctorSourceValidationToolchainStep(
	jobs []integrationDoctorWorkflowJob,
) (integrationDoctorWorkflowStep, bool) {
	return integrationDoctorInstallationStep(jobs, func(step integrationDoctorWorkflowStep) bool {
		return step.name == integrationDoctorSourceValidationToolchainStepName
	})
}

func integrationDoctorSourceValidationToolchainContract(
	step integrationDoctorWorkflowStep,
	validator integrationDoctorWorkflowStep,
) bool {
	for _, required := range []string{
		"go build -trimpath",
		`-o "$NEKO_BIN_DIR/neko" .`,
		`-o "$release_plugin_dir/plugin-release" ./plugin/release`,
		`cp plugin/release/manifest.json "$release_plugin_dir/manifest.json"`,
		`echo "$NEKO_BIN_DIR" >> "$GITHUB_PATH"`,
	} {
		if !strings.Contains(step.run, required) {
			return false
		}
	}
	for _, forbidden := range []string{"curl ", "wget ", "https://", "neko plugin install"} {
		if strings.Contains(step.run, forbidden) {
			return false
		}
	}
	for name, input := range map[string]string{
		"NEKO_BIN_DIR":    "runner.temp",
		"NEKO_PLUGIN_DIR": "runner.temp",
		"RELEASE_VERSION": "inputs.version",
		"RELEASE_SHA":     "inputs.release_sha",
	} {
		if !strings.Contains(integrationDoctorWorkflowStepEnvironment(step, name), input) {
			return false
		}
	}
	pluginDir := integrationDoctorWorkflowStepEnvironment(step, "NEKO_PLUGIN_DIR")
	return pluginDir != "" && pluginDir == integrationDoctorWorkflowStepEnvironment(validator, "NEKO_PLUGIN_DIR")
}

func integrationDoctorWorkflowStepEnvironment(step integrationDoctorWorkflowStep, name string) string {
	return workflowScalar(workflowMappingValue(workflowMappingValue(step.node, "env"), name))
}
