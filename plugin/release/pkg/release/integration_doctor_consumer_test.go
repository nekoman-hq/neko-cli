package release

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"gopkg.in/yaml.v3"
)

func TestIntegrationDoctorVerifiesRepositoryConsumerAndGoReleaserStructure(t *testing.T) {
	for _, behavior := range repositoryWorkflowBehaviors() {
		t.Run(behavior.unit, func(t *testing.T) {
			facts, diagnostics := inspectRepositoryConsumerFixture(t, behavior, nil)
			if integrationDoctorDiagnosticsContainErrors(diagnostics) {
				t.Fatalf("consumer diagnostics = %#v", diagnostics)
			}
			for _, category := range []string{"consumer_structure", "goreleaser_configuration"} {
				if !integrationDoctorVerificationCategoryIsVerified(facts, category) {
					t.Errorf("verified %s fact missing from %#v", category, facts)
				}
			}
		})
	}
}

func TestIntegrationDoctorConsumerStructureRejectsFocusedGaps(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]byte) []byte
		code   string
	}{
		{
			name: "missing snapshot build",
			mutate: func(content []byte) []byte {
				return bytes.Replace(content,
					[]byte("args: build --config ${{ env.GORELEASER_CONFIG }} --snapshot --clean --single-target"),
					[]byte("args: check --config ${{ env.GORELEASER_CONFIG }}"), 1)
			},
			code: "CONSUMER_BUILD_MISSING",
		},
		{
			name: "snapshot-only publication",
			mutate: func(content []byte) []byte {
				return bytes.Replace(content,
					[]byte("args: release --config ${{ env.GORELEASER_CONFIG }} --clean"),
					[]byte("args: release --config ${{ env.GORELEASER_CONFIG }} --snapshot --clean"), 1)
			},
			code: "PUBLICATION_MISSING",
		},
		{
			name: "publication skipped",
			mutate: func(content []byte) []byte {
				return bytes.Replace(content,
					[]byte("args: release --config ${{ env.GORELEASER_CONFIG }} --clean"),
					[]byte("args: release --config ${{ env.GORELEASER_CONFIG }} --clean --skip=publish"), 1)
			},
			code: "PUBLICATION_MISSING",
		},
		{
			name: "missing GoReleaser config",
			mutate: func(content []byte) []byte {
				return bytes.ReplaceAll(content, []byte(".goreleaser.cli.yaml"), []byte(".goreleaser.absent.yaml"))
			},
			code: "GORELEASER_CONFIG_MISSING",
		},
		{
			name: "invalid GoReleaser config reference",
			mutate: func(content []byte) []byte {
				return bytes.ReplaceAll(content, []byte(".goreleaser.cli.yaml"), []byte("../outside.yaml"))
			},
			code: "GORELEASER_CONFIG_REFERENCE_INVALID",
		},
		{
			name: "missing tests",
			mutate: func(content []byte) []byte {
				return bytes.Replace(content, []byte("go test ./..."), []byte("go vet ./..."), 1)
			},
			code: "CONSUMER_TESTS_MISSING",
		},
	}
	cli := repositoryWorkflowBehaviors()[0]
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, diagnostics := inspectRepositoryConsumerFixture(t, cli, test.mutate)
			assertIntegrationDoctorCodes(t, diagnostics, test.code)
		})
	}
}

func TestIntegrationDoctorGoReleaserIdentityRejectsFocusedMismatches(t *testing.T) {
	root := repositoryRootForSelfMigrationTest()
	repository, err := releaseconfig.LoadReleaseRepository(root)
	if err != nil {
		t.Fatalf("load repository: %v", err)
	}
	pluginUnit := integrationDoctorRepositoryUnit(t, repository, "plugin-release")
	content, err := os.ReadFile(filepath.Join(root, ".goreleaser.plugin-release.yaml"))
	if err != nil {
		t.Fatalf("read GoReleaser config: %v", err)
	}
	var valid integrationDoctorGoReleaserConfig
	if err := yaml.Unmarshal(content, &valid); err != nil {
		t.Fatalf("parse GoReleaser config: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*integrationDoctorGoReleaserConfig)
		code   string
	}{
		{name: "unknown build id", mutate: func(config *integrationDoctorGoReleaserConfig) {
			config.Builds[0].ID = "other"
		}, code: "GORELEASER_BUILD_ID_MISMATCH"},
		{name: "binary mismatch", mutate: func(config *integrationDoctorGoReleaserConfig) {
			config.Builds[0].Binary = "other"
		}, code: "GORELEASER_BINARY_MISMATCH"},
		{name: "archive mismatch", mutate: func(config *integrationDoctorGoReleaserConfig) {
			config.Archives[0].NameTemplate = "other_{{ .Os }}_{{ .Arch }}"
		}, code: "GORELEASER_ARCHIVE_MISMATCH"},
		{name: "checksum missing", mutate: func(config *integrationDoctorGoReleaserConfig) {
			config.Checksum = nil
		}, code: "GORELEASER_CHECKSUM_MISSING"},
		{name: "release id mismatch", mutate: func(config *integrationDoctorGoReleaserConfig) {
			config.Release.IDs = []string{"other"}
		}, code: "GORELEASER_RELEASE_ID_MISMATCH"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := valid
			config.Builds = append([]integrationDoctorGoReleaserBuild(nil), valid.Builds...)
			config.Archives = append([]integrationDoctorGoReleaserArchive(nil), valid.Archives...)
			config.Release.IDs = append([]string(nil), valid.Release.IDs...)
			if valid.Checksum != nil {
				checksum := *valid.Checksum
				config.Checksum = &checksum
			}
			test.mutate(&config)
			diagnostics := inspectIntegrationDoctorGoReleaserConfig(
				pluginUnit.Workflow,
				".goreleaser.plugin-release.yaml",
				config,
				[]releaseconfig.ReleaseUnit{pluginUnit},
			)
			assertIntegrationDoctorCodes(t, diagnostics, test.code)
		})
	}
}

