package presentation_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

func TestDeclarationsPreserveTransportShape(t *testing.T) {
	t.Parallel()

	declaration := presentation.Table{
		Columns: []presentation.Column{{Key: "unit", Label: "Unit", RoleKey: "role", Essential: true}},
		Rows:    []map[string]any{{"unit": "api"}},
		Details: &presentation.Properties{Properties: []presentation.Property{{Label: "State", Value: "ready"}}},
		Title:   "Units",
	}
	encoded, err := json.Marshal(declaration)
	if err != nil {
		t.Fatalf("marshal table declaration: %v", err)
	}
	want := `{"columns":[{"key":"unit","label":"Unit","role_key":"role","essential":true}],"rows":[{"unit":"api"}],"details":{"properties":[{"label":"State","value":"ready"}]},"title":"Units"}`
	if string(encoded) != want {
		t.Fatalf("table declaration changed\nwant: %s\n got: %s", want, encoded)
	}

	var decoded presentation.Table
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal table declaration: %v", err)
	}
	if !reflect.DeepEqual(decoded, declaration) {
		t.Fatalf("table declaration round trip changed\nwant: %#v\n got: %#v", declaration, decoded)
	}
}

func TestDeclarationZeroValuesAndStyleRoles(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]any{
		"table":      presentation.Table{},
		"column":     presentation.Column{},
		"properties": presentation.Properties{},
		"property":   presentation.Property{},
		"text":       presentation.Text{},
		"style":      presentation.StyleRole(""),
	} {
		if !reflect.ValueOf(value).IsZero() {
			t.Fatalf("%s declaration has a non-zero default: %#v", name, value)
		}
	}

	roles := []presentation.StyleRole{
		presentation.StyleDefault,
		presentation.StyleEmphasis,
		presentation.StyleSuccess,
		presentation.StyleWarning,
		presentation.StyleError,
		presentation.StyleInfo,
		presentation.StyleMuted,
	}
	want := []string{"default", "emphasis", "success", "warning", "error", "info", "muted"}
	for index, role := range roles {
		if string(role) != want[index] {
			t.Fatalf("style role %d changed: want %q, got %q", index, want[index], role)
		}
	}
}
