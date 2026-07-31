package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestPluginExitOwnershipProcessMatrix(t *testing.T) {
	root := repositoryRootForProcessTest(t)
	binary := buildCoreProcessTestBinary(t, root)

	tests := []struct {
		name             string
		response         string
		pluginStderr     string
		pluginExit       int
		wantDecoded      bool
		wantExitPresent  bool
		wantResponseExit int
		wantCoreExit     int
		wantRenderCount  int
		coreArgs         []string
	}{
		{name: "legacy response and zero subprocess", response: processResponse("success", "", "", ""), wantDecoded: true, wantCoreExit: 0, wantRenderCount: 1},
		{name: "explicit zero and zero subprocess", response: processResponse("success", "", "", `,"exit_code":0`), wantDecoded: true, wantExitPresent: true, wantCoreExit: 0, wantRenderCount: 1},
		{name: "explicit one and zero subprocess", response: processResponse("error", "CHECK_FAILED", "check failed", `,"exit_code":1`), wantDecoded: true, wantExitPresent: true, wantResponseExit: 1, wantCoreExit: 1, wantRenderCount: 1},
		{name: "explicit seven and zero subprocess", response: processResponse("success", "", "", `,"exit_code":7`), wantDecoded: true, wantExitPresent: true, wantResponseExit: 7, wantCoreExit: 7, wantRenderCount: 1},
		{name: "explicit zero owns nonzero subprocess", response: processResponse("success", "", "", `,"exit_code":0`), pluginExit: 7, wantDecoded: true, wantExitPresent: true, wantCoreExit: 0, wantRenderCount: 1},
		{name: "explicit one owns nonzero subprocess", response: processResponse("error", "CHECK_FAILED", "check failed", `,"exit_code":1`), pluginExit: 7, wantDecoded: true, wantExitPresent: true, wantResponseExit: 1, wantCoreExit: 1, wantRenderCount: 1},
		{name: "legacy response owns nonzero subprocess", response: processResponse("success", "", "", ""), pluginExit: 7, wantDecoded: true, wantCoreExit: 0, wantRenderCount: 1},
		{name: "malformed response falls back to subprocess", response: `{not-json`, pluginExit: 7, wantCoreExit: 1},
		{name: "absent response falls back to subprocess", pluginExit: 7, wantCoreExit: 1},
		{name: "response logs and captured stderr", response: processResponseWithLog(), pluginStderr: "10:11:13 [captured] captured log\n", pluginExit: 7, wantDecoded: true, wantExitPresent: true, wantCoreExit: 0, wantRenderCount: 1},
		{name: "mapper style error response", response: processResponse("error", "INVALID_REQUEST", "request is invalid", `,"exit_code":1`), wantDecoded: true, wantExitPresent: true, wantResponseExit: 1, wantCoreExit: 1, wantRenderCount: 1},
		{name: "invalid response exit below zero", response: processResponse("success", "", "", `,"exit_code":-1`), wantDecoded: true, wantExitPresent: true, wantResponseExit: -1, wantCoreExit: 1},
		{name: "invalid response exit above portable range", response: processResponse("success", "", "", `,"exit_code":126`), wantDecoded: true, wantExitPresent: true, wantResponseExit: 126, wantCoreExit: 1},
		{name: "invalid human error envelope", response: processResponse("error", "", "", `,"exit_code":1`), wantDecoded: true, wantExitPresent: true, wantResponseExit: 1, wantCoreExit: 1, coreArgs: []string{"probe", "run"}},
		{name: "renderer failure", response: processRendererFailureResponse(), wantDecoded: true, wantExitPresent: true, wantCoreExit: 1, coreArgs: []string{"probe", "run"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pluginDir, pluginPath := installProcessTestPlugin(t, test.response, test.pluginStderr, test.pluginExit)

			directStdout, directStderr, directExit := runProcessTestCommand(t, pluginPath, nil, nil)
			wantPluginStdout := test.response
			if wantPluginStdout != "" {
				wantPluginStdout += "\n"
			}
			if directStdout != wantPluginStdout || directStderr != test.pluginStderr || directExit != test.pluginExit {
				t.Fatalf("plugin observation = stdout %q, stderr %q, exit %d; want stdout %q, stderr %q, exit %d", directStdout, directStderr, directExit, wantPluginStdout, test.pluginStderr, test.pluginExit)
			}

			var decoded plugin.Response
			decodeErr := json.Unmarshal([]byte(test.response), &decoded)
			if got := decodeErr == nil; got != test.wantDecoded {
				t.Fatalf("decoded response = %t (error %v), want %t", got, decodeErr, test.wantDecoded)
			}
			if test.wantDecoded {
				code, present := decoded.ExplicitExitCode()
				if code != test.wantResponseExit || present != test.wantExitPresent {
					t.Fatalf("decoded exit = (%d, %t), want (%d, %t)", code, present, test.wantResponseExit, test.wantExitPresent)
				}
			}

			coreArgs := test.coreArgs
			if coreArgs == nil {
				coreArgs = []string{"probe", "run", "--output", "json"}
			}
			coreStdout, coreStderr, coreExit := runProcessTestCommand(t, binary, coreArgs, processTestEnvironment(pluginDir))
			if coreExit != test.wantCoreExit {
				t.Fatalf("Core exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", coreExit, test.wantCoreExit, coreStdout, coreStderr)
			}
			if got := strings.Count(coreStdout, "RENDER_MARKER"); got != test.wantRenderCount {
				t.Fatalf("Core render count = %d, want %d\nstdout:\n%s\nstderr:\n%s", got, test.wantRenderCount, coreStdout, coreStderr)
			}
			if test.wantRenderCount == 0 && coreStdout != "" {
				t.Fatalf("Core rendered partial output for invalid transport: %q", coreStdout)
			}
			if strings.Contains(coreStderr, "panic:") || strings.Contains(coreStderr, "goroutine ") {
				t.Fatalf("Core panicked for invalid plugin response:\n%s", coreStderr)
			}
		})
	}
}

