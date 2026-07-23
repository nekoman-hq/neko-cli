package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestCapturedPluginLogsAreANSIFreeBeforeSemanticParsing(t *testing.T) {
	t.Parallel()

	logs := parseLogOutput(strings.Join([]string{
		"10:11:12 \x1b[96m[config]\x1b[0m Configuration loaded",
		"10:11:13 \x1b[93m[pre-flight]\x1b[0m Warning: tag is missing",
		"10:11:14 \x1b[91m[exec]\x1b[0m Release failed",
		"10:11:15 \x1b[92m[exec]\x1b[0m \x1b[35mV$\x1b[0m Inspecting local journals",
	}, "\n"))

	want := []plugin.LogEntry{
		{Timestamp: "10:11:12", Level: "info", Category: "config", Message: "Configuration loaded"},
		{Timestamp: "10:11:13", Level: "warn", Category: "pre-flight", Message: "Warning: tag is missing"},
		{Timestamp: "10:11:14", Level: "error", Category: "exec", Message: "Release failed"},
		{Timestamp: "10:11:15", Level: "verbose", Category: "exec", Message: "V$ Inspecting local journals"},
	}
	if !reflect.DeepEqual(logs, want) {
		t.Fatalf("parsed colored logs\nwant: %#v\n got: %#v", want, logs)
	}
	for _, entry := range logs {
		if strings.Contains(entry.Category, "\x1b") || strings.Contains(entry.Message, "\x1b") {
			t.Fatalf("stored log contains ANSI: %#v", entry)
		}
	}
}

func TestCapturedPluginLogsPreserveUncoloredLinesOrderAndMalformedStderr(t *testing.T) {
	t.Parallel()

	logs := parseLogOutput(strings.Join([]string{
		"10:11:12 [config] Configuration loaded",
		"",
		"non-protocol stderr",
		"10:11:13 [exec] V$ Inspecting local journals",
	}, "\n"))
	if len(logs) != 3 {
		t.Fatalf("parsed log count = %d, want 3: %#v", len(logs), logs)
	}
	if logs[0].Category != "config" || logs[0].Message != "Configuration loaded" {
		t.Fatalf("first structured log changed: %#v", logs[0])
	}
	if logs[1].Category != "plugin" || logs[1].Level != "info" || logs[1].Message != "non-protocol stderr" {
		t.Fatalf("malformed stderr fallback changed: %#v", logs[1])
	}
	if logs[2].Category != "exec" || logs[2].Level != "verbose" || logs[2].Message != "V$ Inspecting local journals" {
		t.Fatalf("verbose structured log changed: %#v", logs[2])
	}
}

func TestDispatcherPreservesResponseLogsBeforeCapturedLogsWithoutExactTransportDuplicates(t *testing.T) {
	t.Parallel()

	dispatcher := installDispatcherTestPlugin(t, "combined", `#!/bin/sh
printf '%s\n' '{"status":"success","metadata":{"timestamp":"2026-07-23T00:00:00Z","plugin":"combined","version":"1.0.0","command":"inspect"},"logs":[{"timestamp":"\u001b[90m09:00:00\u001b[0m","level":"\u001b[90minfo\u001b[0m","category":"\u001b[96mresponse\u001b[0m","message":"\u001b[92mResponse-provided\u001b[0m"},{"timestamp":"10:11:12","level":"info","category":"\u001b[96mconfig\u001b[0m","message":"Shared entry"}]}'
printf '10:11:12 \033[96m[config]\033[0m Shared entry\n' >&2
printf '10:11:13 \033[92m[exec]\033[0m Captured entry\n' >&2
`)

	response, err := dispatcher.Dispatch(context.Background(), "combined", plugin.Request{Command: "inspect"})
	if err != nil {
		t.Fatalf("dispatch combined logs: %v", err)
	}
	want := []plugin.LogEntry{
		{Timestamp: "09:00:00", Level: "info", Category: "response", Message: "Response-provided"},
		{Timestamp: "10:11:12", Level: "info", Category: "config", Message: "Shared entry"},
		{Timestamp: "10:11:13", Level: "info", Category: "exec", Message: "Captured entry"},
	}
	if !reflect.DeepEqual(response.Logs, want) {
		t.Fatalf("combined logs\nwant: %#v\n got: %#v", want, response.Logs)
	}
}

