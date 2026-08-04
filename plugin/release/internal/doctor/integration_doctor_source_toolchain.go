package doctor

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// integrationDoctorPluginDirectoryVariable is the Neko CLI's own plugin
// directory environment contract; the CLI loads plugins from it.
const integrationDoctorPluginDirectoryVariable = "NEKO_PLUGIN_DIR"

func integrationDoctorReleaseStateFile() string {
	return path.Join(releaseconfig.V2Directory, releaseconfig.V2StateFileName)
}

func inspectIntegrationDoctorSourceValidationToolchain(
	repositoryRoot string,
	workflowPath string,
	workflowUnits []releaseconfig.ReleaseUnit,
	repositoryUnits []releaseconfig.ReleaseUnit,
	jobs []integrationDoctorWorkflowJob,
	files integrationDoctorRepositoryFileReader,
) (integrationDoctorVerification, []integrationDoctorDiagnostic) {
	unit, scopeOK := integrationDoctorSourceValidationToolchainUnit(workflowUnits)
	unitID := unit.ID
	if unitID == "" {
		unitID = "self-release"
	}
	fact := newIntegrationDoctorVerification(
		"installation_wiring",
		integrationDoctorVerified,
		"",
		workflowPath,
		unitID,
		workflowPath,
	)
	diagnostics := make([]integrationDoctorDiagnostic, 0)
	add := func(code, message, remediation string) {
		diagnostics = append(diagnostics, newIntegrationDoctorWorkflowDiagnostic(
			integrationDoctorError, workflowPath, unitID, code, message, remediation,
		))
	}

	if !scopeOK {
		add(
			"SOURCE_TOOLCHAIN_SCOPE_INVALID",
			"An exact-source validation toolchain is allowed only in a workflow that releases exactly one configured unit.",
			"Give each self-release unit its own workflow, or keep multi-unit consumer workflows on pinned published tools.",
		)
	}

	releasePluginUnit, releasePluginOK := integrationDoctorReleasePluginUnit(repositoryUnits)
	sources, sourcesOK := inspectIntegrationDoctorSourceToolchainSources(jobs, releasePluginUnit, releasePluginOK)
	if !scopeOK || !sourcesOK || !integrationDoctorSelfReleaseIdentityPrecedesBuild(jobs, unit, sources) {
		add(
			"SOURCE_TOOLCHAIN_CONTRACT_INVALID",
			"The self-release validation toolchain does not validate immutable identity and then build and wire both Neko executables from the exact checkout.",
			"Validate the dispatched identity inline first, then build the CLI and Release Plugin into runner-temporary directories, copy the plugin manifest, expose the CLI through GITHUB_PATH, and pass the produced plugin directory to context validation.",
		)
	}

	references, filesOK := inspectIntegrationDoctorSourceToolchainFiles(
		repositoryRoot, sources, releasePluginUnit, releasePluginOK, files,
	)
	if !filesOK {
		add(
			"SOURCE_TOOLCHAIN_FILES_INVALID",
			"The exact-source validation toolchain inputs are missing or disagree with authoritative Release Plugin metadata.",
			"Build the toolchain from checked-out Go packages whose manifest and authoritative release state agree.",
		)
	}

	if integrationDoctorDiagnosticsContainErrors(diagnostics) {
		fact.State = integrationDoctorMismatch
		fact.Evidence = "The self-release source toolchain is incomplete or outside its bounded single-unit self-release scope."
		return fact, diagnostics
	}
	fact.Evidence = fmt.Sprintf(
		"The dedicated %s self-release workflow validates immutable identity, then builds and wires the CLI and Release Plugin from the exact checked-out commit without published installer dependencies.",
		unit.ID,
	)
	fact.References = newIntegrationDoctorVerification(
		fact.Category, fact.State, fact.Evidence, fact.Workflow, fact.Unit,
		append([]string{workflowPath}, references...)...,
	).References
	return fact, diagnostics
}

// integrationDoctorSourceToolchainSources holds the effective steps and build
// packages an exact-source toolchain was recognized from.
//
//nolint:govet // Logical order keeps the build step before the packages it builds.
type integrationDoctorSourceToolchainSources struct {
	build        integrationDoctorWorkflowStep
	buildIndex   int
	validator    integrationDoctorWorkflowStep
	cliPackage   string
	pluginPackge string
}

