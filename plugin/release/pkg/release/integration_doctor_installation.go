package release

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	goreleaserfacts "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool/goreleaser"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"gopkg.in/yaml.v3"
)

var (
	integrationDoctorInstallSourcePattern  = regexp.MustCompile(`https://raw\.githubusercontent\.com/([A-Za-z0-9._-]+)/([A-Za-z0-9._-]+)/[^/[:space:]]+/install\.sh`)
	integrationDoctorRegistrySourcePattern = regexp.MustCompile(`https://api\.github\.com/repos/([A-Za-z0-9._-]+)/([A-Za-z0-9._-]+)/releases`)
	integrationDoctorCLIArchivePattern     = regexp.MustCompile(`printf '([A-Za-z0-9._-]+)_%s_%s\.%s`)
)

type integrationDoctorPluginManifestIdentity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func inspectIntegrationDoctorInstallation(
	repositoryRoot string,
	workflowPath string,
	repositoryUnits []releaseconfig.ReleaseUnit,
	jobs []integrationDoctorWorkflowJob,
	files integrationDoctorRepositoryFileReader,
	repositoryIdentity integrationDoctorRepositoryIdentity,
	repositoryIdentityErr error,
) (integrationDoctorVerification, []integrationDoctorDiagnostic) {
	fact := newIntegrationDoctorVerification(
		"installation_wiring",
		integrationDoctorVerified,
		"",
		workflowPath,
		"",
		workflowPath,
	)
	diagnostics := make([]integrationDoctorDiagnostic, 0)
	add := func(unit, code, message, remediation string) {
		diagnostics = append(diagnostics, newIntegrationDoctorWorkflowDiagnostic(
			integrationDoctorError, workflowPath, unit, code, message, remediation,
		))
	}
	cliStep, cliOK := integrationDoctorInstallationStep(jobs, func(step integrationDoctorWorkflowStep) bool {
		return strings.Contains(step.run, "install.sh")
	})
	pluginStep, pluginOK := integrationDoctorInstallationStep(jobs, func(step integrationDoctorWorkflowStep) bool {
		return strings.Contains(step.run, "neko plugin install release")
	})
	if !cliOK || !pluginOK {
		fact.State = integrationDoctorMissing
		fact.Evidence = "Required pinned CLI and Release plugin installation steps were not both present."
		return fact, diagnostics
	}
	if !integrationDoctorPinnedRepositoryVariable(cliStep, "NEKO_VERSION") ||
		!integrationDoctorPinnedRepositoryVariable(pluginStep, "NEKO_RELEASE_PLUGIN_VERSION") {
		add("", "INSTALLATION_VERSION_PIN_INVALID", "CLI or Release plugin installation is not pinned through its recognized repository variable.", "Bind NEKO_VERSION and NEKO_RELEASE_PLUGIN_VERSION to repository variables and pass those exact values to the installers.")
	}
	if workflowScalar(workflowMappingValue(workflowMappingValue(cliStep.node, "env"), "NEKO_INSTALL_DIR")) == "" ||
		!strings.Contains(cliStep.run, "NEKO_INSTALL_DIR") || !strings.Contains(cliStep.run, "GITHUB_PATH") {
		add("", "CLI_INSTALL_DIRECTORY_INVALID", "The CLI installation directory is not explicitly wired into the later workflow PATH.", "Set NEKO_INSTALL_DIR and append that exact directory to GITHUB_PATH before validation.")
	}

	installSource := integrationDoctorInstallSourceIdentity(cliStep.run)
	if repositoryIdentityErr != nil || installSource == "" {
		fact.State = integrationDoctorUnverifiable
		fact.LimitationClass = integrationDoctorRemoteLimitation
		fact.Evidence = "Pinned installation commands were identified, but local repository and installer source identity could not both be resolved."
		diagnostics = append(diagnostics, integrationDoctorInstallationLimitation(workflowPath, false))
		return fact, diagnostics
	}

	localInstaller, installerErr := files.ReadFile(repositoryRoot, "install.sh")
	localRegistry, registryErr := files.ReadFile(repositoryRoot, "pkg/plugin/registry.go")
	localManager, managerErr := files.ReadFile(repositoryRoot, "pkg/plugin/manager.go")
	selfRepositoryContracts := installerErr == nil && registryErr == nil && managerErr == nil
	if installSource != repositoryIdentity.Name() {
		if selfRepositoryContracts {
			add("", "INSTALL_SOURCE_IDENTITY_MISMATCH", fmt.Sprintf("Installer source %q conflicts with local origin identity %q.", installSource, repositoryIdentity.Name()), "Use one repository identity for the checked-out source, installer, registry, and publication target.")
			fact.State = integrationDoctorMismatch
			fact.Evidence = "Local installer source and repository origin identity conflict."
			return fact, diagnostics
		}
		fact.State = integrationDoctorUnverifiable
		fact.LimitationClass = integrationDoctorRemoteLimitation
		fact.Evidence = "The workflow installs from an external repository, so its remote installer and artifact identity were not cross-checked locally."
		diagnostics = append(diagnostics, integrationDoctorInstallationLimitation(workflowPath, false))
		return fact, diagnostics
	}
	if !selfRepositoryContracts {
		add("", "LOCAL_INSTALLATION_CONTRACT_MISSING", "The self-repository installer or plugin registry contract could not be inspected locally.", "Restore install.sh and the local plugin registry/manager sources used by the workflow installation contract.")
		fact.State = integrationDoctorMissing
		fact.Evidence = "Self-repository installation contract files are missing or unsafe to read."
		return fact, diagnostics
	}

	cliArchivePrefix := integrationDoctorCLIInstallerArchivePrefix(localInstaller)
	if cliArchivePrefix == "" || !integrationDoctorCLIInstallerContractSupported(localInstaller) {
		add("", "CLI_INSTALLER_CONTRACT_MISMATCH", "The local CLI installer does not expose the supported version normalization, platform archive, binary, and install-directory contract.", "Align install.sh with canonical semantic versions and explicit Darwin/Linux/Windows archive and binary handling.")
	}
	cliUnit, cliConfig, cliConfigPath, cliConfigOK := integrationDoctorFindCLIArtifactConfig(
		repositoryRoot, repositoryUnits, cliArchivePrefix, files,
	)
	if !cliConfigOK || !integrationDoctorCLIArchiveMatchesInstaller(cliConfig, cliArchivePrefix) {
		add(cliUnit.ID, "CLI_INSTALLER_ARCHIVE_MISMATCH", "The CLI installer archive identity is incompatible with local GoReleaser output.", "Align CLI archive prefix, OS/architecture labels, tar.gz defaults, and Windows zip output.")
	}

	registrySource := integrationDoctorRegistrySourceIdentity(localRegistry)
	if registrySource == "" || registrySource != repositoryIdentity.Name() ||
		!integrationDoctorPluginManagerContractSupported(localRegistry, localManager) {
		add("", "PLUGIN_REGISTRY_IDENTITY_MISMATCH", "The local plugin registry and manager do not resolve exact-tag tar.gz assets from the self-repository identity.", "Align the registry repository, exact-tag lookup, asset prefix, platform suffix, and local manifest installation contract.")
	}
	releasePluginUnit, releasePluginOK := integrationDoctorReleasePluginUnit(repositoryUnits)
	if !releasePluginOK {
		add("", "RELEASE_PLUGIN_METADATA_MISSING", "No configured plugin unit provides the installed Release plugin identity.", "Configure one plugin unit named release with manifest, binary, asset prefix, and GoReleaser workflow metadata.")
	} else {
		manifestContent, manifestErr := files.ReadFile(repositoryRoot, releasePluginUnit.PluginManifestPath)
		manifest := integrationDoctorPluginManifestIdentity{}
		if manifestErr != nil || json.Unmarshal(manifestContent, &manifest) != nil ||
			manifest.Name != releasePluginUnit.PluginName || manifest.Version != releasePluginUnit.Version ||
			releasePluginUnit.PluginBinaryName == "" || releasePluginUnit.PluginAssetPrefix == "" {
			add(releasePluginUnit.ID, "RELEASE_PLUGIN_METADATA_MISMATCH", "Release plugin manifest, unit metadata, and current state version are not locally aligned.", "Align plugin name, manifest version, binary name, asset prefix, and state version.")
		}
		pluginConfig, pluginConfigPath, pluginConfigOK := integrationDoctorLoadUnitGoReleaserConfig(repositoryRoot, releasePluginUnit, files)
		if !pluginConfigOK || !integrationDoctorPluginArchiveMatchesInstaller(pluginConfig, releasePluginUnit) {
			add(releasePluginUnit.ID, "RELEASE_PLUGIN_ARCHIVE_MISMATCH", "Release plugin archive identity is incompatible with the local plugin installer contract.", "Publish a tar.gz archive using the configured plugin asset prefix, version, OS, architecture, binary, and manifest identity.")
		}
		fact.References = append(fact.References, releasePluginUnit.PluginManifestPath, pluginConfigPath)
	}

	if integrationDoctorDiagnosticsContainErrors(diagnostics) {
		fact.State = integrationDoctorMismatch
		fact.Evidence = "One or more local installation or artifact identity contracts conflict."
		return fact, diagnostics
	}
	fact.Evidence = "Pinned CLI and Release plugin installation, self-repository identity, version normalization, platform archive names, binaries, manifest, and install-directory wiring are locally consistent."
	fact.References = append(fact.References,
		"install.sh",
		"pkg/plugin/manager.go",
		"pkg/plugin/registry.go",
		cliConfigPath,
	)
	fact.References = newIntegrationDoctorVerification(
		fact.Category, fact.State, fact.Evidence, fact.Workflow, fact.Unit, fact.References...,
	).References
	diagnostics = append(diagnostics, integrationDoctorInstallationLimitation(workflowPath, true))
	return fact, diagnostics
}

