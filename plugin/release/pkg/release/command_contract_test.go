package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

const releaseSecretSentinel = "NEKO_TEST_TOKEN_MUST_NOT_APPEAR"

func TestHandleReleaseV2RequiresExplicitUnitForMultiUnitRepository(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	writeMultiUnitCommandRepository(t, root)
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleRelease(plugin.Request{
		Command: "patch",
		Flags:   map[string]any{"dry-run": true},
	}, Patch)

	assertReleaseCommandError(t, resp, err, "UNIT_RESOLUTION_FAILED", "release unit is required")
	if !strings.Contains(resp.Error.Message, "api, web") {
		t.Fatalf("unit-selection error did not preserve sorted available units: %q", resp.Error.Message)
	}
}

func TestHandleReleaseV2CommandContractPlansPatchMinorAndMajor(t *testing.T) {
	tests := []struct {
		releaseType Type
		nextVersion string
	}{
		{releaseType: Patch, nextVersion: "0.2.1"},
		{releaseType: Minor, nextVersion: "0.3.0"},
		{releaseType: Major, nextVersion: "1.0.0"},
	}

	for _, tt := range tests {
		t.Run(string(tt.releaseType), func(t *testing.T) {
			root := newGitHubActionsDispatchRepository(t)
			t.Setenv("GITHUB_TOKEN", "")
			withWorkingDirectoryRoot(t, root)

			resp, err := HandleRelease(plugin.Request{
				Command: string(tt.releaseType),
				Flags: map[string]any{
					"dry-run": true,
					"unit":    "api",
				},
			}, tt.releaseType)

			if err != nil || resp.Status != "success" {
				t.Fatalf("%s dry-run failed: response=%#v err=%v", tt.releaseType, resp, err)
			}
			if got := responseValueForProperty(t, resp.Data["items"], "New Version"); got != tt.nextVersion {
				t.Fatalf("%s dry-run next version = %s, expected %s", tt.releaseType, got, tt.nextVersion)
			}
			if resp.Metadata.Command != string(tt.releaseType) || resp.RendererHint != "table" {
				t.Fatalf("%s dry-run response contract changed: %#v", tt.releaseType, resp)
			}
		})
	}
}

func TestHandleReleaseAtUsesExplicitRootWithoutProcessWorkingDirectory(t *testing.T) {
	rootPath := newGitHubActionsDispatchRepository(t)
	otherRoot := t.TempDir()
	root, err := workspace.ValidateRepositoryRoot(rootPath)
	if err != nil {
		t.Fatalf("ValidateRepositoryRoot: %v", err)
	}
	t.Setenv("GITHUB_TOKEN", "")
	withWorkingDirectoryRoot(t, otherRoot)

	resp, err := HandleReleaseAt(root, plugin.Request{
		Command: "patch",
		Flags: map[string]any{
			"dry-run": true,
			"unit":    "api",
		},
	}, Patch)

	if err != nil || resp.Status != "success" {
		t.Fatalf("explicit-root dry-run failed: response=%#v err=%v", resp, err)
	}
	if got := responseValueForProperty(t, resp.Data["items"], "New Version"); got != "0.2.1" {
		t.Fatalf("dry-run next version = %s, expected 0.2.1", got)
	}
	if _, err := os.Stat(releaseconfig.V2ConfigPath(otherRoot)); !os.IsNotExist(err) {
		t.Fatalf("HandleReleaseAt touched process cwd; stat err=%v", err)
	}
}

func TestHandleReleaseV2RejectsUnknownUnit(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	writeMultiUnitCommandRepository(t, root)
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleRelease(plugin.Request{
		Command: "patch",
		Flags: map[string]any{
			"dry-run": true,
			"unit":    "worker",
		},
	}, Patch)

	assertReleaseCommandError(t, resp, err, "UNIT_RESOLUTION_FAILED", `unknown release unit "worker"`)
}

