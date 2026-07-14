package release

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

var errResumeOperationDependency = errors.New("resume operation dependency failed")

func TestResumeFromCommitCreatedCreatesOnlyMissingTagThenContinues(t *testing.T) {
	trace := &resumeOperationTrace{}
	release := resumeOperationRelease()
	operation := resumeFromCommitCreatedOperation{
		preparer:     recordingResumePreparer{trace: trace, release: release},
		tags:         recordingResumeTagInspector{trace: trace},
		creator:      recordingResumeTagCreator{trace: trace},
		continuation: recordingTagCreatedContinuation{trace: trace, result: &GitHubActionsReleaseResult{}},
	}

	result, failure := operation.Resume(context.Background(), &resumableReleaseExecution{})

	if failure != nil || result == nil {
		t.Fatalf("result=%#v failure=%#v", result, failure)
	}
	want := []string{"prepare-resume", "inspect-tag", "create-tag", "continue-tag-created"}
	if !reflect.DeepEqual(trace.calls, want) {
		t.Fatalf("calls=%#v want=%#v", trace.calls, want)
	}
}

func TestResumeFromCommitCreatedPreservesExistingExpectedTagBlock(t *testing.T) {
	trace := &resumeOperationTrace{}
	release := resumeOperationRelease()
	operation := resumeFromCommitCreatedOperation{
		preparer:     recordingResumePreparer{trace: trace, release: release},
		tags:         recordingResumeTagInspector{trace: trace, commitSHA: release.CommitSHA},
		creator:      recordingResumeTagCreator{trace: trace},
		continuation: recordingTagCreatedContinuation{trace: trace},
	}

	result, failure := operation.Resume(context.Background(), &resumableReleaseExecution{})

	if result != nil || failure == nil || failure.Code != "RESUME_FAILED" {
		t.Fatalf("result=%#v failure=%#v", result, failure)
	}
	if want := []string{"prepare-resume", "inspect-tag"}; !reflect.DeepEqual(trace.calls, want) {
		t.Fatalf("calls=%#v want=%#v", trace.calls, want)
	}
}

func TestResumeFromTagCreatedUsesActiveDispatchAndPushOperationsInOrder(t *testing.T) {
	trace := &resumeOperationTrace{}
	operation := resumeFromTagCreatedOperation{
		preparer:         recordingResumePreparer{trace: trace, release: resumeOperationRelease()},
		dispatches:       recordingResumeDispatchAssessor{trace: trace, dispatch: resumeOperationDispatch(DispatchJournalPrepared)},
		dispatchPreparer: recordingResumeDispatchPreparer{trace: trace},
		commitPusher:     recordingResumeCommitPusher{trace: trace},
		tagPusher:        recordingResumeTagPusher{trace: trace},
		continuation:     recordingTagPushedContinuation{trace: trace, result: &GitHubActionsReleaseResult{}},
	}

	result, failure := operation.Resume(context.Background(), &resumableReleaseExecution{})

	if failure != nil || result == nil {
		t.Fatalf("result=%#v failure=%#v", result, failure)
	}
	want := []string{"prepare-resume", "assess-dispatch", "prepare-dispatch", "push-commit", "push-tag", "continue-tag-pushed"}
	if !reflect.DeepEqual(trace.calls, want) {
		t.Fatalf("calls=%#v want=%#v", trace.calls, want)
	}
}

func TestResumeFromTagCreatedStopsAtEachFailedOperation(t *testing.T) {
	steps := []string{"assess-dispatch", "prepare-dispatch", "push-commit", "push-tag", "continue-tag-pushed"}
	for index, failed := range steps {
		t.Run(failed, func(t *testing.T) {
			trace := &resumeOperationTrace{failAt: failed}
			operation := resumeFromTagCreatedOperation{
				dispatches:       recordingResumeDispatchAssessor{trace: trace, dispatch: resumeOperationDispatch(DispatchJournalPrepared)},
				dispatchPreparer: recordingResumeDispatchPreparer{trace: trace},
				commitPusher:     recordingResumeCommitPusher{trace: trace},
				tagPusher:        recordingResumeTagPusher{trace: trace},
				continuation:     recordingTagPushedContinuation{trace: trace},
			}

			result, failure := operation.ResumePrepared(context.Background(), resumeOperationRelease())

			if result != nil || failure == nil || failure.Code != "RESUME_FAILED" {
				t.Fatalf("result=%#v failure=%#v", result, failure)
			}
			if want := steps[:index+1]; !reflect.DeepEqual(trace.calls, want) {
				t.Fatalf("calls=%#v want=%#v", trace.calls, want)
			}
		})
	}
}

