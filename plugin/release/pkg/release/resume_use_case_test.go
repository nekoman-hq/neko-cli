package release

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

func TestResumeReleaseUseCaseRunsDiscoveryAssessmentSelectionAndOperation(t *testing.T) {
	trace := &resumeUseCaseTrace{}
	execution := resumeUseCaseExecution(ReleaseExecutionTagCreated)
	result := &GitHubActionsReleaseResult{ExecutionState: ReleaseExecutionHandoffReady}
	selected := &recordingResumeReleaseOperation{trace: trace, result: result}
	useCase := resumeReleaseUseCase{
		locator:  recordingResumableExecutionLocator{trace: trace, execution: execution},
		assessor: recordingResumableExecutionAssessor{trace: trace, assessment: &ReleaseExecutionRecoveryAssessment{}},
		contexts: recordingResumeExecutionContextReconstructor{trace: trace, execution: &resumableReleaseExecution{Discovered: execution, Context: &ReleaseExecutionContext{}}},
		resolver: recordingResumeRecoveryResolver{trace: trace, resolution: resumeRecoveryResolution{Operation: resumeReleaseFromTagCreated}},
		selector: resumeReleaseOperationSelector{fromTagCreated: selected},
	}

	outcome, failure := useCase.Resume(context.Background(), ResumeCommandRequest{UnitID: "api"})

	if failure != nil || outcome != result {
		t.Fatalf("outcome=%#v failure=%#v", outcome, failure)
	}
	want := []string{"locate", "assess", "resolve-policy", "reconstruct-context", "resume-operation"}
	if !reflect.DeepEqual(trace.calls, want) {
		t.Fatalf("calls=%#v want=%#v", trace.calls, want)
	}
}

func TestResumeReleaseUseCaseDryRunReturnsAssessmentBeforeContinuation(t *testing.T) {
	trace := &resumeUseCaseTrace{}
	execution := resumeUseCaseExecution(ReleaseExecutionPrepared)
	useCase := resumeReleaseUseCase{
		locator:  recordingResumableExecutionLocator{trace: trace, execution: execution},
		assessor: recordingResumableExecutionAssessor{trace: trace, assessment: &ReleaseExecutionRecoveryAssessment{Status: ReleaseExecutionRecoveryNotStarted}},
		contexts: recordingResumeExecutionContextReconstructor{trace: trace, failure: failureFromMessage("UNEXPECTED", "context reconstruction must not run")},
		resolver: recordingResumeRecoveryResolver{trace: trace, resolution: resumeRecoveryResolution{Operation: resumeReleaseFromTagCreated}},
	}

	outcome, failure := useCase.Resume(context.Background(), ResumeCommandRequest{UnitID: "api", DryRun: true})

	if failure != nil {
		t.Fatalf("failure=%#v", failure)
	}
	if _, ok := outcome.(*ResumeAssessment); !ok {
		t.Fatalf("outcome=%T, want *ResumeAssessment", outcome)
	}
	if want := []string{"locate", "assess"}; !reflect.DeepEqual(trace.calls, want) {
		t.Fatalf("calls=%#v want=%#v", trace.calls, want)
	}
}

func TestResumeReleaseUseCaseVerboseDryRunLogsAssessmentWithoutContinuation(t *testing.T) {
	originalVerbose := log.Verbose
	log.Verbose = true
	t.Cleanup(func() { log.Verbose = originalVerbose })
	trace := &resumeUseCaseTrace{}
	execution := resumeUseCaseExecution(ReleaseExecutionPrepared)
	execution.resolution.Journal.Identity.SHA256 = strings.Repeat("a", 64)
	execution.resolution.Journal.UnitID = "api"
	useCase := resumeReleaseUseCase{
		locator: recordingResumableExecutionLocator{trace: trace, execution: execution},
		assessor: recordingResumableExecutionAssessor{
			trace: trace,
			assessment: &ReleaseExecutionRecoveryAssessment{
				Status: ReleaseExecutionRecoveryNotStarted, SafeToContinue: true,
			},
		},
		contexts: recordingResumeExecutionContextReconstructor{
			trace: trace, failure: failureFromMessage("UNEXPECTED", "must not continue"),
		},
		resolver: recordingResumeRecoveryResolver{
			trace: trace, resolution: resumeRecoveryResolution{Operation: resumeReleaseFromTagCreated},
		},
	}

	_, stderr := captureReleaseProgressOutput(t, func() {
		outcome, failure := useCase.Resume(context.Background(), ResumeCommandRequest{UnitID: "api", DryRun: true})
		if failure != nil || outcome == nil {
			t.Fatalf("outcome=%#v failure=%#v", outcome, failure)
		}
	})

	assertOrderedSubstrings(t, stderr,
		"Discovering release execution journals for unit=api",
		"Selected exact release execution: identity="+strings.Repeat("a", 64)+" unit=api state=prepared pending=none",
		"Evaluating local recovery evidence for the selected execution",
		"Recovery evaluation completed: status=not-started eligible=true pending=none",
		"Dry-run recovery assessment completed; no continuation was performed",
	)
	for _, forbidden := range []string{
		"/tmp/execution.json",
		"Invoking selected continuation",
		"Resume continuation completed",
		"push continuation completed",
	} {
		if strings.Contains(stderr, forbidden) {
			t.Fatalf("resume dry-run verbose output contained %q:\n%s", forbidden, stderr)
		}
	}
	if want := []string{"locate", "assess"}; !reflect.DeepEqual(trace.calls, want) {
		t.Fatalf("dry-run orchestration changed: calls=%#v want=%#v", trace.calls, want)
	}
}

