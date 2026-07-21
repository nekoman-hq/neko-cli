package doctor

import (
	"fmt"
	"path"
	"sort"
	"strings"

	goreleaserfacts "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool/goreleaser"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"gopkg.in/yaml.v3"
)

type integrationDoctorGoReleaserInvocation struct {
	JobID       string
	StepName    string
	Command     string
	ConfigPath  string
	Snapshot    bool
	SkipPublish bool
	Publishes   bool
}

type integrationDoctorGoReleaserInspection struct {
	References  []string
	Diagnostics []integrationDoctorDiagnostic
	Invocations []integrationDoctorGoReleaserInvocation
	Supported   bool
	Verified    bool
}

func inspectIntegrationDoctorGoReleaser(
	repositoryRoot string,
	workflowPath string,
	units []releaseconfig.ReleaseUnit,
	root *yaml.Node,
	jobs []integrationDoctorWorkflowJob,
	files integrationDoctorRepositoryFileReader,
) integrationDoctorGoReleaserInspection {
	inspection := integrationDoctorGoReleaserInspection{
		Invocations: integrationDoctorGoReleaserInvocations(root, jobs),
	}
	if len(inspection.Invocations) == 0 {
		return inspection
	}
	inspection.Supported = true
	configPaths := make(map[string]struct{})
	for _, invocation := range inspection.Invocations {
		if !integrationDoctorRepositoryEvidencePathValid(invocation.ConfigPath) {
			inspection.Diagnostics = append(inspection.Diagnostics, newIntegrationDoctorWorkflowDiagnostic(
				integrationDoctorError,
				workflowPath,
				"",
				"GORELEASER_CONFIG_REFERENCE_INVALID",
				fmt.Sprintf("GoReleaser step %q does not resolve a repository-relative --config path.", invocation.StepName),
				"Pass --config as a repository-relative literal or through a locally declared environment value.",
			))
			continue
		}
		configPaths[invocation.ConfigPath] = struct{}{}
	}
	paths := make([]string, 0, len(configPaths))
	for configPath := range configPaths {
		paths = append(paths, configPath)
	}
	sort.Strings(paths)
	inspection.References = append(inspection.References, paths...)
	for _, configPath := range paths {
		content, err := files.ReadFile(repositoryRoot, configPath)
		if err != nil {
			inspection.Diagnostics = append(inspection.Diagnostics, newIntegrationDoctorWorkflowDiagnostic(
				integrationDoctorError,
				workflowPath,
				"",
				"GORELEASER_CONFIG_MISSING",
				fmt.Sprintf("Referenced GoReleaser configuration %q could not be inspected locally.", configPath),
				"Restore the repository-confined GoReleaser configuration referenced by the workflow.",
			))
			continue
		}
		config, err := goreleaserfacts.ParseConfig(content)
		if err != nil {
			inspection.Diagnostics = append(inspection.Diagnostics, newIntegrationDoctorWorkflowDiagnostic(
				integrationDoctorError,
				workflowPath,
				"",
				"GORELEASER_CONFIG_INVALID",
				fmt.Sprintf("Referenced GoReleaser configuration %q is not supported YAML.", configPath),
				"Repair the focused GoReleaser build, archive, checksum, and release configuration.",
			))
			continue
		}
		inspection.Diagnostics = append(
			inspection.Diagnostics,
			mapIntegrationDoctorGoReleaserFindings(
				workflowPath,
				configPath,
				goreleaserfacts.VerifyArtifactContract(config, integrationDoctorGoReleaserExpectations(config, units)),
			)...,
		)
	}
	inspection.Verified = len(paths) > 0 && !integrationDoctorDiagnosticsContainErrors(inspection.Diagnostics)
	return inspection
}

