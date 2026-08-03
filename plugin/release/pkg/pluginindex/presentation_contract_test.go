package pluginindex

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

func TestPluginIndexRawResponseRemainsUndecorated(t *testing.T) {
	root := newIndexTestRepo(t)
	t.Chdir(root)

	response, err := HandlePluginIndex(pluginRequest(nil))
	if err != nil {
		t.Fatalf("HandlePluginIndex: %v", err)
	}
	if response.RendererHint != "raw-json" || response.PresentationProperties != nil ||
		response.PresentationTable != nil || response.PresentationText != nil || len(response.Logs) != 0 {
		t.Fatalf("raw response gained presentation or logs: %#v", response)
	}
}

func TestPluginIndexCheckPresentationSeparatesSummaryFromCompleteFacts(t *testing.T) {
	root := newIndexTestRepo(t)
	t.Chdir(root)

	response, err := HandlePluginIndex(pluginRequest(map[string]any{"check": true}))
	if err != nil {
		t.Fatalf("HandlePluginIndex: %v", err)
	}
	assertPluginIndexPresentationProperties(t, response.PresentationProperties, "Plugin Index Check", map[string]string{
		"Repository":   DefaultRepository,
		"Validation":   "Valid",
		"Repositories": "1",
		"Plugins":      "2",
		"Next action":  "Use --output-file <path> to persist the validated schema-v1 artifact.",
	})
	assertPluginIndexPresentationTables(t, response.PresentationTable, []string{
		"Source Resolution",
		"Repository Inventory",
		"Plugin Inventory",
		"Validation Checks",
		"Limitations",
	})
	for table := response.PresentationTable; table != nil; table = table.Following {
		if !table.DescribeOnly {
			t.Fatalf("check detail table %q is visible by default", table.Title)
		}
	}
	wantItems := []map[string]any{
		{"property": "Status", "value": "ok"},
		{"property": "Plugins", "value": 2},
		{"property": "Repository", "value": DefaultRepository},
	}
	assertIndexItems(t, response.Data["items"], wantItems)
}

func TestPluginIndexPersistPresentationUsesSafeTargetAndStableMachineItems(t *testing.T) {
	root := newIndexTestRepo(t)
	t.Chdir(root)
	targetRoot := t.TempDir()
	output := filepath.Join(targetRoot, "nested", "plugin-index.json")

	response, err := HandlePluginIndex(pluginRequest(map[string]any{"output-file": output}))
	if err != nil {
		t.Fatalf("HandlePluginIndex: %v", err)
	}
	assertPluginIndexPresentationProperties(t, response.PresentationProperties, "Plugin Index Write", map[string]string{
		"Result":       "Written",
		"Output file":  "external/plugin-index.json",
		"Format":       "Pretty",
		"Repositories": "1",
		"Plugins":      "2",
		"Validation":   "Valid",
		"Next action":  "Inspect the generated artifact; publication remains a separate external operation.",
	})
	assertPluginIndexPresentationTables(t, response.PresentationTable, []string{
		"Source Resolution",
		"Repository Inventory",
		"Plugin Inventory",
		"Validation Checks",
		"Write Plan",
		"Limitations",
	})
	for table := response.PresentationTable; table != nil; table = table.Following {
		if !table.DescribeOnly {
			t.Fatalf("persist detail table %q is visible by default", table.Title)
		}
	}
	wantItems := []map[string]any{
		{"property": "Status", "value": "written"},
		{"property": "Output", "value": output},
		{"property": "Plugins", "value": 2},
		{"property": "Repository", "value": DefaultRepository},
	}
	assertIndexItems(t, response.Data["items"], wantItems)
	if got := pluginIndexPresentationValues(response.PresentationProperties); strings.Contains(got, targetRoot) {
		t.Fatalf("human presentation exposed absolute fixture root: %s", got)
	}
}

func assertPluginIndexPresentationProperties(
	t *testing.T,
	properties *presentation.Properties,
	title string,
	want map[string]string,
) {
	t.Helper()
	if properties == nil || properties.Title != title {
		t.Fatalf("presentation properties = %#v, want title %q", properties, title)
	}
	got := map[string]string{}
	for _, property := range properties.Properties {
		got[property.Label] = fmt.Sprint(property.Value)
	}
	for label, value := range want {
		if got[label] != value {
			t.Fatalf("presentation property %q = %q, want %q (all: %#v)", label, got[label], value, got)
		}
	}
}

func assertPluginIndexPresentationTables(t *testing.T, table *presentation.Table, want []string) {
	t.Helper()
	got := make([]string, 0, len(want))
	for current := table; current != nil; current = current.Following {
		got = append(got, current.Title)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("presentation tables = %v, want %v", got, want)
	}
}

func pluginIndexPresentationValues(properties *presentation.Properties) string {
	if properties == nil {
		return ""
	}
	values := make([]string, 0, len(properties.Properties))
	for _, property := range properties.Properties {
		values = append(values, fmt.Sprint(property.Value))
	}
	return strings.Join(values, "\n")
}
