package pipelineinspection

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestPipelinePresentationKeepsEssentialAndOptionalVerificationColumnsResponsive(t *testing.T) {
	response := mapPipelineResult(pipelinePresentationFixture())
	normal := ansi.Strip(renderPipelineForTest(t, response, pipelineTestWidth{width: 120, available: true}, false))
	for _, want := range []string{"Release Pipeline Inspection", "Summary", "Verification Facts", "Check", "Status", "Scope", "Subject", "Evidence", "Consumer workflow", "Verified", "release-service.yml", "Configured Pipeline", "Resolve release source", "Neko CLI"} {
		if !strings.Contains(normal, want) {
			t.Fatalf("normal output omitted %q:\n%s", want, normal)
		}
	}
	for _, forbidden := range []string{"consumer_structure", "not_observed", "Source", "doctor", "Runtime and Limitations"} {
		if strings.Contains(normal, forbidden) {
			t.Fatalf("normal output exposed %q:\n%s", forbidden, normal)
		}
	}

	narrow := renderPipelineForTest(t, response, pipelineTestWidth{width: 30, available: true}, false)
	narrowPlain := ansi.Strip(narrow)
	for _, want := range []string{"Check", "Status", "Scope", "Consumer workflow", "Verified", "Local", "Stage", "Runtime", "Owner", "Resolve release", "Limitations"} {
		if !strings.Contains(narrowPlain, want) {
			t.Fatalf("narrow output omitted %q:\n%s", want, narrowPlain)
		}
	}
	assertPipelineLinesFit(t, narrow, 30)
}

func TestPipelinePresentationIsDeterministicAtUnknownWidth(t *testing.T) {
	response := mapPipelineResult(pipelinePresentationFixture())
	first := renderPipelineForTest(t, response, pipelineTestWidth{}, false)
	second := renderPipelineForTest(t, response, pipelineTestWidth{}, false)
	if first != second {
		t.Fatalf("unknown-width output changed:\nfirst=%q\nsecond=%q", first, second)
	}
	plain := ansi.Strip(first)
	for _, want := range []string{"Check: Consumer workflow", "Status: Verified", "Subject: .github/workflows/release-service.yml", "#: 1", "Stage: Resolve release source", "Runtime: —", "Remote Git freshness was not inspected."} {
		if !strings.Contains(plain, want) {
			t.Fatalf("unknown-width output omitted %q:\n%s", want, plain)
		}
	}
}

func TestPipelinePresentationUsesSemanticTTYColorOnly(t *testing.T) {
	response := mapPipelineResult(pipelinePresentationFixture())
	colored := renderPipelineForTest(t, response, pipelineTestWidth{width: 100, available: true}, true)
	plain := renderPipelineForTest(t, response, pipelineTestWidth{width: 100, available: true}, false)
	if !strings.Contains(colored, "\x1b[32mReady\x1b[0m") {
		t.Fatalf("TTY output omitted semantic ready color: %q", colored)
	}
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("redirected/NO_COLOR-equivalent output contains ANSI: %q", plain)
	}
	if ansi.Strip(colored) != plain {
		t.Fatalf("semantic color changed visible output:\ncolored=%q\nplain=%q", ansi.Strip(colored), plain)
	}
}

func TestPipelinePresentationShowsRuntimeRecoveryAndManualIntervention(t *testing.T) {
	result := pipelinePresentationFixture()
	result.Status = pipelineUncertain
	result.Execution = pipelineExecution{Present: true, Identity: "execution-identity", State: "tag-pushed", Observations: []pipelineExecutionJournal{}}
	result.Dispatch = pipelineDispatch{Present: true, Identity: "dispatch-identity", State: "unknown", Correlation: "exact", Observations: []pipelineDispatchJournal{}}
	result.LocalGit = pipelineLocalGit{Scope: "local_only", Consistent: true}
	result.Recovery = pipelineRecovery{Evaluated: true, Classification: "interrupted-after-tag-push", RetrySafety: "automatic_retry_prohibited", Reasons: []string{}}
	result.ManualIntervention = pipelineManualIntervention{Required: true, Reasons: []string{"Inspect the durable dispatch outcome before retrying."}}
	result.Stages[0].RuntimeStatus = RuntimeUnknown
	result.Stages[0].RuntimeEvidence = "dispatch_journal"
	plain := ansi.Strip(renderPipelineForTest(t, mapPipelineResult(result), pipelineTestWidth{}, false))
	for _, want := range []string{
		"Lifecycle\n  Uncertain", "Execution\n  Tag pushed", "Dispatch\n  Unknown",
		"Recovery\n  Interrupted after tag push", "Resume\n  Not eligible",
		"Manual intervention\n  Required", "Runtime: Unknown", "Evidence: Dispatch journal",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("runtime presentation omitted %q:\n%s", want, plain)
		}
	}
}

