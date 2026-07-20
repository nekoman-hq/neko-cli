package release

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	goreleaserfacts "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool/goreleaser"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"gopkg.in/yaml.v3"
)

func TestRepositoryDoctorVerifiesInstallationArtifactIdentity(t *testing.T) {
	result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, repositoryInspectionRoot(t), nil))
	for _, behavior := range repositoryWorkflowBehaviors() {
		fact, ok := integrationDoctorVerificationByCategory(result.Verifications, behavior.path, "installation_wiring")
		if !ok || fact.State != integrationDoctorVerified {
			t.Errorf("%s installation fact = %#v, present=%t", behavior.path, fact, ok)
		}
		for _, reference := range []string{"install.sh", "pkg/plugin/manager.go", "pkg/plugin/registry.go"} {
			if !slices.Contains(fact.References, reference) {
				t.Errorf("%s installation references = %v, missing %q", behavior.path, fact.References, reference)
			}
		}
	}
}

func TestIntegrationDoctorCLIInstallerContractMatrix(t *testing.T) {
	valid := readIntegrationDoctorRepositoryFile(t, "install.sh")
	for name, mutate := range map[string]func(string) string{
		"invalid version normalization": func(value string) string { return strings.Replace(value, "normalize_version", "normalize_release", 1) },
		"binary mismatch":               func(value string) string { return strings.ReplaceAll(value, "/neko", "/other") },
		"unsupported platform naming":   func(value string) string { return strings.ReplaceAll(value, "Windows", "Win32") },
	} {
		t.Run(name, func(t *testing.T) {
			if integrationDoctorCLIInstallerContractSupported([]byte(mutate(string(valid)))) {
				t.Fatal("mutated installer was accepted")
			}
		})
	}
	if !integrationDoctorCLIInstallerContractSupported(valid) {
		t.Fatal("repository CLI installer contract was not recognized")
	}
}

func TestIntegrationDoctorCLIArchiveContractMatrix(t *testing.T) {
	valid := goreleaserfacts.Config{Archives: []goreleaserfacts.Archive{{
		ID: "neko-cli", IDs: []string{"neko-cli"}, Formats: []string{"tar.gz"},
		NameTemplate:    "neko-cli_{{ .Os }}_{{ .Arch }}",
		FormatOverrides: []goreleaserfacts.FormatOverride{{Goos: "windows", Formats: []string{"zip"}}},
	}}}
	if !integrationDoctorCLIArchiveMatchesInstaller(valid, "neko-cli") {
		t.Fatal("valid CLI archive contract was rejected")
	}
	invalid := valid
	invalid.Archives = append([]goreleaserfacts.Archive(nil), valid.Archives...)
	invalid.Archives[0].NameTemplate = "other_{{ .Os }}_{{ .Arch }}"
	if integrationDoctorCLIArchiveMatchesInstaller(invalid, "neko-cli") {
		t.Fatal("CLI archive prefix mismatch was accepted")
	}
}

func TestIntegrationDoctorReleasePluginArtifactContractMatrix(t *testing.T) {
	unit := releaseconfig.ReleaseUnit{
		ID: "plugin-release", IsPlugin: true, PluginName: "release",
		PluginBinaryName: "plugin-release", PluginAssetPrefix: "plugin-release",
	}
	valid := goreleaserfacts.Config{
		Builds: []goreleaserfacts.Build{{ID: "plugin-release", Binary: "plugin-release"}},
		Archives: []goreleaserfacts.Archive{{
			ID: "plugin-release", IDs: []string{"plugin-release"}, Formats: []string{"tar.gz"},
			NameTemplate: "plugin-release_{{ .Os }}_{{ .Arch }}",
		}},
	}
	if !integrationDoctorPluginArchiveMatchesInstaller(valid, unit) {
		t.Fatal("valid Release plugin archive contract was rejected")
	}
	for name, mutate := range map[string]func(goreleaserfacts.Config, releaseconfig.ReleaseUnit) (goreleaserfacts.Config, releaseconfig.ReleaseUnit){
		"binary mismatch": func(config goreleaserfacts.Config, unit releaseconfig.ReleaseUnit) (goreleaserfacts.Config, releaseconfig.ReleaseUnit) {
			config.Builds[0].Binary = "other"
			return config, unit
		},
		"asset-prefix mismatch": func(config goreleaserfacts.Config, unit releaseconfig.ReleaseUnit) (goreleaserfacts.Config, releaseconfig.ReleaseUnit) {
			unit.PluginAssetPrefix = "other"
			return config, unit
		},
	} {
		t.Run(name, func(t *testing.T) {
			config := valid
			config.Builds = append([]goreleaserfacts.Build(nil), valid.Builds...)
			config.Archives = append([]goreleaserfacts.Archive(nil), valid.Archives...)
			config, mutatedUnit := mutate(config, unit)
			if integrationDoctorPluginArchiveMatchesInstaller(config, mutatedUnit) {
				t.Fatal("mismatched plugin artifact was accepted")
			}
		})
	}
	if _, ok := integrationDoctorReleasePluginUnit([]releaseconfig.ReleaseUnit{{IsPlugin: true, PluginName: "other"}}); ok {
		t.Fatal("plugin name mismatch was accepted as the Release plugin")
	}
}

