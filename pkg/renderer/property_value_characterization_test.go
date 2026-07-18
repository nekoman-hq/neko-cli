package renderer

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestPropertyValuePresentationPreservesDeclaredAndItemOrder(t *testing.T) {
	tests := []struct {
		response *plugin.Response
		name     string
	}{
		{
			name: "declared properties",
			response: &plugin.Response{
				Status: "success",
				Data: map[string]any{
					"unit":     "cli",
					"workflow": ".github/workflows/release-neko-cli.yml",
					"status":   "Release plan inspected locally",
				},
				HumanProperties: &plugin.HumanProperties{Properties: []plugin.HumanProperty{
					{Key: "unit", Label: "Unit"},
					{Key: "workflow", Label: "Workflow"},
					{Key: "status", Label: "Status"},
				}},
			},
		},
		{
			name: "property items",
			response: &plugin.Response{
				Status: "success",
				Data: map[string]any{"items": []map[string]any{
					{"property": "Unit", "value": "cli"},
					{"property": "Workflow", "value": ".github/workflows/release-neko-cli.yml"},
					{"property": "Status", "value": "Release plan inspected locally"},
				}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := RenderWithOptionsTo(test.response, RenderOptions{
				Format:        FormatTable,
				WidthProvider: fixedOutputWidth{width: 72, available: true},
			}, &output); err != nil {
				t.Fatalf("RenderWithOptionsTo: %v", err)
			}

			plain := ansi.Strip(output.String())
			unit := strings.Index(plain, "Unit")
			workflow := strings.Index(plain, "Workflow")
			status := strings.Index(plain, "Status")
			if unit < 0 || workflow <= unit || status <= workflow {
				t.Fatalf("property order changed:\n%s", plain)
			}
			for _, value := range []string{"cli", ".github/workflows/release-neko-cli.yml", "Release plan inspected locally"} {
				if !strings.Contains(plain, value) {
					t.Fatalf("property output omitted %q:\n%s", value, plain)
				}
			}
		})
	}
}

func TestPropertyPresentationMetadataDoesNotChangeMachineReadableData(t *testing.T) {
	response := &plugin.Response{
		Status: "success",
		Data: map[string]any{"items": []map[string]any{
			{"property": "Limitations", "value": "local-only: no execution | token-free: no token access"},
		}},
		HumanProperties: &plugin.HumanProperties{Properties: []plugin.HumanProperty{
			{Key: "items", Label: "Ignored in JSON"},
		}},
	}

	var output bytes.Buffer
	if err := RenderTo(response, FormatJSON, &output); err != nil {
		t.Fatalf("RenderTo JSON: %v", err)
	}
	if strings.Contains(output.String(), "human_properties") {
		t.Fatalf("presentation metadata leaked into JSON:\n%s", output.String())
	}

	var decoded struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if len(decoded.Data.Items) != 1 || decoded.Data.Items[0]["property"] != "Limitations" ||
		decoded.Data.Items[0]["value"] != "local-only: no execution | token-free: no token access" {
		t.Fatalf("machine-readable property items changed: %#v", decoded.Data.Items)
	}
}
