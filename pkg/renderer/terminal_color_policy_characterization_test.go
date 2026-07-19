package renderer

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/internal/terminal"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestPresentationRendererUsesEstablishedLoggerPaletteAndResets(t *testing.T) {
	t.Parallel()

	response := &plugin.Response{
		Status: "success",
		Data: map[string]any{"items": []map[string]any{
			{"name": "api", "status": "ready", "version": "v1.2.3"},
		}},
	}
	var output bytes.Buffer
	if err := RenderWithOptionsTo(response, RenderOptions{
		Format:        FormatTable,
		ColorProvider: fixedColorProvider(true),
	}, &output); err != nil {
		t.Fatalf("RenderTo: %v", err)
	}

	got := output.String()
	for name, sequence := range map[string]string{
		"information heading": terminal.Cyan + terminal.Bold,
		"muted separator":     terminal.BrightBlack,
		"success":             terminal.Green,
		"version":             terminal.Purple,
		"reset":               terminal.Reset,
	} {
		if !strings.Contains(got, sequence) {
			t.Errorf("human output omitted %s sequence %q: %q", name, sequence, got)
		}
	}
	if !strings.HasSuffix(got, terminal.Reset+"   \n") {
		t.Fatalf("human row does not reset its final styled value: %q", got)
	}
}

type fixedColorProvider bool

func (enabled fixedColorProvider) ColorEnabled(io.Writer) bool {
	return bool(enabled)
}

func TestMachineOutputModesRemainANSIFree(t *testing.T) {
	t.Parallel()

	response := &plugin.Response{
		Status: "success",
		Data:   map[string]any{"unit": "api", "value": "\x1b-not-an-escape"},
		GitHubOutput: &plugin.GitHubOutput{Fields: []plugin.GitHubOutputField{
			{Name: "unit", DataKey: "unit"},
		}},
	}

	var publicJSON bytes.Buffer
	if err := RenderTo(response, FormatJSON, &publicJSON); err != nil {
		t.Fatalf("render JSON: %v", err)
	}
	assertNoANSISequence(t, "JSON", publicJSON.String())

	rawResponse := *response
	rawResponse.RendererHint = "raw-json"
	rawResponse.Data = map[string]any{"raw": `{"unit":"api"}`}
	var rawJSON bytes.Buffer
	if err := RenderWithOptionsTo(&rawResponse, RenderOptions{Format: FormatTable}, &rawJSON); err != nil {
		t.Fatalf("render raw JSON: %v", err)
	}
	assertNoANSISequence(t, "raw JSON", rawJSON.String())

	destination := filepath.Join(t.TempDir(), "github-output")
	if err := os.WriteFile(destination, nil, 0o600); err != nil {
		t.Fatalf("create GitHub output: %v", err)
	}
	if err := RenderWithOptionsTo(response, RenderOptions{
		Format:           FormatGitHub,
		GitHubOutputFile: destination,
	}, &bytes.Buffer{}); err != nil {
		t.Fatalf("render GitHub output: %v", err)
	}
	githubOutput, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read GitHub output: %v", err)
	}
	assertNoANSISequence(t, "GitHub output", string(githubOutput))
}

func assertNoANSISequence(t *testing.T, outputName, output string) {
	t.Helper()
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("%s contains ANSI: %q", outputName, output)
	}
}
