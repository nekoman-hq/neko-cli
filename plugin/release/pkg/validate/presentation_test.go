package validate

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestValidationDefaultHumanPresentationIsConciseAndSummaryFirst(t *testing.T) {
	t.Parallel()

	result := validationQueryResult{
		SourceFormat:        config.SourceFormatV2,
		ConfiguredUnitCount: 2,
		Units: []config.ReleaseUnit{
			{ID: "api", Version: "1.2.3"},
			{ID: "web", Version: "2.0.0"},
		},
	}
	response := mapValidationQueryResponse(result, nil, time.Time{})
	want := &presentation.Properties{
		Title: validationPresentationTitle,
		Properties: []presentation.Property{
			{Label: "Status", Value: "✓ Valid", Role: presentation.StyleSuccess, Emphasized: true},
			{Label: "Source", Value: "V2 config and state"},
			{Label: "Schema", Value: "v2"},
			{Label: "Configuration", Value: ".neko/release.config.json"},
			{Label: "State", Value: ".neko/release.state.json"},
			{Label: "Configured units", Value: 2},
		},
	}
	if !reflect.DeepEqual(response.PresentationProperties, want) {
		t.Fatalf("summary = %#v, want %#v", response.PresentationProperties, want)
	}
	if response.PresentationTable != nil {
		t.Fatalf("default validate unexpectedly displays unit table: %#v", response.PresentationTable)
	}

	output := renderValidationResponse(t, response)
	if !strings.HasPrefix(output, validationPresentationTitle+"\n") || strings.Contains(output, "Unit api") || strings.Contains(output, "Unit web") {
		t.Fatalf("default human output is not concise and summary-first:\n%s", output)
	}
}

