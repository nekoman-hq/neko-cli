package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	initcmd "github.com/nekoman-hq/neko-cli/plugin/release/pkg/init"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/migrate"
	releasecmd "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

const releaseSetupHelperEnvironment = "NEKO_RELEASE_SETUP_HELPER"

func TestReleaseSetupPluginHelperProcess(t *testing.T) {
	if os.Getenv(releaseSetupHelperEnvironment) != "1" {
		return
	}

	var request plugin.Request
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		t.Fatalf("decode plugin request: %v", err)
	}
	request.Context.WorkingDir = os.Getenv(releaseReadonlyRootEnvironment)
	log.Verbose = request.Context.Verbose

	root, err := workspace.ResolveRepositoryRoot(request.Context.WorkingDir)
	if err != nil {
		t.Fatalf("resolve setup repository: %v", err)
	}
	var response *plugin.Response
	switch request.Command {
	case "init":
		response, err = initcmd.HandleInitAt(root, request)
	case "unit-add":
		response, err = initcmd.HandleUnitAddAt(root, request)
	case "migrate":
		response, err = migrate.HandleMigrate(request)
	case "github-workflow-init":
		response, err = releasecmd.HandleGitHubWorkflowInitAt(root, request)
	default:
		t.Fatalf("unexpected setup helper command %q", request.Command)
	}
	if err != nil {
		t.Fatalf("execute %s: %v", request.Command, err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		t.Fatalf("encode %s response: %v", request.Command, err)
	}
	os.Exit(0)
}

func TestReleaseSetupCommandsPreserveDomainJSONAcrossGlobalModes(t *testing.T) {
	manifest := installReleaseSetupHelperPlugin(t)
	tests := []struct {
		name    string
		command string
		fixture func(*testing.T) (string, []string)
	}{
		{name: "init", command: "init", fixture: newReleaseSetupInitRepository},
		{name: "unit add", command: "unit-add", fixture: newReleaseSetupUnitAddRepository},
		{name: "migrate dry run", command: "migrate", fixture: newReleaseSetupMigrationRepository},
		{name: "workflow dry run", command: "github-workflow-init", fixture: newReleaseSetupWorkflowRepository},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plainRoot, flags := test.fixture(t)
			plain, plainErr := executeReleaseReadonlyCommand(
				t, manifest, plainRoot, test.command, flags, releaseReadonlyMode{format: "json"},
			)
			describeRoot, flags := test.fixture(t)
			described, describedErr := executeReleaseReadonlyCommand(
				t, manifest, describeRoot, test.command, flags, releaseReadonlyMode{format: "json", describe: true},
			)
			verboseRoot, flags := test.fixture(t)
			verboseOutput, verboseErr := executeReleaseReadonlyCommand(
				t, manifest, verboseRoot, test.command, flags, releaseReadonlyMode{format: "json", verbose: true},
			)
			if !samePluginExit(plainErr, describedErr) || !samePluginExit(plainErr, verboseErr) {
				t.Fatalf("global modes changed exit behavior: plain=%v describe=%v verbose=%v", plainErr, describedErr, verboseErr)
			}
			plainResponse := decodeReleaseReadonlyPublicResponse(t, plain)
			plainResponse.Data = normalizeReleaseSetupData(t, plainResponse.Data, plainRoot)
			describeResponse := decodeReleaseReadonlyPublicResponse(t, described)
			describeResponse.Data = normalizeReleaseSetupData(t, describeResponse.Data, describeRoot)
			verboseResponse := decodeReleaseReadonlyPublicResponse(t, verboseOutput)
			verboseResponse.Data = normalizeReleaseSetupData(t, verboseResponse.Data, verboseRoot)
			if !reflect.DeepEqual(plainResponse.Data, describeResponse.Data) {
				t.Fatalf("describe changed %s domain data\nplain=%#v\ndescribe=%#v", test.command, plainResponse.Data, describeResponse.Data)
			}
			if !reflect.DeepEqual(plainResponse.Data, verboseResponse.Data) {
				t.Fatalf("verbose changed %s domain data\nplain=%#v\nverbose=%#v", test.command, plainResponse.Data, verboseResponse.Data)
			}
			for _, output := range []string{plain, described, verboseOutput} {
				for _, forbidden := range []string{"human_table", "human_properties", "describe_only", "\x1b[", "setup-transport-secret"} {
					if strings.Contains(output, forbidden) {
						t.Fatalf("%s public JSON contains %q:\n%s", test.command, forbidden, output)
					}
				}
			}
		})
	}
}