func TestResumeFromTagCreatedPreservesEveryPushFailureBoundary(t *testing.T) {
	tests := []struct {
		name        string
		failAt      string
		state       ReleaseExecutionJournalState
		pending     ReleaseExecutionPendingAction
		laterEffect string
	}{
		{name: "commit pending write", failAt: "pending:push-release-commit", state: ReleaseExecutionDispatchJournalPrepared, pending: ReleaseExecutionPendingNone, laterEffect: "effect:push-commit"},
		{name: "commit push", failAt: "effect:push-commit", state: ReleaseExecutionDispatchJournalPrepared, pending: ReleaseExecutionPendingPushReleaseCommit, laterEffect: "effect:push-tag"},
		{name: "commit confirmation", failAt: "confirm:commit-pushed", state: ReleaseExecutionDispatchJournalPrepared, pending: ReleaseExecutionPendingPushReleaseCommit, laterEffect: "effect:push-tag"},
		{name: "tag pending write", failAt: "pending:push-unit-tag", state: ReleaseExecutionCommitPushed, pending: ReleaseExecutionPendingNone, laterEffect: "effect:push-tag"},
		{name: "tag push", failAt: "effect:push-tag", state: ReleaseExecutionCommitPushed, pending: ReleaseExecutionPendingPushUnitTag, laterEffect: "continue-tag-pushed"},
		{name: "tag confirmation", failAt: "confirm:tag-pushed", state: ReleaseExecutionCommitPushed, pending: ReleaseExecutionPendingPushUnitTag, laterEffect: "continue-tag-pushed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			trace := &resumeOperationTrace{failAt: test.failAt}
			journal := &resumePushBoundaryJournal{trace: trace, state: ReleaseExecutionDispatchJournalPrepared, pending: ReleaseExecutionPendingNone}
			git := resumePushBoundaryGit{trace: trace}
			operation := resumeFromTagCreatedOperation{
				dispatches:       recordingResumeDispatchAssessor{trace: trace, dispatch: resumeOperationDispatch(DispatchJournalPrepared)},
				dispatchPreparer: recordingResumeDispatchPreparer{trace: trace},
				commitPusher:     pushGitHubActionsReleaseCommit{journal: journal, git: git},
				tagPusher:        pushGitHubActionsReleaseTag{journal: journal, git: git},
				continuation:     recordingTagPushedContinuation{trace: trace},
			}

			result, failure := operation.ResumePrepared(context.Background(), resumeOperationRelease())

			if result != nil || failure == nil || failure.Code != "RESUME_FAILED" {
				t.Fatalf("result=%#v failure=%#v", result, failure)
			}
			if journal.state != test.state || journal.pending != test.pending {
				t.Fatalf("state=%s pending=%s want state=%s pending=%s", journal.state, journal.pending, test.state, test.pending)
			}
			assertResumeOperationCallAbsent(t, trace.calls, test.laterEffect)
		})
	}
}

func TestResumeFromTagPushedSelectsAcceptedReuseWithoutFreshDispatch(t *testing.T) {
	trace := &resumeOperationTrace{}
	accepted := recordingResumeDispatchCompletion{trace: trace, call: "reuse-accepted", result: &GitHubActionsReleaseResult{}}
	operation := resumeFromTagPushedOperation{
		dispatches: recordingResumeDispatchAssessor{trace: trace, dispatch: resumeOperationDispatch(DispatchJournalAccepted)},
		selector: resumeDispatchOperationSelector{
			fresh:    recordingResumeDispatchCompletion{trace: trace, call: "fresh-dispatch"},
			accepted: accepted,
		},
	}

	result, failure := operation.ResumePrepared(context.Background(), resumeOperationRelease())

	if failure != nil || result == nil {
		t.Fatalf("result=%#v failure=%#v", result, failure)
	}
	if want := []string{"assess-dispatch", "reuse-accepted"}; !reflect.DeepEqual(trace.calls, want) {
		t.Fatalf("calls=%#v want=%#v", trace.calls, want)
	}
}

