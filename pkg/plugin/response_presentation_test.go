package plugin_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

func TestDeprecatedPresentationAliasesRemainSourceCompatible(t *testing.T) {
	t.Parallel()

	acceptTable := func(plugin.HumanTable) {}
	acceptColumn := func(plugin.HumanColumn) {}
	acceptProperties := func(plugin.HumanProperties) {}
	acceptProperty := func(plugin.HumanProperty) {}
	acceptText := func(plugin.HumanText) {}
	acceptStyleRole := func(plugin.HumanStyleRole) {}
	acceptTable(presentation.Table{})
	acceptColumn(presentation.Column{})
	acceptProperties(presentation.Properties{})
	acceptProperty(presentation.Property{})
	acceptText(presentation.Text{})
	acceptStyleRole(presentation.StyleRole(""))

	legacy := plugin.Response{
		HumanTable:      &plugin.HumanTable{},
		HumanProperties: &plugin.HumanProperties{},
		HumanText:       &plugin.HumanText{},
	}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal deprecated response fields: %v", err)
	}
	for _, tag := range []string{"human_table", "human_properties", "human_text"} {
		if !strings.Contains(string(encoded), `"`+tag+`"`) {
			t.Fatalf("deprecated response field did not retain %q wire tag: %s", tag, encoded)
		}
	}
}

func TestCanonicalPresentationFieldsUseEstablishedWireTags(t *testing.T) {
	t.Parallel()

	response := plugin.Response{
		PresentationTable:      &presentation.Table{},
		PresentationProperties: &presentation.Properties{},
		PresentationText:       &presentation.Text{},
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal canonical response fields: %v", err)
	}
	for _, tag := range []string{"human_table", "human_properties", "human_text"} {
		if !strings.Contains(string(encoded), `"`+tag+`"`) {
			t.Fatalf("canonical response field did not use established %q wire tag: %s", tag, encoded)
		}
	}
	for _, forbidden := range []string{"presentation_table", "presentation_properties", "presentation_text"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("response introduced unversioned wire tag %q: %s", forbidden, encoded)
		}
	}
}

func TestPresentationFieldResolutionRejectsConflicts(t *testing.T) {
	t.Parallel()

	response := plugin.Response{
		PresentationText: &presentation.Text{Content: "canonical"},
		HumanText:        &plugin.HumanText{Content: "deprecated"},
	}
	if _, err := response.TextPresentation(); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("expected conflicting text declarations to fail, got %v", err)
	}
	if _, err := json.Marshal(response); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("expected conflicting response fields to fail marshaling, got %v", err)
	}
}

func TestPresentationWireDecodePopulatesCanonicalAndDeprecatedFields(t *testing.T) {
	t.Parallel()

	var response plugin.Response
	if err := json.Unmarshal([]byte(`{"status":"success","metadata":{"timestamp":"0001-01-01T00:00:00Z","plugin":"","version":"","command":""},"human_text":{"content":"preview"}}`), &response); err != nil {
		t.Fatalf("decode established presentation wire tag: %v", err)
	}
	if response.PresentationText == nil || response.PresentationText.Content != "preview" {
		t.Fatalf("canonical text presentation not populated: %#v", response.PresentationText)
	}
	if response.HumanText != response.PresentationText {
		t.Fatal("deprecated text field does not mirror canonical decoded declaration")
	}
}

func TestResponseExitPresenceRoundTripsExplicitZero(t *testing.T) {
	t.Parallel()

	response := plugin.Response{Status: "success"}
	response.SetExitCode(0)

	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response with explicit zero exit: %v", err)
	}
	if !strings.Contains(string(encoded), `"exit_code":0`) {
		t.Fatalf("explicit zero exit was omitted from transport: %s", encoded)
	}

	var decoded plugin.Response
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode response with explicit zero exit: %v", err)
	}
	code, present := decoded.ExplicitExitCode()
	if !present || code != 0 {
		t.Fatalf("decoded explicit exit = (%d, %t), want (0, true)", code, present)
	}
}

func TestResponseExitPresenceDistinguishesLegacyUnset(t *testing.T) {
	t.Parallel()

	response := plugin.Response{Status: "error", Error: &plugin.ResponseError{Code: "LEGACY", Message: "legacy response"}}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal legacy response: %v", err)
	}
	if strings.Contains(string(encoded), `"exit_code"`) {
		t.Fatalf("legacy response unexpectedly gained exit intent: %s", encoded)
	}

	var decoded plugin.Response
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode legacy response: %v", err)
	}
	code, present := decoded.ExplicitExitCode()
	if present || code != 0 {
		t.Fatalf("decoded legacy exit = (%d, %t), want (0, false)", code, present)
	}
}

func TestResponseExitPresenceKeepsNonzeroStructLiteralsSourceCompatible(t *testing.T) {
	t.Parallel()

	response := plugin.Response{Status: "error", ExitCode: 7}
	code, present := response.ExplicitExitCode()
	if !present || code != 7 {
		t.Fatalf("nonzero struct literal exit = (%d, %t), want (7, true)", code, present)
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal nonzero struct literal: %v", err)
	}
	if !strings.Contains(string(encoded), `"exit_code":7`) {
		t.Fatalf("nonzero struct literal exit was omitted: %s", encoded)
	}
}
