package release

import (
	"fmt"
	"path"
	"sort"
	"strings"

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

type integrationDoctorGoReleaserBuild struct {
	ID     string   `yaml:"id"`
	Binary string   `yaml:"binary"`
	Main   string   `yaml:"main"`
	Goos   []string `yaml:"goos"`
}

type integrationDoctorGoReleaserFormatOverride struct {
	Goos    string   `yaml:"goos"`
	Formats []string `yaml:"formats"`
}

type integrationDoctorGoReleaserArchive struct {
	ID              string                                      `yaml:"id"`
	IDs             []string                                    `yaml:"ids"`
	Formats         []string                                    `yaml:"formats"`
	NameTemplate    string                                      `yaml:"name_template"`
	FormatOverrides []integrationDoctorGoReleaserFormatOverride `yaml:"format_overrides"`
}

type integrationDoctorGoReleaserChecksum struct {
	NameTemplate string `yaml:"name_template"`
}

type integrationDoctorGoReleaserRelease struct {
	IDs []string `yaml:"ids"`
}

//nolint:govet // Field order mirrors the focused GoReleaser YAML contract.
type integrationDoctorGoReleaserConfig struct {
	Version     int                                  `yaml:"version"`
	ProjectName string                               `yaml:"project_name"`
	Builds      []integrationDoctorGoReleaserBuild   `yaml:"builds"`
	Archives    []integrationDoctorGoReleaserArchive `yaml:"archives"`
	Checksum    *integrationDoctorGoReleaserChecksum `yaml:"checksum"`
	Release     integrationDoctorGoReleaserRelease   `yaml:"release"`
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
		var config integrationDoctorGoReleaserConfig
		if err := yaml.Unmarshal(content, &config); err != nil {
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
			inspectIntegrationDoctorGoReleaserConfig(workflowPath, configPath, config, units)...,
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
	jobs []integrationDoctorWorkflowJob,
) []integrationDoctorGoReleaserInvocation {
	invocations := make([]integrationDoctorGoReleaserInvocation, 0)
	for _, job := range jobs {
		for _, step := range job.steps {
			if !strings.HasPrefix(strings.ToLower(step.uses), "goreleaser/goreleaser-action@") {
				continue
			}
			args := workflowScalar(workflowMappingValue(workflowMappingValue(step.node, "with"), "args"))
			normalizedArgs := strings.NewReplacer(
				"${{ ", "${{",
				" }}", "}}",
				"'", "",
				"\"", "",
			).Replace(args)
			fields := strings.Fields(normalizedArgs)
			invocation := integrationDoctorGoReleaserInvocation{JobID: job.id, StepName: step.name}
			if len(fields) > 0 {
				invocation.Command = fields[0]
			}
			for index, field := range fields {
				switch {
				case field == "--snapshot" || field == "--snapshot=true":
					invocation.Snapshot = true
				case strings.HasPrefix(field, "--skip=") && integrationDoctorCommaListContains(strings.TrimPrefix(field, "--skip="), "publish"):
					invocation.SkipPublish = true
				case field == "--skip" && index+1 < len(fields) && integrationDoctorCommaListContains(fields[index+1], "publish"):
					invocation.SkipPublish = true
				case strings.HasPrefix(field, "--config="):
					invocation.ConfigPath = integrationDoctorResolveWorkflowValue(
						strings.TrimPrefix(field, "--config="), root, job, step,
					)
				case field == "--config" && index+1 < len(fields):
					invocation.ConfigPath = integrationDoctorResolveWorkflowValue(fields[index+1], root, job, step)
				}
			}
			invocation.Publishes = integrationDoctorGoReleaserStepPublishes(step)
			invocations = append(invocations, invocation)
		}
	}
	return invocations
}

func integrationDoctorResolveWorkflowValue(
	value string,
	root *yaml.Node,
	job integrationDoctorWorkflowJob,
	step integrationDoctorWorkflowStep,
) string {
	value = strings.TrimSuffix(strings.TrimSpace(value), "\\")
	normalized := strings.NewReplacer("${{ ", "${{", " }}", "}}").Replace(value)
	name := ""
	switch {
	case strings.HasPrefix(normalized, "${{env.") && strings.HasSuffix(normalized, "}}"):
		name = strings.TrimSuffix(strings.TrimPrefix(normalized, "${{env."), "}}")
	case strings.HasPrefix(normalized, "${") && strings.HasSuffix(normalized, "}"):
		name = strings.TrimSuffix(strings.TrimPrefix(normalized, "${"), "}")
	case strings.HasPrefix(normalized, "$"):
		name = strings.TrimPrefix(normalized, "$")
	default:
		return normalized
	}
	for _, env := range []*yaml.Node{
		workflowMappingValue(step.node, "env"),
		job.env,
		workflowMappingValue(root, "env"),
	} {
		if resolved := workflowScalar(workflowMappingValue(env, name)); resolved != "" {
			return resolved
		}
	}
	return ""
}

func inspectIntegrationDoctorGoReleaserConfig(
	workflowPath string,
	configPath string,
	config integrationDoctorGoReleaserConfig,
	units []releaseconfig.ReleaseUnit,
) []integrationDoctorDiagnostic {
	diagnostics := make([]integrationDoctorDiagnostic, 0)
	add := func(unit, code, message, remediation string) {
		diagnostics = append(diagnostics, newIntegrationDoctorWorkflowDiagnostic(
			integrationDoctorError, workflowPath, unit, code, message, remediation,
		))
	}
	if config.Version != 2 || config.ProjectName == "" {
		add("", "GORELEASER_CONFIGURATION_UNSUPPORTED", fmt.Sprintf("GoReleaser configuration %q is not the supported version-2 project shape.", configPath), "Use a version-2 configuration with an explicit project_name, build, archive, and release identity.")
		return diagnostics
	}
	for _, unit := range units {
		expectedID := config.ProjectName
		expectedBinary := config.ProjectName
		if unit.IsPlugin {
			expectedID = unit.PluginBinaryName
			expectedBinary = unit.PluginBinaryName
		}
		build, buildOK := integrationDoctorGoReleaserBuildByID(config.Builds, expectedID)
		if !buildOK {
			add(unit.ID, "GORELEASER_BUILD_ID_MISMATCH", fmt.Sprintf("GoReleaser configuration %q has no build id %q for unit %q.", configPath, expectedID, unit.ID), "Align the supported GoReleaser build id with the unit artifact identity.")
			continue
		}
		if build.Binary != expectedBinary || strings.TrimSpace(build.Main) == "" {
			add(unit.ID, "GORELEASER_BINARY_MISMATCH", fmt.Sprintf("GoReleaser build %q does not declare expected binary %q and a local main package.", expectedID, expectedBinary), "Align the build binary and main package with the configured release unit.")
		}
		if !integrationDoctorStringSetContainsAll(build.Goos, "darwin", "linux", "windows") {
			add(unit.ID, "GORELEASER_PLATFORM_MISMATCH", fmt.Sprintf("GoReleaser build %q does not cover Darwin, Linux, and Windows.", expectedID), "Declare the supported installer and publication operating systems explicitly.")
		}
		archive, archiveOK := integrationDoctorGoReleaserArchiveByID(config.Archives, expectedID)
		if !archiveOK || !integrationDoctorStringSliceContains(archive.IDs, expectedID) {
			add(unit.ID, "GORELEASER_ARCHIVE_ID_MISMATCH", fmt.Sprintf("GoReleaser configuration %q does not archive build %q under the same artifact id.", configPath, expectedID), "Align archive id and ids with the expected build identity.")
			continue
		}
		if !integrationDoctorStringSliceContains(archive.Formats, "tar.gz") ||
			!strings.Contains(archive.NameTemplate, expectedID+"_") ||
			!strings.Contains(archive.NameTemplate, ".Os") ||
			!strings.Contains(archive.NameTemplate, ".Arch") {
			add(unit.ID, "GORELEASER_ARCHIVE_MISMATCH", fmt.Sprintf("GoReleaser archive %q is incompatible with the expected artifact prefix and platform identity.", expectedID), "Use the configured artifact prefix with explicit OS and architecture naming and tar.gz support.")
		}
		if unit.IsPlugin {
			if config.Checksum == nil || !strings.Contains(config.Checksum.NameTemplate, unit.PluginAssetPrefix+"_") || !strings.Contains(config.Checksum.NameTemplate, "checksums.txt") {
				add(unit.ID, "GORELEASER_CHECKSUM_MISSING", fmt.Sprintf("GoReleaser configuration %q does not declare the expected plugin checksum identity.", configPath), "Declare a unit-prefixed checksums.txt name for published plugin artifacts.")
			}
		}
		if !integrationDoctorStringSliceContains(config.Release.IDs, expectedID) {
			add(unit.ID, "GORELEASER_RELEASE_ID_MISMATCH", fmt.Sprintf("GoReleaser release configuration does not include archive id %q.", expectedID), "Publish only the expected unit archive id.")
		}
	}
	return diagnostics
}

func integrationDoctorGoReleaserBuildByID(
	builds []integrationDoctorGoReleaserBuild,
	id string,
) (integrationDoctorGoReleaserBuild, bool) {
	for _, build := range builds {
		if build.ID == id {
			return build, true
		}
	}
	return integrationDoctorGoReleaserBuild{}, false
}

func integrationDoctorGoReleaserArchiveByID(
	archives []integrationDoctorGoReleaserArchive,
	id string,
) (integrationDoctorGoReleaserArchive, bool) {
	for _, archive := range archives {
		if archive.ID == id {
			return archive, true
		}
	}
	return integrationDoctorGoReleaserArchive{}, false
}

func integrationDoctorStringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func integrationDoctorStringSetContainsAll(values []string, wants ...string) bool {
	for _, want := range wants {
		if !integrationDoctorStringSliceContains(values, want) {
			return false
		}
	}
	return true
}

func integrationDoctorDiagnosticsContainErrors(diagnostics []integrationDoctorDiagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == integrationDoctorError {
			return true
		}
	}
	return false
}
