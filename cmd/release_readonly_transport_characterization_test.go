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

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
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
	Status string         `json:"status"`
	Data   map[string]any `json:"data"`
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
