package pipelineinspection

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestPipelineDescribeTransportCharacterization(t *testing.T) {
	response := transportedPipelineResponse(t, mapPipelineResult(pipelinePresentationFixture()))
	response.Logs = []plugin.LogEntry{{Timestamp: "10:11:12", Level: "verbose", Message: "V$ inspected pipeline"}}
	for table := response.PresentationTable; table != nil; table = table.Following {
		table.DescribeOnly = false
	}

	defaultOutput := renderPipelineTransport(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, WidthProvider: pipelineTestWidth{width: 120, available: true},
	})
	describeOutput := renderPipelineTransport(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true, Verbose: true,
		WidthProvider: pipelineTestWidth{width: 120, available: true},
	})
	for _, section := range []string{"Summary", "Verification Facts", "Configured Pipeline", "Limitations"} {
		if !strings.Contains(defaultOutput, section) || !strings.Contains(describeOutput, section) {
			t.Fatalf("current default/describe transport omitted %q\ndefault:\n%s\ndescribe:\n%s", section, defaultOutput, describeOutput)
		}
	}
	if strings.Contains(defaultOutput, "Command Metadata") || strings.Contains(defaultOutput, "Execution Logs") {
		t.Fatalf("default unexpectedly rendered describe framing:\n%s", defaultOutput)
	}
	for _, section := range []string{"Command Metadata", "Execution Logs", "Output"} {
		if !strings.Contains(describeOutput, section) {
			t.Fatalf("current describe framing omitted %q:\n%s", section, describeOutput)
		}
	}
}

func TestPipelineDescribeJSONCharacterizationIsIdentical(t *testing.T) {
	response := transportedPipelineResponse(t, mapPipelineResult(pipelinePresentationFixture()))
	plain := renderPipelineTransport(t, response, renderer.RenderOptions{Format: renderer.FormatJSON})
	described := renderPipelineTransport(t, response, renderer.RenderOptions{Format: renderer.FormatJSON, Describe: true})
	if plain != described {
		t.Fatalf("describe changed JSON\nplain:\n%s\ndescribed:\n%s", plain, described)
	}
	for _, forbidden := range []string{"human_table", "human_properties", "following", "group_key", "note"} {
		if strings.Contains(plain, forbidden) {
			t.Fatalf("public JSON exposed presentation key %q: %s", forbidden, plain)
		}
	}
}

func transportedPipelineResponse(t *testing.T, response *plugin.Response) *plugin.Response {
	t.Helper()
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var transported plugin.Response
	if err := json.Unmarshal(encoded, &transported); err != nil {
		t.Fatal(err)
	}
	return &transported
}

func renderPipelineTransport(t *testing.T, response *plugin.Response, options renderer.RenderOptions) string {
	t.Helper()
	options.ColorProvider = pipelineTestColor(false)
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, options, &output); err != nil {
		t.Fatal(err)
	}
	return output.String()
}