// inspectIntegrationDoctorSourceToolchainSources recognizes the exact-source
// toolchain from the effective step contents alone: one step must build the
// CLI onto the later workflow PATH and the configured Release Plugin binary and
// manifest into an isolated plugin directory that context validation consumes.
func inspectIntegrationDoctorSourceToolchainSources(
	jobs []integrationDoctorWorkflowJob,
	releasePlugin releaseconfig.ReleaseUnit,
	releasePluginOK bool,
) (integrationDoctorSourceToolchainSources, bool) {
	build, buildIndex, buildOK := integrationDoctorSourceToolchainBuildStep(jobs)
	validator, validatorOK := integrationDoctorInstallationStep(jobs, func(candidate integrationDoctorWorkflowStep) bool {
		return strings.Contains(candidate.run, integrationDoctorContextValidatorCommand)
	})
	sources := integrationDoctorSourceToolchainSources{build: build, buildIndex: buildIndex, validator: validator}
	if !buildOK || !validatorOK || !releasePluginOK {
		return sources, false
	}
	evidence := integrationDoctorStepEvidence(build)
	if !strings.Contains(evidence, "-trimpath") || !strings.Contains(evidence, "-ldflags") ||
		!strings.Contains(evidence, "Version=") || !strings.Contains(evidence, integrationDoctorReleaseStateFile()) {
		return sources, false
	}
	for _, forbidden := range []string{"curl ", "wget ", "https://", "neko plugin install"} {
		if strings.Contains(evidence, forbidden) {
			return sources, false
		}
	}
	cliPackage, cliOK := integrationDoctorCLIBuildPackage(build)
	pluginPackage, pluginOK := integrationDoctorPluginBuildPackage(build, releasePlugin)
	if !cliOK || !pluginOK || !integrationDoctorRunnerTemporaryBuild(build) {
		return sources, false
	}
	sources.cliPackage, sources.pluginPackge = cliPackage, pluginPackage
	return sources, integrationDoctorSourceToolchainPluginDirectoryWired(build, validator)
}

// integrationDoctorSourceToolchainBuildStep finds the effective step that
// builds executables from the checked-out source and exposes one of them to the
// later workflow PATH.
func integrationDoctorSourceToolchainBuildStep(
	jobs []integrationDoctorWorkflowJob,
) (integrationDoctorWorkflowStep, int, bool) {
	position := 0
	for _, job := range jobs {
		for _, step := range job.steps {
			if integrationDoctorStepBuildsToolchainFromSource(step) {
				return step, position, true
			}
			position++
		}
	}
	return integrationDoctorWorkflowStep{}, -1, false
}

// integrationDoctorStepBuildsToolchainFromSource reports whether one effective
// step compiles Go executables from the checkout and exposes one of their
// directories to later steps instead of installing published Neko artifacts.
func integrationDoctorStepBuildsToolchainFromSource(step integrationDoctorWorkflowStep) bool {
	if !strings.Contains(step.run, "go build") || !strings.Contains(step.run, "GITHUB_PATH") {
		return false
	}
	for _, output := range integrationDoctorGoBuildOutputs(step.run) {
		if integrationDoctorAppendsDirectoryToPath(step.run, path.Dir(output)) {
			return true
		}
	}
	return false
}

// integrationDoctorCLIBuildPackage returns the Go package whose executable the
// build step appends to the later workflow PATH.
func integrationDoctorCLIBuildPackage(step integrationDoctorWorkflowStep) (string, bool) {
	for output, buildPackage := range integrationDoctorGoBuildPackages(step.run) {
		if integrationDoctorAppendsDirectoryToPath(step.run, path.Dir(output)) {
			return buildPackage, true
		}
	}
	return "", false
}

// integrationDoctorPluginBuildPackage returns the Go package whose executable
// carries the configured Release Plugin binary name and whose directory also
// receives the configured plugin manifest.
func integrationDoctorPluginBuildPackage(
	step integrationDoctorWorkflowStep,
	releasePlugin releaseconfig.ReleaseUnit,
) (string, bool) {
	evidence := integrationDoctorStepEvidence(step)
	for output, buildPackage := range integrationDoctorGoBuildPackages(step.run) {
		if path.Base(output) != releasePlugin.PluginBinaryName || releasePlugin.PluginBinaryName == "" {
			continue
		}
		if !strings.Contains(evidence, releasePlugin.PluginManifestPath) ||
			!strings.Contains(evidence, path.Join(path.Dir(output), "manifest.json")) {
			continue
		}
		return buildPackage, true
	}
	return "", false
}

// integrationDoctorRunnerTemporaryBuild reports whether the produced
// executables land in runner-temporary directories instead of the checkout.
func integrationDoctorRunnerTemporaryBuild(step integrationDoctorWorkflowStep) bool {
	if !strings.Contains(
		integrationDoctorWorkflowStepEnvironment(step, integrationDoctorPluginDirectoryVariable), "runner.temp",
	) {
		return false
	}
	for _, output := range integrationDoctorGoBuildOutputs(step.run) {
		variable := integrationDoctorShellVariableName(output)
		if variable != "" && strings.Contains(integrationDoctorWorkflowStepEnvironment(step, variable), "runner.temp") {
			return true
		}
	}
	return false
}