func TestResumeReleaseUseCaseVerboseExecutionFollowsAuthoritativeCallOrder(t *testing.T) {
	originalVerbose := log.Verbose
	log.Verbose = true
	t.Cleanup(func() { log.Verbose = originalVerbose })
	trace := &resumeUseCaseTrace{}
	execution := resumeUseCaseExecution(ReleaseExecutionTagCreated)
	execution.resolution.Journal.Identity.SHA256 = strings.Repeat("b", 64)
	execution.resolution.Journal.UnitID = "api"
	result := &GitHubActionsReleaseResult{
		ExecutionState: ReleaseExecutionHandoffReady,
		DispatchState:  DispatchJournalAccepted,
	}
	useCase := resumeReleaseUseCase{
		locator: recordingResumableExecutionLocator{trace: trace, execution: execution},
		assessor: recordingResumableExecutionAssessor{
			trace: trace,
			assessment: &ReleaseExecutionRecoveryAssessment{
				Status: ReleaseExecutionRecoveryInterruptedAfterTag, SafeToContinue: true,
			},
		},
		contexts: recordingResumeExecutionContextReconstructor{
			trace: trace,
			execution: &resumableReleaseExecution{
				Discovered: execution, Context: &ReleaseExecutionContext{},
			},
		},
		resolver: recordingResumeRecoveryResolver{
			trace: trace, resolution: resumeRecoveryResolution{Operation: resumeReleaseFromTagCreated},
		},
		selector: resumeReleaseOperationSelector{
			fromTagCreated: &recordingResumeReleaseOperation{trace: trace, result: result},
		},
	}

	_, stderr := captureReleaseProgressOutput(t, func() {
		outcome, failure := useCase.Resume(context.Background(), ResumeCommandRequest{UnitID: "api"})
		if failure != nil || outcome != result {
			t.Fatalf("outcome=%#v failure=%#v", outcome, failure)
		}
	})

	assertOrderedSubstrings(t, stderr,
		"Discovering release execution journals for unit=api",
		"Selected exact release execution:",
		"Evaluating local recovery evidence",
		"Recovery evaluation completed:",
		"Resolving the pending action with the authoritative recovery policy",
		"Validating the selected journal identity against current V2 configuration",
		"Journal and current V2 configuration identity validated",
		"Selecting continuation operation: continue after unit tag",
		"Invoking selected continuation: continue after unit tag",
		"Resume continuation completed: execution=Handoff Ready dispatch=Accepted",
	)
	if want := []string{"locate", "assess", "resolve-policy", "reconstruct-context", "resume-operation"}; !reflect.DeepEqual(trace.calls, want) {
		t.Fatalf("verbose orchestration changed: calls=%#v want=%#v", trace.calls, want)
	}
}