func integrationDoctorInstallationStep(
	jobs []integrationDoctorWorkflowJob,
	matches func(integrationDoctorWorkflowStep) bool,
) (integrationDoctorWorkflowStep, bool) {
	for _, job := range jobs {
		for _, step := range job.steps {
			if matches(step) {
				return step, true
			}
		}
	}
	return integrationDoctorWorkflowStep{}, false
}

func integrationDoctorPinnedRepositoryVariable(step integrationDoctorWorkflowStep, name string) bool {
	value := workflowScalar(workflowMappingValue(workflowMappingValue(step.node, "env"), name))
	return strings.Contains(value, "vars."+name) && strings.Contains(step.run, name)
}

func integrationDoctorInstallSourceIdentity(command string) string {
	match := integrationDoctorInstallSourcePattern.FindStringSubmatch(command)
	if len(match) != 3 {
		return ""
	}
	return match[1] + "/" + match[2]
}

func integrationDoctorRegistrySourceIdentity(content []byte) string {
	match := integrationDoctorRegistrySourcePattern.FindSubmatch(content)
	if len(match) != 3 {
		return ""
	}
	return string(match[1]) + "/" + string(match[2])
}

func integrationDoctorCLIInstallerArchivePrefix(content []byte) string {
	match := integrationDoctorCLIArchivePattern.FindSubmatch(content)
	if len(match) != 2 {
		return ""
	}
	return string(match[1])
}