func TestPluginExitRendersOnceForHumanJSONAndGitHub(t *testing.T) {
	root := repositoryRootForProcessTest(t)
	binary := buildCoreProcessTestBinary(t, root)
	response := `{"status":"success","metadata":{"timestamp":"2026-07-30T00:00:00Z","plugin":"probe","version":"1.0.0","command":"run"},"data":{"marker":"RENDER_MARKER"},"github_output":{"fields":[{"name":"marker","data_key":"marker"}]},"exit_code":7}`
	pluginDir, _ := installProcessTestPlugin(t, response, "", 1)

	for _, format := range []string{"table", "json"} {
		stdout, stderr, exitCode := runProcessTestCommand(t, binary, []string{"probe", "run", "--output", format}, processTestEnvironment(pluginDir))
		if exitCode != 7 || stderr != "" || strings.Count(stdout, "RENDER_MARKER") != 1 {
			t.Fatalf("%s rendering = exit %d, stdout %q, stderr %q", format, exitCode, stdout, stderr)
		}
	}

	destination := filepath.Join(t.TempDir(), "github-output")
	if err := os.WriteFile(destination, nil, 0o600); err != nil {
		t.Fatalf("create GitHub output: %v", err)
	}
	stdout, stderr, exitCode := runProcessTestCommand(
		t,
		binary,
		[]string{"probe", "run", "--output", "github", "--github-output-file", destination},
		processTestEnvironment(pluginDir),
	)
	if exitCode != 7 || stdout != "" || stderr != "" {
		t.Fatalf("GitHub rendering = exit %d, stdout %q, stderr %q", exitCode, stdout, stderr)
	}
	written, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read GitHub output: %v", err)
	}
	if strings.Count(string(written), "marker=RENDER_MARKER") != 1 {
		t.Fatalf("GitHub render count changed: %q", written)
	}
}

func TestGitHubRenderFailureOverridesExplicitResponseExit(t *testing.T) {
	root := repositoryRootForProcessTest(t)
	binary := buildCoreProcessTestBinary(t, root)
	response := `{"status":"success","metadata":{"timestamp":"2026-07-30T00:00:00Z","plugin":"probe","version":"1.0.0","command":"run"},"data":{"marker":"RENDER_MARKER"},"github_output":{"fields":[{"name":"marker","data_key":"marker"}]},"exit_code":7}`
	pluginDir, _ := installProcessTestPlugin(t, response, "", 0)

	stdout, stderr, exitCode := runProcessTestCommand(t, binary, []string{"probe", "run", "--output", "github"}, processTestEnvironment(pluginDir))
	if exitCode != 1 || stdout != "" || strings.Count(stderr, "GITHUB_OUTPUT_DESTINATION_UNAVAILABLE") != 1 {
		t.Fatalf("GitHub failure = exit %d, stdout %q, stderr %q", exitCode, stdout, stderr)
	}
}

