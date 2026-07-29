package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	evidencecmd "github.com/nekoman-hq/neko-cli/plugin/release/pkg/evidence"
	pluginindexcmd "github.com/nekoman-hq/neko-cli/plugin/release/pkg/pluginindex"
	releasecmd "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
	validatecmd "github.com/nekoman-hq/neko-cli/plugin/release/pkg/validate"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	releaseReadonlyHelperEnvironment = "NEKO_RELEASE_READONLY_HELPER"
	releaseReadonlyRootEnvironment   = "NEKO_RELEASE_READONLY_ROOT"
)

func TestReleaseReadonlyPluginHelperProcess(t *testing.T) {
	if os.Getenv(releaseReadonlyHelperEnvironment) != "1" {
		return
	}

	var request plugin.Request
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		t.Fatalf("decode plugin request: %v", err)
	}
	request.Context.WorkingDir = os.Getenv(releaseReadonlyRootEnvironment)
	log.Verbose = request.Context.Verbose

	var (
		response *plugin.Response
		err      error
	)
	switch request.Command {
	case "doctor":
		response, err = releasecmd.HandleDoctor(request)
	case "units":
		response, err = releasecmd.HandleUnits(request)
	case "plan":
		response, err = releasecmd.HandlePlan(request)
	case "validate":
		response, err = validatecmd.HandleValidate(request)
	case "ci-validate-context":
		response, err = releasecmd.HandleReleaseContextValidation(request)
	case "pipeline":
		response, err = releasecmd.HandlePipeline(request)
	case "patch":
		response, err = releasecmd.HandleRelease(request, releasecmd.Patch)
	case "minor":
		response, err = releasecmd.HandleRelease(request, releasecmd.Minor)
	case "major":
		response, err = releasecmd.HandleRelease(request, releasecmd.Major)
	case "resume":
		response, err = releasecmd.HandleResume(request)
	case "evidence":
		response, err = evidencecmd.HandleEvidence(request)
	case "evidence-archive":
		response, err = evidencecmd.HandleEvidenceArchive(request)
	case "plugin-index":
		response, err = pluginindexcmd.HandlePluginIndex(request)
	default:
		t.Fatalf("unexpected helper command %q", request.Command)
	}
	if err != nil {
		t.Fatalf("execute %s: %v", request.Command, err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		t.Fatalf("encode %s response: %v", request.Command, err)
	}
	os.Exit(0)
}

func TestReleaseReadonlyCommandsPreserveDomainJSONAcrossGlobalPresentationModes(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	repositoryRoot := releaseReadonlyRepositoryRoot(t)
	contextRoot, contextFlags := newReleaseReadonlyContextRepository(t)

	tests := []struct {
		name    string
		root    string
		command string
		flags   []string
	}{
		{name: "doctor", root: repositoryRoot, command: "doctor", flags: []string{"--unit", "cli"}},
		{name: "units", root: repositoryRoot, command: "units"},
		{name: "plan", root: repositoryRoot, command: "plan", flags: []string{"--change", "patch", "--unit", "cli"}},
		{name: "validate", root: repositoryRoot, command: "validate", flags: []string{"--unit", "cli"}},
		{name: "ci context", root: contextRoot, command: "ci-validate-context", flags: contextFlags},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plain, plainErr := executeReleaseReadonlyCommand(t, manifest, test.root, test.command, test.flags, releaseReadonlyMode{format: "json"})
			described, describedErr := executeReleaseReadonlyCommand(t, manifest, test.root, test.command, test.flags, releaseReadonlyMode{format: "json", describe: true})
			verboseOutput, verboseErr := executeReleaseReadonlyCommand(t, manifest, test.root, test.command, test.flags, releaseReadonlyMode{format: "json", verbose: true})

			if !samePluginExit(plainErr, describedErr) || !samePluginExit(plainErr, verboseErr) {
				t.Fatalf("global presentation flags changed exit behavior: plain=%v describe=%v verbose=%v", plainErr, describedErr, verboseErr)
			}
			plainResponse := decodeReleaseReadonlyPublicResponse(t, plain)
			describedResponse := decodeReleaseReadonlyPublicResponse(t, described)
			verboseResponse := decodeReleaseReadonlyPublicResponse(t, verboseOutput)
			if !reflect.DeepEqual(plainResponse.Data, describedResponse.Data) {
				t.Fatalf("describe changed %s domain JSON\nplain: %#v\ndescribe: %#v", test.command, plainResponse.Data, describedResponse.Data)
			}
			if !reflect.DeepEqual(plainResponse.Data, verboseResponse.Data) {
				t.Fatalf("verbose changed %s domain JSON\nplain: %#v\nverbose: %#v", test.command, plainResponse.Data, verboseResponse.Data)
			}
			for _, output := range []string{plain, described, verboseOutput} {
				for _, forbidden := range []string{"human_table", "human_properties", "describe_only", "\x1b[", "readonly-transport-secret"} {
					if strings.Contains(output, forbidden) {
						t.Fatalf("%s public JSON contains %q:\n%s", test.command, forbidden, output)
					}
				}
			}
		})
	}
}