func integrationDoctorCLIInstallerContractSupported(content []byte) bool {
	text := string(content)
	for _, required := range []string{
		"normalize_version()", `version=$(normalize_version "$version")`, "NEKO_VERSION",
		"platform_asset()", `os_label="Darwin"`, `os_label="Linux"`, `os_label="Windows"`,
		`arch_label="x86_64"`, `arch_label="arm64"`, `arch_label="i386"`,
		`extension="tar.gz"`, `extension="zip"`, "NEKO_INSTALL_DIR", `destination="${INSTALL_DIR}/neko"`,
	} {
		if !strings.Contains(text, required) {
			return false
		}
	}
	return true
}

func integrationDoctorPluginManagerContractSupported(registry, manager []byte) bool {
	registryText := string(registry)
	managerText := string(manager)
	for _, required := range []string{
		"fetchReleaseByTag", "entry.AssetPrefix", "_%s_%s.tar.gz", "plugin-registry", "plugin-index.json",
	} {
		if !strings.Contains(registryText, required) {
			return false
		}
	}
	for _, required := range []string{
		"ResolveReleaseTag", "runtime.GOOS", "runtime.GOARCH", "x86_64", "manifest.json", "downloadAndInstall",
	} {
		if !strings.Contains(managerText, required) {
			return false
		}
	}
	return !strings.Contains(registryText, "/releases/latest")
}

