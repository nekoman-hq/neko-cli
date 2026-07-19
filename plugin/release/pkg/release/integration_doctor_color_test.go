package release

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestIntegrationDoctorReadinessUsesSemanticInteractiveStyles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		readiness integrationDoctorReadiness
		severity  integrationDoctorSeverity
		ansiColor string
	}{
		{readiness: integrationDoctorNotReady, severity: integrationDoctorError, ansiColor: "\x1b[31m"},
		{readiness: integrationDoctorReadyWithWarnings, severity: integrationDoctorWarning, ansiColor: "\x1b[33m"},
		{readiness: integrationDoctorReady, severity: integrationDoctorRecommendation, ansiColor: "\x1b[32m"},
	}
	for _, test := range tests {
		t.Run(string(test.readiness), func(t *testing.T) {
			result := integrationDoctorResult{Diagnostics: []integrationDoctorDiagnostic{{
				Severity: test.severity, Scope: "source", Code: "PRESENTATION_TEST", Message: "message", Remediation: "remediation",
			}}}
			finalizeIntegrationDoctorResult(&result)
			if result.Readiness != test.readiness {
				t.Fatalf("fixture readiness = %q, want %q", result.Readiness, test.readiness)
			}

			output := renderIntegrationDoctorWithColorForTest(t, mapIntegrationDoctorResultForTest(result), 80, true)
			sequence := test.ansiColor + "\x1b[1m" + string(test.readiness) + "\x1b[0m"
			if !strings.Contains(output, sequence) {
				t.Fatalf("readiness %q omitted semantic emphasis %q:\n%q", test.readiness, sequence, output)
			}
		})
	}
}

func TestIntegrationDoctorSeveritiesAndCountsUseSemanticRoles(t *testing.T) {
	t.Parallel()

	result := integrationDoctorResult{Diagnostics: []integrationDoctorDiagnostic{
		{Severity: integrationDoctorError, Scope: "source", Code: "ERROR_CODE", Message: "error message", Remediation: "error remediation"},
		{Severity: integrationDoctorWarning, Scope: "source", Code: "WARNING_CODE", Message: "warning message", Remediation: "warning remediation"},
		{Severity: integrationDoctorRecommendation, Scope: "source", Code: "RECOMMENDATION_CODE", Message: "recommendation message", Remediation: "recommendation remediation"},
		{Severity: integrationDoctorNotVerifiable, Scope: "remote", Code: "NOT_VERIFIABLE_CODE", Message: "not verifiable message", Remediation: "not verifiable remediation"},
	}}
	finalizeIntegrationDoctorResult(&result)
	output := renderIntegrationDoctorWithColorForTest(t, mapIntegrationDoctorResultForTest(result), 120, true)

	for _, expected := range []struct {
		color    string
		severity string
		code     string
	}{
		{color: "\x1b[31m", severity: "ERROR", code: "ERROR_CODE"},
		{color: "\x1b[33m", severity: "WARNING", code: "WARNING_CODE"},
		{color: "\x1b[36m", severity: "RECOMMENDATION", code: "RECOMMENDATION_CODE"},
		{color: "\x1b[90m", severity: "NOT_VERIFIABLE", code: "NOT_VERIFIABLE_CODE"},
	} {
		for _, value := range []string{expected.severity, expected.code} {
			if !strings.Contains(output, expected.color+value+"\x1b[0m") {
				t.Errorf("output omitted semantic style for %s:\n%q", value, output)
			}
		}
	}
	for _, expected := range []string{
		"\x1b[31m1\x1b[0m",
		"\x1b[33m1\x1b[0m",
		"\x1b[36m1\x1b[0m",
		"\x1b[90m1\x1b[0m",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("output omitted styled positive count %q:\n%q", expected, output)
		}
	}
	if strings.Contains(output, "\x1b[31m0\x1b[0m") || strings.Contains(output, "\x1b[33m0\x1b[0m") ||
		strings.Contains(output, "\x1b[36m0\x1b[0m") || strings.Contains(output, "\x1b[90m0\x1b[0m") {
		t.Fatalf("zero count received an active semantic severity color:\n%q", output)
	}

	emptyResult := integrationDoctorResult{}
	finalizeIntegrationDoctorResult(&emptyResult)
	emptyOutput := renderIntegrationDoctorWithColorForTest(t, mapIntegrationDoctorResultForTest(emptyResult), 80, true)
	for _, styledZero := range []string{"\x1b[31m0", "\x1b[33m0", "\x1b[36m0", "\x1b[90m0"} {
		if strings.Contains(emptyOutput, styledZero) {
			t.Fatalf("zero Doctor count uses active semantic color %q:\n%q", styledZero, emptyOutput)
		}
	}
}

