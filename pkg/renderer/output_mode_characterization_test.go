package renderer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestPluginOutputModesKeepHumanJSONWideAndRawJSONContracts(t *testing.T) {
	response := &plugin.Response{
		Status:       "success",
		Data:         map[string]any{"items": []map[string]any{{"property": "Unit", "value": "api"}}},
		RendererHint: "table",
	}

	for _, format := range []OutputFormat{FormatTable, FormatWide} {
		var output bytes.Buffer
		if err := RenderTo(response, format, &output); err != nil {
			t.Fatalf("RenderTo(%q): %v", format, err)
		}
		if got := output.String(); !strings.Contains(got, "PROPERTY") || !strings.Contains(got, "api") {
			t.Fatalf("human output for %q = %q", format, got)
		}
	}

	var jsonOutput bytes.Buffer
	if err := RenderTo(response, FormatJSON, &jsonOutput); err != nil {
		t.Fatalf("RenderTo(json): %v", err)
	}
	if got := jsonOutput.String(); !strings.Contains(got, `"status": "success"`) || !strings.Contains(got, `"renderer_hint": "table"`) {
		t.Fatalf("JSON output = %q", got)
	}

	rawResponse := &plugin.Response{Status: "success", RendererHint: "raw-json", Data: map[string]any{"raw": "{\"unit\":\"api\"}\n"}}
	var rawOutput bytes.Buffer
	if err := RenderWithOptionsTo(rawResponse, RenderOptions{Format: FormatTable}, &rawOutput); err != nil {
		t.Fatalf("RenderWithOptionsTo(raw-json): %v", err)
	}
	if got := rawOutput.String(); got != "{\"unit\":\"api\"}\n" {
		t.Fatalf("raw JSON output = %q", got)
	}
}