func integrationDoctorFindCLIArtifactConfig(
	repositoryRoot string,
	units []releaseconfig.ReleaseUnit,
	archivePrefix string,
	files integrationDoctorRepositoryFileReader,
) (releaseconfig.ReleaseUnit, goreleaserfacts.Config, string, bool) {
	for _, unit := range units {
		if unit.IsPlugin {
			continue
		}
		config, configPath, ok := integrationDoctorLoadUnitGoReleaserConfig(repositoryRoot, unit, files)
		if ok && config.ProjectName == archivePrefix {
			return unit, config, configPath, true
		}
	}
	return releaseconfig.ReleaseUnit{}, goreleaserfacts.Config{}, "", false
}

func integrationDoctorLoadUnitGoReleaserConfig(
	repositoryRoot string,
	unit releaseconfig.ReleaseUnit,
	files integrationDoctorRepositoryFileReader,
) (goreleaserfacts.Config, string, bool) {
	workflowContent, err := files.ReadFile(repositoryRoot, unit.Workflow)
	if err != nil {
		return goreleaserfacts.Config{}, "", false
	}
	var document yaml.Node
	if yaml.Unmarshal(workflowContent, &document) != nil {
		return goreleaserfacts.Config{}, "", false
	}
	root := workflowDocumentRoot(&document)
	invocations := integrationDoctorGoReleaserInvocations(root, integrationDoctorWorkflowJobs(root))
	for _, invocation := range invocations {
		if !integrationDoctorRepositoryEvidencePathValid(invocation.ConfigPath) {
			continue
		}
		content, readErr := files.ReadFile(repositoryRoot, invocation.ConfigPath)
		if readErr != nil {
			continue
		}
		config, parseErr := goreleaserfacts.ParseConfig(content)
		if parseErr == nil {
			return config, invocation.ConfigPath, true
		}
	}
	return goreleaserfacts.Config{}, "", false
}

func integrationDoctorCLIArchiveMatchesInstaller(
	config goreleaserfacts.Config,
	archivePrefix string,
) bool {
	archive, ok := goreleaserfacts.ArchiveByID(config.Archives, archivePrefix)
	if !ok || !goreleaserfacts.Contains(archive.Formats, "tar.gz") ||
		!strings.Contains(archive.NameTemplate, archivePrefix+"_") ||
		!strings.Contains(archive.NameTemplate, ".Os") ||
		!strings.Contains(archive.NameTemplate, ".Arch") {
		return false
	}
	for _, override := range archive.FormatOverrides {
		if override.Goos == "windows" && goreleaserfacts.Contains(override.Formats, "zip") {
			return true
		}
	}
	return false
}

func integrationDoctorPluginArchiveMatchesInstaller(
	config goreleaserfacts.Config,
	unit releaseconfig.ReleaseUnit,
) bool {
	build, buildOK := goreleaserfacts.BuildByID(config.Builds, unit.PluginBinaryName)
	archive, archiveOK := goreleaserfacts.ArchiveByID(config.Archives, unit.PluginAssetPrefix)
	return buildOK && archiveOK && build.Binary == unit.PluginBinaryName &&
		goreleaserfacts.Contains(archive.IDs, build.ID) &&
		goreleaserfacts.Contains(archive.Formats, "tar.gz") &&
		strings.Contains(archive.NameTemplate, unit.PluginAssetPrefix+"_") &&
		strings.Contains(archive.NameTemplate, ".Os") && strings.Contains(archive.NameTemplate, ".Arch")
}

func integrationDoctorReleasePluginUnit(units []releaseconfig.ReleaseUnit) (releaseconfig.ReleaseUnit, bool) {
	for _, unit := range units {
		if unit.IsPlugin && unit.PluginName == "release" {
			return unit, true
		}
	}
	return releaseconfig.ReleaseUnit{}, false
}

func integrationDoctorInstallationLimitation(
	workflowPath string,
	locallyVerified bool,
) integrationDoctorDiagnostic {
	message := "Pinned installation wiring was locally identified; remote installer and artifact availability, download, extraction, execution, and plugin loading were not checked."
	if locallyVerified {
		message = "Installation wiring and expected CLI and Release plugin artifact identity were locally verified; remote installer and artifact availability, download, extraction, execution, and plugin loading remain runtime or remote uncertainties."
	}
	return newIntegrationDoctorWorkflowDiagnostic(
		integrationDoctorNotVerifiable,
		workflowPath,
		"",
		"INSTALLATION_ARTIFACTS_NOT_VERIFIABLE",
		message,
		"Verify the exact remote release assets during a controlled workflow run and retain installation/runtime evidence.",
	)
}