func normalizeReleaseSetupData(t *testing.T, data map[string]any, root string) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("encode setup data: %v", err)
	}
	normalized := strings.ReplaceAll(string(encoded), root, "<fixture-root>")
	var result map[string]any
	if err := json.Unmarshal([]byte(normalized), &result); err != nil {
		t.Fatalf("decode normalized setup data: %v", err)
	}
	return result
}

func installReleaseSetupHelperPlugin(t *testing.T) plugin.Manifest {
	t.Helper()
	manifestData, err := os.ReadFile(filepath.Join("..", "plugin", "release", "manifest.json"))
	if err != nil {
		t.Fatalf("read release manifest: %v", err)
	}
	var manifest plugin.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("decode release manifest: %v", err)
	}
	pluginDir := installFakePlugin(t, manifest)
	restorePluginDir(t, pluginDir)

	binaryPath := filepath.Join(pluginDir, manifest.Name, "plugin-"+manifest.Name)
	script := "#!/bin/sh\nexec \"$NEKO_RELEASE_SETUP_TEST_BINARY\" -test.run=^TestReleaseSetupPluginHelperProcess$\n"
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write setup helper plugin: %v", err)
	}
	t.Setenv(releaseSetupHelperEnvironment, "1")
	t.Setenv("NEKO_RELEASE_SETUP_TEST_BINARY", os.Args[0])
	t.Setenv("GITHUB_TOKEN", "setup-transport-secret")
	t.Setenv("GH_TOKEN", "setup-transport-secret")
	return manifest
}

func newReleaseSetupInitRepository(t *testing.T) (string, []string) {
	t.Helper()
	root := newReleaseSetupRepository(t)
	writeReleaseSetupFile(t, filepath.Join(root, ".github", "workflows", "release-api.yml"), "name: release api\n")
	return root, []string{
		"--unit", "api",
		"--display-name", "API",
		"--version", "1.2.3",
		"--executor", "goreleaser",
		"--delivery", "github-actions",
		"--workflow", ".github/workflows/release-api.yml",
		"--tag-prefix", "api/v",
		"--working-directory", ".",
		"--paths", "services/api/**",
	}
}

func newReleaseSetupUnitAddRepository(t *testing.T) (string, []string) {
	t.Helper()
	root := newReleaseSetupRepository(t)
	writeReleaseSetupFile(t, filepath.Join(root, ".neko", "release.config.json"), `{
  "schemaVersion": 2,
  "units": [
    {
      "id": "api",
      "displayName": "API",
      "paths": ["services/api/**"],
      "workingDirectory": ".",
      "tagPrefix": "api/v",
      "executor": {
        "type": "goreleaser",
        "delivery": "github-actions",
        "workflow": ".github/workflows/release-api.yml"
      }
    }
  ]
}
`)
	writeReleaseSetupFile(t, filepath.Join(root, ".neko", "release.state.json"), `{
  "schemaVersion": 2,
  "units": {
    "api": {"version": "1.2.3"}
  }
}
`)
	writeReleaseSetupFile(t, filepath.Join(root, ".github", "workflows", "release-api.yml"), "name: release api\n")
	writeReleaseSetupFile(t, filepath.Join(root, ".github", "workflows", "release-web.yml"), "name: release web\n")
	return root, []string{
		"--unit", "web",
		"--display-name", "Web",
		"--version", "2.3.4",
		"--executor", "goreleaser",
		"--delivery", "github-actions",
		"--workflow", ".github/workflows/release-web.yml",
		"--tag-prefix", "web/v",
		"--working-directory", ".",
		"--paths", "services/web/**",
	}
}

func newReleaseSetupMigrationRepository(t *testing.T) (string, []string) {
	t.Helper()
	root := t.TempDir()
	runReleaseReadonlyGit(t, root, "init")
	writeReleaseSetupFile(t, filepath.Join(root, ".release.neko.json"), `{
  "project-name": "example",
  "project-owner": "example-owner",
  "project-type": "backend",
  "release-system": "jreleaser",
  "version": "1.2.3"
}
`)
	return root, []string{"--dry-run"}
}

func newReleaseSetupWorkflowRepository(t *testing.T) (string, []string) {
	t.Helper()
	root, _ := newReleaseSetupUnitAddRepository(t)
	return root, []string{"--unit", "api", "--dry-run"}
}

func newReleaseSetupRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("create Git marker: %v", err)
	}
	return root
}

func writeReleaseSetupFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
