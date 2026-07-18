package renderer

import (
	"bytes"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
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
	if err := RenderTo(response, FormatTable, &output); err != nil {
		t.Fatalf("RenderTo: %v", err)
	}

	want := log.ColorCyan + log.ColorBold + "COMMITS  FROM    VERSION  " + log.ColorReset + "\n" +
		log.ColorBrightBlack + "──────────────────────────" + log.ColorReset + "\n" +
		"2        " + log.ColorPurple + "v1.2.2" + log.ColorReset + "  " + log.ColorPurple + "v1.2.3" + log.ColorReset + "   \n" +
		"5        " + log.ColorPurple + "v1.2.3" + log.ColorReset + "  " + log.ColorPurple + "v1.3.0" + log.ColorReset + "   \n"
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
	if err := RenderTo(response, FormatTable, &output); err != nil {
		t.Fatalf("RenderTo: %v", err)
	}

	want := log.ColorCyan + log.ColorBold + "Status" + log.ColorReset + "\n" +
		"  ready\n\n" +
		log.ColorCyan + log.ColorBold + "Unit" + log.ColorReset + "\n" +
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
	if err := RenderDescribeTo(response, FormatTable, &output); err != nil {
		t.Fatalf("RenderDescribeTo: %v", err)
	}

	want := "\n" + log.ColorCyan + log.ColorBold + "━━━ Command Metadata ━━━" + log.ColorReset + "\n" +
		log.ColorBrightBlack + "Plugin:" + log.ColorReset + "     release\n" +
		log.ColorBrightBlack + "Command:" + log.ColorReset + "    validate\n" +
		log.ColorBrightBlack + "Version:" + log.ColorReset + "    4.2.0\n" +
		log.ColorBrightBlack + "Timestamp:" + log.ColorReset + "  2026-07-18 10:11:12\n" +
		log.ColorBrightBlack + "Status:" + log.ColorReset + "     " + log.ColorGreen + "✓ success" + log.ColorReset + "\n\n" +
		"\n" + log.ColorYellow + log.ColorBold + "━━━ Execution Logs (1 entries) ━━━" + log.ColorReset + "\n" +
		log.ColorBrightBlack + "10:11:12 " + log.ColorBrightBlack + "• " + log.ColorReset + "checked\n\n" +
		"\n" + log.ColorGreen + log.ColorBold + "━━━ Output ━━━" + log.ColorReset + "\n" +
		log.ColorCyan + log.ColorBold + "Status" + log.ColorReset + "\n" +
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