func TestIntegrationDoctorRejectsPluginRegistryScopeAndOrderGaps(t *testing.T) {
	t.Run("normal unit registry mutation", func(t *testing.T) {
		behavior := repositoryWorkflowBehaviors()[0]
		_, diagnostics := inspectRepositoryConsumerFixture(t, behavior, func(content []byte) []byte {
			insertion := []byte(`      - name: Generate plugin registry index
        run: .github/scripts/generate-plugin-index.sh
      - name: Publish plugin registry index
        run: .github/scripts/publish-plugin-index.sh

`)
			return bytes.Replace(content, []byte("      - name: Publish summary\n"), append(insertion, []byte("      - name: Publish summary\n")...), 1)
		})
		assertIntegrationDoctorCodes(t, diagnostics, "NORMAL_UNIT_PLUGIN_REGISTRY_MUTATION")
	})

	t.Run("plugin registry before release", func(t *testing.T) {
		behavior := repositoryWorkflowBehaviors()[1]
		_, diagnostics := inspectRepositoryConsumerFixture(t, behavior, func(content []byte) []byte {
			content = bytes.Replace(content, []byte(".github/scripts/generate-plugin-index.sh"), []byte(".github/scripts/order-placeholder.sh"), 1)
			content = bytes.Replace(content, []byte("gh release create \"$RELEASE_TAG\""), []byte(".github/scripts/generate-plugin-index.sh\n          gh release create \"$RELEASE_TAG\""), 1)
			return content
		})
		assertIntegrationDoctorCodes(t, diagnostics, "PLUGIN_REGISTRY_ORDER_INVALID")
	})
}

func TestIntegrationDoctorRecognizesSuccessfulNoOpPublication(t *testing.T) {
	jobs := []integrationDoctorWorkflowJob{{steps: []integrationDoctorWorkflowStep{{
		name: "Publish release",
		run:  "set -euo pipefail\necho published",
	}}}}
	if !integrationDoctorHasPublicationNoOp(jobs) {
		t.Fatal("publication-shaped echo-only step was not classified as a no-op")
	}
}

func inspectRepositoryConsumerFixture(
	t *testing.T,
	behavior repositoryWorkflowBehavior,
	mutate func([]byte) []byte,
) ([]integrationDoctorVerification, []integrationDoctorDiagnostic) {
	t.Helper()
	repositoryRoot := repositoryRootForSelfMigrationTest()
	content, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(behavior.path)))
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	if mutate != nil {
		content = mutate(content)
	}
	var document yaml.Node
	if parseErr := yaml.Unmarshal(content, &document); parseErr != nil {
		t.Fatalf("parse workflow: %v", parseErr)
	}
	root := workflowDocumentRoot(&document)
	repository, err := releaseconfig.LoadReleaseRepository(repositoryRoot)
	if err != nil {
		t.Fatalf("load release repository: %v", err)
	}
	unit := integrationDoctorRepositoryUnit(t, repository, behavior.unit)
	return inspectIntegrationDoctorConsumerReleaseStructure(
		repositoryRoot,
		behavior.path,
		[]releaseconfig.ReleaseUnit{unit},
		root,
		integrationDoctorWorkflowJobs(root),
		filesystemIntegrationDoctorRepositoryFileReader{},
	)
}

func integrationDoctorRepositoryUnit(
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
	t.Fatalf("unit %q not found", unitID)
	return releaseconfig.ReleaseUnit{}
}

func integrationDoctorVerificationCategoryIsVerified(
	facts []integrationDoctorVerification,
	category string,
) bool {
	for _, fact := range facts {
		if fact.Category == category && fact.State == integrationDoctorVerified {
			return true
		}
	}
	return false
}