func TestReleaseReadonlyCommandsKeepRedirectedAndNoColorOutputANSIAndCredentialFree(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	repositoryRoot := releaseReadonlyRepositoryRoot(t)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("GITHUB_TOKEN", "readonly-transport-secret")
	t.Setenv("GH_TOKEN", "readonly-transport-secret")

	tests := []struct {
		command string
		flags   []string
	}{
		{command: "doctor", flags: []string{"--unit", "cli"}},
		{command: "units"},
		{command: "plan", flags: []string{"--change", "patch", "--unit", "cli"}},
		{command: "validate", flags: []string{"--unit", "cli"}},
	}
	for _, test := range tests {
		for _, mode := range []releaseReadonlyMode{
			{},
			{describe: true},
			{verbose: true},
			{describe: true, verbose: true},
		} {
			output, _ := executeReleaseReadonlyCommand(t, manifest, repositoryRoot, test.command, test.flags, mode)
			if strings.Contains(output, "\x1b[") {
				t.Fatalf("%s redirected output contains ANSI in mode %#v:\n%q", test.command, mode, output)
			}
			if strings.Contains(output, "readonly-transport-secret") {
				t.Fatalf("%s output contains credential sentinel in mode %#v", test.command, mode)
			}
		}
	}
}

func TestPluginIndexOutputCollisionCharacterizesCurrentTransport(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	repositoryRoot := newPluginIndexTransportRepository(t)

	raw, rawErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "plugin-index", nil, releaseReadonlyMode{},
	)
	pretty, prettyErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "plugin-index", []string{"--pretty"}, releaseReadonlyMode{},
	)
	if rawErr != nil || prettyErr != nil {
		t.Fatalf("raw plugin-index exits: default=%v pretty=%v", rawErr, prettyErr)
	}
	if raw != pretty || !strings.HasPrefix(raw, "{\n  \"schemaVersion\": 1,") || !strings.HasSuffix(raw, "\n") {
		t.Fatalf("raw plugin-index contract changed:\ndefault=%q\npretty=%q", raw, pretty)
	}

	check, checkErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "plugin-index", []string{"--check"}, releaseReadonlyMode{},
	)
	if checkErr != nil {
		t.Fatalf("check plugin-index exit: %v", checkErr)
	}
	for _, want := range []string{"Status", "ok", "Plugins", "2", "Repository", "nekoman-hq/neko-cli"} {
		if !strings.Contains(check, want) {
			t.Fatalf("check output omitted %q:\n%s", want, check)
		}
	}

	for _, format := range []string{"json", "table"} {
		t.Run(format, func(t *testing.T) {
			output, err := executeReleaseReadonlyCommand(
				t,
				manifest,
				repositoryRoot,
				"plugin-index",
				[]string{"--output", format},
				releaseReadonlyMode{},
			)
			if err != nil {
				t.Fatalf("--output %s exit: %v", format, err)
			}
			target := filepath.Join(repositoryRoot, format)
			data, readErr := os.ReadFile(target)
			if readErr != nil {
				t.Fatalf("--output %s did not create the shadowed local target: %v", format, readErr)
			}
			if removeErr := os.Remove(target); removeErr != nil {
				t.Fatalf("remove test-owned collision file %s: %v", target, removeErr)
			}
			if string(data) != raw {
				t.Fatalf("--output %s persisted bytes differ from raw output", format)
			}
			for _, want := range []string{"Status", "written", "Output", format} {
				if !strings.Contains(output, want) {
					t.Fatalf("--output %s structured response omitted %q:\n%s", format, want, output)
				}
			}
			if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
				t.Fatalf("test-owned collision file was not removed immediately: %v", statErr)
			}
		})
	}

	arbitraryTarget := filepath.Join(repositoryRoot, "arbitrary-plugin-index.json")
	output, err := executeReleaseReadonlyCommand(
		t,
		manifest,
		repositoryRoot,
		"plugin-index",
		[]string{"--output", arbitraryTarget},
		releaseReadonlyMode{},
	)
	if err != nil {
		t.Fatalf("arbitrary local --output exit: %v", err)
	}
	if _, statErr := os.Stat(arbitraryTarget); statErr != nil {
		t.Fatalf("arbitrary local --output did not persist: %v", statErr)
	}
	if removeErr := os.Remove(arbitraryTarget); removeErr != nil {
		t.Fatalf("remove arbitrary test-owned collision file: %v", removeErr)
	}
	if !strings.Contains(output, arbitraryTarget) {
		t.Fatalf("arbitrary local --output response omitted target:\n%s", output)
	}
}

