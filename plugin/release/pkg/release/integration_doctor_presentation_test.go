package release

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestIntegrationDoctorHumanPresentationUsesSummaryIndexAndCompleteDetails(t *testing.T) {
	result := integrationDoctorHighVolumePresentationFixture()
	response := mapIntegrationDoctorResultForTest(result)

	wantSummary := []plugin.HumanProperty{
		{Label: "Readiness", Value: string(integrationDoctorNotReady)},
		{Label: "Errors", Value: 3},
		{Label: "Warnings", Value: 2},
		{Label: "Recommendations", Value: 0},
		{Label: "Not verifiable", Value: 4},
		{Label: "Inspected units", Value: 2},
		{Label: "Inspected workflows", Value: 2},
	}
	if response.HumanProperties == nil || !reflect.DeepEqual(response.HumanProperties.Properties, wantSummary) {
		t.Fatalf("summary properties = %#v, want %#v", response.HumanProperties, wantSummary)
	}
	wantColumns := []plugin.HumanColumn{
		{Key: "severity", Label: "Severity", Essential: true},
		{Key: "code", Label: "Code", Essential: true},
		{Key: "target", Label: "Target"},
		{Key: "scope", Label: "Scope"},
	}
	if response.HumanTable == nil || !reflect.DeepEqual(response.HumanTable.Columns, wantColumns) {
		t.Fatalf("diagnostic columns = %#v, want %#v", response.HumanTable, wantColumns)
	}

	output := ansi.Strip(renderReleasePlanForTest(
		t,
		response,
		renderer.FormatTable,
		releasePlanOutputWidth{width: 140, available: true},
	))
	readinessAt := strings.Index(output, "Readiness")
	indexAt := strings.Index(output, "Severity")
	detailsAt := strings.Index(output, "Diagnostic")
	if readinessAt < 0 || indexAt <= readinessAt || detailsAt <= indexAt {
		t.Fatalf("human sections are not summary -> index -> details:\n%s", output)
	}
	index := output[indexAt:detailsAt]
	if strings.Contains(index, "Message") || strings.Contains(index, "Remediation") {
		t.Fatalf("compact index contains long diagnostic fields:\n%s", index)
	}
	if got := strings.Count(output[detailsAt:], "Diagnostic"); got != len(result.Diagnostics) {
		t.Fatalf("detail blocks = %d, want %d:\n%s", got, len(result.Diagnostics), output)
	}

	detailOutput := strings.Join(strings.Fields(output[detailsAt:]), " ")
	previousCodeAt := -1
	for _, diagnostic := range result.Diagnostics {
		codeAt := strings.Index(detailOutput, diagnostic.Code)
		if codeAt <= previousCodeAt {
			t.Fatalf("detail order changed at %s:\n%s", diagnostic.Code, detailOutput)
		}
		previousCodeAt = codeAt
		assertDoctorTextPreserved(t, detailOutput, diagnostic.Message, "message", diagnostic.Code)
		assertDoctorTextPreserved(t, detailOutput, diagnostic.Remediation, "remediation", diagnostic.Code)
		if diagnostic.Workflow != "" {
			assertDoctorTokenPreserved(t, detailOutput, diagnostic.Workflow, "workflow", diagnostic.Code)
		}
	}
}

func TestIntegrationDoctorHumanPresentationFitsKnownWidthsAndUnknownWriter(t *testing.T) {
	result := integrationDoctorHighVolumePresentationFixture()
	response := mapIntegrationDoctorResultForTest(result)
	for _, width := range []int{140, 120, 100, 80, 60, 40} {
		t.Run(fmt.Sprintf("width_%d", width), func(t *testing.T) {
			output := renderReleasePlanForTest(
				t,
				response,
				renderer.FormatTable,
				releasePlanOutputWidth{width: width, available: true},
			)
			assertReleasePlanLinesFit(t, output, width)
			plain := ansi.Strip(output)
			if !strings.Contains(plain, "Readiness") || !strings.Contains(plain, "Diagnostic") {
				t.Fatalf("width %d lost presentation sections:\n%s", width, plain)
			}
			for _, diagnostic := range result.Diagnostics {
				assertDoctorTokenPreserved(t, plain, diagnostic.Code, "code", diagnostic.Code)
				assertDoctorTextPreserved(t, plain, diagnostic.Message, "message", diagnostic.Code)
				assertDoctorTextPreserved(t, plain, diagnostic.Remediation, "remediation", diagnostic.Code)
			}
		})
	}

	first := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{})
	second := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{})
	if first != second || strings.Contains(ansi.Strip(first), "────") {
		t.Fatalf("unknown-width output is not deterministic vertical output:\n%s", first)
	}
	var nonTerminal bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatTable, &nonTerminal); err != nil {
		t.Fatalf("render non-TTY output: %v", err)
	}
	if nonTerminal.String() != first {
		t.Fatalf("non-TTY output differs from unknown-width fallback:\nunknown=%q\nnon-TTY=%q", first, nonTerminal.String())
	}
}

