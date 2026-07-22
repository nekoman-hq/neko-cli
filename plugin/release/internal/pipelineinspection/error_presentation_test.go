package pipelineinspection

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestPipelineStructuredFailuresRenderOnceAndKeepJSONErrors(t *testing.T) {
	multi := writePipelineRepository(t, []pipelineFixtureUnit{
		{ID: "cli", Version: "1.2.3"}, {ID: "plugin-release", Version: "2.0.0"},
	})
	v1Directory := t.TempDir()
	//nolint:staticcheck // The error matrix must preserve the typed V1 unsupported contract.
	writePipelineFile(t, filepath.Join(v1Directory, releaseconfig.V1FileName), `{"project-name":"legacy","project-owner":"owner","project-type":"backend","release-system":"goreleaser","version":"1.2.3"}`)
	v1, err := workspace.ValidateRepositoryRoot(v1Directory)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		root workspace.RepositoryRoot
		code string
		req  plugin.Request
	}{
		{name: "missing unit", root: multi, req: plugin.Request{Command: pipelineCommandName}, code: "PIPELINE_UNIT_INVALID"},
		{name: "unknown unit", root: multi, req: plugin.Request{Command: pipelineCommandName, Flags: map[string]any{"unit": "missing"}}, code: "PIPELINE_UNIT_INVALID"},
		{name: "unsupported V1 source", root: v1, req: plugin.Request{Command: pipelineCommandName}, code: "PIPELINE_SOURCE_UNSUPPORTED"},
		{name: "malformed request", root: multi, req: plugin.Request{Command: pipelineCommandName, Args: []string{"cli"}}, code: "INVALID_PIPELINE_REQUEST"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := runPipelineAt(t, test.root, test.req)
			if response.ExitCode != 1 || response.Status != "error" || response.Error == nil || response.Error.Code != test.code {
				t.Fatalf("structured error envelope = %#v", response)
			}

			var human bytes.Buffer
			if err := renderer.RenderTo(response, renderer.FormatTable, &human); err != nil {
				t.Fatal(err)
			}
			for value, want := range map[string]int{
				"✗ ERROR": 1, response.Error.Code: 1, response.Error.Message: 1,
			} {
				if got := strings.Count(human.String(), value); got != want {
					t.Errorf("human %q count = %d, want %d:\n%s", value, got, want, human.String())
				}
			}
			if strings.Contains(human.String(), "Error: "+response.Error.Code) {
				t.Fatalf("human output contains a Core fallback duplicate:\n%s", human.String())
			}

			var machine bytes.Buffer
			if err := renderer.RenderTo(response, renderer.FormatJSON, &machine); err != nil {
				t.Fatal(err)
			}
			if !json.Valid(machine.Bytes()) {
				t.Fatalf("JSON error is invalid: %s", machine.String())
			}
			var envelope struct {
				Error plugin.ResponseError `json:"error"`
			}
			if err := json.Unmarshal(machine.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Error.Code != response.Error.Code || envelope.Error.Message != response.Error.Message ||
				strings.Count(machine.String(), response.Error.Code) != 1 ||
				strings.Contains(machine.String(), "✗ ERROR") || strings.Contains(machine.String(), "Error: "+response.Error.Code) {
				t.Fatalf("JSON error contains duplicate textual rendering: %s", machine.String())
			}
		})
	}
}
