package renderer

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestHumanTextRendersPreformattedContentWithoutChangingJSONData(t *testing.T) {
	response := &plugin.Response{
		Status: "success",
		Data: map[string]any{
			"target":            ".github/workflows/release.yml",
			"generated_content": "name: Release\n",
		},
		HumanText: &plugin.HumanText{Content: "Target: .github/workflows/release.yml\n\nname: Release\n"},
	}

	var human bytes.Buffer
	if err := RenderTo(response, FormatTable, &human); err != nil {
		t.Fatalf("RenderTo human text: %v", err)
	}
	if got := human.String(); got != response.HumanText.Content {
		t.Fatalf("human text = %q, want %q", got, response.HumanText.Content)
	}

	var machine bytes.Buffer
	if err := RenderTo(response, FormatJSON, &machine); err != nil {
		t.Fatalf("RenderTo JSON: %v", err)
	}
	var public map[string]any
	if err := json.Unmarshal(machine.Bytes(), &public); err != nil {
		t.Fatalf("decode public JSON: %v", err)
	}
	if _, present := public["human_text"]; present {
		t.Fatal("transport-only human text leaked into public JSON")
	}
	if !strings.Contains(machine.String(), `"generated_content": "name: Release\n"`) {
		t.Fatalf("typed JSON data changed: %s", machine.String())
	}
}

func TestHumanTextRejectsEmptyDeclaration(t *testing.T) {
	response := &plugin.Response{Status: "success", HumanText: &plugin.HumanText{}}
	if err := RenderTo(response, FormatTable, &bytes.Buffer{}); err == nil {
		t.Fatal("empty human text declaration must fail")
	}
}