func integrationDoctorRepositoryEvidencePathValid(relativePath string) bool {
	if relativePath == "" || strings.HasPrefix(relativePath, "/") || strings.Contains(relativePath, `\`) {
		return false
	}
	clean := path.Clean(relativePath)
	return clean == relativePath && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

func integrationDoctorGoReleaserInvocations(
	root *yaml.Node,
	_ []integrationDoctorWorkflowJob,
) []integrationDoctorGoReleaserInvocation {
	facts, err := releaseworkflow.InspectConsumerWorkflowDocument(root, false)
	if err != nil {
		return nil
	}
	invocations := make([]integrationDoctorGoReleaserInvocation, 0)
	for _, operation := range facts.Operations {
		if operation.ToolCommand == "" {
			continue
		}
		invocations = append(invocations, integrationDoctorGoReleaserInvocation{
			JobID: operation.JobID, StepName: operation.StepName,
			Command: operation.ToolCommand, ConfigPath: operation.ConfigReference,
			Snapshot: operation.Snapshot, SkipPublish: operation.SkipPublication,
			Publishes: operation.Publishes,
		})
	}
	return invocations
}

func integrationDoctorGoReleaserExpectations(
	config goreleaserfacts.Config,
	units []releaseconfig.ReleaseUnit,
) []goreleaserfacts.ArtifactExpectation {
	expectations := make([]goreleaserfacts.ArtifactExpectation, 0, len(units))
	for _, unit := range units {
		expectedID := config.ProjectName
		expectedBinary := config.ProjectName
		if unit.IsPlugin {
			expectedID = unit.PluginBinaryName
			expectedBinary = unit.PluginBinaryName
		}
		expectations = append(expectations, goreleaserfacts.ArtifactExpectation{
			UnitID:         unit.ID,
			BuildID:        expectedID,
			Binary:         expectedBinary,
			ChecksumPrefix: unit.PluginAssetPrefix,
			Plugin:         unit.IsPlugin,
		})
	}
	return expectations
}

func mapIntegrationDoctorGoReleaserFindings(
	workflowPath string,
	configPath string,
	findings []goreleaserfacts.Finding,
) []integrationDoctorDiagnostic {
	diagnostics := make([]integrationDoctorDiagnostic, 0)
	add := func(unit, code, message, remediation string) {
		diagnostics = append(diagnostics, newIntegrationDoctorWorkflowDiagnostic(
			integrationDoctorError, workflowPath, unit, code, message, remediation,
		))
	}
	for _, finding := range findings {
		switch finding.Kind {
		case goreleaserfacts.FindingConfigurationUnsupported:
			add("", "GORELEASER_CONFIGURATION_UNSUPPORTED", fmt.Sprintf("GoReleaser configuration %q is not the supported version-2 project shape.", configPath), "Use a version-2 configuration with an explicit project_name, build, archive, and release identity.")
		case goreleaserfacts.FindingBuildIdentityMismatch:
			add(finding.UnitID, "GORELEASER_BUILD_ID_MISMATCH", fmt.Sprintf("GoReleaser configuration %q has no build id %q for unit %q.", configPath, finding.ExpectedID, finding.UnitID), "Align the supported GoReleaser build id with the unit artifact identity.")
		case goreleaserfacts.FindingBinaryMismatch:
			add(finding.UnitID, "GORELEASER_BINARY_MISMATCH", fmt.Sprintf("GoReleaser build %q does not declare expected binary %q and a local main package.", finding.ExpectedID, finding.ExpectedBinary), "Align the build binary and main package with the configured release unit.")
		case goreleaserfacts.FindingPlatformMismatch:
			add(finding.UnitID, "GORELEASER_PLATFORM_MISMATCH", fmt.Sprintf("GoReleaser build %q does not cover Darwin, Linux, and Windows.", finding.ExpectedID), "Declare the supported installer and publication operating systems explicitly.")
		case goreleaserfacts.FindingArchiveIdentityMismatch:
			add(finding.UnitID, "GORELEASER_ARCHIVE_ID_MISMATCH", fmt.Sprintf("GoReleaser configuration %q does not archive build %q under the same artifact id.", configPath, finding.ExpectedID), "Align archive id and ids with the expected build identity.")
		case goreleaserfacts.FindingArchiveMismatch:
			add(finding.UnitID, "GORELEASER_ARCHIVE_MISMATCH", fmt.Sprintf("GoReleaser archive %q is incompatible with the expected artifact prefix and platform identity.", finding.ExpectedID), "Use the configured artifact prefix with explicit OS and architecture naming and tar.gz support.")
		case goreleaserfacts.FindingChecksumMissing:
			add(finding.UnitID, "GORELEASER_CHECKSUM_MISSING", fmt.Sprintf("GoReleaser configuration %q does not declare the expected plugin checksum identity.", configPath), "Declare a unit-prefixed checksums.txt name for published plugin artifacts.")
		case goreleaserfacts.FindingReleaseIdentityMismatch:
			add(finding.UnitID, "GORELEASER_RELEASE_ID_MISMATCH", fmt.Sprintf("GoReleaser release configuration does not include archive id %q.", finding.ExpectedID), "Publish only the expected unit archive id.")
		}
	}
	return diagnostics
}

func integrationDoctorDiagnosticsContainErrors(diagnostics []integrationDoctorDiagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == integrationDoctorError {
			return true
		}
	}
	return false
}