func TestResumeFromTagPushedRefusesTerminalDispatchBeforeSelection(t *testing.T) {
	trace := &resumeOperationTrace{}
	operation := resumeFromTagPushedOperation{
		dispatches: recordingResumeDispatchAssessor{trace: trace, dispatch: resumeOperationDispatch(DispatchJournalUnknown)},
		selector: resumeDispatchOperationSelector{
			fresh:    recordingResumeDispatchCompletion{trace: trace, call: "fresh-dispatch"},
			accepted: recordingResumeDispatchCompletion{trace: trace, call: "reuse-accepted"},
		},
	}

	result, failure := operation.ResumePrepared(context.Background(), resumeOperationRelease())

	if result != nil || failure == nil || failure.Code != "RESUME_FAILED" {
		t.Fatalf("result=%#v failure=%#v", result, failure)
	}
	if want := []string{"assess-dispatch"}; !reflect.DeepEqual(trace.calls, want) {
		t.Fatalf("calls=%#v want=%#v", trace.calls, want)
	}
}

func TestFreshResumeDispatchResolvesTokenDispatchesThenConfirms(t *testing.T) {
	trace := &resumeOperationTrace{}
	operation := requestFreshGitHubActionsResumeDispatch{
		tokens:     recordingResumeTokenResolver{trace: trace, token: "test-token"},
		dispatcher: recordingResumeWorkflowDispatcher{trace: trace, result: &GitHubActionsDispatchResult{Accepted: true, State: DispatchJournalAccepted}},
		handoff:    recordingResumeHandoffConfirmer{trace: trace},
	}

	result, failure := operation.Complete(context.Background(), resumeOperationRelease(), resumeOperationDispatch(DispatchJournalPrepared))

	if failure != nil || result == nil || result.ExecutionState != ReleaseExecutionHandoffReady {
		t.Fatalf("result=%#v failure=%#v", result, failure)
	}
	if want := []string{"resolve-token", "dispatch-workflow", "confirm-handoff"}; !reflect.DeepEqual(trace.calls, want) {
		t.Fatalf("calls=%#v want=%#v", trace.calls, want)
	}
}

func TestFreshResumeDispatchStopsAtTokenDispatchAndHandoffFailures(t *testing.T) {
	steps := []string{"resolve-token", "dispatch-workflow", "confirm-handoff"}
	for index, failed := range steps {
		t.Run(failed, func(t *testing.T) {
			trace := &resumeOperationTrace{failAt: failed}
			operation := requestFreshGitHubActionsResumeDispatch{
				tokens:     recordingResumeTokenResolver{trace: trace, token: "secret-token"},
				dispatcher: recordingResumeWorkflowDispatcher{trace: trace, result: &GitHubActionsDispatchResult{Accepted: true, State: DispatchJournalAccepted}},
				handoff:    recordingResumeHandoffConfirmer{trace: trace},
			}

			result, failure := operation.Complete(context.Background(), resumeOperationRelease(), resumeOperationDispatch(DispatchJournalPrepared))

			wantCode := "RESUME_FAILED"
			if failed == "resolve-token" {
				wantCode = "TOKEN_MISSING"
			}
			if result != nil || failure == nil || failure.Code != wantCode {
				t.Fatalf("result=%#v failure=%#v", result, failure)
			}
			if want := steps[:index+1]; !reflect.DeepEqual(trace.calls, want) {
				t.Fatalf("calls=%#v want=%#v", trace.calls, want)
			}
			if failure.Cause != nil && failure.Cause.Error() == "secret-token" {
				t.Fatal("token leaked through failure")
			}
		})
	}
}

func TestAcceptedResumeDispatchReuseConfirmsWithoutTokenOrDispatchDependencies(t *testing.T) {
	trace := &resumeOperationTrace{}
	operation := reuseAcceptedGitHubActionsResumeDispatch{handoff: recordingResumeHandoffConfirmer{trace: trace}}

	result, failure := operation.Complete(context.Background(), resumeOperationRelease(), resumeOperationDispatch(DispatchJournalAccepted))

	if failure != nil || result == nil || result.DispatchState != DispatchJournalAccepted {
		t.Fatalf("result=%#v failure=%#v", result, failure)
	}
	if want := []string{"confirm-handoff"}; !reflect.DeepEqual(trace.calls, want) {
		t.Fatalf("calls=%#v want=%#v", trace.calls, want)
	}
}

