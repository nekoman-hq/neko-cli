package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

type repositoryWorkflowBehavior struct {
	unit             string
	path             string
	goreleaserConfig string
	buildCommand     string
	packageCommand   string
	publishCommand   string
	versionVariable  string
	pluginRegistry   bool
}

func repositoryWorkflowBehaviors() []repositoryWorkflowBehavior {
	return []repositoryWorkflowBehavior{
		{
			unit: "cli", path: ".github/workflows/release-neko-cli.yml",
			goreleaserConfig: ".goreleaser.cli.yaml",
			buildCommand:     "build --config ${{ env.GORELEASER_CONFIG }} --snapshot --clean --single-target",
			packageCommand:   "check --config ${{ env.GORELEASER_CONFIG }}",
			publishCommand:   "release --config ${{ env.GORELEASER_CONFIG }} --clean",
			versionVariable:  "CLI_VERSION",
		},
		{
			unit: "plugin-release", path: ".github/workflows/release-plugin-release.yml",
			goreleaserConfig: ".goreleaser.plugin-release.yaml",
			buildCommand:     "build --config ${{ env.GORELEASER_CONFIG }} --snapshot --clean --single-target",
			packageCommand:   "release --config ${{ env.GORELEASER_CONFIG }} --snapshot --clean --skip=publish",
			publishCommand:   "gh release create \"$RELEASE_TAG\"",
			versionVariable:  "PLUGIN_RELEASE_VERSION",
			pluginRegistry:   true,
		},
		{
			unit: "plugin-ui", path: ".github/workflows/release-plugin-ui.yml",
			goreleaserConfig: ".goreleaser.plugin-ui.yaml",
			buildCommand:     "build --config ${{ env.GORELEASER_CONFIG }} --snapshot --clean --single-target",
			packageCommand:   "release --config ${{ env.GORELEASER_CONFIG }} --snapshot --clean --skip=publish",
			publishCommand:   "gh release create \"$RELEASE_TAG\"",
			versionVariable:  "PLUGIN_UI_VERSION",
			pluginRegistry:   true,
		},
	}
}

func repositoryRootForSelfMigrationTest() string {
	return filepath.Clean(filepath.Join("..", "..", "..", ".."))
}

func repositoryInspectionRoot(t *testing.T) workspace.RepositoryRoot {
	t.Helper()
	root, err := workspace.ResolveInspectionRepositoryRoot(repositoryRootForSelfMigrationTest())
	if err != nil {
		t.Fatalf("resolve repository inspection root: %v", err)
	}
	return root
}

// integrationDoctorPinnedConsumerWorkflowFixture holds the published
// pinned-toolchain consumer workflow contract. The repository's own workflows
// build their toolchain from the exact checkout, so the pinned installation and
// repository-variable boundaries need a dedicated fixture.
const integrationDoctorPinnedConsumerWorkflowFixture = "testdata/pinned-consumer-release-workflow.yml"

// integrationDoctorPinnedRepositoryFiles lists the repository contracts the
// pinned installation boundary is verified against.
var integrationDoctorPinnedRepositoryFiles = []string{
	"go.mod",
	"install.sh",
	"pkg/plugin/manager.go",
	"pkg/plugin/registry.go",
	"plugin/release/main.go",
	"plugin/release/manifest.json",
	"plugin/ui/manifest.json",
	".goreleaser.cli.yaml",
	".goreleaser.plugin-release.yaml",
	".goreleaser.plugin-ui.yaml",
	".neko/release.config.json",
	".neko/release.state.json",
	".github/scripts/generate-plugin-index.sh",
	".github/scripts/publish-plugin-index.sh",
	".github/workflows/release-plugin-release.yml",
	".github/workflows/release-plugin-ui.yml",
	".github/actions/setup-source-neko-toolchain/action.yml",
	".github/actions/validate-neko-release-context/action.yml",
	".github/actions/publish-plugin-index/action.yml",
}

