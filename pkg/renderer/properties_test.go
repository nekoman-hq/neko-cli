package renderer

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

func TestPresentationPropertiesRenderInDeclaredOrder(t *testing.T) {
	response := &plugin.Response{
		Status: "success",
		Data: map[string]any{
			"unit":         "api",
			"version":      "2.4.0",
			"head_matches": true,
		},
		PresentationProperties: &presentation.Properties{Properties: []presentation.Property{
			{Key: "version", Label: "Version"},
			{Key: "unit", Label: "Unit"},
			{Key: "head_matches", Label: "HEAD matches"},
		}},
	}

	var output bytes.Buffer
	if err := RenderTo(response, FormatTable, &output); err != nil {
		t.Fatalf("RenderTo: %v", err)
	}
	plain := ansi.Strip(output.String())
	version := strings.Index(plain, "Version")
	unit := strings.Index(plain, "Unit")
	head := strings.Index(plain, "HEAD matches")
	if version < 0 || unit <= version || head <= unit || !strings.Contains(plain, "true") {
		t.Fatalf("human properties were not rendered in declaration order:\n%s", plain)
	}
}

func TestPresentationPropertyAndIntegrationMetadataStayOutOfPublicJSON(t *testing.T) {
	response := githubOutputTestResponse()
	response.ExitCode = 1
	response.PresentationProperties = &presentation.Properties{Properties: []presentation.Property{{Key: "unit", Label: "Unit"}}}

	transport, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal transport: %v", err)
	}
	for _, key := range []string{`"human_properties"`, `"github_output"`, `"exit_code"`} {
		if !bytes.Contains(transport, []byte(key)) {
			t.Fatalf("wire response omitted %s: %s", key, transport)
		}
	}

	var output bytes.Buffer
	if err := RenderTo(response, FormatJSON, &output); err != nil {
		t.Fatalf("RenderTo JSON: %v", err)
	}
	for _, key := range []string{"human_properties", "github_output", "exit_code"} {
		if strings.Contains(output.String(), key) {
			t.Fatalf("transport metadata %q leaked into public JSON:\n%s", key, output.String())
		}
	}
	if !strings.Contains(output.String(), `"unit": "api"`) {
		t.Fatalf("public JSON lost canonical data:\n%s", output.String())
	}
}

func TestInvalidPresentationPropertiesFailWithoutNondeterministicFallback(t *testing.T) {
	tests := []presentation.Properties{
		{},
		{Properties: []presentation.Property{{Key: "unit", Label: " Unit"}}},
		{Properties: []presentation.Property{{Key: "unit", Label: "Unit"}, {Key: "unit", Label: "Again"}}},
		{Properties: []presentation.Property{{Key: "missing", Label: "Missing"}}},
		{Properties: []presentation.Property{{Key: "unit", Label: "Ambiguous", Value: "direct"}}},
		{Properties: []presentation.Property{{Label: "Missing value"}}},
	}
	for index := range tests {
		response := &plugin.Response{Status: "success", Data: map[string]any{"unit": "api"}, PresentationProperties: &tests[index]}
		var output bytes.Buffer
		if err := RenderTo(response, FormatTable, &output); err == nil {
			t.Fatalf("case %d unexpectedly rendered: %q", index, output.String())
		}
	}
}