type resumeOperationTrace struct {
	failAt string
	calls  []string
}

func (trace *resumeOperationTrace) call(name string) error {
	trace.calls = append(trace.calls, name)
	if trace.failAt == name {
		return errResumeOperationDependency
	}
	return nil
}

type recordingResumePreparer struct {
	trace   *resumeOperationTrace
	release reconstructedResumeRelease
}

func (preparer recordingResumePreparer) Prepare(*resumableReleaseExecution) (reconstructedResumeRelease, *CommandFailure) {
	if err := preparer.trace.call("prepare-resume"); err != nil {
		return reconstructedResumeRelease{}, failureFromError("PREPARATION_FAILED", err)
	}
	return preparer.release, nil
}

type recordingResumeTagInspector struct {
	trace     *resumeOperationTrace
	commitSHA string
}

func (inspector recordingResumeTagInspector) TagCommit(string, string) (string, error) {
	return inspector.commitSHA, inspector.trace.call("inspect-tag")
}

type recordingResumeTagCreator struct{ trace *resumeOperationTrace }

func (creator recordingResumeTagCreator) Create(*ReleaseExecutionContext, preparedGitHubActionsReleaseExecution, string) error {
	return creator.trace.call("create-tag")
}

type recordingTagCreatedContinuation struct {
	trace  *resumeOperationTrace
	result *GitHubActionsReleaseResult
}

func (continuation recordingTagCreatedContinuation) ResumePrepared(context.Context, reconstructedResumeRelease) (*GitHubActionsReleaseResult, *CommandFailure) {
	if err := continuation.trace.call("continue-tag-created"); err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	return continuation.result, nil
}

type recordingResumeDispatchAssessor struct {
	trace    *resumeOperationTrace
	dispatch assessedGitHubActionsResumeDispatch
}

func (assessor recordingResumeDispatchAssessor) Assess(reconstructedResumeRelease) (assessedGitHubActionsResumeDispatch, error) {
	return assessor.dispatch, assessor.trace.call("assess-dispatch")
}

type recordingResumeDispatchPreparer struct{ trace *resumeOperationTrace }

func (preparer recordingResumeDispatchPreparer) Prepare(*ReleaseExecutionContext, preparedGitHubActionsReleaseExecution, validatedGitHubActionsReleasePreflight, KnownReleaseFiles, string) (preparedGitHubActionsReleaseDispatch, error) {
	return preparedGitHubActionsReleaseDispatch{}, preparer.trace.call("prepare-dispatch")
}

type recordingResumeCommitPusher struct{ trace *resumeOperationTrace }

func (pusher recordingResumeCommitPusher) Push(*ReleaseExecutionContext, preparedGitHubActionsReleaseExecution, validatedGitHubActionsReleasePreflight, string) error {
	return pusher.trace.call("push-commit")
}

type recordingResumeTagPusher struct{ trace *resumeOperationTrace }

func (pusher recordingResumeTagPusher) Push(*ReleaseExecutionContext, preparedGitHubActionsReleaseExecution, validatedGitHubActionsReleasePreflight, string) error {
	return pusher.trace.call("push-tag")
}

type recordingTagPushedContinuation struct {
	trace  *resumeOperationTrace
	result *GitHubActionsReleaseResult
}

func (continuation recordingTagPushedContinuation) ResumePrepared(context.Context, reconstructedResumeRelease) (*GitHubActionsReleaseResult, *CommandFailure) {
	if err := continuation.trace.call("continue-tag-pushed"); err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	return continuation.result, nil
}

type recordingResumeDispatchCompletion struct {
	trace  *resumeOperationTrace
	result *GitHubActionsReleaseResult
	call   string
}

func (operation recordingResumeDispatchCompletion) Complete(context.Context, reconstructedResumeRelease, assessedGitHubActionsResumeDispatch) (*GitHubActionsReleaseResult, *CommandFailure) {
	if err := operation.trace.call(operation.call); err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	return operation.result, nil
}