func TestIntegrationDoctorDiagnosticDetailsPreserveExactFieldOrder(t *testing.T) {
	result := integrationDoctorHighVolumePresentationFixture()
	response := mapIntegrationDoctorResultForTest(result)
	if response.HumanTable == nil || response.HumanTable.Details == nil {
		t.Fatal("Doctor response omitted diagnostic details")
	}

	properties := response.HumanTable.Details.Properties
	offset := 0
	for index, diagnostic := range result.Diagnostics {
		want := []plugin.HumanProperty{
			{Label: "Diagnostic", Value: fmt.Sprintf("%d of %d", index+1, len(result.Diagnostics))},
			{Label: "Severity", Value: strings.ToUpper(string(diagnostic.Severity))},
			{Label: "Code", Value: diagnostic.Code},
			{Label: "Scope", Value: diagnostic.Scope},
		}
		if diagnostic.Unit != "" {
			want = append(want, plugin.HumanProperty{Label: "Unit", Value: diagnostic.Unit})
		}
		if diagnostic.Workflow != "" {
			want = append(want, plugin.HumanProperty{Label: "Workflow", Value: diagnostic.Workflow})
		}
		want = append(want,
			plugin.HumanProperty{Label: "Message", Value: diagnostic.Message},
			plugin.HumanProperty{Label: "Remediation", Value: diagnostic.Remediation},
		)
		if offset+len(want) > len(properties) || !reflect.DeepEqual(properties[offset:offset+len(want)], want) {
			t.Fatalf("detail %d fields = %#v, want %#v", index+1, properties[offset:], want)
		}
		offset += len(want)
	}
	if offset != len(properties) {
		t.Fatalf("detail properties consumed %d of %d", offset, len(properties))
	}
}

func TestIntegrationDoctorCompactTargetsShortenOnlyUnambiguousWorkflowBasenames(t *testing.T) {
	diagnostics := []integrationDoctorDiagnostic{
		{Severity: integrationDoctorError, Scope: "workflow", Unit: "api", Workflow: ".github/workflows/team-a/release.yml", Code: "A"},
		{Severity: integrationDoctorWarning, Scope: "workflow", Unit: "web", Workflow: ".github/workflows/team-b/release.yml", Code: "B"},
		{Severity: integrationDoctorNotVerifiable, Scope: "workflow", Unit: "docs", Workflow: ".github/workflows/docs-release.yml", Code: "C"},
		{Severity: integrationDoctorError, Scope: "source", Code: "D"},
	}
	rows := integrationDoctorDiagnosticRows(diagnostics)
	wantTargets := []string{
		"api · .github/workflows/team-a/release.yml",
		"web · .github/workflows/team-b/release.yml",
		"docs · docs-release.yml",
		"source",
	}
	for index, want := range wantTargets {
		if got := rows[index]["target"]; got != want {
			t.Fatalf("target %d = %#v, want %q", index, got, want)
		}
	}
}

func TestIntegrationDoctorSummaryHandlesNoFindingsWithoutSyntheticTableRows(t *testing.T) {
	result := integrationDoctorResult{}
	finalizeIntegrationDoctorResult(&result)
	response := mapIntegrationDoctorResultForTest(result)
	if result.Readiness != integrationDoctorReady || response.ExitCode != 0 || response.HumanTable != nil {
		t.Fatalf("no-finding response = %#v, result = %#v", response, result)
	}
	output := ansi.Strip(renderReleasePlanForTest(
		t,
		response,
		renderer.FormatTable,
		releasePlanOutputWidth{width: 80, available: true},
	))
	if !strings.Contains(output, "Readiness") || !strings.Contains(output, "ready") || strings.Contains(output, "No resources found") {
		t.Fatalf("no-finding summary changed:\n%s", output)
	}
}

func TestIntegrationDoctorJSONRendererExcludesHumanPresentation(t *testing.T) {
	root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	writeIntegrationDoctorWorkflow(t, root, ".github/workflows/release.yml", customIntegrationDoctorWorkflow(t))
	response := runIntegrationDoctor(t, root, nil)

	var output bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatJSON, &output); err != nil {
		t.Fatalf("render JSON: %v", err)
	}
	jsonOutput := output.String()
	for _, metadata := range []string{
		`"human_properties"`, `"human_table"`, `"rows"`, `"details"`, `"target":`, "Diagnostic", "Inspected units",
	} {
		if strings.Contains(jsonOutput, metadata) {
			t.Fatalf("Doctor JSON leaked presentation metadata %q:\n%s", metadata, jsonOutput)
		}
	}
	if !strings.Contains(jsonOutput, `"readiness": "ready"`) || !strings.Contains(jsonOutput, `"diagnostics"`) {
		t.Fatalf("Doctor JSON data changed:\n%s", jsonOutput)
	}
}

func assertDoctorTextPreserved(t *testing.T, output, value, field, code string) {
	t.Helper()
	normalizedOutput := doctorTextWithoutWhitespace(output)
	normalizedValue := doctorTextWithoutWhitespace(value)
	if !strings.Contains(normalizedOutput, normalizedValue) {
		t.Fatalf("%s for %s was not preserved:\nwant %q\noutput %s", field, code, value, output)
	}
}

func assertDoctorTokenPreserved(t *testing.T, output, value, field, code string) {
	t.Helper()
	if !strings.Contains(doctorTextWithoutWhitespace(output), value) {
		t.Fatalf("%s for %s was not preserved:\nwant %q\noutput %s", field, code, value, output)
	}
}

func doctorTextWithoutWhitespace(value string) string {
	return strings.Map(func(value rune) rune {
		if value == ' ' || value == '\n' || value == '\r' || value == '\t' {
			return -1
		}
		return value
	}, value)
}