func TestReleasePipelineReferenceTransportRemainsStable(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	repositoryRoot := releaseReadonlyRepositoryRoot(t)

	defaultOutput, defaultErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "pipeline", []string{"--unit", "cli"}, releaseReadonlyMode{},
	)
	describeOutput, describeErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "pipeline", []string{"--unit", "cli"}, releaseReadonlyMode{describe: true},
	)
	jsonOutput, jsonErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "pipeline", []string{"--unit", "cli"}, releaseReadonlyMode{format: "json"},
	)
	if defaultErr != nil || describeErr != nil || jsonErr != nil {
		t.Fatalf("pipeline transport exits: default=%v describe=%v json=%v", defaultErr, describeErr, jsonErr)
	}
	for _, value := range []string{"Release Pipeline Inspection", "Summary"} {
		if !strings.Contains(defaultOutput, value) {
			t.Fatalf("pipeline default omitted %q:\n%s", value, defaultOutput)
		}
	}
	for _, value := range []string{"Verification", "Configured Pipeline", "Limitations"} {
		if !strings.Contains(describeOutput, value) {
			t.Fatalf("pipeline describe omitted %q:\n%s", value, describeOutput)
		}
	}
	response := decodeReleaseReadonlyPublicResponse(t, jsonOutput)
	if response.Data["schema_version"] != float64(1) {
		t.Fatalf("pipeline schema version = %#v, want 1", response.Data["schema_version"])
	}
}

func TestReleasePlanUsesDescribeForCompleteStructuredDetail(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	repositoryRoot := releaseReadonlyRepositoryRoot(t)
	flags := []string{"--change", "patch", "--unit", "cli"}

	defaultOutput, defaultErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "plan", flags, releaseReadonlyMode{},
	)
	describeOutput, describeErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "plan", flags, releaseReadonlyMode{describe: true},
	)
	verboseOutput, verboseErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "plan", flags, releaseReadonlyMode{verbose: true},
	)
	if defaultErr != nil || describeErr != nil || verboseErr != nil {
		t.Fatalf("plan transport exits: default=%v describe=%v verbose=%v", defaultErr, describeErr, verboseErr)
	}
	for _, value := range []string{
		"Release Plan",
		"Current version",
		"Next version",
		"Operations",
		"Mutation boundary",
	} {
		if !strings.Contains(defaultOutput, value) {
			t.Fatalf("plan default omitted %q:\n%s", value, defaultOutput)
		}
	}
	for _, value := range []string{
		"Plan Details",
		"Known Release Files",
		"Assumptions and Limitations",
	} {
		if strings.Contains(defaultOutput, value) {
			t.Fatalf("plan default exposed describe-only section %q:\n%s", value, defaultOutput)
		}
		if !strings.Contains(describeOutput, value) {
			t.Fatalf("plan describe omitted %q:\n%s", value, describeOutput)
		}
	}
	if defaultOutput != verboseOutput {
		t.Fatalf("plan verbose added execution narration:\ndefault:\n%s\nverbose:\n%s", defaultOutput, verboseOutput)
	}
}