type recordingResumeTokenResolver struct {
	trace *resumeOperationTrace
	token string
}

func (resolver recordingResumeTokenResolver) ResolveGitHubActionsDispatchToken(context.Context) (GitHubActionsDispatchToken, error) {
	if err := resolver.trace.call("resolve-token"); err != nil {
		return GitHubActionsDispatchToken{}, err
	}
	return NewGitHubActionsDispatchToken(resolver.token)
}

type recordingResumeWorkflowDispatcher struct {
	trace  *resumeOperationTrace
	result *GitHubActionsDispatchResult
}

func (dispatcher recordingResumeWorkflowDispatcher) Dispatch(context.Context, *ReleaseExecutionContext, preparedGitHubActionsReleaseExecution, preparedGitHubActionsReleaseDispatch, GitHubActionsDispatchToken) (*GitHubActionsDispatchResult, error) {
	if err := dispatcher.trace.call("dispatch-workflow"); err != nil {
		return nil, err
	}
	return dispatcher.result, nil
}

type recordingResumeHandoffConfirmer struct{ trace *resumeOperationTrace }

func (confirmer recordingResumeHandoffConfirmer) Confirm(preparedGitHubActionsReleaseExecution) error {
	return confirmer.trace.call("confirm-handoff")
}

type resumePushBoundaryJournal struct {
	trace   *resumeOperationTrace
	state   ReleaseExecutionJournalState
	pending ReleaseExecutionPendingAction
}

func (journal *resumePushBoundaryJournal) BeginPending(_ ReleaseExecutionIdentity, action ReleaseExecutionPendingAction) (*ReleaseExecutionJournalResolution, error) {
	if err := journal.trace.call("pending:" + string(action)); err != nil {
		return nil, err
	}
	journal.pending = action
	return &ReleaseExecutionJournalResolution{}, nil
}

func (journal *resumePushBoundaryJournal) ConfirmPhase(_ ReleaseExecutionIdentity, state ReleaseExecutionJournalState, _ ReleaseExecutionJournalUpdate) (*ReleaseExecutionJournalResolution, error) {
	if err := journal.trace.call("confirm:" + string(state)); err != nil {
		return nil, err
	}
	journal.state = state
	journal.pending = ReleaseExecutionPendingNone
	return &ReleaseExecutionJournalResolution{}, nil
}

func (journal *resumePushBoundaryJournal) RecordLastError(ReleaseExecutionIdentity, string) (*ReleaseExecutionJournalResolution, error) {
	journal.trace.calls = append(journal.trace.calls, "record-error")
	return &ReleaseExecutionJournalResolution{}, nil
}

type resumePushBoundaryGit struct{ trace *resumeOperationTrace }

func (git resumePushBoundaryGit) PushCommit(*ReleaseExecutionContext, string, string, string) error {
	return git.trace.call("effect:push-commit")
}

func (git resumePushBoundaryGit) PushTag(*ReleaseExecutionContext, string, string, string) error {
	return git.trace.call("effect:push-tag")
}

func resumeOperationRelease() reconstructedResumeRelease {
	return reconstructedResumeRelease{
		Context: &ReleaseExecutionContext{
			Unit:        releaseUseCaseTestContext().Unit,
			NextVersion: "1.2.3",
			Tag:         "neko/v1.2.3",
			Workflow:    ".github/workflows/release.yml",
		},
		Execution: preparedGitHubActionsReleaseExecution{Path: "/tmp/execution.json"},
		Preflight: validatedGitHubActionsReleasePreflight{Git: GitReleasePreflight{Remote: "origin", UpstreamBranch: "main"}},
		CommitSHA: "2222222222222222222222222222222222222222",
	}
}

func resumeOperationDispatch(state DispatchJournalState) assessedGitHubActionsResumeDispatch {
	return assessedGitHubActionsResumeDispatch{
		Dispatch: preparedGitHubActionsReleaseDispatch{Path: "/tmp/dispatch.json"},
		Journal:  &DispatchJournal{State: state},
	}
}

func assertResumeOperationCallAbsent(t *testing.T, calls []string, unwanted string) {
	t.Helper()
	for _, call := range calls {
		if call == unwanted {
			t.Fatalf("calls=%#v contain later operation %q", calls, unwanted)
		}
	}
}