func TestIntegrationDoctorStylesHeadingsWithoutColoringOrdinaryFields(t *testing.T) {
	t.Parallel()

	result := integrationDoctorResult{Diagnostics: []integrationDoctorDiagnostic{{
		Severity:    integrationDoctorError,
		Scope:       "workflow",
		Unit:        "versioned-unit",
		Workflow:    ".github/workflows/versioned-release.yml",
		Code:        "CHECKOUT_REF_INVALID",
		Message:     "verifiable.",
		Remediation: "v1 remains ordinary.",
	}}}
	finalizeIntegrationDoctorResult(&result)
	output := renderIntegrationDoctorWithColorForTest(t, mapIntegrationDoctorResultForTest(result), 100, true)

	for _, heading := range []string{integrationDoctorHumanTitle, integrationDoctorDiagnosticsTitle} {
		sequence := "\x1b[1m" + heading + "\x1b[0m"
		if strings.Count(output, sequence) != 1 {
			t.Fatalf("heading %q is not rendered once with neutral emphasis:\n%q", heading, output)
		}
	}
	for _, ordinary := range []string{
		"versioned-unit",
		".github/workflows/versioned-release.yml",
		"workflow",
		"verifiable.",
		"v1 remains ordinary.",
	} {
		for _, color := range []string{"\x1b[31m", "\x1b[33m", "\x1b[36m", "\x1b[90m"} {
			if strings.Contains(output, color+ordinary) {
				t.Fatalf("ordinary Doctor field %q received semantic color %q:\n%q", ordinary, color, output)
			}
		}
	}
	if strings.Contains(output, "\x1b[35m") {
		t.Fatalf("ordinary Doctor values received legacy version-prefix color:\n%q", output)
	}
	if !strings.Contains(output, "\x1b[31mERROR\x1b[0m") ||
		!strings.Contains(output, "\x1b[31mCHECKOUT_REF_INVALID\x1b[0m") ||
		!strings.Contains(output, "\x1b[31m\x1b[1mERROR · CHECKOUT_REF_INVALID\x1b[0m") {
		t.Fatalf("Doctor severity tokens and record heading are not independently reset:\n%q", output)
	}
}

func TestIntegrationDoctorColorDisabledOutputPreservesVisibleText(t *testing.T) {
	t.Parallel()

	result := integrationDoctorHighVolumePresentationFixture()
	response := mapIntegrationDoctorResultForTest(result)
	colored := renderIntegrationDoctorWithColorForTest(t, response, 100, true)
	plain := renderIntegrationDoctorWithColorForTest(t, response, 100, false)
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("color-disabled Doctor output contains ANSI: %q", plain)
	}
	if stripped := ansi.Strip(colored); stripped != plain {
		t.Fatalf("color changed Doctor text:\ncolored=%q\nplain=%q", stripped, plain)
	}
}

func TestIntegrationDoctorMachineModesRemainANSIFree(t *testing.T) {
	t.Parallel()

	response := mapIntegrationDoctorResultForTest(integrationDoctorHighVolumePresentationFixture())
	var publicJSON bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
		Format:        renderer.FormatJSON,
		ColorProvider: integrationDoctorTestColorProvider(true),
	}, &publicJSON); err != nil {
		t.Fatalf("render Doctor JSON: %v", err)
	}
	assertIntegrationDoctorNoANSI(t, "JSON", publicJSON.String())

	rawResponse := *response
	rawResponse.RendererHint = "raw-json"
	rawResponse.Data = map[string]any{"raw": `{"readiness":"not_ready"}`}
	var rawJSON bytes.Buffer
	if err := renderer.RenderWithOptionsTo(&rawResponse, renderer.RenderOptions{
		Format:        renderer.FormatTable,
		ColorProvider: integrationDoctorTestColorProvider(true),
	}, &rawJSON); err != nil {
		t.Fatalf("render Doctor raw JSON: %v", err)
	}
	assertIntegrationDoctorNoANSI(t, "raw JSON", rawJSON.String())

	var githubOutput bytes.Buffer
	err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
		Format:        renderer.FormatGitHub,
		ColorProvider: integrationDoctorTestColorProvider(true),
	}, &githubOutput)
	if err == nil {
		t.Fatal("Doctor unexpectedly declared GitHub command-file output")
	}
	assertIntegrationDoctorNoANSI(t, "GitHub output failure", githubOutput.String())
}

func renderIntegrationDoctorWithColorForTest(
	t *testing.T,
	response *plugin.Response,
	width int,
	color bool,
) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
		Format:        renderer.FormatTable,
		WidthProvider: releasePlanOutputWidth{width: width, available: true},
		ColorProvider: integrationDoctorTestColorProvider(color),
	}, &output); err != nil {
		t.Fatalf("render Doctor: %v", err)
	}
	return output.String()
}

type integrationDoctorTestColorProvider bool

func (enabled integrationDoctorTestColorProvider) ColorEnabled(io.Writer) bool {
	return bool(enabled)
}

func assertIntegrationDoctorNoANSI(t *testing.T, name, output string) {
	t.Helper()
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("Doctor %s contains ANSI: %q", name, output)
	}
}
