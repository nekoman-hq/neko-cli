package renderer

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/internal/terminalstyle"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestSemanticHumanStylesUseSharedPaletteOnlyWhenEnabled(t *testing.T) {
	t.Parallel()

	response := semanticPropertyResponse()
	enabled := renderStyledResponseForTest(t, response, fixedColorProvider(true))
	for name, sequence := range map[string]string{
		"title":    terminalstyle.Bold + "Semantic output" + terminalstyle.Reset,
		"emphasis": terminalstyle.Bold + "emphasis" + terminalstyle.Reset,
		"success":  terminalstyle.Green + "success" + terminalstyle.Reset,
		"warning":  terminalstyle.Yellow + "warning" + terminalstyle.Reset,
		"error":    terminalstyle.Red + "error" + terminalstyle.Reset,
		"info":     terminalstyle.Cyan + "info" + terminalstyle.Reset,
		"muted":    terminalstyle.BrightBlack + "muted" + terminalstyle.Reset,
	} {
		if !strings.Contains(enabled, sequence) {
			t.Errorf("styled output omitted %s sequence %q:\n%q", name, sequence, enabled)
		}
	}

	disabled := renderStyledResponseForTest(t, response, fixedColorProvider(false))
	if strings.Contains(disabled, "\x1b[") {
		t.Fatalf("disabled human output contains ANSI: %q", disabled)
	}
	if got := ansi.Strip(enabled); got != disabled {
		t.Fatalf("color changed visible text:\nenabled=%q\ndisabled=%q", got, disabled)
	}
}

func TestSemanticTableRolesStyleOnlyDeclaredCells(t *testing.T) {
	t.Parallel()

	response := &plugin.Response{
		Status: "success",
		Data:   map[string]any{"items": []map[string]any{{"machine": "unchanged"}}},
		HumanTable: &plugin.HumanTable{
			Title: "Diagnostics",
			Columns: []plugin.HumanColumn{
				{Key: "severity", Label: "Severity", RoleKey: "severity_role", Essential: true},
				{Key: "code", Label: "Code", RoleKey: "severity_role", Essential: true},
				{Key: "target", Label: "Target"},
			},
			Rows: []map[string]any{{
				"severity": "ERROR", "code": "CONFIG_INVALID", "target": "api", "severity_role": string(plugin.HumanStyleError),
			}},
		},
	}
	output := renderStyledResponseForTest(t, response, fixedColorProvider(true))
	for _, styled := range []string{
		terminalstyle.Red + "ERROR" + terminalstyle.Reset,
		terminalstyle.Red + "CONFIG_INVALID" + terminalstyle.Reset,
	} {
		if !strings.Contains(output, styled) {
			t.Fatalf("semantic table omitted %q:\n%q", styled, output)
		}
	}
	if strings.Contains(output, terminalstyle.Red+"api") {
		t.Fatalf("semantic table colored the neutral target cell:\n%q", output)
	}
}

func TestHumanPropertyHeadingsCreateResponsiveStyledRecords(t *testing.T) {
	t.Parallel()

	response := &plugin.Response{
		Status: "success",
		Data:   map[string]any{"machine": "unchanged"},
		HumanProperties: &plugin.HumanProperties{Properties: []plugin.HumanProperty{
			{Label: "ERROR · CONFIG_INVALID", Role: plugin.HumanStyleError, Emphasized: true, Heading: true},
			{Label: "Scope", Value: "source"},
			{Label: "Message", Value: "Configuration is invalid."},
			{Label: "WARNING · CONCURRENCY_MISSING", Role: plugin.HumanStyleWarning, Emphasized: true, Heading: true},
			{Label: "Workflow", Value: ".github/workflows/release.yml"},
		}},
	}
	output := renderStyledResponseForTest(t, response, fixedColorProvider(true))
	if !strings.Contains(output, terminalstyle.Red+terminalstyle.Bold+"ERROR · CONFIG_INVALID"+terminalstyle.Reset) ||
		!strings.Contains(output, terminalstyle.Yellow+terminalstyle.Bold+"WARNING · CONCURRENCY_MISSING"+terminalstyle.Reset) {
		t.Fatalf("record headings lost semantic styles:\n%q", output)
	}
	plain := ansi.Strip(output)
	if strings.Contains(plain, "PROPERTY") || strings.Count(plain, "CONFIG_INVALID") != 1 ||
		!strings.Contains(plain, "Scope\n  source") || !strings.Contains(plain, "Workflow\n  .github/workflows/release.yml") {
		t.Fatalf("record hierarchy changed:\n%s", plain)
	}
	assertRenderedLinesFit(t, output, 80)
}