func TestReleaseValidateKeepsShowCompatibilityAndUsesDescribeForDetail(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	repositoryRoot := releaseReadonlyRepositoryRoot(t)
	flags := []string{"--unit", "cli"}

	defaultOutput, defaultErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "validate", flags, releaseReadonlyMode{},
	)
	describeOutput, describeErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "validate", flags, releaseReadonlyMode{describe: true},
	)
	verboseOutput, verboseErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "validate", flags, releaseReadonlyMode{verbose: true},
	)
	showOutput, showErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "validate", append(flags, "--show"), releaseReadonlyMode{},
	)
	showDescribeOutput, showDescribeErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "validate", append(flags, "--show"), releaseReadonlyMode{describe: true},
	)
	if defaultErr != nil || describeErr != nil || verboseErr != nil || showErr != nil || showDescribeErr != nil {
		t.Fatalf(
			"validate transport exits: default=%v describe=%v verbose=%v show=%v show-describe=%v",
			defaultErr, describeErr, verboseErr, showErr, showDescribeErr,
		)
	}
	if defaultOutput != verboseOutput {
		t.Fatalf("validate verbose added query narration:\ndefault:\n%s\nverbose:\n%s", defaultOutput, verboseOutput)
	}
	if strings.Contains(defaultOutput, "Validated Units") || strings.Contains(defaultOutput, "Validation Scope") {
		t.Fatalf("validate default exposed structured detail:\n%s", defaultOutput)
	}
	for _, output := range []string{describeOutput, showDescribeOutput} {
		for _, want := range []string{"Validated Units", "Validation Scope", "Local configuration"} {
			if !strings.Contains(output, want) {
				t.Fatalf("validate detailed output omitted %q:\n%s", want, output)
			}
		}
	}
	if !strings.Contains(showOutput, "Validated Units") || strings.Contains(showOutput, "Validation Scope") {
		t.Fatalf("validate --show compatibility changed:\n%s", showOutput)
	}
	if strings.Count(showDescribeOutput, "Validated Units") != 1 {
		t.Fatalf("validate --show --describe duplicated the established detail view:\n%s", showDescribeOutput)
	}

	showJSON, showJSONErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "validate", append(flags, "--show"), releaseReadonlyMode{format: "json"},
	)
	showDescribeJSON, showDescribeJSONErr := executeReleaseReadonlyCommand(
		t,
		manifest,
		repositoryRoot,
		"validate",
		append(flags, "--show"),
		releaseReadonlyMode{format: "json", describe: true},
	)
	if showJSONErr != nil || showDescribeJSONErr != nil {
		t.Fatalf("validate show JSON exits: plain=%v describe=%v", showJSONErr, showDescribeJSONErr)
	}
	if !reflect.DeepEqual(
		decodeReleaseReadonlyPublicResponse(t, showJSON).Data,
		decodeReleaseReadonlyPublicResponse(t, showDescribeJSON).Data,
	) {
		t.Fatalf("describe changed validate --show machine facts:\nplain=%s\ndescribe=%s", showJSON, showDescribeJSON)
	}
}

func TestReleaseContextValidationUsesDescribeForCompleteChecks(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	repositoryRoot, flags := newReleaseReadonlyContextRepository(t)
	defaultOutput, defaultErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "ci-validate-context", flags, releaseReadonlyMode{},
	)
	describeOutput, describeErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "ci-validate-context", flags, releaseReadonlyMode{describe: true},
	)
	verboseOutput, verboseErr := executeReleaseReadonlyCommand(
		t, manifest, repositoryRoot, "ci-validate-context", flags, releaseReadonlyMode{verbose: true},
	)
	if defaultErr != nil || describeErr != nil || verboseErr != nil {
		t.Fatalf("context transport exits: default=%v describe=%v verbose=%v", defaultErr, describeErr, verboseErr)
	}
	if defaultOutput != verboseOutput {
		t.Fatalf("context verbose changed deterministic output:\ndefault:\n%s\nverbose:\n%s", defaultOutput, verboseOutput)
	}
	for _, want := range []string{"Validated Release Context", "Release context", "Valid", "Git consistency"} {
		if !strings.Contains(defaultOutput, want) {
			t.Fatalf("context default omitted %q:\n%s", want, defaultOutput)
		}
	}
	for _, hidden := range []string{"Context Checks", "Resolved Context", "GitHub Output Mapping", "Limitations"} {
		if strings.Contains(defaultOutput, hidden) || !strings.Contains(describeOutput, hidden) {
			t.Fatalf("context describe visibility for %q is incorrect:\ndefault:\n%s\ndescribe:\n%s", hidden, defaultOutput, describeOutput)
		}
	}
}

