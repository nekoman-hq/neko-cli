package release

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

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
				Type: releaseconfig.ExecutorGoReleaser, Delivery: releaseconfig.DeliveryGitHubActions,
				Workflow: units[unitID],
			},
		})
		state.Units[unitID] = releaseconfig.V2UnitState{Version: fmt.Sprintf("0.%d.0", index+1)}
	}
	writeSupportingCommandJSON(t, releaseconfig.V2ConfigPath(root), config)
	writeSupportingCommandJSON(t, releaseconfig.V2StatePath(root), state)
	resolved, err := workspace.ValidateRepositoryRoot(root)
	if err != nil {
		t.Fatalf("validate repository root: %v", err)
	}
	return resolved
}

func writeSupportingCommandJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func validatedReleaseContextFixture() *ValidatedReleaseContext {
	return &ValidatedReleaseContext{
		UnitID: "api", DisplayName: "API μservice", Version: "2.4.0", TagPrefix: "api/v",
		Tag: "api/v2.4.0", ReleaseSHA: strings.Repeat("a", 40),
		WorkingDirectory: "services/api with spaces", Executor: "jreleaser", Delivery: "github-actions",
		Workflow: ".github/workflows/release.yml", GitObjectFormat: GitObjectFormatSHA1,
		HeadMatches: true, TagTargetMatches: true,
	}
}
