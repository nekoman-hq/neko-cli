package validate

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/internal/terminal"
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
	if response.PresentationProperties != nil {
		t.Fatalf("default validate uses headerless properties instead of a table: %#v", response.PresentationProperties)
	}
	wantColumns := []presentation.Column{
		{Key: "property", Label: "PROPERTY", Essential: true},
		{Key: "value", Label: "VALUE", RoleKey: "value_role", Essential: true},
	}
	wantRows := []map[string]any{
		{"property": "Status", "value": "✓ Valid", "value_role": "success"},
		{"property": "Source", "value": "V2 config and state", "value_role": "default"},
		{"property": "Schema", "value": "v2", "value_role": "default"},
		{"property": "Configuration", "value": ".neko/release.config.json", "value_role": "default"},
		{"property": "State", "value": ".neko/release.state.json", "value_role": "default"},
		{"property": "Configured units", "value": 2, "value_role": "default"},
	}
	if response.PresentationTable == nil || response.PresentationTable.Title != validationPresentationTitle ||
		!reflect.DeepEqual(response.PresentationTable.Columns, wantColumns) ||
		!reflect.DeepEqual(response.PresentationTable.Rows, wantRows) || response.PresentationTable.Details != nil {
		t.Fatalf("default summary table = %#v, want columns=%#v rows=%#v", response.PresentationTable, wantColumns, wantRows)
	}

	output := renderValidationResponseAtWidth(t, response, 80)
	if !strings.HasPrefix(output, validationPresentationTitle+"\n") || !strings.Contains(output, "PROPERTY") ||
		!strings.Contains(output, "VALUE") || strings.Contains(output, "Unit api") || strings.Contains(output, "Unit web") {
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
				DisplayName:      "Public API",
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
		{Key: "unit", Label: "Unit", RoleKey: validationUnitRoleKey, Essential: true},
		{Key: "version", Label: "Version", RoleKey: validationVersionRoleKey, Essential: true},
		{Key: "kind", Label: "Kind", RoleKey: validationKindRoleKey, Essential: true},
		{Key: "executor", Label: "Executor"},
		{Key: "delivery", Label: "Delivery"},
		{Key: "workflow", Label: "Workflow"},
	}
	if response.PresentationTable == nil || !reflect.DeepEqual(response.PresentationTable.Columns, wantColumns) {
		t.Fatalf("unit columns = %#v, want %#v", response.PresentationTable, wantColumns)
	}
	wantRows := []map[string]any{
		{"unit": "api", "version": "1.2.3", "kind": "release", "executor": "goreleaser", "delivery": "github-actions", "workflow": ".github/workflows/release-api.yml", validationUnitRoleKey: "emphasis", validationVersionRoleKey: "info", validationKindRoleKey: "default"},
		{"unit": "plugin-release", "version": "4.0.2", "kind": "plugin", "executor": "goreleaser", "delivery": "github-actions", "workflow": ".github/workflows/release-plugin-release.yml", validationUnitRoleKey: "emphasis", validationVersionRoleKey: "info", validationKindRoleKey: "info"},
	}
	if !reflect.DeepEqual(response.PresentationTable.Rows, wantRows) {
		t.Fatalf("unit rows = %#v, want %#v", response.PresentationTable.Rows, wantRows)
	}

	wantDetails := []presentation.Property{
		{Label: "Unit api", Role: presentation.StyleInfo, Heading: true, Emphasized: true},
		{Label: "Display name", Value: "Public API"},
		{Label: "Version", Value: "1.2.3", Role: presentation.StyleInfo},
		{Label: "Kind", Value: "release", Role: presentation.StyleDefault},
		{Label: "Working directory", Value: "services/api"},
		{Label: "Tag prefix", Value: "api/v"},
		{Label: "Executor", Value: "goreleaser"},
		{Label: "Delivery", Value: "github-actions"},
		{Label: "Workflow", Value: ".github/workflows/release-api.yml"},
		{Label: "Paths", Value: "services/api/**\nshared/**"},
		{Label: "Unit plugin-release", Role: presentation.StyleInfo, Heading: true, Emphasized: true},
		{Label: "Version", Value: "4.0.2", Role: presentation.StyleInfo},
		{Label: "Kind", Value: "plugin", Role: presentation.StyleInfo},
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
		{Key: "unit", Label: "Unit", RoleKey: validationUnitRoleKey, Essential: true},
		{Key: "version", Label: "Version", RoleKey: validationVersionRoleKey, Essential: true},
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

func TestValidationV2ShowUsesResponsiveLayoutsAndPreservesLongDetails(t *testing.T) {
	t.Parallel()

	paths := []string{
		"services/plugin-release/**",
		"shared/release/**",
		"docs/release/**",
		"docs/plugins/release.md",
		"tooling/release/**",
		"examples/release/**",
	}
	response := mapValidationQueryResponse(validationQueryResult{
		SourceFormat:        config.SourceFormatV2,
		Show:                true,
		ConfiguredUnitCount: 1,
		Units: []config.ReleaseUnit{{
			ID:                 "plugin-release",
			Version:            "4.2.0",
			Kind:               "plugin",
			WorkingDirectory:   "services/release/plugin/with/a/long/working-directory",
			TagPrefix:          "plugin-release/v",
			ExecutorType:       "goreleaser",
			Delivery:           "github-actions",
			Workflow:           ".github/workflows/release-plugin-release-with-a-long-consumer-name.yml",
			Paths:              paths,
			IsPlugin:           true,
			PluginName:         "release",
			PluginManifestPath: "plugin/release/manifest.json",
			PluginAssetPrefix:  "plugin-release",
			PluginBinaryName:   "plugin-release",
		}}}, nil, time.Time{})

	normal := renderValidationResponseAtWidth(t, response, 96)
	if strings.Count(normal, validationPresentationTitle) != 1 {
		t.Fatalf("normal output title count changed:\n%s", normal)
	}
	summaryAt := strings.Index(normal, "Status")
	rowAt := strings.Index(normal, "plugin-release")
	detailAt := strings.Index(normal, "Unit plugin-release")
	if summaryAt < 0 || rowAt <= summaryAt || detailAt <= rowAt {
		t.Fatalf("normal output hierarchy changed:\n%s", normal)
	}
	for _, value := range append(paths,
		"services/release/plugin/with/a/long/working-directory",
		".github/workflows/release-plugin-release-with-a-long-consumer-name.yml",
	) {
		if !strings.Contains(normal, value) {
			t.Fatalf("normal output omitted complete detail %q:\n%s", value, normal)
		}
	}

	narrow := renderValidationResponseAtWidth(t, response, 24)
	for _, want := range []string{"Unit: plugin-release", "Version: 4.2.0", "Kind: plugin"} {
		if !strings.Contains(narrow, want) {
			t.Fatalf("narrow output omitted essential field %q:\n%s", want, narrow)
		}
	}
	if strings.Contains(narrow, "────────────────") {
		t.Fatalf("narrow single-unit output emitted an oversized separator:\n%s", narrow)
	}

	firstUnknown := renderValidationResponse(t, response)
	secondUnknown := renderValidationResponse(t, response)
	if firstUnknown != secondUnknown || !strings.Contains(firstUnknown, "Unit: plugin-release") {
		t.Fatalf("unknown-width output is not deterministic vertical records:\nfirst=%q\nsecond=%q", firstUnknown, secondUnknown)
	}
}

func TestValidationV2ShowUsesCoreSemanticColorWithoutColoringPaths(t *testing.T) {
	t.Parallel()

	workflow := ".github/workflows/release-plugin-release.yml"
	path := "plugin/release/**"
	response := mapValidationQueryResponse(validationQueryResult{
		SourceFormat:        config.SourceFormatV2,
		Show:                true,
		ConfiguredUnitCount: 1,
		Units: []config.ReleaseUnit{{
			ID:                 "plugin-release",
			Version:            "4.2.0",
			Kind:               "plugin",
			WorkingDirectory:   ".",
			TagPrefix:          "plugin-release/v",
			ExecutorType:       "goreleaser",
			Delivery:           "github-actions",
			Workflow:           workflow,
			Paths:              []string{path},
			IsPlugin:           true,
			PluginName:         "release",
			PluginManifestPath: "plugin/release/manifest.json",
			PluginAssetPrefix:  "plugin-release",
			PluginBinaryName:   "plugin-release",
		}}}, nil, time.Time{})

	enabled := renderValidationResponseWithOptions(t, response, 120, validationColor(true))
	for name, want := range map[string]string{
		"valid status": terminal.Green + terminal.Bold + "✓ Valid" + terminal.Reset,
		"unit":         terminal.Bold + "plugin-release" + terminal.Reset,
		"version":      terminal.Cyan + "4.2.0" + terminal.Reset,
		"plugin kind":  terminal.Cyan + "plugin" + terminal.Reset,
		"unit heading": terminal.Cyan + terminal.Bold + "Unit plugin-release" + terminal.Reset,
	} {
		if !strings.Contains(enabled, want) {
			t.Fatalf("semantic output omitted %s style %q:\n%s", name, want, enabled)
		}
	}
	for _, neutral := range []string{workflow, path} {
		if !strings.Contains(enabled, neutral) || strings.Contains(enabled, terminal.Cyan+neutral) {
			t.Fatalf("ordinary path %q was omitted or semantically colored:\n%s", neutral, enabled)
		}
	}

	disabled := renderValidationResponseWithOptions(t, response, 120, validationColor(false))
	if strings.Contains(disabled, "\x1b") || !strings.Contains(disabled, "✓ Valid") || !strings.Contains(disabled, workflow) {
		t.Fatalf("color-disabled output changed visible content or retained ANSI:\n%q", disabled)
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

func renderValidationResponseAtWidth(t *testing.T, response *plugin.Response, width int) string {
	return renderValidationResponseWithOptions(t, response, width, nil)
}

func renderValidationResponseWithOptions(
	t *testing.T,
	response *plugin.Response,
	width int,
	color renderer.ColorProvider,
) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
		Format:        renderer.FormatTable,
		WidthProvider: validationOutputWidth(width),
		ColorProvider: color,
	}, &output); err != nil {
		t.Fatalf("render response at width %d: %v", width, err)
	}
	return output.String()
}

type validationOutputWidth int

func (width validationOutputWidth) Width(io.Writer) (int, bool) {
	return int(width), true
}

type validationColor bool

func (enabled validationColor) ColorEnabled(io.Writer) bool {
	return bool(enabled)
}