func TestHandleReleaseV2InvalidRepositoryReturnsConfigNotFound(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		state       string
		messagePart string
	}{
		{
			name:        "malformed config",
			config:      `{`,
			state:       `{"schemaVersion":2,"units":{"api":{"version":"0.2.0"}}}`,
			messagePart: "v2 config",
		},
		{
			name:        "malformed state",
			config:      singleUnitGitHubActionsConfig,
			state:       `{`,
			messagePart: "v2 state",
		},
		{
			name:        "config state mismatch",
			config:      singleUnitGitHubActionsConfig,
			state:       `{"schemaVersion":2,"units":{}}`,
			messagePart: `v2 state is missing unit "api"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newGitHubActionsDispatchRepository(t)
			if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(tt.config), 0644); err != nil {
				t.Fatalf("write config fixture: %v", err)
			}
			if err := os.WriteFile(releaseconfig.V2StatePath(root), []byte(tt.state), 0644); err != nil {
				t.Fatalf("write state fixture: %v", err)
			}
			withWorkingDirectoryRoot(t, root)

			resp, err := HandleRelease(plugin.Request{Command: "patch", Flags: map[string]any{"dry-run": true}}, Patch)

			assertReleaseCommandError(t, resp, err, "CONFIG_NOT_FOUND", tt.messagePart)
			if got := valueAsString(resp.Error.Details["hint"]); !strings.Contains(got, "neko release init") || !strings.Contains(got, "neko release migrate") {
				t.Fatalf("config error omitted recovery hint: %#v", resp.Error.Details)
			}
		})
	}
}

func TestHandleReleaseV2BlocksLocalDeliveryExecution(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	config := `{"schemaVersion":2,"units":[{"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"goreleaser","delivery":"local"}}]}`
	if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(config), 0644); err != nil {
		t.Fatalf("write local-delivery config: %v", err)
	}
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleRelease(plugin.Request{Command: "patch", Flags: map[string]any{"unit": "api"}}, Patch)

	assertReleaseCommandError(t, resp, err, "V2_LOCAL_DELIVERY_BLOCKED", "not available yet")
}

func TestHandleReleaseV2RejectsDirtyWorktreeWithoutLeakingToken(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	if err := os.WriteFile(filepath.Join(root, "unrelated.txt"), []byte("user work\n"), 0644); err != nil {
		t.Fatalf("write unrelated change: %v", err)
	}
	t.Setenv("GITHUB_TOKEN", releaseSecretSentinel)
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleRelease(plugin.Request{Command: "patch", Flags: map[string]any{"unit": "api"}}, Patch)

	assertReleaseCommandError(t, resp, err, "V2_GITHUB_ACTIONS_RELEASE_FAILED", "clean worktree")
	assertSecretAbsentFromResponse(t, resp)
	commonDir := strings.TrimSpace(gitOutput(t, root, "rev-parse", "--git-common-dir"))
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(root, commonDir)
	}
	if _, statErr := os.Stat(filepath.Join(commonDir, "neko", "release", "executions")); !os.IsNotExist(statErr) {
		t.Fatalf("dirty-worktree rejection created an execution journal: %v", statErr)
	}
}

func assertReleaseCommandError(t *testing.T, resp *plugin.Response, err error, code, messagePart string) {
	t.Helper()
	if err != nil {
		t.Fatalf("release command returned a Go error: %v", err)
	}
	if resp == nil || resp.Status != "error" || resp.Error == nil {
		t.Fatalf("expected plugin error response, got %#v", resp)
	}
	if resp.Error.Code != code || !strings.Contains(resp.Error.Message, messagePart) {
		t.Fatalf("unexpected release error contract: code=%q message=%q", resp.Error.Code, resp.Error.Message)
	}
	if resp.Metadata.Command != "patch" || resp.Metadata.Plugin == "" || resp.Metadata.Version == "" || resp.Metadata.Timestamp.IsZero() {
		t.Fatalf("release error metadata is incomplete: %#v", resp.Metadata)
	}
}

func assertSecretAbsentFromResponse(t *testing.T, resp *plugin.Response) {
	t.Helper()
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal plugin response: %v", err)
	}
	if strings.Contains(string(data), releaseSecretSentinel) {
		t.Fatal("secret sentinel appeared in the plugin response")
	}
}

func writeMultiUnitCommandRepository(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".github", "workflows", "release-web.yml"), []byte("name: release web\n"), 0644); err != nil {
		t.Fatalf("write web workflow: %v", err)
	}
	config := `{"schemaVersion":2,"units":[
  {"id":"web","paths":["web/**"],"workingDirectory":".","tagPrefix":"web/v","executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-web.yml"}},
  {"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-api.yml"}}
]}`
	state := `{"schemaVersion":2,"units":{"api":{"version":"0.2.0"},"web":{"version":"1.4.0"}}}`
	if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(config), 0644); err != nil {
		t.Fatalf("write multi-unit config: %v", err)
	}
	if err := os.WriteFile(releaseconfig.V2StatePath(root), []byte(state), 0644); err != nil {
		t.Fatalf("write multi-unit state: %v", err)
	}
}

const singleUnitGitHubActionsConfig = `{"schemaVersion":2,"units":[{"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-api.yml"}}]}`
