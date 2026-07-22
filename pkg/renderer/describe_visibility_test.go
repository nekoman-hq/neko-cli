package renderer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

func TestDescribeVisibilityAndVerboseLogsRemainIndependent(t *testing.T) {
	response := &plugin.Response{
		Status:   "success",
		Metadata: plugin.ResponseMetadata{Plugin: "release", Command: "pipeline", Version: "4.2.0"},
		Data:     map[string]any{"schema_version": 1},
		Logs:     []plugin.LogEntry{{Timestamp: "10:11:12", Level: "verbose", Message: "V$ inspected journals"}},
		PresentationProperties: &presentation.Properties{
			Title: "Inspection", SectionTitle: "Summary",
			Properties: []presentation.Property{{Label: "Lifecycle", Value: "Ready"}},
		},
		PresentationTable: &presentation.Table{
			Title: "Findings", Columns: []presentation.Column{{Key: "finding", Label: "Finding", Essential: true}},
			Rows: []map[string]any{{"finding": "Actionable"}},
			Following: &presentation.Table{
				Title: "Verification Facts", DescribeOnly: true,
				Columns: []presentation.Column{{Key: "check", Label: "Check", Essential: true}},
				Rows:    []map[string]any{{"check": "Complete inventory"}},
			},
		},
	}

	tests := []struct {
		name                      string
		describe, verbose         bool
		wantMetadata, wantLogs    bool
		wantDetails, wantFindings bool
	}{
		{name: "default", wantFindings: true},
		{name: "describe", describe: true, wantMetadata: true, wantDetails: true, wantFindings: true},
		{name: "verbose", verbose: true, wantLogs: true, wantFindings: true},
		{name: "combined", describe: true, verbose: true, wantMetadata: true, wantLogs: true, wantDetails: true, wantFindings: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := renderDescribeVisibilityFixture(t, response, RenderOptions{
				Format: FormatTable, Describe: test.describe, Verbose: test.verbose,
				WidthProvider: fixedOutputWidth{width: 120, available: true},
			})
			for value, want := range map[string]bool{
				"Command Metadata":   test.wantMetadata,
				"Execution Logs":     test.wantLogs,
				"Verification Facts": test.wantDetails,
				"Findings":           test.wantFindings,
			} {
				if strings.Contains(output, value) != want {
					t.Fatalf("%s visibility for %q = %t, want %t:\n%s", test.name, value, strings.Contains(output, value), want, output)
				}
			}
		})
	}
}

func TestDescribeAndVerboseDoNotChangeJSONOrRawJSON(t *testing.T) {
	response := &plugin.Response{
		Status: "success", Data: map[string]any{"schema_version": 1, "raw": `{"schema_version":1}`},
		PresentationTable: &presentation.Table{
			DescribeOnly: true, Columns: []presentation.Column{{Key: "value", Label: "Value", Essential: true}},
			Rows: []map[string]any{{"value": "detail"}},
		},
	}
	plain := renderDescribeVisibilityFixture(t, response, RenderOptions{Format: FormatJSON})
	for _, options := range []RenderOptions{
		{Format: FormatJSON, Describe: true},
		{Format: FormatJSON, Verbose: true},
		{Format: FormatJSON, Describe: true, Verbose: true},
	} {
		if got := renderDescribeVisibilityFixture(t, response, options); got != plain {
			t.Fatalf("render options changed JSON\nwant: %s\n got: %s", plain, got)
		}
	}
	if strings.Contains(plain, "describe_only") {
		t.Fatalf("public JSON exposed visibility metadata: %s", plain)
	}

	response.RendererHint = "raw-json"
	raw := renderDescribeVisibilityFixture(t, response, RenderOptions{Format: FormatTable})
	for _, options := range []RenderOptions{{Format: FormatTable, Describe: true}, {Format: FormatTable, Verbose: true}} {
		if got := renderDescribeVisibilityFixture(t, response, options); got != raw {
			t.Fatalf("render options changed raw JSON\nwant: %s\n got: %s", raw, got)
		}
	}
}

func renderDescribeVisibilityFixture(t *testing.T, response *plugin.Response, options RenderOptions) string {
	t.Helper()
	options.ColorProvider = fixedColorProvider(false)
	var output bytes.Buffer
	if err := RenderWithOptionsTo(response, options, &output); err != nil {
		t.Fatal(err)
	}
	return output.String()
}
