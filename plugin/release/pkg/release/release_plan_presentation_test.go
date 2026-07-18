package release

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestReleasePlanHumanOutputPresentsLimitationsSemantically(t *testing.T) {
	response := MapReleasePlanInspection(releasePlanPresentationFixture(), time.Date(2026, time.July, 18, 18, 30, 0, 0, time.UTC))
	output := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{width: 72, available: true})
	plain := ansi.Strip(output)

	for _, want := range []string{
		"Local Inspection Only",
		"Evidence Not Inspected",
		"Remote Checks Not",
		"Performed",
		"Token Free",
		"This inspection uses local planning facts only",
		"Execution journals, dispatch journals",
		"Remote tags, releases, workflow runs",
		"Tokens and provider authorization",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("semantic limitation output omitted %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, " | ") || strings.Contains(plain, "local-only:") {
		t.Fatalf("human output retained the machine-oriented limitation mega-string:\n%s", plain)
	}
	assertReleasePlanLinesFit(t, output, 72)
}

func TestReleasePlanHumanOutputUsesVerticalLayoutAtNarrowAndUnknownWidths(t *testing.T) {
	response := MapReleasePlanInspection(releasePlanPresentationFixture(), time.Date(2026, time.July, 18, 18, 31, 0, 0, time.UTC))

	narrow := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{width: 28, available: true})
	narrowPlain := ansi.Strip(narrow)
	if strings.Contains(narrowPlain, "PROPERTY") || !strings.Contains(narrowPlain, "Unit\n  cli") ||
		!strings.Contains(narrowPlain, "Local Inspection Only\n  This") {
		t.Fatalf("narrow release-plan output did not use vertical properties:\n%s", narrowPlain)
	}
	assertReleasePlanLinesFit(t, narrow, 28)

	first := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{})
	second := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{})
	if first != second {
		t.Fatalf("unknown-width release-plan output is not deterministic:\nfirst=%q\nsecond=%q", first, second)
	}
	unknownPlain := ansi.Strip(first)
	if strings.Contains(unknownPlain, "PROPERTY") || !strings.Contains(unknownPlain, "Token Free\n  Tokens and provider authorization") {
		t.Fatalf("unknown-width release-plan output did not use vertical properties:\n%s", unknownPlain)
	}
}

func TestReleasePlanPresentationKeepsMachineLimitationsAndSecretsOut(t *testing.T) {
	result := releasePlanPresentationFixture()
	response := MapReleasePlanInspection(result, time.Date(2026, time.July, 18, 18, 32, 0, 0, time.UTC))

	if got := responseValueForProperty(t, response.Data["items"], "Limitations"); !strings.Contains(got, " | ") ||
		!strings.Contains(got, "local-only:") || !strings.Contains(got, "token-free:") {
		t.Fatalf("machine limitation row changed: %q", got)
	}
	var jsonOutput bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatJSON, &jsonOutput); err != nil {
		t.Fatalf("RenderTo JSON: %v", err)
	}
	if !strings.Contains(jsonOutput.String(), `"property": "Limitations"`) ||
		!strings.Contains(jsonOutput.String(), "local-only:") ||
		strings.Contains(jsonOutput.String(), "Local Inspection Only") ||
		strings.Contains(jsonOutput.String(), releaseSecretSentinel) {
		t.Fatalf("release-plan JSON contract or secret isolation changed:\n%s", jsonOutput.String())
	}
}

func TestReleaseContextPropertiesRemainReadableWithResponsiveLayout(t *testing.T) {
	response := MapValidatedReleaseContext(validatedReleaseContextFixture(), time.Date(2026, time.July, 18, 18, 33, 0, 0, time.UTC))
	output := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{width: 54, available: true})
	plain := ansi.Strip(output)

	for _, want := range []string{"Release commit", "Working directory", "services/api"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("CI context property output omitted %q:\n%s", want, plain)
		}
	}
	if compact := strings.Join(strings.Fields(plain), ""); !strings.Contains(compact, strings.Repeat("a", 40)) {
		t.Fatalf("CI context property output lost the wrapped release commit:\n%s", plain)
	}
	assertReleasePlanLinesFit(t, output, 54)
}

func releasePlanPresentationFixture() *ReleasePlanInspection {
	return &ReleasePlanInspection{
		Source:           "v2",
		Unit:             ReleasePlanInspectionUnit{ID: "cli", DisplayName: "Neko CLI"},
		CurrentVersion:   "2.4.0",
		RequestedChange:  Patch,
		NextVersion:      "2.4.1",
		Tag:              "v2.4.1",
		Executor:         "goreleaser",
		Delivery:         "github-actions",
		Workflow:         ".github/workflows/release-neko-cli.yml",
		WorkingDirectory: ".",
		UnitRoot:         "/Users/benjamin/Developer/Projects/nekoman/neko-cli",
		Readiness:        LocalPlanReady,
		Limitations:      appendCommonPlanLimitations(nil),
	}
}

type releasePlanOutputWidth struct {
	width     int
	available bool
}

func (width releasePlanOutputWidth) Width(io.Writer) (int, bool) {
	return width.width, width.available
}

func renderReleasePlanForTest(
	t *testing.T,
	response *plugin.Response,
	format renderer.OutputFormat,
	width renderer.OutputWidthProvider,
) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{Format: format, WidthProvider: width}, &output); err != nil {
		t.Fatalf("RenderWithOptionsTo: %v", err)
	}
	return output.String()
}

func assertReleasePlanLinesFit(t *testing.T, output string, width int) {
	t.Helper()
	for lineNumber, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if got := ansi.StringWidth(line); got > width {
			t.Fatalf("line %d visible width = %d, want at most %d: %q", lineNumber+1, got, width, line)
		}
	}
}
