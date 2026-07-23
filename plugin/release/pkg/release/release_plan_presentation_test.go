package release

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestReleasePlanReadableOutputPresentsLimitationsSemantically(t *testing.T) {
	response := MapReleasePlanInspection(releasePlanPresentationFixture(), time.Date(2026, time.July, 18, 18, 30, 0, 0, time.UTC))
	concise := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{width: 96, available: true})
	concisePlain := ansi.Strip(concise)

	for _, want := range []string{
		"Release Plan",
		"Unit",
		"cli",
		"Current version",
		"2.4.0",
		"Requested change",
		"patch",
		"Next version",
		"2.4.1",
		"Tag",
		"v2.4.1",
		"Local readiness",
		"Ready",
		"Mutation boundary",
		"Inspection only",
		"Operations",
		"Resolve release identity",
		"Prepare tag",
		"Materialize files",
		"Release execution",
		"Primary Materialized Files",
		".neko/release.state.json",
		"plugin/release/manifest.json",
	} {
		if !strings.Contains(concisePlain, want) {
			t.Fatalf("concise release-plan output omitted %q:\n%s", want, concisePlain)
		}
	}
	for _, hidden := range []string{
		"Plan Details",
		"Known Release Files",
		"Assumptions and Limitations",
		"This inspection uses local planning facts only",
		"Execution journals, dispatch journals",
		"Remote tags, releases, workflow runs",
		"Tokens and provider authorization",
		"/Users/benjamin/Developer/Projects/nekoman/neko-cli",
	} {
		if strings.Contains(concisePlain, hidden) {
			t.Fatalf("concise release-plan output exposed describe-only value %q:\n%s", hidden, concisePlain)
		}
	}

	described := renderReleasePlanWithOptionsForTest(
		t,
		response,
		renderer.RenderOptions{
			Format: renderer.FormatTable, Describe: true,
			WidthProvider: releasePlanOutputWidth{width: 96, available: true},
		},
	)
	describedPlain := ansi.Strip(described)
	for _, want := range []string{
		"Plan Details",
		"V2 config and state",
		"Neko CLI",
		"goreleaser",
		"github-actions",
		".github/workflows/release-neko-cli.yml",
		"Materialized File Facts",
		"v2 release state",
		"sync plugin manifest version",
		"Known Release Files",
		"Assumptions and Limitations",
		"Local Inspection Only",
		"Evidence Not Inspected",
		"Remote Checks Not Performed",
		"Token Free",
		"This inspection uses local planning facts only",
		"Execution journals, dispatch journals",
		"Remote tags, releases, workflow runs",
		"Tokens and provider authorization",
	} {
		if !strings.Contains(describedPlain, want) {
			t.Fatalf("described release-plan output omitted %q:\n%s", want, describedPlain)
		}
	}
	if strings.Contains(describedPlain, " | ") || strings.Contains(describedPlain, "local-only:") ||
		strings.Contains(describedPlain, "/Users/benjamin/Developer/Projects/nekoman/neko-cli") {
		t.Fatalf("described human output retained a machine mega-string or absolute path:\n%s", describedPlain)
	}
	assertReleasePlanLinesFit(t, described, 96)
}

func TestReleasePlanDefaultShowsEveryBlocker(t *testing.T) {
	result := releasePlanPresentationFixture()
	result.Readiness = LocalPlanBlocked
	result.Blockers = []LocalPlanBlocker{
		{Category: "materialization-blocked", Message: "Configured output leaves the selected unit."},
		{Category: "unsupported-delivery", Message: "The selected delivery cannot execute this plan."},
	}
	response := MapReleasePlanInspection(result, time.Date(2026, time.July, 18, 18, 30, 30, 0, time.UTC))
	output := ansi.Strip(renderReleasePlanForTest(
		t,
		response,
		renderer.FormatTable,
		releasePlanOutputWidth{width: 72, available: true},
	))

	for _, want := range []string{
		"Blockers",
		"Materialization Blocked",
		"Configured output leaves the selected unit.",
		"Unsupported Delivery",
		"The selected delivery cannot execute this plan.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("blocked release-plan default omitted %q:\n%s", want, output)
		}
	}
}

