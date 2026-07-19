package release

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestIntegrationDoctorPresentationUsesSummaryIndexAndCompleteDetails(t *testing.T) {
	result := integrationDoctorHighVolumePresentationFixture()
	response := mapIntegrationDoctorResultForTest(result)

	wantSummary := []presentation.Property{
		{Label: "Readiness", Value: string(integrationDoctorNotReady), Role: presentation.StyleError, Emphasized: true},
		{Label: "Errors", Value: 3, Role: presentation.StyleError},
		{Label: "Warnings", Value: 2, Role: presentation.StyleWarning},
		{Label: "Recommendations", Value: 0},
		{Label: "Not verifiable", Value: 4, Role: presentation.StyleMuted},
		{Label: "Locally verified", Value: 0},
		{Label: "Inspected units", Value: 2},
		{Label: "Inspected workflows", Value: 2},
	}
	if response.PresentationProperties == nil || response.PresentationProperties.Title != integrationDoctorPresentationTitle ||
		!reflect.DeepEqual(response.PresentationProperties.Properties, wantSummary) {
		t.Fatalf("summary properties = %#v, want %#v", response.PresentationProperties, wantSummary)
	}
	wantColumns := []presentation.Column{
		{Key: "severity", Label: "Severity", RoleKey: integrationDoctorSemanticRoleKey, Essential: true},
		{Key: "code", Label: "Code", RoleKey: integrationDoctorSemanticRoleKey, Essential: true},
		{Key: "target", Label: "Target", RoleKey: integrationDoctorDefaultRoleKey},
		{Key: "scope", Label: "Scope", RoleKey: integrationDoctorDefaultRoleKey},
	}
	if response.PresentationTable == nil || response.PresentationTable.Title != integrationDoctorDiagnosticsTitle ||
		!reflect.DeepEqual(response.PresentationTable.Columns, wantColumns) {
		t.Fatalf("diagnostic columns = %#v, want %#v", response.PresentationTable, wantColumns)
	}

	output := ansi.Strip(renderReleasePlanForTest(
		t,
		response,
		renderer.FormatTable,
		releasePlanOutputWidth{width: 140, available: true},
	))
	titleAt := strings.Index(output, integrationDoctorPresentationTitle)
	readinessAt := strings.Index(output, "Readiness")
	diagnosticsAt := strings.Index(output, integrationDoctorDiagnosticsTitle)
	indexAt := strings.Index(output, "Severity")
	firstHeading := strings.ToUpper(string(result.Diagnostics[0].Severity)) + " · " + result.Diagnostics[0].Code
	detailsAt := strings.Index(output, firstHeading)
	if titleAt < 0 || readinessAt <= titleAt || diagnosticsAt <= readinessAt || indexAt <= diagnosticsAt || detailsAt <= indexAt {
		t.Fatalf("human sections are not summary -> index -> details:\n%s", output)
	}
	index := output[indexAt:detailsAt]
	if strings.Contains(index, "Message") || strings.Contains(index, "Remediation") {
		t.Fatalf("compact index contains long diagnostic fields:\n%s", index)
	}

	detailOutput := strings.Join(strings.Fields(output[detailsAt:]), " ")
	previousCodeAt := -1
	for _, diagnostic := range result.Diagnostics {
		heading := strings.ToUpper(string(diagnostic.Severity)) + " · " + diagnostic.Code
		if got := strings.Count(output[detailsAt:], heading); got != 1 {
			t.Fatalf("detail heading %q count = %d, want 1:\n%s", heading, got, output)
		}
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

func TestIntegrationDoctorPresentationFitsKnownWidthsAndUnknownWriter(t *testing.T) {
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
	if response.PresentationTable == nil || response.PresentationTable.Details == nil {
		t.Fatal("Doctor response omitted diagnostic details")
	}

	properties := response.PresentationTable.Details.Properties
	offset := 0
	for _, diagnostic := range result.Diagnostics {
		want := []presentation.Property{
			{
				Label:      strings.ToUpper(string(diagnostic.Severity)) + " · " + diagnostic.Code,
				Role:       integrationDoctorSeverityRole(diagnostic.Severity),
				Emphasized: true,
				Heading:    true,
			},
			{Label: "Scope", Value: diagnostic.Scope, Role: presentation.StyleDefault},
		}
		if diagnostic.Unit != "" {
			want = append(want, presentation.Property{
				Label: "Unit", Value: diagnostic.Unit, Role: presentation.StyleDefault,
			})
		}
		if diagnostic.Workflow != "" {
			want = append(want, presentation.Property{
				Label: "Workflow", Value: diagnostic.Workflow, Role: presentation.StyleDefault,
			})
		}
		want = append(want,
			presentation.Property{Label: "Message", Value: diagnostic.Message, Role: presentation.StyleDefault},
			presentation.Property{Label: "Remediation", Value: diagnostic.Remediation, Role: presentation.StyleDefault},
		)
		if offset+len(want) > len(properties) || !reflect.DeepEqual(properties[offset:offset+len(want)], want) {
			t.Fatalf("detail for %s fields = %#v, want %#v", diagnostic.Code, properties[offset:], want)
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
	wantRoles := []presentation.StyleRole{
		presentation.StyleError,
		presentation.StyleWarning,
		presentation.StyleMuted,
		presentation.StyleError,
	}
	for index, want := range wantTargets {
		if got := rows[index]["target"]; got != want {
			t.Fatalf("target %d = %#v, want %q", index, got, want)
		}
		if got := rows[index][integrationDoctorSemanticRoleKey]; got != string(wantRoles[index]) {
			t.Fatalf("semantic role %d = %#v, want %q", index, got, wantRoles[index])
		}
		if got := rows[index][integrationDoctorDefaultRoleKey]; got != string(presentation.StyleDefault) {
			t.Fatalf("default role %d = %#v, want %q", index, got, presentation.StyleDefault)
		}
	}
}

func TestIntegrationDoctorSummaryHandlesNoFindingsWithoutSyntheticTableRows(t *testing.T) {
	result := integrationDoctorResult{}
	finalizeIntegrationDoctorResult(&result)
	response := mapIntegrationDoctorResultForTest(result)
	if result.Readiness != integrationDoctorReady || response.ExitCode != 0 || response.PresentationTable != nil {
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

func TestIntegrationDoctorJSONRendererExcludesPresentation(t *testing.T) {
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