type releaseReadonlyMode struct {
	format   string
	describe bool
	verbose  bool
}

func executeReleaseReadonlyCommand(
	t *testing.T,
	manifest plugin.Manifest,
	repositoryRoot string,
	command string,
	flags []string,
	mode releaseReadonlyMode,
) (string, error) {
	t.Helper()
	oldDescribe, oldVerbose, oldFormat, oldGitHubOutput := describe, verbose, outputFormat, githubOutputFile
	describe, verbose, outputFormat, githubOutputFile = false, false, "table", ""
	t.Cleanup(func() {
		describe, verbose, outputFormat, githubOutputFile = oldDescribe, oldVerbose, oldFormat, oldGitHubOutput
	})
	t.Setenv(releaseReadonlyRootEnvironment, repositoryRoot)

	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = writeEnd
	output := make(chan []byte, 1)
	readErr := make(chan error, 1)
	go func() {
		data, err := io.ReadAll(readEnd)
		output <- data
		readErr <- err
	}()

	root := &cobra.Command{Use: "neko", SilenceUsage: true}
	root.PersistentFlags().BoolVar(&describe, "describe", false, "structured details")
	root.PersistentFlags().BoolVar(&verbose, "verbose", false, "execution logs")
	root.PersistentFlags().StringVar(&outputFormat, "output", "table", "output format")
	root.PersistentFlags().StringVar(&githubOutputFile, "github-output-file", "", "GitHub output file")
	root.AddCommand(CreatePluginCommand(manifest))
	args := []string{"release", command}
	args = append(args, flags...)
	if mode.describe {
		args = append(args, "--describe")
	}
	if mode.verbose {
		args = append(args, "--verbose")
	}
	if mode.format != "" && mode.format != "table" {
		args = append(args, "--output", mode.format)
	}
	root.SetArgs(args)
	executeErr := root.Execute()
	closeErr := writeEnd.Close()
	os.Stdout = oldStdout
	data := <-output
	copyErr := <-readErr
	_ = readEnd.Close()
	if closeErr != nil {
		t.Fatalf("close captured stdout: %v", closeErr)
	}
	if copyErr != nil {
		t.Fatalf("capture command stdout: %v", copyErr)
	}
	return string(data), executeErr
}

func installReleaseReadonlyHelperPlugin(t *testing.T) plugin.Manifest {
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
	script := "#!/bin/sh\nexec \"$NEKO_RELEASE_READONLY_TEST_BINARY\" -test.run=^TestReleaseReadonlyPluginHelperProcess$\n"
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write release helper plugin: %v", err)
	}
	t.Setenv(releaseReadonlyHelperEnvironment, "1")
	t.Setenv("NEKO_RELEASE_READONLY_TEST_BINARY", os.Args[0])
	return manifest
}

func releaseReadonlyRepositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	if _, err := workspace.ValidateRepositoryRoot(root); err != nil {
		t.Fatalf("validate repository root: %v", err)
	}
	return root
}

type releaseReadonlyPublicResponse struct {
	Data   map[string]any `json:"data"`
	Status string         `json:"status"`
}

func decodeReleaseReadonlyPublicResponse(t *testing.T, output string) releaseReadonlyPublicResponse {
	t.Helper()
	var response releaseReadonlyPublicResponse
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		t.Fatalf("decode public response: %v\n%s", err, output)
	}
	return response
}

func samePluginExit(left, right error) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return errors.Is(left, right) || left.Error() == right.Error()
}

