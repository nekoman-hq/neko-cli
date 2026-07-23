package renderer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

func TestDeclaredErrorPresentationReplacesUnsafeMachineMessageOnlyInHumanOutput(t *testing.T) {
	response := &plugin.Response{
		Status: "error",
		Error: &plugin.ResponseError{
			Code:    "SETUP_FAILED",
			Message: "failure at /private/tmp/secret-fixture/config.json",
		},
		PresentationTable: &presentation.Table{
			Title: "Setup Blocked",
			Columns: []presentation.Column{
				{Key: "code", Label: "Code", Essential: true},
				{Key: "reason", Label: "Reason", Essential: true},
			},
			Rows: []map[string]any{{
				"code": "SETUP_FAILED", "reason": "The repository-relative configuration is invalid.",
			}},
		},
	}

	var human bytes.Buffer
	if err := RenderTo(response, FormatTable, &human); err != nil {
		t.Fatalf("render human error: %v", err)
	}
	if !strings.Contains(human.String(), "Setup Blocked") || !strings.Contains(human.String(), "SETUP_FAILED") {
		t.Fatalf("declared error presentation was not rendered:\n%s", human.String())
	}
	if strings.Contains(human.String(), "/private/tmp/secret-fixture") {
		t.Fatalf("machine error leaked into declared human presentation:\n%s", human.String())
	}

	var machine bytes.Buffer
	if err := RenderTo(response, FormatJSON, &machine); err != nil {
		t.Fatalf("render JSON error: %v", err)
	}
	if !strings.Contains(machine.String(), "/private/tmp/secret-fixture/config.json") {
		t.Fatalf("JSON lost the established machine error:\n%s", machine.String())
	}
}

func TestUndeclaredErrorPresentationKeepsGenericHumanError(t *testing.T) {
	response := &plugin.Response{
		Status: "error",
		Error:  &plugin.ResponseError{Code: "FAILED", Message: "plain failure"},
	}
	var output bytes.Buffer
	if err := RenderTo(response, FormatTable, &output); err != nil {
		t.Fatalf("render generic error: %v", err)
	}
	for _, want := range []string{"ERROR", "FAILED", "plain failure"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("generic error omitted %q:\n%s", want, output.String())
		}
	}
}
