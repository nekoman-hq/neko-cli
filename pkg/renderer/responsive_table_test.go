package renderer

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestResponsiveTableUsesDeclaredOrderAndFitsOptionalColumnsByPriority(t *testing.T) {
	response := responsiveTestResponse()
	output := renderResponsiveForTest(t, response, FormatTable, fixedOutputWidth{width: 26, available: true})
	plain := ansi.Strip(output)

	lines := strings.Split(plain, "\n")
	if len(lines) == 0 || lines[0] != "Family    State     Unit  " {
		t.Fatalf("responsive header = %q, want declared essential columns followed by the fitting optional column", lines[0])
	}
	if strings.Contains(lines[0], "Version") {
		t.Fatalf("lower-priority optional column should not fit at width 26: %q", lines[0])
	}
	assertRenderedLinesFit(t, output, 26)
}

func TestResponsiveTableFallsBackToVerticalRecordsWhenEssentialColumnsDoNotFit(t *testing.T) {
	response := responsiveTestResponse()
	output := renderResponsiveForTest(t, response, FormatTable, fixedOutputWidth{width: 19, available: true})
	plain := ansi.Strip(output)

	for _, want := range []string{"Family: dispatch", "State: accepted", "Unit: api", "Version: 1.2.4"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("vertical output missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "────") {
		t.Fatalf("narrow output unexpectedly rendered a table:\n%s", plain)
	}
	assertRenderedLinesFit(t, output, 19)
}

func TestResponsiveTableUsesDeterministicVerticalFallbackWhenWidthIsUnavailable(t *testing.T) {
	response := responsiveTestResponse()

	first := renderResponsiveForTest(t, response, FormatTable, fixedOutputWidth{})
	second := renderResponsiveForTest(t, response, FormatTable, fixedOutputWidth{})
	if first != second {
		t.Fatalf("unavailable-width fallback is not deterministic:\nfirst=%q\nsecond=%q", first, second)
	}
	plain := ansi.Strip(first)
	if !strings.Contains(plain, "Family: dispatch") || strings.Contains(plain, "────") {
		t.Fatalf("unavailable width did not use vertical records:\n%s", plain)
	}
}

func TestResponsiveTableTreatsNonTerminalWriterAsWidthUnavailable(t *testing.T) {
	response := responsiveTestResponse()
	var output bytes.Buffer
	if err := RenderTo(response, FormatTable, &output); err != nil {
		t.Fatalf("RenderTo: %v", err)
	}

	plain := ansi.Strip(output.String())
	if !strings.Contains(plain, "Family: dispatch") || strings.Contains(plain, "────") {
		t.Fatalf("bytes.Buffer should use deterministic vertical fallback:\n%s", plain)
	}
}

func TestResponsiveWideIncludesEveryDeclaredSummaryColumnOrUsesVerticalLayout(t *testing.T) {
	response := responsiveTestResponse()

	wideTable := ansi.Strip(renderResponsiveForTest(t, response, FormatWide, fixedOutputWidth{width: 35, available: true}))
	if firstLine := strings.Split(wideTable, "\n")[0]; firstLine != "Family    State     Unit  Version  " {
		t.Fatalf("wide header = %q, want every declared summary column", firstLine)
	}

	wideVertical := ansi.Strip(renderResponsiveForTest(t, response, FormatWide, fixedOutputWidth{width: 26, available: true}))
	if !strings.Contains(wideVertical, "Version: 1.2.4") || strings.Contains(wideVertical, "────") {
		t.Fatalf("wide output that cannot fit every column must retain all summary fields vertically:\n%s", wideVertical)
	}
}

func TestResponsiveOptionalColumnUsesBoundedANSIAndUnicodeSafeTruncation(t *testing.T) {
	longTag := "\x1b[35m版本/猫猫猫猫猫猫猫猫猫猫猫猫猫猫\x1b[0m"
	response := &plugin.Response{
		Status: "success",
		Data:   map[string]any{"items": []map[string]any{{"family": "dispatch", "tag": longTag}}},
		HumanTable: &plugin.HumanTable{
			Columns: []plugin.HumanColumn{
				{Key: "family", Label: "Family", Essential: true},
				{Key: "tag", Label: "Tag"},
			},
		},
	}

	output := renderResponsiveForTest(t, response, FormatTable, fixedOutputWidth{width: 30, available: true})
	plain := ansi.Strip(output)
	if !strings.Contains(plain, "…") {
		t.Fatalf("bounded optional value was not visibly truncated:\n%s", plain)
	}
	if strings.Contains(plain, ansi.Strip(longTag)) {
		t.Fatalf("unbounded optional value consumed the table:\n%s", plain)
	}
	if !utf8.ValidString(output) {
		t.Fatalf("truncation produced invalid UTF-8: %q", output)
	}
	assertRenderedLinesFit(t, output, 30)
}

func TestVisibleWidthAndTruncationHandleANSICombiningAndWideGlyphs(t *testing.T) {
	value := "\x1b[31m猫e\u0301abc\x1b[0m"
	if got := visibleWidth(value); got != 6 {
		t.Fatalf("visibleWidth(%q) = %d, want 6 display cells", value, got)
	}

	truncated := truncateVisible(value, 4)
	if !utf8.ValidString(truncated) {
		t.Fatalf("truncateVisible produced invalid UTF-8: %q", truncated)
	}
	if got := visibleWidth(truncated); got > 4 {
		t.Fatalf("truncated width = %d, want at most 4: %q", got, truncated)
	}
	if !strings.Contains(ansi.Strip(truncated), "…") {
		t.Fatalf("truncateVisible omitted ellipsis: %q", truncated)
	}
}

func TestResponsiveVerticalWrappingUsesVisibleCells(t *testing.T) {
	response := &plugin.Response{
		Status:     "success",
		Data:       map[string]any{"items": []map[string]any{{"family": "\x1b[31m猫猫猫猫猫猫猫猫\x1b[0m"}}},
		HumanTable: &plugin.HumanTable{Columns: []plugin.HumanColumn{{Key: "family", Label: "Family", Essential: true}}},
	}

	output := renderResponsiveForTest(t, response, FormatTable, fixedOutputWidth{width: 10, available: true})
	if !strings.Contains(output, "\n") || !utf8.ValidString(output) {
		t.Fatalf("vertical wrapping produced invalid output: %q", output)
	}
	assertRenderedLinesFit(t, output, 10)
}

func TestHumanTableMetadataCrossesTransportButDoesNotLeakIntoPublicJSON(t *testing.T) {
	response := responsiveTestResponse()

	transport, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal transport response: %v", err)
	}
	if !bytes.Contains(transport, []byte(`"human_table"`)) {
		t.Fatalf("transport response omitted human table declaration: %s", transport)
	}

	var output bytes.Buffer
	if err := RenderTo(response, FormatJSON, &output); err != nil {
		t.Fatalf("RenderTo JSON: %v", err)
	}
	if strings.Contains(output.String(), "human_table") {
		t.Fatalf("presentation metadata leaked into public JSON:\n%s", output.String())
	}
	if !strings.Contains(output.String(), `"items"`) || !strings.Contains(output.String(), `"version": "1.2.4"`) {
		t.Fatalf("public JSON lost response data:\n%s", output.String())
	}
}