func newPluginIndexTransportRepository(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("/private/tmp", "neko-plugin-index-collision-*")
	if err != nil {
		t.Fatalf("create plugin-index fixture: %v", err)
	}
	t.Cleanup(func() {
		if removeErr := os.RemoveAll(root); removeErr != nil {
			t.Errorf("remove plugin-index fixture %s: %v", root, removeErr)
		}
	})

	files := map[string]string{
		".neko/release.config.json": `{
  "schemaVersion": 2,
  "units": [
    {
      "id": "plugin-release",
      "displayName": "Release Plugin",
      "paths": ["plugin/release/**"],
      "workingDirectory": ".",
      "tagPrefix": "plugin-release/v",
      "kind": "plugin",
      "plugin": {
        "name": "release",
        "manifest": "plugin/release/manifest.json",
        "assetPrefix": "plugin-release",
        "binaryName": "plugin-release"
      },
      "executor": {
        "type": "goreleaser",
        "delivery": "github-actions",
        "workflow": ".github/workflows/release-plugin-release.yml"
      }
    },
    {
      "id": "plugin-ui",
      "displayName": "UI Plugin",
      "paths": ["plugin/ui/**"],
      "workingDirectory": ".",
      "tagPrefix": "plugin-ui/v",
      "kind": "plugin",
      "plugin": {
        "name": "ui",
        "manifest": "plugin/ui/manifest.json",
        "assetPrefix": "plugin-ui",
        "binaryName": "plugin-ui"
      },
      "executor": {
        "type": "goreleaser",
        "delivery": "github-actions",
        "workflow": ".github/workflows/release-plugin-ui.yml"
      }
    }
  ]
}
`,
		".neko/release.state.json": `{
  "schemaVersion": 2,
  "units": {
    "plugin-release": {"version": "4.2.0"},
    "plugin-ui": {"version": "1.0.1"}
  }
}
`,
		"plugin/release/manifest.json": `{
  "name": "release",
  "version": "4.2.0",
  "description": "Release management plugin"
}
`,
		"plugin/ui/manifest.json": `{
  "name": "ui",
  "version": "1.0.1",
  "description": "UI component plugin"
}
`,
	}
	for name, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if mkdirErr := os.MkdirAll(filepath.Dir(path), 0o755); mkdirErr != nil {
			t.Fatalf("create plugin-index fixture directory: %v", mkdirErr)
		}
		if writeErr := os.WriteFile(path, []byte(contents), 0o644); writeErr != nil {
			t.Fatalf("write plugin-index fixture %s: %v", name, writeErr)
		}
	}
	return root
}

