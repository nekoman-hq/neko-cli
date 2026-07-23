package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/spf13/cobra"
)

func TestPluginDescribeTransportFiltersStructuredSectionsAndKeepsLogsIndependent(t *testing.T) {
	manifest, requestPath := installDescribeTransportPlugin(t)

	tests := []struct {
		name              string
		describe, verbose bool
		wantMetadata      bool
		wantLogs          bool
		wantDetails       bool
	}{
		{name: "default"},
		{name: "describe", describe: true, wantMetadata: true, wantDetails: true},
		{name: "verbose", verbose: true, wantLogs: true},
		{name: "combined", describe: true, verbose: true, wantMetadata: true, wantLogs: true, wantDetails: true},
	}
	requests := make(map[string]plugin.Request, len(tests))
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := executeDescribeTransportCommand(t, manifest, test.describe, test.verbose, "table")
			for _, visible := range []string{"Release Pipeline Inspection", "Summary", "Findings", "Expected release asset is missing."} {
				assertContains(t, output, visible)
			}
			for value, want := range map[string]bool{
				"Command Metadata":       test.wantMetadata,
				"Execution Logs":         test.wantLogs,
				"Verification Facts":     test.wantDetails,
				"Configured Pipeline":    test.wantDetails,
				"Complete limitation.":   test.wantDetails,
				"Healthy local workflow": test.wantDetails,
			} {
				if strings.Contains(output, value) != want {
					t.Fatalf("%s visibility for %q = %t, want %t:\n%s", test.name, value, strings.Contains(output, value), want, output)
				}
			}

			data, err := os.ReadFile(requestPath)
			if err != nil {
				t.Fatalf("read dispatched request: %v", err)
			}
			if strings.Contains(string(data), "describe") {
				t.Fatalf("describe leaked into plugin request: %s", data)
			}
			var request plugin.Request
			if err := json.Unmarshal(data, &request); err != nil {
				t.Fatalf("decode dispatched request: %v", err)
			}
			if request.Context.Verbose != test.verbose {
				t.Fatalf("request verbose = %t, want %t", request.Context.Verbose, test.verbose)
			}
			if !reflect.DeepEqual(request.Flags, map[string]any{"unit": "cli"}) {
				t.Fatalf("plugin request flags = %#v, want only the command-local unit flag", request.Flags)
			}
			requests[test.name] = request
		})
	}

	if !reflect.DeepEqual(requests["default"], requests["describe"]) {
		t.Fatalf("describe changed the plugin request\ndefault: %#v\ndescribe: %#v", requests["default"], requests["describe"])
	}
}