// newIntegrationDoctorPinnedInstallationRepository mirrors the repository's
// installation, artifact, and release-source contracts into a temporary root
// whose cli workflow installs the pinned published Neko toolchain.
func newIntegrationDoctorPinnedInstallationRepository(t *testing.T) workspace.RepositoryRoot {
	t.Helper()
	root := t.TempDir()
	writeIntegrationDoctorFixtureFile(t, root, ".git/config",
		[]byte("[remote \"origin\"]\n\turl = https://github.com/nekoman-hq/neko-cli.git\n"))
	for _, relativePath := range integrationDoctorPinnedRepositoryFiles {
		writeIntegrationDoctorFixtureFile(t, root, relativePath, readIntegrationDoctorRepositoryFile(t, relativePath))
	}
	workflow, err := os.ReadFile(integrationDoctorPinnedConsumerWorkflowFixture)
	if err != nil {
		t.Fatalf("read pinned consumer workflow fixture: %v", err)
	}
	writeIntegrationDoctorFixtureFile(t, root, ".github/workflows/release-neko-cli.yml", workflow)
	resolved, err := workspace.ResolveInspectionRepositoryRoot(root)
	if err != nil {
		t.Fatalf("resolve pinned installation repository root: %v", err)
	}
	return resolved
}

func writeIntegrationDoctorFixtureFile(t *testing.T, root, relativePath string, content []byte) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatalf("create %s parent: %v", relativePath, err)
	}
	if err := os.WriteFile(target, content, 0644); err != nil {
		t.Fatalf("write %s: %v", relativePath, err)
	}
}

// newIntegrationDoctorLocalActionOverlay copies the repository's local actions
// into a temporary root and rewrites one action definition, so composite action
// contracts can be characterized without touching the repository itself.
func newIntegrationDoctorLocalActionOverlay(
	t *testing.T,
	directory string,
	mutate func(string) string,
) string {
	t.Helper()
	overlay := t.TempDir()
	source := filepath.Join(repositoryRootForSelfMigrationTest(), ".github", "actions")
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatalf("read local actions: %v", err)
	}
	mutated := false
	for _, entry := range entries {
		relative := filepath.Join(".github", "actions", entry.Name(), "action.yml")
		content, readErr := os.ReadFile(filepath.Join(repositoryRootForSelfMigrationTest(), relative))
		if readErr != nil {
			continue
		}
		if filepath.ToSlash(filepath.Dir(relative)) == directory {
			content = []byte(mutate(string(content)))
			mutated = true
		}
		target := filepath.Join(overlay, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatalf("create overlay action directory: %v", err)
		}
		if err := os.WriteFile(target, content, 0644); err != nil {
			t.Fatalf("write overlay action: %v", err)
		}
	}
	if !mutated {
		t.Fatalf("local action %q was not found for mutation", directory)
	}
	return overlay
}

func newWorkflowScaffoldRepository(t *testing.T, units map[string]string) workspace.RepositoryRoot {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("create git marker: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, releaseconfig.V2Directory), 0755); err != nil {
		t.Fatalf("create V2 directory: %v", err)
	}
	unitIDs := make([]string, 0, len(units))
	for unitID := range units {
		unitIDs = append(unitIDs, unitID)
	}
	slices.Sort(unitIDs)
	config := releaseconfig.V2ReleaseConfig{SchemaVersion: 2}
	state := releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{}}
	for index, unitID := range unitIDs {
		config.Units = append(config.Units, releaseconfig.V2Unit{
			ID: unitID, Paths: []string{unitID + "/**"}, WorkingDirectory: ".", TagPrefix: unitID + "/v",
			Executor: releaseconfig.V2Executor{
				Type: releaseconfig.ExecutorGoReleaser, Delivery: releaseconfig.DeliveryGitHubActions, Workflow: units[unitID],
			},
		})
		state.Units[unitID] = releaseconfig.V2UnitState{Version: fmt.Sprintf("0.%d.0", index+1)}
	}
	writeWorkflowScaffoldJSON(t, releaseconfig.V2ConfigPath(root), config)
	writeWorkflowScaffoldJSON(t, releaseconfig.V2StatePath(root), state)
	resolved, err := workspace.ValidateRepositoryRoot(root)
	if err != nil {
		t.Fatalf("validate repository root: %v", err)
	}
	return resolved
}

func writeWorkflowScaffoldJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
