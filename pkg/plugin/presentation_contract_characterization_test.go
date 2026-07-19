package plugin

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPresentationTransportContractCharacterization(t *testing.T) {
	t.Parallel()

	response := Response{
		Status: "success",
		HumanTable: &HumanTable{
			Columns: []HumanColumn{{Key: "unit", Label: "Unit", RoleKey: "role", Essential: true}},
			Rows:    []map[string]any{{"unit": "api"}},
			Details: &HumanProperties{Properties: []HumanProperty{{Label: "State", Value: "ready"}}},
			Title:   "Units",
		},
		HumanProperties: &HumanProperties{
			Properties: []HumanProperty{{Key: "unit", Label: "Unit", Role: HumanStyleInfo, Emphasized: true, Heading: true}},
			Title:      "Summary",
		},
		HumanText: &HumanText{Content: "preview\n"},
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	want := `{"status":"success","metadata":{"timestamp":"0001-01-01T00:00:00Z","plugin":"","version":"","command":""},"human_table":{"columns":[{"key":"unit","label":"Unit","role_key":"role","essential":true}],"rows":[{"unit":"api"}],"details":{"properties":[{"label":"State","value":"ready"}]},"title":"Units"},"human_properties":{"properties":[{"key":"unit","label":"Unit","role":"info","emphasized":true,"heading":true}],"title":"Summary"},"human_text":{"content":"preview\n"}}`
	if string(encoded) != want {
		t.Fatalf("presentation wire changed\nwant: %s\n got: %s", want, encoded)
	}

	var decoded Response
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !reflect.DeepEqual(decoded.HumanTable, response.HumanTable) ||
		!reflect.DeepEqual(decoded.HumanProperties, response.HumanProperties) ||
		!reflect.DeepEqual(decoded.HumanText, response.HumanText) {
		t.Fatalf("deprecated presentation fields did not round trip\nwant: %#v\n got: %#v", response, decoded)
	}
	if !reflect.DeepEqual(decoded.PresentationTable, response.HumanTable) ||
		!reflect.DeepEqual(decoded.PresentationProperties, response.HumanProperties) ||
		!reflect.DeepEqual(decoded.PresentationText, response.HumanText) {
		t.Fatalf("canonical presentation fields were not populated\nwant: %#v\n got: %#v", response, decoded)
	}
}

func TestExportedPresentationTypeShapeCharacterization(t *testing.T) {
	t.Parallel()

	assertJSONFields(t, reflect.TypeOf(HumanTable{}), []fieldContract{
		{name: "Columns", jsonName: "columns", typeName: "[]presentation.Column"},
		{name: "Rows", jsonName: "rows,omitempty", typeName: "[]map[string]interface {}"},
		{name: "Details", jsonName: "details,omitempty", typeName: "*presentation.Properties"},
		{name: "Title", jsonName: "title,omitempty", typeName: "string"},
	})
	assertJSONFields(t, reflect.TypeOf(HumanColumn{}), []fieldContract{
		{name: "Key", jsonName: "key", typeName: "string"},
		{name: "Label", jsonName: "label", typeName: "string"},
		{name: "RoleKey", jsonName: "role_key,omitempty", typeName: "string"},
		{name: "Essential", jsonName: "essential,omitempty", typeName: "bool"},
	})
	assertJSONFields(t, reflect.TypeOf(HumanProperties{}), []fieldContract{
		{name: "Properties", jsonName: "properties", typeName: "[]presentation.Property"},
		{name: "Title", jsonName: "title,omitempty", typeName: "string"},
	})
	assertJSONFields(t, reflect.TypeOf(HumanProperty{}), []fieldContract{
		{name: "Key", jsonName: "key,omitempty", typeName: "string"},
		{name: "Label", jsonName: "label", typeName: "string"},
		{name: "Value", jsonName: "value,omitempty", typeName: "interface {}"},
		{name: "Role", jsonName: "role,omitempty", typeName: "presentation.StyleRole"},
		{name: "Emphasized", jsonName: "emphasized,omitempty", typeName: "bool"},
		{name: "Heading", jsonName: "heading,omitempty", typeName: "bool"},
	})
	assertJSONFields(t, reflect.TypeOf(HumanText{}), []fieldContract{
		{name: "Content", jsonName: "content", typeName: "string"},
	})

	wantRoles := []HumanStyleRole{
		HumanStyleDefault,
		HumanStyleEmphasis,
		HumanStyleSuccess,
		HumanStyleWarning,
		HumanStyleError,
		HumanStyleInfo,
		HumanStyleMuted,
	}
	wantValues := []string{"default", "emphasis", "success", "warning", "error", "info", "muted"}
	for index, role := range wantRoles {
		if string(role) != wantValues[index] {
			t.Fatalf("style role %d changed: want %q, got %q", index, wantValues[index], role)
		}
	}
}

type fieldContract struct {
	name     string
	jsonName string
	typeName string
}

func assertJSONFields(t *testing.T, valueType reflect.Type, want []fieldContract) {
	t.Helper()
	if valueType.NumField() != len(want) {
		t.Fatalf("%s field count changed: want %d, got %d", valueType, len(want), valueType.NumField())
	}
	for index, contract := range want {
		field := valueType.Field(index)
		if field.Name != contract.name || field.Tag.Get("json") != contract.jsonName || field.Type.String() != contract.typeName {
			t.Fatalf("%s field %d changed: want %#v, got name=%q json=%q type=%q",
				valueType, index, contract, field.Name, field.Tag.Get("json"), field.Type)
		}
	}
}