func TestResumeReleaseUseCaseStopsAtDiscoveryAssessmentAndContextFailures(t *testing.T) {
	tests := []struct {
		name     string
		locator  resumableExecutionLocator
		assessor resumableExecutionAssessor
		contexts resumeExecutionContextReconstructor
		code     string
	}{
		{
			name:    "discovery",
			locator: recordingResumableExecutionLocator{failure: failureFromMessage("DISCOVERY_FAILED", "discovery failed")},
			code:    "DISCOVERY_FAILED",
		},
		{
			name:     "assessment",
			locator:  recordingResumableExecutionLocator{execution: resumeUseCaseExecution(ReleaseExecutionTagCreated)},
			assessor: recordingResumableExecutionAssessor{failure: failureFromMessage("ASSESSMENT_FAILED", "assessment failed")},
			code:     "ASSESSMENT_FAILED",
		},
		{
			name:     "context reconstruction",
			locator:  recordingResumableExecutionLocator{execution: resumeUseCaseExecution(ReleaseExecutionTagCreated)},
			assessor: recordingResumableExecutionAssessor{assessment: &ReleaseExecutionRecoveryAssessment{}},
			contexts: recordingResumeExecutionContextReconstructor{failure: failureFromMessage("CONTEXT_FAILED", "context failed")},
			code:     "CONTEXT_FAILED",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useCase := resumeReleaseUseCase{
				locator:  test.locator,
				assessor: test.assessor,
				contexts: test.contexts,
				resolver: recordingResumeRecoveryResolver{resolution: resumeRecoveryResolution{Operation: resumeReleaseFromTagCreated}},
				selector: resumeReleaseOperationSelector{fromTagCreated: &recordingResumeReleaseOperation{}},
			}

			outcome, failure := useCase.Resume(context.Background(), ResumeCommandRequest{})

			if outcome != nil || failure == nil || failure.Code != test.code {
				t.Fatalf("outcome=%#v failure=%#v", outcome, failure)
			}
		})
	}
}

func TestResumeReleaseUseCaseBlocksUnsafeRecoveryBeforeContextReconstruction(t *testing.T) {
	trace := &resumeUseCaseTrace{}
	execution := resumeUseCaseExecution(ReleaseExecutionTagPushed)
	useCase := resumeReleaseUseCase{
		locator:  recordingResumableExecutionLocator{trace: trace, execution: execution},
		assessor: recordingResumableExecutionAssessor{trace: trace, assessment: &ReleaseExecutionRecoveryAssessment{}},
		contexts: recordingResumeExecutionContextReconstructor{trace: trace},
		resolver: recordingResumeRecoveryResolver{trace: trace, resolution: refusedResumeRecovery(resumeRecoveryRefusalAmbiguousTagPush, ReleaseExecutionTagPushed, "")},
	}

	outcome, failure := useCase.Resume(context.Background(), ResumeCommandRequest{})

	if outcome != nil || failure == nil || failure.Code != "RESUME_BLOCKED" {
		t.Fatalf("outcome=%#v failure=%#v", outcome, failure)
	}
	if want := []string{"locate", "assess", "resolve-policy"}; !reflect.DeepEqual(trace.calls, want) {
		t.Fatalf("calls=%#v want=%#v", trace.calls, want)
	}
}

func TestResumeReleaseUseCaseChecksConfigBeforePhaseRefusal(t *testing.T) {
	trace := &resumeUseCaseTrace{}
	execution := resumeUseCaseExecution(ReleaseExecutionPrepared)
	useCase := resumeReleaseUseCase{
		locator:  recordingResumableExecutionLocator{trace: trace, execution: execution},
		assessor: recordingResumableExecutionAssessor{trace: trace, assessment: &ReleaseExecutionRecoveryAssessment{}},
		contexts: recordingResumeExecutionContextReconstructor{trace: trace, failure: failureFromMessage("JOURNAL_CONFLICT", "config conflict")},
		resolver: recordingResumeRecoveryResolver{trace: trace, resolution: refusedResumeRecovery(resumeRecoveryRefusalBeforeCommit, ReleaseExecutionPrepared, "")},
	}

	outcome, failure := useCase.Resume(context.Background(), ResumeCommandRequest{})

	if outcome != nil || failure == nil || failure.Code != "JOURNAL_CONFLICT" {
		t.Fatalf("outcome=%#v failure=%#v", outcome, failure)
	}
	if want := []string{"locate", "assess", "resolve-policy", "reconstruct-context"}; !reflect.DeepEqual(trace.calls, want) {
		t.Fatalf("calls=%#v want=%#v", trace.calls, want)
	}
}

