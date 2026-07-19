package release

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestUnitOverviewHumanOutputKeepsEssentialColumnsAndInvalidUnitsVisible(t *testing.T) {
	root := newUnitOverviewRepository(t)
	config := releaseconfig.V2ReleaseConfig{SchemaVersion: 2, Units: []releaseconfig.V2Unit{
		unitOverviewConfigUnit("worker", "Worker with a deliberately long display name", "worker/v", ".github/workflows/release-shared-with-a-long-name.yml"),
		unitOverviewConfigUnit("api", "API", "api/v", ".github/workflows/release-api.yml"),
	}}
	state := releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{
		"worker": {Version: "2.0.0"},
		"api":    {Version: "not-semver"},
	}}
	writeUnitOverviewPair(t, root.Path(), config, state)
	response := runUnitOverview(t, root)

	normal := ansi.Strip(renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{width: 120, available: true}))
	for _, want := range []string{"Unit", "Version", "Status", "Name", "Tag prefix", "Executor", "Delivery", "api", "not-semver", "invalid"} {
		if !strings.Contains(normal, want) {
			t.Fatalf("normal unit overview omitted %q:\n%s", want, normal)
		}
	}

	narrow := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{width: 28, available: true})
	narrowPlain := ansi.Strip(narrow)
	for _, want := range []string{"Unit", "Version", "Status", "api", "not-semver", "invalid", "worker", "2.0.0", "aligned"} {
		if !strings.Contains(narrowPlain, want) {
			t.Fatalf("narrow unit overview omitted %q:\n%s", want, narrowPlain)
		}
	}
	assertReleasePlanLinesFit(t, narrow, 28)
}

func TestUnitOverviewHumanOutputIsDeterministicAtUnknownWidthAndWrapsWideDetails(t *testing.T) {
	root := newUnitOverviewRepository(t)
	unit := unitOverviewConfigUnit(
		"api",
		"API with a very long but stable display name",
		"api/v",
		".github/workflows/release-api-with-a-very-long-but-still-repository-relative-name.yml",
	)
	writeUnitOverviewPair(t, root.Path(),
		releaseconfig.V2ReleaseConfig{SchemaVersion: 2, Units: []releaseconfig.V2Unit{unit}},
		releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{"api": {Version: "1.2.3"}}},
	)
	response := runUnitOverview(t, root)

	first := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{})
	second := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{})
	if first != second {
		t.Fatalf("unknown-width unit overview is nondeterministic:\nfirst=%q\nsecond=%q", first, second)
	}
	unknown := ansi.Strip(first)
	for _, want := range []string{"Unit: api", "Version: 1.2.3", "Status: aligned", "Workflow:", unit.Executor.Workflow} {
		if !strings.Contains(unknown, want) {
			t.Fatalf("unknown-width unit overview omitted %q:\n%s", want, unknown)
		}
	}

	wide := renderReleasePlanForTest(t, response, renderer.FormatWide, releasePlanOutputWidth{width: 54, available: true})
	widePlain := ansi.Strip(wide)
	if compact := strings.Join(strings.Fields(widePlain), ""); !strings.Contains(compact, strings.ReplaceAll(unit.Executor.Workflow, " ", "")) {
		t.Fatalf("wide unit overview lost the configured workflow path:\n%s", widePlain)
	}
	assertReleasePlanLinesFit(t, wide, 54)
}

func TestUnitOverviewJSONAndMachineDataAreDeterministicAndPresentationFree(t *testing.T) {
	root := newUnitOverviewRepository(t)
	writeValidUnitOverviewPair(t, root.Path())
	response := runUnitOverview(t, root)

	var first bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatJSON, &first); err != nil {
		t.Fatalf("render JSON: %v", err)
	}
	var second bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatJSON, &second); err != nil {
		t.Fatalf("render second JSON: %v", err)
	}
	if first.String() != second.String() {
		t.Fatalf("public JSON is nondeterministic:\n%s\n%s", first.String(), second.String())
	}
	raw, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatalf("marshal machine data: %v", err)
	}
	for _, output := range []string{first.String(), string(raw)} {
		for _, required := range []string{`"status"`, `"summary"`, `"units"`, `"tag_shape"`, `"configured_tag"`, `"issues"`} {
			if !strings.Contains(output, required) {
				t.Fatalf("JSON omitted %s: %s", required, output)
			}
		}
		for _, forbidden := range []string{"human_table", "human_properties", "human_text", "\x1b[", "next_version", "planned_version", "tag_exists"} {
			if strings.Contains(output, forbidden) {
				t.Fatalf("JSON contains presentation or planning field %q: %s", forbidden, output)
			}
		}
	}
}
