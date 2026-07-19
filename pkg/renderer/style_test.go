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
	"github.com/nekoman-hq/neko-cli/internal/terminal"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

func TestSemanticPresentationStylesUseSharedPaletteOnlyWhenEnabled(t *testing.T) {
	t.Parallel()

	response := semanticPropertyResponse()
	enabled := renderStyledResponseForTest(t, response, fixedColorProvider(true))
	for name, sequence := range map[string]string{
		"title":    terminal.Bold + "Semantic output" + terminal.Reset,
		"emphasis": terminal.Bold + "emphasis" + terminal.Reset,
		"success":  terminal.Green + "success" + terminal.Reset,
		"warning":  terminal.Yellow + "warning" + terminal.Reset,
		"error":    terminal.Red + "error" + terminal.Reset,
		"info":     terminal.Cyan + "info" + terminal.Reset,
		"muted":    terminal.BrightBlack + "muted" + terminal.Reset,
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
		PresentationTable: &presentation.Table{
			Title: "Diagnostics",
			Columns: []presentation.Column{
				{Key: "severity", Label: "Severity", RoleKey: "severity_role", Essential: true},
				{Key: "code", Label: "Code", RoleKey: "severity_role", Essential: true},
				{Key: "target", Label: "Target"},
			},
			Rows: []map[string]any{{
				"severity": "ERROR", "code": "CONFIG_INVALID", "target": "api", "severity_role": string(presentation.StyleError),
			}},
		},
	}
	output := renderStyledResponseForTest(t, response, fixedColorProvider(true))
	for _, styled := range []string{
		terminal.Red + "ERROR" + terminal.Reset,
		terminal.Red + "CONFIG_INVALID" + terminal.Reset,
	} {
		if !strings.Contains(output, styled) {
			t.Fatalf("semantic table omitted %q:\n%q", styled, output)
		}
	}
	if strings.Contains(output, terminal.Red+"api") {
		t.Fatalf("semantic table colored the neutral target cell:\n%q", output)
	}
}

func TestPresentationPropertyHeadingsCreateResponsiveStyledRecords(t *testing.T) {
	t.Parallel()

	response := &plugin.Response{
		Status: "success",
		Data:   map[string]any{"machine": "unchanged"},
		PresentationProperties: &presentation.Properties{Properties: []presentation.Property{
			{Label: "ERROR · CONFIG_INVALID", Role: presentation.StyleError, Emphasized: true, Heading: true},
			{Label: "Scope", Value: "source"},
			{Label: "Message", Value: "Configuration is invalid."},
			{Label: "WARNING · CONCURRENCY_MISSING", Role: presentation.StyleWarning, Emphasized: true, Heading: true},
			{Label: "Workflow", Value: ".github/workflows/release.yml"},
		}},
	}
	output := renderStyledResponseForTest(t, response, fixedColorProvider(true))
	if !strings.Contains(output, terminal.Red+terminal.Bold+"ERROR · CONFIG_INVALID"+terminal.Reset) ||
		!strings.Contains(output, terminal.Yellow+terminal.Bold+"WARNING · CONCURRENCY_MISSING"+terminal.Reset) {
		t.Fatalf("record headings lost semantic styles:\n%q", output)
	}
	plain := ansi.Strip(output)
	if strings.Contains(plain, "PROPERTY") || strings.Count(plain, "CONFIG_INVALID") != 1 ||
		!strings.Contains(plain, "Scope\n  source") || !strings.Contains(plain, "Workflow\n  .github/workflows/release.yml") {
		t.Fatalf("record hierarchy changed:\n%s", plain)
	}
	assertRenderedLinesFit(t, output, 80)
}

func TestDefaultPresentationColorPolicyKeepsNonTTYDestinationsANSIFree(t *testing.T) {
	t.Parallel()

	response := semanticPropertyResponse()
	assertDefaultPresentationOutputANSIFree(t, "buffer", &bytes.Buffer{}, response)

	filePath := filepath.Join(t.TempDir(), "redirected.txt")
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("create redirected output: %v", err)
	}
	if renderErr := RenderWithOptionsTo(response, RenderOptions{Format: FormatTable}, file); renderErr != nil {
		_ = file.Close()
		t.Fatalf("render redirected output: %v", renderErr)
	}
	if closeErr := file.Close(); closeErr != nil {
		t.Fatalf("close redirected output: %v", closeErr)
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
	if renderErr := RenderWithOptionsTo(response, RenderOptions{Format: FormatTable}, writer); renderErr != nil {
		_ = writer.Close()
		_ = reader.Close()
		t.Fatalf("render piped output: %v", renderErr)
	}
	if closeErr := writer.Close(); closeErr != nil {
		_ = reader.Close()
		t.Fatalf("close pipe writer: %v", closeErr)
	}
	piped, err := io.ReadAll(reader)
	if err != nil {
		_ = reader.Close()
		t.Fatalf("read piped output: %v", err)
	}
	if closeErr := reader.Close(); closeErr != nil {
		t.Fatalf("close pipe reader: %v", closeErr)
	}
	assertNoANSISequence(t, "piped output", string(piped))
}

func TestSemanticDeclarationsRejectUnknownRolesAndMissingRoleKeys(t *testing.T) {
	t.Parallel()

	responses := []*plugin.Response{
		{
			Status: "success",
			Data:   map[string]any{"value": "unsafe"},
			PresentationProperties: &presentation.Properties{Properties: []presentation.Property{
				{Key: "value", Label: "Value", Role: "red"},
			}},
		},
		{
			Status: "success",
			Data:   map[string]any{"items": []map[string]any{{"severity": "ERROR"}}},
			PresentationTable: &presentation.Table{Columns: []presentation.Column{
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
	response.PresentationTable = &presentation.Table{
		Title: "Diagnostics",
		Columns: []presentation.Column{
			{Key: "severity", Label: "Severity", RoleKey: "role", Essential: true},
		},
		Rows: []map[string]any{{"severity": "ERROR", "role": string(presentation.StyleError)}},
		Details: &presentation.Properties{Properties: []presentation.Property{
			{Label: "ERROR · CONFIG_INVALID", Role: presentation.StyleError, Emphasized: true, Heading: true},
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
		PresentationProperties: &presentation.Properties{
			Title: "Semantic output",
			Properties: []presentation.Property{
				{Label: "Emphasis", Value: "emphasis", Role: presentation.StyleEmphasis, Emphasized: true},
				{Label: "Success", Value: "success", Role: presentation.StyleSuccess},
				{Label: "Warning", Value: "warning", Role: presentation.StyleWarning},
				{Label: "Error", Value: "error", Role: presentation.StyleError},
				{Label: "Information", Value: "info", Role: presentation.StyleInfo},
				{Label: "Secondary", Value: "muted", Role: presentation.StyleMuted},
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

func assertDefaultPresentationOutputANSIFree(t *testing.T, name string, writer *bytes.Buffer, response *plugin.Response) {
	t.Helper()
	if err := RenderWithOptionsTo(response, RenderOptions{Format: FormatTable}, writer); err != nil {
		t.Fatalf("render %s: %v", name, err)
	}
	assertNoANSISequence(t, name, writer.String())
}