func TestNonPluginCoreProcessExitBehaviorRemainsGeneric(t *testing.T) {
	root := repositoryRootForProcessTest(t)
	binary := buildCoreProcessTestBinary(t, root)
	emptyPluginDir := filepath.Join(t.TempDir(), "plugins")

	stdout, stderr, exitCode := runProcessTestCommand(t, binary, []string{"--help"}, processTestEnvironment(emptyPluginDir))
	if exitCode != 0 || !strings.Contains(stdout, "Neko CLI") || stderr != "" {
		t.Fatalf("Core help = exit %d, stdout %q, stderr %q", exitCode, stdout, stderr)
	}

	stdout, stderr, exitCode = runProcessTestCommand(t, binary, []string{"not-a-command"}, processTestEnvironment(emptyPluginDir))
	if exitCode != 1 || stdout != "" || !strings.Contains(stderr, "unknown command") {
		t.Fatalf("generic Core failure = exit %d, stdout %q, stderr %q", exitCode, stdout, stderr)
	}
}

func processResponse(status, code, message, exitField string) string {
	errorField := ""
	if code != "" {
		errorField = `,"error":{"code":"` + code + `","message":"` + message + `"}`
	}
	return `{"status":"` + status + `","metadata":{"timestamp":"2026-07-30T00:00:00Z","plugin":"probe","version":"1.0.0","command":"run"},"data":{"marker":"RENDER_MARKER"}` + errorField + exitField + `}`
}

func processResponseWithLog() string {
	return `{"status":"success","metadata":{"timestamp":"2026-07-30T00:00:00Z","plugin":"probe","version":"1.0.0","command":"run"},"data":{"marker":"RENDER_MARKER"},"logs":[{"timestamp":"10:11:12","level":"info","category":"response","message":"response log"}],"exit_code":0}`
}

func processRendererFailureResponse() string {
	return `{"status":"success","metadata":{"timestamp":"2026-07-30T00:00:00Z","plugin":"probe","version":"1.0.0","command":"run"},"data":{"marker":"RENDER_MARKER"},"human_properties":{"properties":[{"key":"marker","label":"Marker","role":"invalid"}]},"exit_code":0}`
}

func repositoryRootForProcessTest(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func buildCoreProcessTestBinary(t *testing.T, root string) string {
	t.Helper()
	directory, err := os.MkdirTemp("/private/tmp", "neko-cli-core-exit-")
	if err != nil {
		t.Fatalf("create Core build directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	binary := filepath.Join(directory, "neko")
	command := exec.Command("go", "build", "-o", binary, ".")
	command.Dir = root
	command.Env = append(processTestEnvironment(""), "GOPROXY=off", "GOSUMDB=off", "GOCACHE=/private/tmp/neko-cli-go-build")
	if output, buildErr := command.CombinedOutput(); buildErr != nil {
		t.Fatalf("build Core process test binary: %v\n%s", buildErr, output)
	}
	return binary
}

func installProcessTestPlugin(t *testing.T, response, stderr string, exitCode int) (string, string) {
	t.Helper()
	base, err := os.MkdirTemp("/private/tmp", "neko-cli-plugin-exit-")
	if err != nil {
		t.Fatalf("create plugin directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	directory := filepath.Join(base, "probe")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatalf("create probe directory: %v", err)
	}
	manifest := `{"name":"probe","version":"1.0.0","description":"Exit transport probe","commands":[{"name":"run","description":"Run probe","outputs":["table","json","github"]}],"renderer_types":["table","json","github"]}`
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write probe manifest: %v", err)
	}
	script := "#!/bin/sh\n"
	if response != "" {
		script += "printf '%s\\n' '" + response + "'\n"
	}
	if stderr != "" {
		script += "printf '%s' '" + stderr + "' >&2\n"
	}
	script += "exit " + strconv.Itoa(exitCode) + "\n"
	path := filepath.Join(directory, "plugin-probe")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write probe plugin: %v", err)
	}
	return base, path
}

func runProcessTestCommand(t *testing.T, path string, args, environment []string) (string, string, int) {
	t.Helper()
	command := exec.Command(path, args...)
	if environment != nil {
		command.Env = environment
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		t.Fatalf("execute %s: %v", path, err)
	}
	return stdout.String(), stderr.String(), exitError.ExitCode()
}

func processTestEnvironment(pluginDir string) []string {
	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "NEKO_PLUGIN_DIR=") || strings.HasPrefix(entry, "GITHUB_TOKEN=") || strings.HasPrefix(entry, "GH_TOKEN=") || strings.HasPrefix(entry, "GOPROXY=") || strings.HasPrefix(entry, "GOSUMDB=") || strings.HasPrefix(entry, "GOCACHE=") {
			continue
		}
		environment = append(environment, entry)
	}
	if pluginDir != "" {
		environment = append(environment, "NEKO_PLUGIN_DIR="+pluginDir)
	}
	return environment
}