func newReleaseReadonlyContextRepository(t *testing.T) (string, []string) {
	t.Helper()
	root := t.TempDir()
	runReleaseReadonlyGit(t, root, "init")
	runReleaseReadonlyGit(t, root, "config", "user.email", "readonly@example.invalid")
	runReleaseReadonlyGit(t, root, "config", "user.name", "Read-only Contract")
	if err := os.MkdirAll(filepath.Join(root, ".neko"), 0o755); err != nil {
		t.Fatalf("create .neko: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755); err != nil {
		t.Fatalf("create workflows: %v", err)
	}
	config := `{"schemaVersion":2,"units":[{"id":"api","displayName":"API","paths":["**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release.yml"}}]}`
	state := `{"schemaVersion":2,"units":{"api":{"version":"1.2.3"}}}`
	if err := os.WriteFile(filepath.Join(root, ".neko", "release.config.json"), []byte(config+"\n"), 0o644); err != nil {
		t.Fatalf("write release config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".neko", "release.state.json"), []byte(state+"\n"), 0o644); err != nil {
		t.Fatalf("write release state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".github", "workflows", "release.yml"), []byte("name: release\n"), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	runReleaseReadonlyGit(t, root, "add", ".")
	runReleaseReadonlyGit(t, root, "commit", "-m", "release context")
	sha := strings.TrimSpace(runReleaseReadonlyGit(t, root, "rev-parse", "HEAD"))
	runReleaseReadonlyGit(t, root, "tag", "api/v1.2.3", sha)
	return root, []string{
		"--unit", "api",
		"--version", "1.2.3",
		"--tag", "api/v1.2.3",
		"--release-sha", sha,
	}
}

func runReleaseReadonlyGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := append([]string{"-C", root}, args...)
	output, err := exec.Command("git", command...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(bytes.TrimSpace(output))
}

//nolint:staticcheck // Explicit V1 compatibility fixture.
func newReleaseLifecycleV1Repository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runReleaseReadonlyGit(t, root, "init")
	runReleaseReadonlyGit(t, root, "config", "user.email", "lifecycle@example.invalid")
	runReleaseReadonlyGit(t, root, "config", "user.name", "Lifecycle Contract")
	if err := releaseconfig.V1SaveConfigAt(root, releaseconfig.V1ReleaseConfig{
		ProjectName:   "legacy",
		ProjectOwner:  "example",
		ProjectType:   releaseconfig.V1ProjectTypeBackend,
		ReleaseSystem: releaseconfig.V1ReleaseTypeGoReleaser,
		Version:       "1.2.3",
	}); err != nil {
		t.Fatalf("write V1 release config: %v", err)
	}
	runReleaseReadonlyGit(t, root, "add", ".")
	runReleaseReadonlyGit(t, root, "commit", "-m", "V1 lifecycle fixture")
	runReleaseReadonlyGit(t, root, "tag", "v1.2.3")
	return root
}

func newReleaseLifecycleV2Repository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runReleaseReadonlyGit(t, root, "init")
	runReleaseReadonlyGit(t, root, "config", "user.email", "lifecycle@example.invalid")
	runReleaseReadonlyGit(t, root, "config", "user.name", "Lifecycle Contract")
	if err := os.MkdirAll(filepath.Join(root, ".neko"), 0o755); err != nil {
		t.Fatalf("create release directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755); err != nil {
		t.Fatalf("create workflow directory: %v", err)
	}
	files := map[string]string{
		".goreleaser.yml":                   "{}\n",
		".github/workflows/release-api.yml": "name: release\n",
		".neko/release.config.json": `{"schemaVersion":2,"units":[{"id":"api","displayName":"API","paths":["**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-api.yml"}}]}
`,
		".neko/release.state.json": `{"schemaVersion":2,"units":{"api":{"version":"2.3.4"}}}
`,
	}
	for path, contents := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(contents), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	runReleaseReadonlyGit(t, root, "add", ".")
	runReleaseReadonlyGit(t, root, "commit", "-m", "V2 lifecycle fixture")
	runReleaseReadonlyGit(t, root, "remote", "add", "origin", "https://github.com/example/repository.git")
	branch := strings.TrimSpace(runReleaseReadonlyGit(t, root, "symbolic-ref", "--short", "HEAD"))
	runReleaseReadonlyGit(t, root, "config", "branch."+branch+".remote", "origin")
	runReleaseReadonlyGit(t, root, "config", "branch."+branch+".merge", "refs/heads/"+branch)
	return root
}

func newReleaseLifecycleResumeRepository(t *testing.T) string {
	t.Helper()
	root := newReleaseLifecycleV2Repository(t)
	repository, err := releaseconfig.LoadV2Repository(root)
	if err != nil {
		t.Fatalf("load V2 release repository: %v", err)
	}
	execution, err := releasecmd.BuildV2ReleaseExecutionContext(root, repository.Units[0], releasecmd.Patch, false)
	if err != nil {
		t.Fatalf("build release execution context: %v", err)
	}
	knownFiles, err := releasecmd.NewKnownReleaseFiles(execution, nil)
	if err != nil {
		t.Fatalf("build known release files: %v", err)
	}
	baseCommit := strings.TrimSpace(runReleaseReadonlyGit(t, root, "rev-parse", "HEAD"))
	remote := strings.TrimSpace(runReleaseReadonlyGit(t, root, "remote", "get-url", "origin"))
	journal, err := releasecmd.BuildReleaseExecutionJournal(
		execution,
		releasecmd.BuildReleasePlan(execution),
		knownFiles,
		baseCommit,
		remote,
		time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("build release execution journal: %v", err)
	}
	if _, err := releasecmd.NewReleaseExecutionJournalStore(root).Prepare(journal); err != nil {
		t.Fatalf("prepare release execution journal: %v", err)
	}
	return root
}
