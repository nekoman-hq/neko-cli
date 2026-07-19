package renderer

import (
	"bytes"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/internal/terminal"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestLegacyCompactListRenderingRemainsStable(t *testing.T) {
	response := &plugin.Response{
		Status: "success",
		Data: map[string]any{
			"items": []map[string]any{
				{"version": "v1.2.3", "from": "v1.2.2", "commits": 2},
				{"version": "v1.3.0", "from": "v1.2.3", "commits": 5},
			},
		},
	}

	var output bytes.Buffer
	if err := RenderWithOptionsTo(response, RenderOptions{Format: FormatTable, ColorProvider: fixedColorProvider(true)}, &output); err != nil {
		t.Fatalf("RenderTo: %v", err)
	}

	want := terminal.Cyan + terminal.Bold + "COMMITS  FROM    VERSION  " + terminal.Reset + "\n" +
		terminal.BrightBlack + "──────────────────────────" + terminal.Reset + "\n" +
		"2        " + terminal.Purple + "v1.2.2" + terminal.Reset + "  " + terminal.Purple + "v1.2.3" + terminal.Reset + "   \n" +
		"5        " + terminal.Purple + "v1.2.3" + terminal.Reset + "  " + terminal.Purple + "v1.3.0" + terminal.Reset + "   \n"
	if output.String() != want {
		t.Fatalf("legacy compact list output changed:\nwant %q\n got %q", want, output.String())
	}
}

func TestPropertyValueRenderingUsesDeterministicVerticalFallbackWithoutWidth(t *testing.T) {
	response := &plugin.Response{
		Status: "success",
		Data: map[string]any{
			"items": []map[string]any{
				{"property": "Status", "value": "ready"},
				{"property": "Unit", "value": "api"},
			},
		},
	}

	var output bytes.Buffer
	if err := RenderWithOptionsTo(response, RenderOptions{Format: FormatTable, ColorProvider: fixedColorProvider(true)}, &output); err != nil {
		t.Fatalf("RenderTo: %v", err)
	}

	want := terminal.Cyan + terminal.Bold + "Status" + terminal.Reset + "\n" +
		"  ready\n\n" +
		terminal.Cyan + terminal.Bold + "Unit" + terminal.Reset + "\n" +
		"  api\n"
	if output.String() != want {
		t.Fatalf("legacy property/value output changed:\nwant %q\n got %q", want, output.String())
	}
}

func TestLegacyDescribeRenderingRemainsStable(t *testing.T) {
	timestamp := time.Date(2026, time.July, 18, 10, 11, 12, 0, time.UTC)
	response := &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Timestamp: timestamp,
			Plugin:    "release",
			Version:   "4.2.0",
			Command:   "validate",
		},
		Data: map[string]any{
			"items": []map[string]any{{"property": "Status", "value": "ready"}},
		},
		Logs: []plugin.LogEntry{{Timestamp: "10:11:12", Level: "info", Category: "plugin", Message: "checked"}},
	}

	var output bytes.Buffer
	if err := RenderWithOptionsTo(response, RenderOptions{Format: FormatTable, Describe: true, ColorProvider: fixedColorProvider(true)}, &output); err != nil {
		t.Fatalf("RenderDescribeTo: %v", err)
	}

	want := "\n" + terminal.Cyan + terminal.Bold + "━━━ Command Metadata ━━━" + terminal.Reset + "\n" +
		terminal.BrightBlack + "Plugin:" + terminal.Reset + "     release\n" +
		terminal.BrightBlack + "Command:" + terminal.Reset + "    validate\n" +
		terminal.BrightBlack + "Version:" + terminal.Reset + "    4.2.0\n" +
		terminal.BrightBlack + "Timestamp:" + terminal.Reset + "  2026-07-18 10:11:12\n" +
		terminal.BrightBlack + "Status:" + terminal.Reset + "     " + terminal.Green + "✓ success" + terminal.Reset + "\n\n" +
		"\n" + terminal.Yellow + terminal.Bold + "━━━ Execution Logs (1 entries) ━━━" + terminal.Reset + "\n" +
		terminal.BrightBlack + "10:11:12" + terminal.Reset + " " + terminal.BrightBlack + "• " + terminal.Reset + "checked\n\n" +
		"\n" + terminal.Green + terminal.Bold + "━━━ Output ━━━" + terminal.Reset + "\n" +
		terminal.Cyan + terminal.Bold + "Status" + terminal.Reset + "\n" +
		"  ready\n"
	if output.String() != want {
		t.Fatalf("legacy describe output changed:\nwant %q\n got %q", want, output.String())
	}
}

func TestLegacyJSONRenderingRemainsStable(t *testing.T) {
	response := &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Timestamp: time.Date(2026, time.July, 18, 10, 11, 12, 0, time.UTC),
			Plugin:    "release",
			Version:   "4.2.0",
			Command:   "history",
		},
		Data: map[string]any{
			"items": []map[string]any{{"commits": 2, "from": "v1.2.2", "version": "v1.2.3"}},
		},
		RendererHint: "table",
	}

	var output bytes.Buffer
	if err := RenderTo(response, FormatJSON, &output); err != nil {
		t.Fatalf("RenderTo: %v", err)
	}

	want := "{\n" +
		"  \"status\": \"success\",\n" +
		"  \"metadata\": {\n" +
		"    \"timestamp\": \"2026-07-18T10:11:12Z\",\n" +
		"    \"plugin\": \"release\",\n" +
		"    \"version\": \"4.2.0\",\n" +
		"    \"command\": \"history\"\n" +
		"  },\n" +
		"  \"data\": {\n" +
		"    \"items\": [\n" +
		"      {\n" +
		"        \"commits\": 2,\n" +
		"        \"from\": \"v1.2.2\",\n" +
		"        \"version\": \"v1.2.3\"\n" +
		"      }\n" +
		"    ]\n" +
		"  },\n" +
		"  \"renderer_hint\": \"table\"\n" +
		"}\n"
	if output.String() != want {
		t.Fatalf("legacy JSON output changed:\nwant %q\n got %q", want, output.String())
	}
}

func TestDeprecatedResponsePresentationFieldsStillRender(t *testing.T) {
	response := &plugin.Response{
		Status:    "success",
		HumanText: &plugin.HumanText{Content: "preview\n"},
	}

	var output bytes.Buffer
	if err := RenderTo(response, FormatTable, &output); err != nil {
		t.Fatalf("render deprecated text presentation field: %v", err)
	}
	if output.String() != "preview\n" {
		t.Fatalf("deprecated text presentation output changed: %q", output.String())
	}
}
