package releaseworkflow

import (
	"reflect"
	"testing"
)

func TestCanonicalDispatchInputContract(t *testing.T) {
	want := []DispatchInputDefinition{
		{Name: "unit", Description: "Neko Release V2 unit id"},
		{Name: "version", Description: "Neko-authoritative release version"},
		{Name: "tag", Description: "Neko-created unit tag"},
		{Name: "release_sha", Description: "Neko-created release commit SHA"},
	}
	if got := CanonicalDispatchInputContract(); !reflect.DeepEqual(got, want) {
		t.Fatalf("contract = %#v, want %#v", got, want)
	}
	values := CanonicalDispatchInputValues("api", "1.2.3", "api/v1.2.3", "release-commit")
	filtered := CanonicalDispatchInputs(map[string]string{
		"unit": values["unit"], "version": values["version"], "tag": values["tag"],
		"release_sha": values["release_sha"], "ignored": "must-not-pass",
	})
	if !reflect.DeepEqual(filtered, values) {
		t.Fatalf("filtered values = %#v, want %#v", filtered, values)
	}
}

func TestCanonicalDispatchInputCollectionsAreFresh(t *testing.T) {
	contract := CanonicalDispatchInputContract()
	contract[0].Name = "changed"
	if got := CanonicalDispatchInputContract()[0].Name; got != DispatchInputUnit {
		t.Fatalf("contract accessor retained caller mutation: %q", got)
	}

	values := CanonicalDispatchInputValues("api", "1.2.3", "api/v1.2.3", "release-commit")
	values[DispatchInputUnit] = "changed"
	if got := CanonicalDispatchInputValues("api", "1.2.3", "api/v1.2.3", "release-commit")[DispatchInputUnit]; got != "api" {
		t.Fatalf("values constructor retained caller mutation: %q", got)
	}

	filtered := CanonicalDispatchInputs(nil)
	if len(filtered) != 4 {
		t.Fatalf("filtered nil input count = %d, want 4", len(filtered))
	}
	for _, value := range filtered {
		if value != "" {
			t.Fatalf("filtered nil input contains %q", value)
		}
	}
}
