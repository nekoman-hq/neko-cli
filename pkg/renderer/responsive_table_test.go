package renderer

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
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

func TestResponsiveTablePrioritizesUnitRowsOverWorkflowSummaryLists(t *testing.T) {
	units := []map[string]any{{"id": "api", "version": "1.2.3"}}
	data := map[string]any{
		"workflow_paths": []string{".github/workflows/release.yml"},
		"units":          units,
	}
	if got := findListInData(data); !reflect.DeepEqual(got, units) {
		t.Fatalf("selected list = %#v, want unit rows", got)
	}
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
		PresentationTable: &presentation.Table{
			Columns: []presentation.Column{
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
		Status:            "success",
		Data:              map[string]any{"items": []map[string]any{{"family": "\x1b[31m猫猫猫猫猫猫猫猫\x1b[0m"}}},
		PresentationTable: &presentation.Table{Columns: []presentation.Column{{Key: "family", Label: "Family", Essential: true}}},
	}

	output := renderResponsiveForTest(t, response, FormatTable, fixedOutputWidth{width: 10, available: true})
	if !strings.Contains(output, "\n") || !utf8.ValidString(output) {
		t.Fatalf("vertical wrapping produced invalid output: %q", output)
	}
	assertRenderedLinesFit(t, output, 10)
}

func TestTablePresentationMetadataCrossesTransportButDoesNotLeakIntoPublicJSON(t *testing.T) {
	response := responsiveTestResponse()

	transport, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal transport response: %v", err)
	}
	if !bytes.Contains(transport, []byte(`"human_table"`)) {
		t.Fatalf("transport response omitted table presentation declaration: %s", transport)
	}
	if bytes.Contains(transport, []byte(`"rows"`)) || bytes.Contains(transport, []byte(`"details"`)) {
		t.Fatalf("zero-value table extensions changed existing transport: %s", transport)
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

func TestTablePresentationMetadataDoesNotLeakIntoRawJSON(t *testing.T) {
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

func TestResponsiveTableComposesSummaryRowsAndPropertyDetails(t *testing.T) {
	response := &plugin.Response{
		Status: "success",
		Data: map[string]any{
			"items": []map[string]any{{"name": "machine item", "state": "stable"}},
		},
		PresentationProperties: &presentation.Properties{Properties: []presentation.Property{
			{Label: "Readiness", Value: "review"},
		}},
		PresentationTable: &presentation.Table{
			Columns: []presentation.Column{
				{Key: "severity", Label: "Severity", Essential: true},
				{Key: "code", Label: "Code", Essential: true},
			},
			Rows: []map[string]any{{"severity": "warning", "code": "CONFIG_REVIEW"}},
			Details: &presentation.Properties{Properties: []presentation.Property{
				{Label: "Detail", Value: "Review the complete local configuration before continuing."},
			}},
		},
	}

	output := ansi.Strip(renderResponsiveForTest(t, response, FormatTable, fixedOutputWidth{width: 64, available: true}))
	summaryAt := strings.Index(output, "Readiness")
	rowAt := strings.Index(output, "CONFIG_REVIEW")
	detailAt := strings.Index(output, "Review the complete local configuration")
	if summaryAt < 0 || rowAt <= summaryAt || detailAt <= rowAt || strings.Contains(output, "machine item") {
		t.Fatalf("composed presentation order or row isolation changed:\n%s", output)
	}
	assertRenderedLinesFit(t, output, 64)
}

func TestResponsiveTableRowsAndDetailsCrossTransportButStayOutOfMachineOutput(t *testing.T) {
	response := &plugin.Response{
		Status: "success",
		Data:   map[string]any{"items": []map[string]any{{"name": "machine item"}}},
		PresentationProperties: &presentation.Properties{Properties: []presentation.Property{
			{Label: "Summary", Value: "human summary"},
		}},
		PresentationTable: &presentation.Table{
			Columns: []presentation.Column{{Key: "name", Label: "Name", Essential: true}},
			Rows:    []map[string]any{{"name": "human row"}},
			Details: &presentation.Properties{Properties: []presentation.Property{
				{Label: "Detail", Value: "human detail"},
			}},
		},
	}

	transport, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal transport: %v", err)
	}
	for _, value := range []string{`"rows"`, `"details"`, "human row", "human detail"} {
		if !bytes.Contains(transport, []byte(value)) {
			t.Fatalf("transport omitted %q: %s", value, transport)
		}
	}
	var transported plugin.Response
	if err := json.Unmarshal(transport, &transported); err != nil {
		t.Fatalf("unmarshal transport: %v", err)
	}
	output := ansi.Strip(renderResponsiveForTest(t, &transported, FormatTable, fixedOutputWidth{width: 60, available: true}))
	if !strings.Contains(output, "human row") || !strings.Contains(output, "human detail") {
		t.Fatalf("transported presentation was not rendered:\n%s", output)
	}

	var publicJSON bytes.Buffer
	if err := RenderTo(&transported, FormatJSON, &publicJSON); err != nil {
		t.Fatalf("render public JSON: %v", err)
	}
	for _, value := range []string{"human row", "human detail", "human summary", "human_table"} {
		if strings.Contains(publicJSON.String(), value) {
			t.Fatalf("public JSON leaked %q:\n%s", value, publicJSON.String())
		}
	}
	if !strings.Contains(publicJSON.String(), "machine item") {
		t.Fatalf("public JSON lost machine data:\n%s", publicJSON.String())
	}

	transported.RendererHint = "raw-json"
	transported.Data = map[string]any{"raw": `{"value":"machine only"}`}
	var rawJSON bytes.Buffer
	if err := RenderWithOptionsTo(&transported, RenderOptions{Format: FormatTable}, &rawJSON); err != nil {
		t.Fatalf("render raw JSON: %v", err)
	}
	if rawJSON.String() != `{"value":"machine only"}` {
		t.Fatalf("raw JSON changed: %q", rawJSON.String())
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

func TestWideRenderingRemainsUnchangedWithoutTablePresentationOptIn(t *testing.T) {
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
		PresentationTable: &presentation.Table{
			Columns: []presentation.Column{
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