func TestValidationV2ShowDeclaresResponsiveUnitSummaryAndCompleteDetails(t *testing.T) {
	t.Parallel()

	result := validationQueryResult{
		SourceFormat:        config.SourceFormatV2,
		Show:                true,
		SelectedUnit:        "plugin-release",
		ConfiguredUnitCount: 2,
		Units: []config.ReleaseUnit{
			{
				ID:               "api",
				Version:          "1.2.3",
				WorkingDirectory: "services/api",
				TagPrefix:        "api/v",
				ExecutorType:     "goreleaser",
				Delivery:         "github-actions",
				Workflow:         ".github/workflows/release-api.yml",
				Paths:            []string{"services/api/**", "shared/**"},
			},
			{
				ID:                 "plugin-release",
				Version:            "4.0.2",
				WorkingDirectory:   ".",
				TagPrefix:          "plugin-release/v",
				Kind:               "plugin",
				ExecutorType:       "goreleaser",
				Delivery:           "github-actions",
				Workflow:           ".github/workflows/release-plugin-release.yml",
				Paths:              []string{"plugin/release/**"},
				IsPlugin:           true,
				PluginName:         "release",
				PluginManifestPath: "plugin/release/manifest.json",
				PluginAssetPrefix:  "plugin-release",
				PluginBinaryName:   "plugin-release",
			},
		},
	}
	response := mapValidationQueryResponse(result, nil, time.Time{})

	wantColumns := []presentation.Column{
		{Key: "unit", Label: "Unit", Essential: true},
		{Key: "version", Label: "Version", Essential: true},
		{Key: "kind", Label: "Kind", Essential: true},
		{Key: "executor", Label: "Executor"},
		{Key: "delivery", Label: "Delivery"},
		{Key: "workflow", Label: "Workflow"},
	}
	if response.PresentationTable == nil || !reflect.DeepEqual(response.PresentationTable.Columns, wantColumns) {
		t.Fatalf("unit columns = %#v, want %#v", response.PresentationTable, wantColumns)
	}
	wantRows := []map[string]any{
		{"unit": "api", "version": "1.2.3", "kind": "release", "executor": "goreleaser", "delivery": "github-actions", "workflow": ".github/workflows/release-api.yml"},
		{"unit": "plugin-release", "version": "4.0.2", "kind": "plugin", "executor": "goreleaser", "delivery": "github-actions", "workflow": ".github/workflows/release-plugin-release.yml"},
	}
	if !reflect.DeepEqual(response.PresentationTable.Rows, wantRows) {
		t.Fatalf("unit rows = %#v, want %#v", response.PresentationTable.Rows, wantRows)
	}

	wantDetails := []presentation.Property{
		{Label: "Unit api", Heading: true, Emphasized: true},
		{Label: "Version", Value: "1.2.3"},
		{Label: "Kind", Value: "release"},
		{Label: "Working directory", Value: "services/api"},
		{Label: "Tag prefix", Value: "api/v"},
		{Label: "Executor", Value: "goreleaser"},
		{Label: "Delivery", Value: "github-actions"},
		{Label: "Workflow", Value: ".github/workflows/release-api.yml"},
		{Label: "Paths", Value: "services/api/**\nshared/**"},
		{Label: "Unit plugin-release", Heading: true, Emphasized: true},
		{Label: "Version", Value: "4.0.2"},
		{Label: "Kind", Value: "plugin"},
		{Label: "Working directory", Value: "."},
		{Label: "Tag prefix", Value: "plugin-release/v"},
		{Label: "Executor", Value: "goreleaser"},
		{Label: "Delivery", Value: "github-actions"},
		{Label: "Workflow", Value: ".github/workflows/release-plugin-release.yml"},
		{Label: "Paths", Value: "plugin/release/**"},
		{Label: "Plugin name", Value: "release"},
		{Label: "Plugin manifest", Value: "plugin/release/manifest.json"},
		{Label: "Plugin asset prefix", Value: "plugin-release"},
		{Label: "Plugin binary", Value: "plugin-release"},
	}
	if response.PresentationTable.Details == nil || !reflect.DeepEqual(response.PresentationTable.Details.Properties, wantDetails) {
		t.Fatalf("unit details = %#v, want %#v", response.PresentationTable.Details, wantDetails)
	}

	properties := response.PresentationProperties.Properties
	if got := properties[len(properties)-2:]; !reflect.DeepEqual(got, []presentation.Property{
		{Label: "Selected unit", Value: "plugin-release"},
		{Label: "Configured units", Value: 2},
	}) {
		t.Fatalf("focused summary tail = %#v", got)
	}
	output := renderValidationResponse(t, response)
	for _, forbidden := range []string{"version=1.2.3", "paths=[", "pluginManifest="} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("human output retained serialized unit field %q:\n%s", forbidden, output)
		}
	}
	apiPath := strings.Index(output, "services/api/**")
	sharedPath := strings.Index(output, "shared/**")
	if apiPath < 0 || sharedPath <= apiPath || !strings.Contains(output[apiPath:sharedPath], "\n") {
		t.Fatalf("paths are not rendered semantically on separate lines:\n%s", output)
	}
}

func TestValidationV1ShowUsesOneVirtualUnitWithoutV2Fields(t *testing.T) {
	t.Parallel()

	result := validationQueryResult{
		SourceFormat: config.SourceFormatV1,
		Show:         true,
		SelectedUnit: "default",
		Legacy: legacyValidationDetails{
			ProjectName:   "neko-cli",
			ProjectOwner:  "nekoman-hq",
			ProjectType:   "backend",
			ReleaseSystem: "goreleaser",
			Version:       "1.2.3",
			UnitID:        "default",
		},
	}
	response := mapValidationQueryResponse(result, nil, time.Time{})

	wantColumns := []presentation.Column{
		{Key: "unit", Label: "Unit", Essential: true},
		{Key: "version", Label: "Version", Essential: true},
		{Key: "project_type", Label: "Project type", Essential: true},
		{Key: "release_system", Label: "Release system"},
	}
	if response.PresentationTable == nil || !reflect.DeepEqual(response.PresentationTable.Columns, wantColumns) {
		t.Fatalf("legacy columns = %#v, want %#v", response.PresentationTable, wantColumns)
	}
	if _, present := response.PresentationTable.Rows[0]["state"]; present {
		t.Fatalf("legacy row unexpectedly contains V2 state: %#v", response.PresentationTable.Rows[0])
	}
	if strings.Contains(renderValidationResponse(t, response), ".neko/release.state.json") {
		t.Fatal("legacy presentation unexpectedly contains V2 state path")
	}
}

func renderValidationResponse(t *testing.T, response *plugin.Response) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatTable, &output); err != nil {
		t.Fatalf("render response: %v", err)
	}
	return output.String()
}
