package renderer

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestPropertyValuesUseBoundedTwoColumnLayoutAndWrapValues(t *testing.T) {
	response := propertyValueTestResponse()
	const outputWidth = 52
	output := renderPropertyValuesForTest(t, response, FormatTable, fixedOutputWidth{width: outputWidth, available: true})
	plain := ansi.Strip(output)

	properties, ok := itemPropertyValues(response.Data["items"])
	if !ok {
		t.Fatal("test response was not recognized as property/value presentation")
	}
	layout := calculatePropertyLayout(properties, outputWidth, true)
	if !layout.twoColumn || layout.labelWidth > outputWidth/3 || layout.valueWidth < minimumPropertyValueWidth {
		t.Fatalf("layout = %#v", layout)
	}
	lines := strings.Split(strings.TrimSuffix(plain, "\n"), "\n")
	if len(lines) < 4 || !strings.Contains(lines[0], "PROPERTY") || !strings.Contains(lines[0], "VALUE") {
		t.Fatalf("two-column header missing:\n%s", plain)
	}
	if got := ansi.StringWidth(lines[1]); got != outputWidth {
		t.Fatalf("separator width = %d, want %d: %q", got, outputWidth, lines[1])
	}
	continuationIndent := strings.Repeat(" ", layout.labelWidth+propertyColumnGap)
	continuationFound := false
	for _, line := range lines[2:] {
		if strings.HasPrefix(line, continuationIndent) && strings.TrimSpace(line) != "" {
			continuationFound = true
			break
		}
	}
	if !continuationFound {
		t.Fatalf("wrapped value continuation was not aligned under VALUE:\n%s", plain)
	}
	if !strings.Contains(plain, "Release plan inspected locally;") ||
		!strings.Contains(plain, "\n"+continuationIndent+"no release execution was") {
		t.Fatalf("property values were not wrapped at readable word boundaries:\n%s", plain)
	}
	assertRenderedLinesFit(t, output, outputWidth)
}

func TestPropertyValuesKeepWideOutputWithinActualWidth(t *testing.T) {
	const outputWidth = 88
	output := renderPropertyValuesForTest(t, propertyValueTestResponse(), FormatWide, fixedOutputWidth{width: outputWidth, available: true})
	plain := ansi.Strip(output)
	if !strings.Contains(plain, "PROPERTY") || !strings.Contains(plain, "Release plan inspected locally") {
		t.Fatalf("wide property output lost content:\n%s", plain)
	}
	assertRenderedLinesFit(t, output, outputWidth)
}

func TestPropertyValuesUseVerticalLayoutForNarrowOutput(t *testing.T) {
	const outputWidth = 24
	output := renderPropertyValuesForTest(t, propertyValueTestResponse(), FormatTable, fixedOutputWidth{width: outputWidth, available: true})
	plain := ansi.Strip(output)

	if strings.Contains(plain, "PROPERTY") || !strings.Contains(plain, "Unit\n  cli\n\n") {
		t.Fatalf("narrow output did not use vertical property layout:\n%s", plain)
	}
	assertRenderedLinesFit(t, output, outputWidth)
}

func TestPropertyValuesUseDeterministicVerticalLayoutWhenWidthIsUnknown(t *testing.T) {
	response := propertyValueTestResponse()
	var first bytes.Buffer
	var second bytes.Buffer
	if err := RenderTo(response, FormatTable, &first); err != nil {
		t.Fatalf("RenderTo first: %v", err)
	}
	if err := RenderTo(response, FormatTable, &second); err != nil {
		t.Fatalf("RenderTo second: %v", err)
	}
	if first.String() != second.String() {
		t.Fatalf("unknown-width output is not deterministic:\nfirst=%q\nsecond=%q", first.String(), second.String())
	}
	plain := ansi.Strip(first.String())
	if strings.Contains(plain, "PROPERTY") || !strings.Contains(plain, "Status\n  Release plan inspected locally") {
		t.Fatalf("unknown-width output did not use vertical property layout:\n%s", plain)
	}
}

func TestPropertyValuesWrapLongLabelsANSICombiningAndWideGlyphsByVisibleWidth(t *testing.T) {
	response := &plugin.Response{
		Status: "success",
		Data: map[string]any{"items": []map[string]any{
			{
				"property": "\x1b[36mA deliberately long 猫 property label\x1b[0m",
				"value":    "\x1b[35m猫猫e\u0301猫猫e\u0301猫猫e\u0301猫猫e\u0301\x1b[0m",
			},
		}},
	}
	const outputWidth = 40
	output := renderPropertyValuesForTest(t, response, FormatTable, fixedOutputWidth{width: outputWidth, available: true})
	if !utf8.ValidString(output) {
		t.Fatalf("property wrapping produced invalid UTF-8: %q", output)
	}
	if !strings.Contains(ansi.Strip(output), "猫") {
		t.Fatalf("Unicode property content was lost: %q", output)
	}
	assertRenderedLinesFit(t, output, outputWidth)
}

func TestDirectHumanPropertyValuesRemainPresentationOnly(t *testing.T) {
	response := &plugin.Response{
		Status: "success",
		Data: map[string]any{"items": []map[string]any{
			{"property": "Limitations", "value": "stable machine value"},
		}},
		HumanProperties: &plugin.HumanProperties{Properties: []plugin.HumanProperty{
			{Label: "Local Inspection Only", Value: "No release execution is started."},
			{Label: "Token Free", Value: "Tokens are not read or reported."},
		}},
	}
	transport, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal transport response: %v", err)
	}
	var transported plugin.Response
	if err := json.Unmarshal(transport, &transported); err != nil {
		t.Fatalf("unmarshal transport response: %v", err)
	}
	output := renderPropertyValuesForTest(t, &transported, FormatTable, fixedOutputWidth{width: 64, available: true})
	plain := ansi.Strip(output)
	if !strings.Contains(plain, "Local Inspection") || !strings.Contains(plain, "Tokens are not read") || strings.Contains(plain, "stable machine value") {
		t.Fatalf("direct human properties were not isolated from machine data:\n%s", plain)
	}

	var jsonOutput bytes.Buffer
	if err := RenderTo(&transported, FormatJSON, &jsonOutput); err != nil {
		t.Fatalf("RenderTo JSON: %v", err)
	}
	if !strings.Contains(jsonOutput.String(), "stable machine value") || strings.Contains(jsonOutput.String(), "Local Inspection Only") {
		t.Fatalf("presentation-only values changed JSON:\n%s", jsonOutput.String())
	}
}

func propertyValueTestResponse() *plugin.Response {
	return &plugin.Response{
		Status: "success",
		Data: map[string]any{"items": []map[string]any{
			{"property": "Unit", "value": "cli"},
			{"property": "A deliberately long property label", "value": "label wrapping remains bounded"},
			{"property": "Unit Root", "value": "/Users/benjamin/Developer/Projects/nekoman/neko-cli/with/a/long/property/value"},
			{"property": "Status", "value": "Release plan inspected locally; no release execution was started."},
		}},
	}
}

func renderPropertyValuesForTest(
	t *testing.T,
	response *plugin.Response,
	format OutputFormat,
	width OutputWidthProvider,
) string {
	t.Helper()
	var output bytes.Buffer
	if err := RenderWithOptionsTo(response, RenderOptions{Format: format, WidthProvider: width}, &output); err != nil {
		t.Fatalf("RenderWithOptionsTo: %v", err)
	}
	return output.String()
}