func TestResumeReleaseOperationSelectorReturnsExactlyOneNamedOperation(t *testing.T) {
	commit := &recordingResumeReleaseOperation{}
	tag := &recordingResumeReleaseOperation{}
	pushed := &recordingResumeReleaseOperation{}
	completed := &recordingResumeReleaseOperation{}
	selector := resumeReleaseOperationSelector{
		fromCommitCreated: commit,
		fromTagCreated:    tag,
		fromTagPushed:     pushed,
		completedHandoff:  completed,
	}
	tests := []struct {
		want resumeReleaseExecutionOperation
		kind resumeReleaseOperationKind
	}{
		{kind: resumeReleaseFromCommitCreated, want: commit},
		{kind: resumeReleaseFromTagCreated, want: tag},
		{kind: resumeReleaseFromTagPushed, want: pushed},
		{kind: returnCompletedReleaseHandoff, want: completed},
	}
	for _, test := range tests {
		got, err := selector.Select(test.kind)
		if err != nil || got != test.want {
			t.Fatalf("kind=%d got=%T err=%v", test.kind, got, err)
		}
	}
	if _, err := selector.Select(resumeReleaseOperationUnknown); err == nil {
		t.Fatal("unknown operation was selected")
	}
}

func TestReturnCompletedReleaseHandoffHasNoDependenciesOrEffects(t *testing.T) {
	execution := resumeUseCaseExecution(ReleaseExecutionHandoffReady)
	execution.resolution.Journal.UnitID = "api"
	execution.resolution.Journal.NextVersion = "1.2.4"
	execution.resolution.Journal.Tag = "api/v1.2.4"
	execution.resolution.Journal.WorkflowPath = ".github/workflows/release.yml"
	compatible := &resumableReleaseExecution{Discovered: execution}

	result, failure := (returnCompletedReleaseHandoffOperation{}).Resume(context.Background(), compatible)

	if failure != nil || result.ExecutionState != ReleaseExecutionHandoffReady || result.RecoveryGuidance != "Release was already handed off." {
		t.Fatalf("result=%#v failure=%#v", result, failure)
	}
}

type resumeUseCaseTrace struct {
	calls []string
}

func (trace *resumeUseCaseTrace) add(call string) {
	if trace != nil {
		trace.calls = append(trace.calls, call)
	}
}

type recordingResumableExecutionLocator struct {
	trace     *resumeUseCaseTrace
	execution *resumableExecution
	failure   *CommandFailure
}

func (locator recordingResumableExecutionLocator) Find(string) (*resumableExecution, *CommandFailure) {
	locator.trace.add("locate")
	return locator.execution, locator.failure
}

type recordingResumableExecutionAssessor struct {
	trace      *resumeUseCaseTrace
	assessment *ReleaseExecutionRecoveryAssessment
	failure    *CommandFailure
}

func (assessor recordingResumableExecutionAssessor) Assess(*resumableExecution) (*ReleaseExecutionRecoveryAssessment, *CommandFailure) {
	assessor.trace.add("assess")
	return assessor.assessment, assessor.failure
}

type recordingResumeExecutionContextReconstructor struct {
	trace     *resumeUseCaseTrace
	execution *resumableReleaseExecution
	failure   *CommandFailure
}

func (reconstructor recordingResumeExecutionContextReconstructor) Reconstruct(*resumableExecution) (*resumableReleaseExecution, *CommandFailure) {
	reconstructor.trace.add("reconstruct-context")
	return reconstructor.execution, reconstructor.failure
}

type recordingResumeRecoveryResolver struct {
	trace      *resumeUseCaseTrace
	resolution resumeRecoveryResolution
}

func (resolver recordingResumeRecoveryResolver) Resolve(*ReleaseExecutionJournal, *ReleaseExecutionRecoveryAssessment) resumeRecoveryResolution {
	resolver.trace.add("resolve-policy")
	return resolver.resolution
}

type recordingResumeReleaseOperation struct {
	trace   *resumeUseCaseTrace
	result  *GitHubActionsReleaseResult
	failure *CommandFailure
}

func (operation *recordingResumeReleaseOperation) Resume(context.Context, *resumableReleaseExecution) (*GitHubActionsReleaseResult, *CommandFailure) {
	operation.trace.add("resume-operation")
	return operation.result, operation.failure
}

func resumeUseCaseExecution(state ReleaseExecutionJournalState) *resumableExecution {
	return &resumableExecution{resolution: ReleaseExecutionJournalResolution{
		Path: "/tmp/execution.json",
		Journal: &ReleaseExecutionJournal{
			State:            state,
			PendingAction:    ReleaseExecutionPendingNone,
			ReleaseCommitSHA: "1111111111111111111111111111111111111111",
		},
	}}
}