func TestIntegrationDoctorInstallationRejectsUnpinnedAndLateInstallations(t *testing.T) {
	content := string(customIntegrationDoctorWorkflow(t))
	unpinned := strings.Replace(content, "${{ vars.NEKO_VERSION }}", "latest", 1)
	unpinnedRoot := parseIntegrationDoctorWorkflowBytes(t, []byte(unpinned))
	jobs := integrationDoctorWorkflowJobs(unpinnedRoot)
	cliStep, ok := integrationDoctorInstallationStep(jobs, func(step integrationDoctorWorkflowStep) bool { return strings.Contains(step.run, "install.sh") })
	if !ok || integrationDoctorPinnedRepositoryVariable(cliStep, "NEKO_VERSION") {
		t.Fatal("unpinned CLI installation was accepted")
	}

	codes := make([]string, 0)
	inspectIntegrationDoctorReleaseSteps(integrationDoctorWorkflowJob{steps: []integrationDoctorWorkflowStep{
		{run: "curl install.sh"},
		{run: "neko release ci-validate-context"},
		{run: "neko plugin install release --version 1.0.0"},
	}}, 1, func(_ integrationDoctorSeverity, code, _, _ string) {
		codes = append(codes, code)
	})
	if !slices.Contains(codes, "INSTALL_ORDER_INVALID") {
		t.Fatalf("late installation codes = %v", codes)
	}
}

func TestIntegrationDoctorRepositoryIdentityMatchesOriginGenerically(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	writeIntegrationDoctorBytes(t, filepath.Join(root, ".git", "config"), []byte("[remote \"origin\"]\n\turl = git@github.com:example/tools.git\n"))
	identity, err := (filesystemIntegrationDoctorRepositoryIdentityReader{}).ReadOrigin(root)
	if err != nil || identity.Name() != "example/tools" {
		t.Fatalf("identity = %#v, err=%v", identity, err)
	}
	if got := integrationDoctorInstallSourceIdentity("curl https://raw.githubusercontent.com/example/tools/v1/install.sh"); got != identity.Name() {
		t.Fatalf("install identity = %q, want %q", got, identity.Name())
	}
	if got := integrationDoctorInstallSourceIdentity("curl https://raw.githubusercontent.com/other/tools/v1/install.sh"); got == identity.Name() {
		t.Fatal("mismatched install source was accepted")
	}
}

func integrationDoctorVerificationByCategory(
	facts []integrationDoctorVerification,
	workflow, category string,
) (integrationDoctorVerification, bool) {
	for _, fact := range facts {
		if fact.Workflow == workflow && fact.Category == category {
			return fact, true
		}
	}
	return integrationDoctorVerification{}, false
}

func readIntegrationDoctorRepositoryFile(t *testing.T, relativePath string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repositoryInspectionRoot(t).Path(), filepath.FromSlash(relativePath)))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	return content
}

func parseIntegrationDoctorWorkflowBytes(t *testing.T, content []byte) *yaml.Node {
	t.Helper()
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatalf("parse workflow: %v", err)
	}
	root := workflowDocumentRoot(&document)
	if root == nil {
		t.Fatal("workflow root is nil")
	}
	return root
}