// integrationDoctorSelfReleaseIdentityPrecedesBuild reports whether an inline
// workflow step rejects a mismatched release identity before any candidate code
// builds the validation toolchain.
func integrationDoctorSelfReleaseIdentityPrecedesBuild(
	jobs []integrationDoctorWorkflowJob,
	unit releaseconfig.ReleaseUnit,
	sources integrationDoctorSourceToolchainSources,
) bool {
	position := 0
	for _, job := range jobs {
		for _, step := range job.steps {
			if integrationDoctorSelfReleaseIdentityContract(step, unit) {
				return sources.buildIndex >= 0 && position < sources.buildIndex
			}
			position++
		}
	}
	return false
}

// inspectIntegrationDoctorSourceToolchainFiles verifies that the packages the
// toolchain builds exist in the checkout and that the Release Plugin manifest
// agrees with authoritative release state.
func inspectIntegrationDoctorSourceToolchainFiles(
	repositoryRoot string,
	sources integrationDoctorSourceToolchainSources,
	releasePlugin releaseconfig.ReleaseUnit,
	releasePluginOK bool,
	files integrationDoctorRepositoryFileReader,
) ([]string, bool) {
	if !releasePluginOK || sources.cliPackage == "" || sources.pluginPackge == "" {
		return nil, false
	}
	cliModule := path.Join(sources.cliPackage, "go.mod")
	pluginMain := path.Join(sources.pluginPackge, "main.go")
	references := []string{cliModule, pluginMain, releasePlugin.PluginManifestPath}
	for _, reference := range references {
		if !integrationDoctorRepositoryEvidencePathValid(reference) {
			return nil, false
		}
	}
	manifestContent, manifestErr := files.ReadFile(repositoryRoot, releasePlugin.PluginManifestPath)
	_, moduleErr := files.ReadFile(repositoryRoot, cliModule)
	_, pluginMainErr := files.ReadFile(repositoryRoot, pluginMain)
	manifest := integrationDoctorPluginManifestIdentity{}
	if manifestErr != nil || moduleErr != nil || pluginMainErr != nil ||
		json.Unmarshal(manifestContent, &manifest) != nil ||
		manifest.Name != releasePlugin.PluginName || manifest.Version != releasePlugin.Version {
		return nil, false
	}
	return references, true
}

// integrationDoctorSelfReleaseIdentityContract verifies the inline guard that
// must reject a mismatched release identity before any candidate code runs. The
// guard stays in the workflow itself and is never expanded from a local action.
func integrationDoctorSelfReleaseIdentityContract(
	step integrationDoctorWorkflowStep,
	unit releaseconfig.ReleaseUnit,
) bool {
	if step.action.Expanded() || unit.ID == "" {
		return false
	}
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
	command := integrationDoctorShellCommand(step.run)
	for _, required := range []string{
		`if [[ "$RELEASE_UNIT" != "` + unit.ID + `" ]]`,
		`if [[ "$RELEASE_TAG" != "` + unit.TagPrefix + `${RELEASE_VERSION}" ]]`,
		`head_sha="$(git rev-parse HEAD)"`,
		`tag_sha="$(git rev-list -n 1 "$RELEASE_TAG")"`,
		`'.units["` + unit.ID + `"].version == $version'`,
	} {
		if !strings.Contains(command, required) {
			return false
		}
	}
	if unit.IsPlugin && !strings.Contains(command, `'.version == $version' `+unit.PluginManifestPath) {
		return false
	}
	return true
}

// integrationDoctorSourceValidationToolchainUnit bounds the exact-source
// toolchain to a workflow that releases exactly one configured unit. Whether
// the checkout really owns the Neko sources is decided by the toolchain's build
// packages and Release Plugin metadata, not by a unit name.
func integrationDoctorSourceValidationToolchainUnit(units []releaseconfig.ReleaseUnit) (releaseconfig.ReleaseUnit, bool) {
	if len(units) != 1 {
		return releaseconfig.ReleaseUnit{}, false
	}
	return units[0], true
}

// integrationDoctorSourceToolchainPluginDirectoryWired reports whether context
// validation loads the Release Plugin the exact-source toolchain produced,
// either through the same literal directory or through an output of the exact
// workflow step that invoked the toolchain.
func integrationDoctorSourceToolchainPluginDirectoryWired(
	toolchain integrationDoctorWorkflowStep,
	validator integrationDoctorWorkflowStep,
) bool {
	produced := integrationDoctorWorkflowStepEnvironment(toolchain, integrationDoctorPluginDirectoryVariable)
	consumed := integrationDoctorWorkflowStepEnvironment(validator, integrationDoctorPluginDirectoryVariable)
	if produced == "" || consumed == "" {
		return false
	}
	if produced == consumed {
		return true
	}
	invocation := toolchain.action.CallerID
	return invocation != "" && strings.Contains(consumed, "steps."+invocation+".outputs.")
}

func integrationDoctorWorkflowStepEnvironment(step integrationDoctorWorkflowStep, name string) string {
	return workflowScalar(workflowMappingValue(workflowMappingValue(step.node, "env"), name))
}
