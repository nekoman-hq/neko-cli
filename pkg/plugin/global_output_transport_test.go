package plugin

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestPluginContextKeepsOnlyExecutionOwnedTransportFields(t *testing.T) {
	t.Parallel()

	contextType := reflect.TypeOf(Context{})
	want := []struct {
		name string
		json string
	}{
		{name: "WorkingDir", json: "working_dir"},
		{name: "User", json: "user"},
		{name: "Verbose", json: "verbose"},
	}
	if contextType.NumField() != len(want) {
		t.Fatalf("Context field count = %d, want %d", contextType.NumField(), len(want))
	}
	for index, fieldContract := range want {
		field := contextType.Field(index)
		if field.Name != fieldContract.name || field.Tag.Get("json") != fieldContract.json {
			t.Fatalf("Context field %d = name %q json %q, want %#v", index, field.Name, field.Tag.Get("json"), fieldContract)
		}
	}
}

func TestCorePresentationChoicesAreAbsentFromPluginRequestJSON(t *testing.T) {
	t.Parallel()

	request := Request{
		Command: "pipeline",
		Flags:   map[string]any{"unit": "cli"},
		Context: Context{WorkingDir: "/private/tmp/neko-cli-request", User: "tester", Verbose: true},
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	for _, forbidden := range []string{"describe", "output", "authorization", "credential", "token"} {
		if strings.Contains(strings.ToLower(string(data)), forbidden) {
			t.Fatalf("request JSON contains Core-only or secret-bearing field %q: %s", forbidden, data)
		}
	}
	if !strings.Contains(string(data), `"verbose":true`) {
		t.Fatalf("request JSON omitted Context.Verbose: %s", data)
	}
}