func TestDefaultHumanColorPolicyKeepsNonTTYDestinationsANSIFree(t *testing.T) {
	t.Parallel()

	response := semanticPropertyResponse()
	assertDefaultHumanOutputANSIFree(t, "buffer", &bytes.Buffer{}, response)

	filePath := filepath.Join(t.TempDir(), "redirected.txt")
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("create redirected output: %v", err)
	}
	if err := RenderWithOptionsTo(response, RenderOptions{Format: FormatTable}, file); err != nil {
		_ = file.Close()
		t.Fatalf("render redirected output: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close redirected output: %v", err)
	}
	redirected, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read redirected output: %v", err)
	}
	assertNoANSISequence(t, "redirected output", string(redirected))

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	if err := RenderWithOptionsTo(response, RenderOptions{Format: FormatTable}, writer); err != nil {
		_ = writer.Close()
		_ = reader.Close()
		t.Fatalf("render piped output: %v", err)
	}
	if err := writer.Close(); err != nil {
		_ = reader.Close()
		t.Fatalf("close pipe writer: %v", err)
	}
	piped, err := io.ReadAll(reader)
	if err != nil {
		_ = reader.Close()
		t.Fatalf("read piped output: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close pipe reader: %v", err)
	}
	assertNoANSISequence(t, "piped output", string(piped))
}

func TestSemanticDeclarationsRejectUnknownRolesAndMissingRoleKeys(t *testing.T) {
	t.Parallel()

	responses := []*plugin.Response{
		{
			Status: "success",
			Data:   map[string]any{"value": "unsafe"},
			HumanProperties: &plugin.HumanProperties{Properties: []plugin.HumanProperty{
				{Key: "value", Label: "Value", Role: "red"},
			}},
		},
		{
			Status: "success",
			Data:   map[string]any{"items": []map[string]any{{"severity": "ERROR"}}},
			HumanTable: &plugin.HumanTable{Columns: []plugin.HumanColumn{
				{Key: "severity", Label: "Severity", RoleKey: "missing", Essential: true},
			}},
		},
	}
	for index, response := range responses {
		var output bytes.Buffer
		if err := RenderWithOptionsTo(response, RenderOptions{Format: FormatTable}, &output); err == nil {
			t.Fatalf("case %d accepted invalid semantic declaration: %q", index, output.String())
		}
	}
}

func TestSemanticPresentationMetadataCrossesTransportButStaysOutOfMachineJSON(t *testing.T) {
	t.Parallel()

	response := semanticPropertyResponse()
	response.HumanTable = &plugin.HumanTable{
		Title: "Diagnostics",
		Columns: []plugin.HumanColumn{
			{Key: "severity", Label: "Severity", RoleKey: "role", Essential: true},
		},
		Rows: []map[string]any{{"severity": "ERROR", "role": string(plugin.HumanStyleError)}},
		Details: &plugin.HumanProperties{Properties: []plugin.HumanProperty{
			{Label: "ERROR · CONFIG_INVALID", Role: plugin.HumanStyleError, Emphasized: true, Heading: true},
		}},
	}
	transport, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal transport: %v", err)
	}
	for _, field := range []string{`"title"`, `"role_key"`, `"role"`, `"emphasized"`, `"heading"`} {
		if !bytes.Contains(transport, []byte(field)) {
			t.Fatalf("transport omitted semantic field %s: %s", field, transport)
		}
	}

	var publicJSON bytes.Buffer
	if err := RenderTo(response, FormatJSON, &publicJSON); err != nil {
		t.Fatalf("render public JSON: %v", err)
	}
	for _, value := range []string{"Semantic output", "Diagnostics", "role_key", "emphasized", "human_properties", "human_table", "\x1b["} {
		if strings.Contains(publicJSON.String(), value) {
			t.Fatalf("public JSON leaked presentation value %q: %s", value, publicJSON.String())
		}
	}
	if !strings.Contains(publicJSON.String(), `"machine": "unchanged"`) {
		t.Fatalf("public JSON lost machine data: %s", publicJSON.String())
	}
}

func semanticPropertyResponse() *plugin.Response {
	return &plugin.Response{
		Status: "success",
		Data:   map[string]any{"machine": "unchanged"},
		HumanProperties: &plugin.HumanProperties{
			Title: "Semantic output",
			Properties: []plugin.HumanProperty{
				{Label: "Emphasis", Value: "emphasis", Role: plugin.HumanStyleEmphasis, Emphasized: true},
				{Label: "Success", Value: "success", Role: plugin.HumanStyleSuccess},
				{Label: "Warning", Value: "warning", Role: plugin.HumanStyleWarning},
				{Label: "Error", Value: "error", Role: plugin.HumanStyleError},
				{Label: "Information", Value: "info", Role: plugin.HumanStyleInfo},
				{Label: "Secondary", Value: "muted", Role: plugin.HumanStyleMuted},
			},
		},
	}
}

func renderStyledResponseForTest(t *testing.T, response *plugin.Response, colors ColorProvider) string {
	t.Helper()
	var output bytes.Buffer
	if err := RenderWithOptionsTo(response, RenderOptions{
		Format:        FormatTable,
		WidthProvider: fixedOutputWidth{width: 80, available: true},
		ColorProvider: colors,
	}, &output); err != nil {
		t.Fatalf("render styled response: %v", err)
	}
	return output.String()
}

func assertDefaultHumanOutputANSIFree(t *testing.T, name string, writer *bytes.Buffer, response *plugin.Response) {
	t.Helper()
	if err := RenderWithOptionsTo(response, RenderOptions{Format: FormatTable}, writer); err != nil {
		t.Fatalf("render %s: %v", name, err)
	}
	assertNoANSISequence(t, name, writer.String())
}
