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