func TestReleasePlanVerboseIsNoOpForLegacyAndCurrentSources(t *testing.T) {
	legacyRoot := t.TempDir()
	if err := releaseconfig.V1SaveConfigAt(legacyRoot, *validV1ReleaseConfig("1.2.3")); err != nil {
		t.Fatalf("write legacy release config: %v", err)
	}
	root, err := workspace.ValidateRepositoryRoot(legacyRoot)
	if err != nil {
		t.Fatalf("validate legacy repository root: %v", err)
	}
	originalVerbose := log.Verbose
	log.Verbose = true
	t.Cleanup(func() { log.Verbose = originalVerbose })

	stderr := captureReleaseUseCaseStderr(t, func() {
		response, handleErr := HandlePlanAt(root, plugin.Request{
			Command: "plan",
			Flags:   map[string]any{"change": "patch", "unit": "default"},
			Context: plugin.Context{Verbose: true},
		})
		if handleErr != nil || response == nil || response.Status != "success" {
			t.Fatalf("HandlePlanAt legacy source: response=%#v err=%v", response, handleErr)
		}
	})

	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("verbose release plan emitted generic execution narration:\n%s", stderr)
	}
}

func TestReleasePlanReadableOutputUsesVerticalLayoutAtNarrowAndUnknownWidths(t *testing.T) {
	response := MapReleasePlanInspection(releasePlanPresentationFixture(), time.Date(2026, time.July, 18, 18, 31, 0, 0, time.UTC))

	narrow := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{width: 28, available: true})
	narrowPlain := ansi.Strip(narrow)
	if strings.Contains(narrowPlain, "PROPERTY") || !strings.Contains(narrowPlain, "Unit\n  cli") ||
		!strings.Contains(narrowPlain, "Record 1") || !strings.Contains(narrowPlain, "Action: Update") {
		t.Fatalf("narrow release-plan output did not preserve summary and essential table records:\n%s", narrowPlain)
	}
	assertReleasePlanLinesFit(t, narrow, 28)

	options := renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true,
		WidthProvider: releasePlanOutputWidth{},
	}
	first := renderReleasePlanWithOptionsForTest(t, response, options)
	second := renderReleasePlanWithOptionsForTest(t, response, options)
	if first != second {
		t.Fatalf("unknown-width release-plan output is not deterministic:\nfirst=%q\nsecond=%q", first, second)
	}
	unknownPlain := ansi.Strip(first)
	if strings.Contains(unknownPlain, "PROPERTY") ||
		!strings.Contains(unknownPlain, "Assumption: Token Free") ||
		!strings.Contains(unknownPlain, "Source ownership: V2 config and state") ||
		!strings.Contains(unknownPlain, "Record 1") {
		t.Fatalf("unknown-width described release-plan output did not use deterministic vertical records:\n%s", unknownPlain)
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
	output := renderReleasePlanWithOptionsForTest(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true,
		WidthProvider: releasePlanOutputWidth{width: 54, available: true},
	})
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
		MaterializedOutputs: []PlannedMaterializedOutput{
			{
				Path: ".neko/release.state.json", Reason: "v2 release state",
				RequiredForReleaseCommit: true, Exists: true,
			},
			{
				Path: "plugin/release/manifest.json", Reason: "sync plugin manifest version with release plan",
				RequiredForReleaseCommit: true, Exists: true,
			},
		},
		KnownReleaseFiles: []InspectedKnownReleaseFile{
			{Path: ".goreleaser.yml", Reason: "release executor configuration"},
			{Path: ".neko/release.state.json", Reason: "v2 release state", RequiredForReleaseCommit: true},
			{Path: "plugin/release/manifest.json", Reason: "release plugin manifest", RequiredForReleaseCommit: true},
		},
		Readiness:   LocalPlanReady,
		Limitations: appendCommonPlanLimitations(nil),
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
	return renderReleasePlanWithOptionsForTest(t, response, renderer.RenderOptions{
		Format: format, WidthProvider: width,
	})
}

func renderReleasePlanWithOptionsForTest(
	t *testing.T,
	response *plugin.Response,
	options renderer.RenderOptions,
) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, options, &output); err != nil {
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
