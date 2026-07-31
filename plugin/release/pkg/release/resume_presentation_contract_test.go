package release

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestResumeAssessmentPresentationSeparatesDecisionFromEvidence(t *testing.T) {
	response, err := MapResumeCommandOutcome(&ResumeAssessment{
		UnitID:               "api",
		NextVersion:          "2.4.1",
		Tag:                  "api/v2.4.1",
		ExecutionJournalPath: "/private/tmp/resume-contract/.git/neko/release/executions/id.json",
		State:                ReleaseExecutionCommitCreated,
		PendingAction:        ReleaseExecutionPendingPushReleaseCommit,
		RecoveryStatus:       ReleaseExecutionRecoveryInterruptedBeforePush,
		SafeToContinue:       true,
		KnownFilePaths:       []string{".neko/release.state.json", "services/api/package.json"},
		Guidance:             "Resume can safely continue the pending commit push.",
	}, time.Date(2026, time.July, 22, 13, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("MapResumeCommandOutcome: %v", err)
	}
	if code, present := response.ExplicitExitCode(); !present || code != 0 {
		t.Fatalf("resume dry-run assessment exit = (%d, %t), want (0, true)", code, present)
	}

	concise := ansi.Strip(renderLifecycleResponse(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, WidthProvider: releasePlanOutputWidth{width: 84, available: true},
	}))
	for _, want := range []string{
		"Resume Summary",
		"Execution identity",
		"Journal phase",
		"Commit Created",
		"Pending action",
		"Push Release Commit",
		"Resume eligibility",
		"Eligible",
		"Retry safety",
		"Safe",
		"Planned Continuation",
		"Mutation boundary",
		"Dry run",
	} {
		if !strings.Contains(concise, want) {
			t.Fatalf("concise resume output omitted %q:\n%s", want, concise)
		}
	}
	for _, hidden := range []string{"Recovery Journal", "Local Git Evidence", "Recovery Assessment", "Continuation and Handoff", "Limitations"} {
		if strings.Contains(concise, hidden) {
			t.Fatalf("concise resume output exposed describe-only %q:\n%s", hidden, concise)
		}
	}

	described := ansi.Strip(renderLifecycleResponse(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true,
		WidthProvider: releasePlanOutputWidth{width: 84, available: true},
	}))
	for _, want := range []string{
		"Recovery Journal",
		"Local Git Evidence",
		"Recovery Assessment",
		"Continuation and Handoff",
		"Limitations",
		".git/neko/release/executions/id.json",
		"Remote evidence was not inspected",
	} {
		if !strings.Contains(described, want) {
			t.Fatalf("described resume output omitted %q:\n%s", want, described)
		}
	}
	if strings.Contains(described, "/private/tmp/resume-contract") {
		t.Fatalf("described resume output exposed an absolute path:\n%s", described)
	}
}