func TestCapturedPluginLogsRemainANSIFreeInJSONRedirectsAndNoColor(t *testing.T) {
	dispatcher := installDispatcherTestPlugin(t, "colored", `#!/bin/sh
printf '%s\n' '{"status":"success","metadata":{"timestamp":"2026-07-23T00:00:00Z","plugin":"colored","version":"1.0.0","command":"inspect"},"data":{"result":"ready"}}'
printf '10:11:12 \033[96m[config]\033[0m Configuration loaded\n' >&2
printf '10:11:13 \033[92m[exec]\033[0m \033[35mV$\033[0m Inspecting local journals\n' >&2
`)
	response, err := dispatcher.Dispatch(context.Background(), "colored", plugin.Request{Command: "inspect", Context: plugin.Context{Verbose: true}})
	if err != nil {
		t.Fatalf("dispatch colored logs: %v", err)
	}

	var publicJSON bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{Format: renderer.FormatJSON, Verbose: true}, &publicJSON); err != nil {
		t.Fatalf("render JSON: %v", err)
	}
	if strings.Contains(publicJSON.String(), "\x1b") || strings.Contains(publicJSON.String(), `\u001b`) {
		t.Fatalf("JSON contains ANSI: %q", publicJSON.String())
	}
	var decoded struct {
		Logs []plugin.LogEntry `json:"logs"`
	}
	if err := json.Unmarshal(publicJSON.Bytes(), &decoded); err != nil {
		t.Fatalf("decode JSON logs: %v", err)
	}
	if len(decoded.Logs) != 2 || decoded.Logs[1].Level != "verbose" || decoded.Logs[1].Category != "exec" {
		t.Fatalf("JSON log metadata changed: %#v", decoded.Logs)
	}

	t.Setenv("NO_COLOR", "1")
	var redirected bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{Format: renderer.FormatTable, Verbose: true}, &redirected); err != nil {
		t.Fatalf("render redirected human output: %v", err)
	}
	if strings.Contains(redirected.String(), "\x1b") {
		t.Fatalf("redirected verbose output contains ANSI: %q", redirected.String())
	}
	for _, visible := range []string{"Execution Logs (2 entries)", "config Configuration loaded", "V$ exec V$ Inspecting local journals"} {
		if !strings.Contains(redirected.String(), visible) {
			t.Fatalf("redirected verbose output omitted %q:\n%s", visible, redirected.String())
		}
	}
}

func TestDispatcherPreservesSubprocessOutcomeContracts(t *testing.T) {
	t.Parallel()

	//nolint:govet // Table fields follow assertion readability rather than pointer packing.
	tests := []struct {
		name         string
		script       string
		wantResponse bool
		wantStatus   string
		wantExitCode int
		wantError    string
	}{
		{
			name:         "success",
			script:       "#!/bin/sh\nprintf '%s\\n' '{\"status\":\"success\",\"metadata\":{\"timestamp\":\"2026-07-23T00:00:00Z\",\"plugin\":\"success\",\"version\":\"1.0.0\",\"command\":\"inspect\"}}'\n",
			wantResponse: true,
			wantStatus:   "success",
		},
		{
			name:         "structured-error",
			script:       "#!/bin/sh\nprintf '%s\\n' '{\"status\":\"error\",\"metadata\":{\"timestamp\":\"2026-07-23T00:00:00Z\",\"plugin\":\"structured-error\",\"version\":\"1.0.0\",\"command\":\"inspect\"},\"error\":{\"code\":\"INVALID_REQUEST\",\"message\":\"unit is invalid\"},\"exit_code\":1}'\nexit 7\n",
			wantResponse: true,
			wantStatus:   "error",
			wantExitCode: 1,
		},
		{
			name:      "subprocess-failure",
			script:    "#!/bin/sh\nprintf '\\033[91mprocess failed\\033[0m\\n' >&2\nexit 7\n",
			wantError: "plugin execution failed: exit status 7\nStderr: process failed",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dispatcher := installDispatcherTestPlugin(t, test.name, test.script)
			response, err := dispatcher.Dispatch(context.Background(), test.name, plugin.Request{Command: "inspect"})
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("dispatch error = %v, want fragment %q", err, test.wantError)
				}
				if strings.Contains(err.Error(), "\x1b") {
					t.Fatalf("subprocess error contains ANSI: %q", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("dispatch: %v", err)
			}
			if !test.wantResponse || response.Status != test.wantStatus || response.ExitCode != test.wantExitCode {
				t.Fatalf("response = %#v", response)
			}
		})
	}
}

func installDispatcherTestPlugin(t *testing.T, name, script string) *Dispatcher {
	t.Helper()

	base, err := os.MkdirTemp("/private/tmp", "neko-cli-dispatcher-test-")
	if err != nil {
		t.Fatalf("create private temp directory: %v", err)
	}
	t.Cleanup(func() {
		if removeErr := os.RemoveAll(base); removeErr != nil {
			t.Errorf("remove private temp directory: %v", removeErr)
		}
	})
	installDir := filepath.Join(base, name)
	if err := os.MkdirAll(installDir, 0o700); err != nil {
		t.Fatalf("create plugin directory: %v", err)
	}
	binaryPath := filepath.Join(installDir, "plugin-"+name)
	if err := os.WriteFile(binaryPath, []byte(script), 0o700); err != nil {
		t.Fatalf("write plugin executable: %v", err)
	}
	return NewDispatcher(base)
}