func TestPluginDescribeTransportKeepsPublicJSONIdentical(t *testing.T) {
	manifest, _ := installDescribeTransportPlugin(t)
	plain := executeDescribeTransportCommand(t, manifest, false, false, "json")
	described := executeDescribeTransportCommand(t, manifest, true, false, "json")
	if described != plain {
		t.Fatalf("describe changed public JSON\nplain: %s\ndescribe: %s", plain, described)
	}
	for _, forbidden := range []string{"human_table", "human_properties", "describe_only"} {
		assertNotContains(t, plain, forbidden)
	}
	var response struct {
		Data struct {
			Facts         []any `json:"verification_facts"`
			Stages        []any `json:"stages"`
			SchemaVersion int   `json:"schema_version"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(plain), &response); err != nil {
		t.Fatalf("decode public JSON: %v", err)
	}
	if response.Data.SchemaVersion != 1 || len(response.Data.Facts) != 1 || len(response.Data.Stages) != 1 {
		t.Fatalf("incomplete public JSON data: %#v", response.Data)
	}
}

func TestPluginVerboseTransportKeepsDomainJSONStable(t *testing.T) {
	manifest, _ := installDescribeTransportPlugin(t)
	plain := executeDescribeTransportCommand(t, manifest, false, false, "json")
	verboseOutput := executeDescribeTransportCommand(t, manifest, false, true, "json")

	var plainResponse, verboseResponse struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(plain), &plainResponse); err != nil {
		t.Fatalf("decode plain public JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(verboseOutput), &verboseResponse); err != nil {
		t.Fatalf("decode verbose public JSON: %v", err)
	}
	if !reflect.DeepEqual(plainResponse.Data, verboseResponse.Data) {
		t.Fatalf("verbose changed domain JSON\nplain: %#v\nverbose: %#v", plainResponse.Data, verboseResponse.Data)
	}
}

func TestPluginTransportDoesNotInjectEnvironmentCredentials(t *testing.T) {
	manifest, requestPath := installDescribeTransportPlugin(t)
	t.Setenv("GITHUB_TOKEN", "transport-secret-token")
	t.Setenv("AUTHORIZATION", "Bearer transport-secret-authorization")

	_ = executeDescribeTransportCommand(t, manifest, false, true, "json")
	data, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("read dispatched request: %v", err)
	}
	for _, secret := range []string{"transport-secret-token", "transport-secret-authorization", "Bearer"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("plugin request contains environment credential %q: %s", secret, data)
		}
	}
}

func executeDescribeTransportCommand(t *testing.T, manifest plugin.Manifest, describeValue, verboseValue bool, format string) string {
	t.Helper()
	oldDescribe, oldVerbose, oldFormat, oldGitHubOutput := describe, verbose, outputFormat, githubOutputFile
	describe, verbose, outputFormat, githubOutputFile = false, false, "table", ""
	t.Cleanup(func() {
		describe, verbose, outputFormat, githubOutputFile = oldDescribe, oldVerbose, oldFormat, oldGitHubOutput
	})

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

	testRoot := &cobra.Command{Use: "neko"}
	testRoot.PersistentFlags().BoolVar(&describe, "describe", false, "structured details")
	testRoot.PersistentFlags().BoolVar(&verbose, "verbose", false, "execution logs")
	testRoot.PersistentFlags().StringVar(&outputFormat, "output", "table", "output format")
	testRoot.PersistentFlags().StringVar(&githubOutputFile, "github-output-file", "", "GitHub output file")
	testRoot.AddCommand(CreatePluginCommand(manifest))
	args := []string{"release", "pipeline", "--unit", "cli"}
	if describeValue {
		args = append(args, "--describe")
	}
	if verboseValue {
		args = append(args, "--verbose")
	}
	if format != "table" {
		args = append(args, "--output", format)
	}
	testRoot.SetArgs(args)
	executeErr := testRoot.Execute()
	closeErr := writeEnd.Close()
	os.Stdout = oldStdout
	data := <-output
	copyErr := <-readErr
	_ = readEnd.Close()
	if executeErr != nil {
		t.Fatalf("execute fake pipeline command: %v", executeErr)
	}
	if closeErr != nil {
		t.Fatalf("close captured stdout: %v", closeErr)
	}
	if copyErr != nil {
		t.Fatalf("capture command stdout: %v", copyErr)
	}
	return string(data)
}

func installDescribeTransportPlugin(t *testing.T) (plugin.Manifest, string) {
	t.Helper()
	manifest := plugin.Manifest{
		Name: "release", Version: "4.2.0", Description: "Release management plugin",
		Commands: []plugin.Command{{
			Name: "pipeline", Description: "Inspect release pipeline", Outputs: []string{"table", "json"},
			Flags: []plugin.Flag{
				{Name: "unit", Type: "string"},
				{Name: "verify-remote", Type: "bool"},
			},
		}},
		RendererTypes: []string{"table", "json"},
	}
	pluginDir := installFakePlugin(t, manifest)
	restorePluginDir(t, pluginDir)
	requestPath := filepath.Join(t.TempDir(), "request.json")
	responsePath := filepath.Join(t.TempDir(), "response.json")
	t.Setenv("NEKO_TEST_REQUEST_FILE", requestPath)
	t.Setenv("NEKO_TEST_RESPONSE_FILE", responsePath)

	response := plugin.Response{
		Status:   "success",
		Metadata: plugin.ResponseMetadata{Plugin: "release", Version: "4.2.0", Command: "pipeline"},
		Data: map[string]any{
			"schema_version":     1,
			"verification_facts": []any{map[string]any{"status": "verified"}},
			"stages":             []any{map[string]any{"id": "plan"}},
		},
		PresentationProperties: &presentation.Properties{
			Title: "Release Pipeline Inspection", SectionTitle: "Summary",
			Properties: []presentation.Property{{Label: "Lifecycle", Value: "Blocked"}},
		},
		PresentationTable: &presentation.Table{
			Title: "Findings", Columns: []presentation.Column{{Key: "details", Label: "Details", Essential: true}},
			Rows: []map[string]any{{"details": "Expected release asset is missing."}},
			Following: &presentation.Table{
				Title: "Verification Facts", DescribeOnly: true,
				Columns: []presentation.Column{{Key: "evidence", Label: "Evidence", Essential: true}},
				Rows:    []map[string]any{{"evidence": "Healthy local workflow"}},
				Following: &presentation.Table{
					Title: "Configured Pipeline", DescribeOnly: true,
					Columns: []presentation.Column{{Key: "stage", Label: "Stage", Essential: true}},
					Rows:    []map[string]any{{"stage": "Plan release"}},
					Following: &presentation.Table{
						Title: "Limitations", DescribeOnly: true,
						Columns: []presentation.Column{{Key: "limitation", Label: "Limitation", Essential: true}},
						Rows:    []map[string]any{{"limitation": "Complete limitation."}},
					},
				},
			},
		},
	}
	responseData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal fake response: %v", err)
	}
	if err := os.WriteFile(responsePath, responseData, 0o600); err != nil {
		t.Fatalf("write fake response: %v", err)
	}
	binaryPath := filepath.Join(pluginDir, manifest.Name, "plugin-"+manifest.Name)
	binary := `#!/bin/sh
cat > "$NEKO_TEST_REQUEST_FILE"
cat "$NEKO_TEST_RESPONSE_FILE"
printf '%s\n' '10:11:12 [pipeline] Inspecting execution journals' >&2
`
	if err := os.WriteFile(binaryPath, []byte(binary), 0o755); err != nil {
		t.Fatalf("write fake plugin: %v", err)
	}
	return manifest, requestPath
}