func TestResumePresentationKeepsDomainJSONInvariant(t *testing.T) {
	response, err := MapResumeCommandOutcome(&ResumeAssessment{
		UnitID: "api", NextVersion: "2.4.1", Tag: "api/v2.4.1",
		ExecutionJournalPath: "/private/tmp/resume-contract/.git/neko/release/executions/id.json",
		State:                ReleaseExecutionTagCreated, PendingAction: ReleaseExecutionPendingPushReleaseCommit,
		RecoveryStatus: ReleaseExecutionRecoveryConflicted, SafeToContinue: false,
		KnownFilePaths: []string{".neko/release.state.json"},
		Guidance:       "Inspect remote commit evidence before retrying.",
	}, time.Date(2026, time.July, 22, 13, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("MapResumeCommandOutcome: %v", err)
	}
	if code, present := response.ExplicitExitCode(); !present || code != 0 {
		t.Fatalf("unsafe resume dry-run exit = (%d, %t), want (0, true)", code, present)
	}
	plain := renderLifecycleResponse(t, response, renderer.RenderOptions{Format: renderer.FormatJSON})
	for _, options := range []renderer.RenderOptions{
		{Format: renderer.FormatJSON, Describe: true},
		{Format: renderer.FormatJSON, Verbose: true},
		{Format: renderer.FormatJSON, Describe: true, Verbose: true},
	} {
		if got := renderLifecycleResponse(t, response, options); got != plain {
			t.Fatalf("resume JSON changed with presentation mode:\nplain=%s\nmode=%s", plain, got)
		}
	}
	for _, forbidden := range []string{"human_table", "human_properties", "describe_only", "\x1b[", "resume-contract-secret"} {
		if strings.Contains(plain, forbidden) {
			t.Fatalf("resume JSON contains %q:\n%s", forbidden, plain)
		}
	}
}

func TestResumePresentationUsesDeterministicResponsiveRecords(t *testing.T) {
	response, err := MapResumeCommandOutcome(&ResumeAssessment{
		UnitID: "api", NextVersion: "2.4.1", Tag: "api/v2.4.1",
		ExecutionJournalPath: "/private/tmp/resume-contract/.git/neko/release/executions/id.json",
		State:                ReleaseExecutionDispatchJournalPrepared, PendingAction: ReleaseExecutionPendingPushUnitTag,
		RecoveryStatus: ReleaseExecutionRecoveryReadyForDispatch, SafeToContinue: true,
		KnownFilePaths: []string{".neko/release.state.json"},
		Guidance:       "Continue the pending tag push.",
	}, time.Date(2026, time.July, 22, 13, 2, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("MapResumeCommandOutcome: %v", err)
	}
	narrow := renderLifecycleResponse(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true,
		WidthProvider: releasePlanOutputWidth{width: 34, available: true},
	})
	for _, want := range []string{"Resume Summary", "Pending", "Push Unit Tag", "Planned Continuation", "Action:"} {
		if !strings.Contains(ansi.Strip(narrow), want) {
			t.Fatalf("narrow resume output omitted %q:\n%s", want, ansi.Strip(narrow))
		}
	}
	assertReleasePlanLinesFit(t, narrow, 34)

	options := renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true,
		WidthProvider: releasePlanOutputWidth{},
	}
	var first, second bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, options, &first); err != nil {
		t.Fatalf("first unknown-width render: %v", err)
	}
	if err := renderer.RenderWithOptionsTo(response, options, &second); err != nil {
		t.Fatalf("second unknown-width render: %v", err)
	}
	if first.String() != second.String() || !strings.Contains(ansi.Strip(first.String()), "Record 1") {
		t.Fatalf("unknown-width resume output is not deterministic and vertical:\n%s", first.String())
	}
}

func TestResumeDefaultKeepsEveryRecoveryDecisionActionable(t *testing.T) {
	tests := []struct {
		name        string
		status      ReleaseExecutionRecoveryStatus
		pending     ReleaseExecutionPendingAction
		guidance    string
		eligibility string
		safe        bool
	}{
		{
			name: "resumable commit push", status: ReleaseExecutionRecoveryInterruptedBeforePush,
			pending: ReleaseExecutionPendingPushReleaseCommit, safe: true,
			guidance: "Continue the confirmed release commit push.", eligibility: "Eligible",
		},
		{
			name: "uncertain tag push", status: ReleaseExecutionRecoveryInterruptedAfterCommitPush,
			pending:  ReleaseExecutionPendingPushUnitTag,
			guidance: "Verify remote tag evidence manually before retry.", eligibility: "Not eligible",
		},
		{
			name: "conflicting evidence", status: ReleaseExecutionRecoveryConflicted,
			pending:  ReleaseExecutionPendingNone,
			guidance: "Resolve the mismatched commit and tag evidence manually.", eligibility: "Not eligible",
		},
		{
			name: "already handed off", status: ReleaseExecutionRecoveryAlreadyHandedOff,
			pending:  ReleaseExecutionPendingNone,
			guidance: "Release was already handed off.", eligibility: "Not eligible",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, err := MapResumeCommandOutcome(&ResumeAssessment{
				UnitID: "api", NextVersion: "2.4.1", Tag: "api/v2.4.1",
				ExecutionJournalPath: ".git/neko/release/executions/id.json",
				State:                ReleaseExecutionTagCreated, PendingAction: test.pending,
				RecoveryStatus: test.status, SafeToContinue: test.safe,
				KnownFilePaths: []string{".neko/release.state.json"},
				Guidance:       test.guidance,
			}, time.Date(2026, time.July, 22, 13, 3, 0, 0, time.UTC))
			if err != nil {
				t.Fatalf("MapResumeCommandOutcome: %v", err)
			}
			output := ansi.Strip(renderLifecycleResponse(t, response, renderer.RenderOptions{
				Format: renderer.FormatTable,
			}))
			for _, want := range []string{
				releaseLifecycleReadableValue(string(test.status)),
				releaseLifecycleReadableValue(string(test.pending)),
				test.eligibility,
				test.guidance,
				"Retry safety",
			} {
				if !strings.Contains(output, want) {
					t.Fatalf("resume default omitted %q:\n%s", want, output)
				}
			}
		})
	}
}