func TestPipelinePresentationKeepsLifecycleAndVerificationStatusesSeparate(t *testing.T) {
	result := pipelinePresentationFixture()
	result.Status = pipelineReady
	result.Verification = projectPipelineVerification(VerificationSnapshot{
		RemoteStatus: "complete", RemoteRequested: true, RemoteAttempted: true,
		Facts: []VerificationFact{
			{
				Category: "consumer_structure", Class: VerificationLocal, Status: VerificationVerified,
				Subject: "workflow", Evidence: "The local workflow is valid.", Source: "doctor", Scope: "workflow",
			},
			{
				Category: "remote_workflow_identity", Class: VerificationRemote, Status: VerificationFailed,
				Subject: "owner/repository", Evidence: "The exact remote fact is missing.",
				Source: "doctor", Scope: "repository", References: []string{".git/config"},
			},
		},
	})
	plain := ansi.Strip(renderPipelineForTest(t, mapPipelineResult(result), pipelineTestWidth{}, false))
	for _, want := range []string{
		"Lifecycle\n  Ready", "Verification\n  Local checks passed; remote checks failed", "Remote verification\n  Complete — 1 failed",
		"Check: Remote workflow identity", "Status: Failed", "Subject: owner/repository",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("separate verification presentation omitted %q:\n%s", want, plain)
		}
	}
}

func renderPipelineForTest(t *testing.T, response *plugin.Response, width renderer.OutputWidthProvider, color bool) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true, WidthProvider: width, ColorProvider: pipelineTestColor(color),
	}, &output); err != nil {
		t.Fatalf("RenderWithOptionsTo: %v", err)
	}
	return output.String()
}

type pipelineTestWidth struct {
	width     int
	available bool
}

func (width pipelineTestWidth) Width(io.Writer) (int, bool) {
	return width.width, width.available
}

type pipelineTestColor bool

func (color pipelineTestColor) ColorEnabled(io.Writer) bool {
	return bool(color)
}

func assertPipelineLinesFit(t *testing.T, output string, width int) {
	t.Helper()
	for lineNumber, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if visible := ansi.StringWidth(line); visible > width {
			t.Fatalf("line %d width = %d, want <= %d: %q", lineNumber+1, visible, width, line)
		}
	}
}

func pipelinePresentationFixture() *pipelineResult {
	return &pipelineResult{
		SchemaVersion: 1, Status: pipelineReady,
		Unit:     pipelineUnit{ID: "service", Kind: "release", Executor: "goreleaser", Delivery: "github-actions", ConfiguredVersion: "1.2.3", WorkingDirectory: "."},
		Workflow: pipelineWorkflow{Path: ".github/workflows/release-service.yml"},
		Stages: []LifecycleStage{{
			ID: "source-unit-resolution", Label: "Resolve release source",
			Owner: StageOwnerNekoCLI, Location: StageLocationLocalProcess,
			Mutation: MutationNone, ConfigurationStatus: StageConfigured,
			Source: "pkg/release/release_start_v2.go",
		}},
		ProgressInspection: pipelineProgressInspection{JournalsInspected: true, ExecutionProgress: "not_started"},
		Execution:          pipelineExecution{Validity: "valid", Observations: []pipelineExecutionJournal{}},
		Dispatch:           pipelineDispatch{Correlation: "none", Observations: []pipelineDispatchJournal{}},
		LocalGit:           pipelineLocalGit{Scope: "local_only", RemoteFreshness: "remote_not_inspected"},
		Verification: projectPipelineVerification(VerificationSnapshot{
			RemoteStatus: "not_requested",
			Facts: []VerificationFact{{
				Category: "consumer_structure", Class: VerificationLocal, Status: VerificationVerified,
				Subject: ".github/workflows/release-service.yml", Evidence: "Consumer structure is locally verified.",
				Source: "doctor", Scope: "workflow", References: []string{".github/workflows/release-service.yml"},
			}},
		}),
		Limitations: []string{
			"Only local execution evidence was inspected; remote Git freshness was not inspected.",
			"Workflow execution and publication state were not inspected remotely.",
			"Runtime inspection is read-only and does not resume, retry, repair, or clean a release.",
		},
	}
}