func TestHumanTableMetadataDoesNotLeakIntoRawJSON(t *testing.T) {
	response := responsiveTestResponse()
	response.RendererHint = "raw-json"
	response.Data = map[string]any{"raw": `{"value":"complete"}`}

	var output bytes.Buffer
	if err := RenderWithOptionsTo(response, RenderOptions{Format: FormatTable}, &output); err != nil {
		t.Fatalf("RenderWithOptionsTo: %v", err)
	}
	if output.String() != `{"value":"complete"}` {
		t.Fatalf("raw JSON output changed: %q", output.String())
	}
}

func TestResponsiveWidthProviderReceivesActualOutputWriter(t *testing.T) {
	response := responsiveTestResponse()
	provider := &recordingOutputWidth{width: 35, available: true}
	var output bytes.Buffer

	if err := RenderWithOptionsTo(response, RenderOptions{Format: FormatTable, WidthProvider: provider}, &output); err != nil {
		t.Fatalf("RenderWithOptionsTo: %v", err)
	}
	if provider.calls != 1 || provider.writer != &output {
		t.Fatalf("width provider calls = %d, writer = %T; want one call with actual output buffer", provider.calls, provider.writer)
	}
}

func TestWideRenderingRemainsUnchangedWithoutHumanTableOptIn(t *testing.T) {
	response := &plugin.Response{
		Status: "success",
		Data:   map[string]any{"items": []map[string]any{{"name": "api", "status": "ready"}}},
	}
	var tableOutput bytes.Buffer
	var wideOutput bytes.Buffer
	if err := RenderTo(response, FormatTable, &tableOutput); err != nil {
		t.Fatalf("RenderTo table: %v", err)
	}
	if err := RenderTo(response, FormatWide, &wideOutput); err != nil {
		t.Fatalf("RenderTo wide: %v", err)
	}
	if tableOutput.String() != wideOutput.String() {
		t.Fatalf("non-opted wide output changed:\ntable=%q\nwide=%q", tableOutput.String(), wideOutput.String())
	}
}

func responsiveTestResponse() *plugin.Response {
	return &plugin.Response{
		Status: "success",
		Data: map[string]any{
			"items": []map[string]any{{
				"family":  "dispatch",
				"state":   "accepted",
				"unit":    "api",
				"version": "1.2.4",
			}},
		},
		HumanTable: &plugin.HumanTable{
			Columns: []plugin.HumanColumn{
				{Key: "family", Label: "Family", Essential: true},
				{Key: "state", Label: "State", Essential: true},
				{Key: "unit", Label: "Unit"},
				{Key: "version", Label: "Version"},
			},
		},
	}
}

func renderResponsiveForTest(t *testing.T, response *plugin.Response, format OutputFormat, width OutputWidthProvider) string {
	t.Helper()
	var output bytes.Buffer
	if err := RenderWithOptionsTo(response, RenderOptions{Format: format, WidthProvider: width}, &output); err != nil {
		t.Fatalf("RenderWithOptionsTo: %v", err)
	}
	return output.String()
}

func assertRenderedLinesFit(t *testing.T, output string, width int) {
	t.Helper()
	for lineNumber, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if got := ansi.StringWidth(line); got > width {
			t.Fatalf("line %d visible width = %d, want at most %d: %q", lineNumber+1, got, width, line)
		}
	}
}

type fixedOutputWidth struct {
	width     int
	available bool
}

func (width fixedOutputWidth) Width(io.Writer) (int, bool) {
	return width.width, width.available
}

type recordingOutputWidth struct {
	writer    io.Writer
	width     int
	calls     int
	available bool
}

func (width *recordingOutputWidth) Width(writer io.Writer) (int, bool) {
	width.calls++
	width.writer = writer
	return width.width, width.available
}
