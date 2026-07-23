package unitoverview

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestUnitOverviewDefaultRetainsUsefulInventoryAndActionableIssues(t *testing.T) {
	result := unitOverviewDescribeContractFixture()
	response := mapUnitOverviewResult(result, time.Time{})

	if response.PresentationTable == nil || response.PresentationTable.Title != "Release Units" {
		t.Fatalf("default unit table = %#v", response.PresentationTable)
	}
	if got := len(response.PresentationTable.Rows); got != 2 {
		t.Fatalf("default unit rows = %d, want 2", got)
	}
	issues := response.PresentationTable.Following
	if issues == nil || issues.Title != "Issues" || issues.DescribeOnly || len(issues.Rows) != 1 {
		t.Fatalf("actionable issue table = %#v", issues)
	}

	output := ansi.Strip(renderUnitOverviewContract(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, WidthProvider: releasePlanOutputWidth{width: 180, available: true},
	}))
	for _, expected := range []string{
		"Release Units", "api", "1.2.3", "ready", "worker", "has issues",
		"Issues", "State Missing", "Add the unit's current canonical version",
	} {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(expected)) {
			t.Fatalf("unit default omitted %q:\n%s", expected, output)
		}
	}
	for _, hidden := range []string{"Unit Details", "Source ownership", "Configured tag", "Limitations"} {
		if strings.Contains(output, hidden) {
			t.Fatalf("unit default exposed describe-only value %q:\n%s", hidden, output)
		}
	}
}

func TestUnitOverviewDescribeExposesCompleteOwnedFacts(t *testing.T) {
	response := mapUnitOverviewResult(unitOverviewDescribeContractFixture(), time.Time{})
	output := ansi.Strip(renderUnitOverviewContract(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true,
		WidthProvider: releasePlanOutputWidth{width: 180, available: true},
	}))
	for _, expected := range []string{
		"Unit Details", "Source ownership", "Config and state", "Config only",
		"api/v<version>", "api/v1.2.3", ".github/workflows/release-api.yml",
		"Plugin name", "release", "Plugin manifest", "plugin/release/manifest.json",
		"Limitations", "Workflow contents are not inspected",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("unit describe omitted %q:\n%s", expected, output)
		}
	}
}

func TestUnitOverviewPresentationPreservesMachineRowsOrderingAndExit(t *testing.T) {
	result := unitOverviewDescribeContractFixture()
	response := mapUnitOverviewResult(result, time.Time{})

	wantUnits := []map[string]any{
		unitOverviewMachineRow(result.Units[0]),
		unitOverviewMachineRow(result.Units[1]),
	}
	if got := response.Data["units"]; !reflect.DeepEqual(got, wantUnits) {
		t.Fatalf("unit JSON rows changed: %#v", got)
	}
	if response.ExitCode != 1 {
		t.Fatalf("unit issue exit = %d, want 1", response.ExitCode)
	}
	detailTable := unitOverviewTableByTitle(response.PresentationTable, "Unit Details")
	if detailTable == nil || !detailTable.DescribeOnly {
		t.Fatalf("unit details are not describe-only: %#v", detailTable)
	}
}

func unitOverviewDescribeContractFixture() *unitOverviewResult {
	return &unitOverviewResult{
		Status: unitOverviewHasIssues,
		Summary: unitOverviewSummary{
			Total: 2, Aligned: 1, Incomplete: 1, WorkflowPaths: 2, SourceUsable: true,
		},
		SourceUsable: true,
		Units: []unitOverviewRow{
			{
				ID: "api", DisplayName: "API", Version: "1.2.3", ConfiguredVersion: "1.2.3",
				TagPrefix: "api/v", TagShape: "api/v<version>", ConfiguredTag: "api/v1.2.3",
				Executor: "goreleaser", Delivery: "github-actions",
				WorkflowPath: ".github/workflows/release-api.yml", WorkingDirectory: "services/api",
				Alignment: unitOverviewAligned, Issues: []unitOverviewIssue{},
				ConfigPresent: true, StatePresent: true,
				Kind: "plugin", PluginName: "release", PluginManifest: "plugin/release/manifest.json",
				PluginAssetPrefix: "plugin-release", PluginBinaryName: "plugin-release",
			},
			{
				ID: "worker", DisplayName: "Worker", TagPrefix: "worker/v",
				Executor: "goreleaser", Delivery: "github-actions",
				WorkflowPath: ".github/workflows/release-worker.yml", WorkingDirectory: ".",
				Alignment: unitOverviewConfigOnly,
				Issues: []unitOverviewIssue{{
					Severity: unitOverviewIssueWarning, Unit: "worker", Code: "UNIT_STATE_MISSING",
					Message:     "The unit has config but no state entry.",
					Remediation: "Add the unit's current canonical version to .neko/release.state.json.",
				}},
				ConfigPresent: true,
			},
		},
		WorkflowPaths: []string{".github/workflows/release-api.yml", ".github/workflows/release-worker.yml"},
	}
}

func renderUnitOverviewContract(
	t *testing.T,
	response *plugin.Response,
	options renderer.RenderOptions,
) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, options, &output); err != nil {
		t.Fatalf("render Unit Overview: %v", err)
	}
	return output.String()
}

func unitOverviewTableByTitle(table *presentation.Table, title string) *presentation.Table {
	for table != nil {
		if table.Title == title {
			return table
		}
		table = table.Following
	}
	return nil
}
