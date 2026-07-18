package release

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestIntegrationDoctorHumanOutputIsResponsiveAtNarrowAndUnknownWidths(t *testing.T) {
	root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	writeIntegrationDoctorWorkflow(t, root, ".github/workflows/release.yml", canonicalIntegrationDoctorWorkflow(t))
	response := runIntegrationDoctor(t, root, nil)

	narrow := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{width: 28, available: true})
	plain := ansi.Strip(narrow)
	for _, want := range []string{"Readiness", "not_ready", "Remediation"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("narrow doctor output omitted %q:\n%s", want, plain)
		}
	}
	if !strings.Contains(strings.Join(strings.Fields(plain), ""), "CONSUMER_PLACEHOLDER_PRESENT") {
		t.Fatalf("narrow doctor output lost wrapped diagnostic code:\n%s", plain)
	}
	assertReleasePlanLinesFit(t, narrow, 28)

	first := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{})
	second := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{})
	if first != second || strings.Contains(ansi.Strip(first), "PROPERTY") {
		t.Fatalf("unknown-width output is not deterministic vertical properties:\n%s", first)
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
	if strings.Contains(jsonOutput, "human_properties") ||
		!strings.Contains(jsonOutput, `"readiness": "ready"`) || !strings.Contains(jsonOutput, `"diagnostics"`) {
		t.Fatalf("doctor JSON isolation changed:\n%s", jsonOutput)
	}
}
