package release

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestGitHubActionsReleaseMutationOperationsPersistAroundSideEffects(t *testing.T) {
	for _, test := range githubActionsReleaseMutationOperationCases() {
		t.Run(test.name, func(t *testing.T) {
			trace := &releaseOperationTrace{}
			journal := &recordingReleaseOperationJournal{trace: trace, state: test.previous, pending: ReleaseExecutionPendingNone}
			effects := &recordingReleaseOperationEffects{trace: trace}
			if err := test.run(journal, effects); err != nil {
				t.Fatalf("run operation: %v", err)
			}
			want := []string{
				"pending:" + string(test.pending),
				"effect:" + test.effect,
				"confirm:" + string(test.confirmed),
			}
			if !reflect.DeepEqual(trace.calls, want) {
				t.Fatalf("calls = %#v, want %#v", trace.calls, want)
			}
			if journal.state != test.confirmed || journal.pending != ReleaseExecutionPendingNone {
				t.Fatalf("journal state=%s pending=%s, want state=%s pending=%s", journal.state, journal.pending, test.confirmed, ReleaseExecutionPendingNone)
			}
		})
	}
}

func TestGitHubActionsReleaseMutationOperationsStopWhenPendingCannotPersist(t *testing.T) {
	for _, test := range githubActionsReleaseMutationOperationCases() {
		t.Run(test.name, func(t *testing.T) {
			trace := &releaseOperationTrace{}
			journal := &recordingReleaseOperationJournal{trace: trace, state: test.previous, pending: ReleaseExecutionPendingNone, failPending: true}
			effects := &recordingReleaseOperationEffects{trace: trace}
			if err := test.run(journal, effects); !errors.Is(err, errReleaseOperationBoundary) {
				t.Fatalf("error = %v, want pending failure", err)
			}
			assertReleaseOperationTraceContains(t, trace.calls, "pending:"+string(test.pending))
			assertReleaseOperationTraceOmits(t, trace.calls, "effect:"+test.effect)
			assertReleaseOperationTraceOmitsPrefix(t, trace.calls, "confirm:")
			switch test.name {
			case "write-state":
				assertReleaseOperationTraceOrder(t, trace.calls, "pending:"+string(test.pending), "cleanup:materialization", "record-error")
			case "stage-files", "create-commit":
				assertReleaseOperationTraceOrder(t, trace.calls, "pending:"+string(test.pending), "cleanup:state", "cleanup:materialization", "cleanup:unstage-files", "record-error")
			}
			if journal.state != test.previous || journal.pending != ReleaseExecutionPendingNone {
				t.Fatalf("journal changed after pending persistence failure: state=%s pending=%s", journal.state, journal.pending)
			}
		})
	}
}

func TestGitHubActionsReleaseMutationOperationsLeavePendingWhenSideEffectFails(t *testing.T) {
	for _, test := range githubActionsReleaseMutationOperationCases() {
		t.Run(test.name, func(t *testing.T) {
			trace := &releaseOperationTrace{}
			journal := &recordingReleaseOperationJournal{trace: trace, state: test.previous, pending: ReleaseExecutionPendingNone}
			effects := &recordingReleaseOperationEffects{trace: trace, failAt: test.effect}
			if err := test.run(journal, effects); !errors.Is(err, errReleaseOperationBoundary) {
				t.Fatalf("error = %v, want side-effect failure", err)
			}
			assertReleaseOperationTraceOrder(t, trace.calls,
				"pending:"+string(test.pending),
				"effect:"+test.effect,
			)
			assertReleaseOperationTraceOmitsPrefix(t, trace.calls, "confirm:")
			switch test.name {
			case "write-state":
				assertReleaseOperationTraceOrder(t, trace.calls, "effect:write-state", "cleanup:state", "cleanup:materialization", "record-error")
			case "stage-files":
				assertReleaseOperationTraceOrder(t, trace.calls, "effect:stage-files", "cleanup:state", "cleanup:materialization", "cleanup:unstage-files", "record-error")
			}
			if journal.state != test.previous || journal.pending != test.pending {
				t.Fatalf("journal state=%s pending=%s, want state=%s pending=%s", journal.state, journal.pending, test.previous, test.pending)
			}
		})
	}
}

func TestGitHubActionsReleaseMutationOperationsExposeConfirmationFailures(t *testing.T) {
	for _, test := range githubActionsReleaseMutationOperationCases() {
		t.Run(test.name, func(t *testing.T) {
			trace := &releaseOperationTrace{}
			journal := &recordingReleaseOperationJournal{trace: trace, state: test.previous, pending: ReleaseExecutionPendingNone, failConfirm: true}
			effects := &recordingReleaseOperationEffects{trace: trace}
			if err := test.run(journal, effects); !errors.Is(err, errReleaseOperationBoundary) {
				t.Fatalf("error = %v, want confirmation failure", err)
			}
			assertReleaseOperationTraceOrder(t, trace.calls,
				"pending:"+string(test.pending),
				"effect:"+test.effect,
				"confirm:"+string(test.confirmed),
			)
			switch test.name {
			case "apply-materialization":
				assertReleaseOperationTraceOrder(t, trace.calls, "confirm:"+string(test.confirmed), "cleanup:materialization", "record-error")
			case "write-state":
				assertReleaseOperationTraceOrder(t, trace.calls, "confirm:"+string(test.confirmed), "cleanup:state", "cleanup:materialization", "record-error")
			case "stage-files":
				assertReleaseOperationTraceOrder(t, trace.calls, "confirm:"+string(test.confirmed), "cleanup:state", "cleanup:materialization", "cleanup:unstage-files", "record-error")
			}
			if journal.state != test.previous || journal.pending != test.pending {
				t.Fatalf("journal state=%s pending=%s, want state=%s pending=%s", journal.state, journal.pending, test.previous, test.pending)
			}
		})
	}
}

func TestPreCommitRestorationReportsCleanupFailuresWithoutLosingCause(t *testing.T) {
	cause := errors.New("stage failed")
	stateErr := errors.New("state restore failed")
	materializationErr := errors.New("materialization restore failed")
	unstageErr := errors.New("unstage failed")
	failure := restoreStagedReleaseFilesAfterFailure(
		cause,
		failingOperationStateRollback{err: stateErr},
		failingOperationMaterializationRollback{err: materializationErr},
		failingOperationUnstager{err: unstageErr},
		KnownReleaseFiles{},
	)
	for _, expected := range []error{cause, stateErr, materializationErr, unstageErr} {
		if !errors.Is(failure, expected) {
			t.Fatalf("failure %q lost cause %q", failure, expected)
		}
	}
}

type failingOperationStateRollback struct{ err error }

func (rollback failingOperationStateRollback) RestoreSnapshot() error { return rollback.err }

type failingOperationMaterializationRollback struct{ err error }

func (rollback failingOperationMaterializationRollback) Restore() error { return rollback.err }

type failingOperationUnstager struct{ err error }

func (unstager failingOperationUnstager) UnstageKnown(KnownReleaseFiles) error { return unstager.err }

type githubActionsReleaseMutationOperationCase struct {
	run       func(journal *recordingReleaseOperationJournal, effects *recordingReleaseOperationEffects) error
	name      string
	pending   ReleaseExecutionPendingAction
	effect    string
	previous  ReleaseExecutionJournalState
	confirmed ReleaseExecutionJournalState
}

func githubActionsReleaseMutationOperationCases() []githubActionsReleaseMutationOperationCase {
	execCtx := releaseUseCaseTestContext()
	execution := preparedGitHubActionsReleaseExecution{Identity: ReleaseExecutionIdentity{SHA256: "execution"}}
	preflight := validatedGitHubActionsReleasePreflight{Git: GitReleasePreflight{Remote: "origin", UpstreamBranch: "main"}}
	files := KnownReleaseFiles{}
	commitSHA := "2222222222222222222222222222222222222222"
	return []githubActionsReleaseMutationOperationCase{
		{
			name:      "apply-materialization",
			pending:   ReleaseExecutionPendingApplyMaterialization,
			effect:    "apply-materialization",
			previous:  ReleaseExecutionPreflightValidated,
			confirmed: ReleaseExecutionMaterializationApplied,
			run: func(journal *recordingReleaseOperationJournal, effects *recordingReleaseOperationEffects) error {
				_, err := (applyGitHubActionsReleaseMaterialization{
					journal:      journal,
					transactions: recordingMaterializationTransactionFactory{effects},
				}).Apply(execution, &MaterializationPlan{})
				return err
			},
		},
		{
			name:      "write-state",
			pending:   ReleaseExecutionPendingWriteState,
			effect:    "write-state",
			previous:  ReleaseExecutionMaterializationApplied,
			confirmed: ReleaseExecutionStateWritten,
			run: func(journal *recordingReleaseOperationJournal, effects *recordingReleaseOperationEffects) error {
				_, err := (writeGitHubActionsReleaseState{
					journal:      journal,
					transactions: recordingStateTransactionFactory{effects},
				}).Write(execCtx, execution, recordingOperationMaterializationRollback{effects.trace})
				return err
			},
		},
		{
			name:      "stage-files",
			pending:   ReleaseExecutionPendingStageReleaseFiles,
			effect:    "stage-files",
			previous:  ReleaseExecutionStateWritten,
			confirmed: ReleaseExecutionReleaseFilesStaged,
			run: func(journal *recordingReleaseOperationJournal, effects *recordingReleaseOperationEffects) error {
				return (stageGitHubActionsReleaseFiles{journal: journal, git: recordingReleaseOperationGit{effects}}).Stage(execCtx, execution, files, recordingOperationStateRollback{effects.trace}, recordingOperationMaterializationRollback{effects.trace})
			},
		},
		{
			name:      "create-commit",
			pending:   ReleaseExecutionPendingCreateReleaseCommit,
			effect:    "create-commit",
			previous:  ReleaseExecutionReleaseFilesStaged,
			confirmed: ReleaseExecutionCommitCreated,
			run: func(journal *recordingReleaseOperationJournal, effects *recordingReleaseOperationEffects) error {
				_, err := (createGitHubActionsReleaseCommit{journal: journal, git: recordingReleaseOperationGit{effects}}).Create(execCtx, execution, files, recordingOperationStateRollback{effects.trace}, recordingOperationMaterializationRollback{effects.trace})
				return err
			},
		},
		{
			name:      "create-tag",
			pending:   ReleaseExecutionPendingCreateUnitTag,
			effect:    "create-tag",
			previous:  ReleaseExecutionCommitCreated,
			confirmed: ReleaseExecutionTagCreated,
			run: func(journal *recordingReleaseOperationJournal, effects *recordingReleaseOperationEffects) error {
				return (createGitHubActionsReleaseTag{journal: journal, git: recordingReleaseOperationGit{effects}}).Create(execCtx, execution, commitSHA)
			},
		},
		{
			name:      "prepare-dispatch-journal",
			pending:   ReleaseExecutionPendingCreateDispatchJournal,
			effect:    "prepare-dispatch-journal",
			previous:  ReleaseExecutionTagCreated,
			confirmed: ReleaseExecutionDispatchJournalPrepared,
			run: func(journal *recordingReleaseOperationJournal, effects *recordingReleaseOperationEffects) error {
				_, err := (prepareGitHubActionsReleaseDispatch{
					journal:  journal,
					dispatch: recordingDispatchJournalPreparation{effects},
					requests: recordingReleaseDispatchRequestBuilder{},
				}).Prepare(execCtx, execution, preflight, files, commitSHA)
				return err
			},
		},
		{
			name:      "push-commit",
			pending:   ReleaseExecutionPendingPushReleaseCommit,
			effect:    "push-commit",
			previous:  ReleaseExecutionDispatchJournalPrepared,
			confirmed: ReleaseExecutionCommitPushed,
			run: func(journal *recordingReleaseOperationJournal, effects *recordingReleaseOperationEffects) error {
				return (pushGitHubActionsReleaseCommit{journal: journal, git: recordingReleaseOperationGit{effects}}).Push(execCtx, execution, preflight, commitSHA)
			},
		},
		{
			name:      "push-tag",
			pending:   ReleaseExecutionPendingPushUnitTag,
			effect:    "push-tag",
			previous:  ReleaseExecutionCommitPushed,
			confirmed: ReleaseExecutionTagPushed,
			run: func(journal *recordingReleaseOperationJournal, effects *recordingReleaseOperationEffects) error {
				return (pushGitHubActionsReleaseTag{journal: journal, git: recordingReleaseOperationGit{effects}}).Push(execCtx, execution, preflight, commitSHA)
			},
		},
	}
}

var errReleaseOperationBoundary = errors.New("release operation boundary failed")

type releaseOperationTrace struct {
	calls []string
}

func (trace *releaseOperationTrace) add(call string) {
	trace.calls = append(trace.calls, call)
}

type recordingReleaseOperationJournal struct {
	trace       *releaseOperationTrace
	state       ReleaseExecutionJournalState
	pending     ReleaseExecutionPendingAction
	failPending bool
	failConfirm bool
}

func (journal *recordingReleaseOperationJournal) BeginPending(_ ReleaseExecutionIdentity, action ReleaseExecutionPendingAction) (*ReleaseExecutionJournalResolution, error) {
	journal.trace.add("pending:" + string(action))
	if journal.failPending {
		return nil, errReleaseOperationBoundary
	}
	journal.pending = action
	return &ReleaseExecutionJournalResolution{}, nil
}

func (journal *recordingReleaseOperationJournal) ConfirmPhase(_ ReleaseExecutionIdentity, state ReleaseExecutionJournalState, _ ReleaseExecutionJournalUpdate) (*ReleaseExecutionJournalResolution, error) {
	journal.trace.add("confirm:" + string(state))
	if journal.failConfirm {
		return nil, errReleaseOperationBoundary
	}
	journal.state = state
	journal.pending = ReleaseExecutionPendingNone
	return &ReleaseExecutionJournalResolution{}, nil
}

func (journal *recordingReleaseOperationJournal) RecordLastError(ReleaseExecutionIdentity, string) (*ReleaseExecutionJournalResolution, error) {
	journal.trace.add("record-error")
	return &ReleaseExecutionJournalResolution{}, nil
}

type recordingReleaseOperationEffects struct {
	trace  *releaseOperationTrace
	failAt string
}

func (effects *recordingReleaseOperationEffects) run(name string) error {
	effects.trace.add("effect:" + name)
	if effects.failAt == name {
		return errReleaseOperationBoundary
	}
	return nil
}

type recordingMaterializationTransactionFactory struct {
	effects *recordingReleaseOperationEffects
}

func (factory recordingMaterializationTransactionFactory) New(*MaterializationPlan) releaseMaterializationTransaction {
	return recordingMaterializationTransaction(factory)
}

type recordingMaterializationTransaction struct {
	effects *recordingReleaseOperationEffects
}

func (recordingMaterializationTransaction) CaptureSnapshots() error { return nil }

func (transaction recordingMaterializationTransaction) Apply() (*AppliedMaterialization, error) {
	return &AppliedMaterialization{}, transaction.effects.run("apply-materialization")
}

func (transaction recordingMaterializationTransaction) Restore() error {
	transaction.effects.trace.add("cleanup:materialization")
	return nil
}

type recordingOperationMaterializationRollback struct{ trace *releaseOperationTrace }

func (rollback recordingOperationMaterializationRollback) Restore() error {
	rollback.trace.add("cleanup:materialization")
	return nil
}

type recordingStateTransactionFactory struct {
	effects *recordingReleaseOperationEffects
}

func (factory recordingStateTransactionFactory) New(string) releaseStateTransaction {
	return recordingStateTransaction(factory)
}

type recordingStateTransaction struct {
	effects *recordingReleaseOperationEffects
}

func (recordingStateTransaction) CaptureSnapshot() error { return nil }

func (transaction recordingStateTransaction) WriteUnitVersion(string, string) error {
	return transaction.effects.run("write-state")
}

func (transaction recordingStateTransaction) RestoreSnapshot() error {
	transaction.effects.trace.add("cleanup:state")
	return nil
}

type recordingOperationStateRollback struct{ trace *releaseOperationTrace }

func (rollback recordingOperationStateRollback) RestoreSnapshot() error {
	rollback.trace.add("cleanup:state")
	return nil
}

type recordingReleaseOperationGit struct {
	effects *recordingReleaseOperationEffects
}

func (git recordingReleaseOperationGit) Stage(*ReleaseExecutionContext, KnownReleaseFiles) error {
	return git.effects.run("stage-files")
}

func (git recordingReleaseOperationGit) UnstageKnown(KnownReleaseFiles) error {
	git.effects.trace.add("cleanup:unstage-files")
	return nil
}

func (git recordingReleaseOperationGit) Commit(*ReleaseExecutionContext, KnownReleaseFiles) (string, error) {
	if err := git.effects.run("create-commit"); err != nil {
		return "", err
	}
	return "2222222222222222222222222222222222222222", nil
}

func (git recordingReleaseOperationGit) CreateTag(*ReleaseExecutionContext, string) (bool, error) {
	return true, git.effects.run("create-tag")
}

func (git recordingReleaseOperationGit) PushCommit(*ReleaseExecutionContext, string, string, string) error {
	return git.effects.run("push-commit")
}

func (git recordingReleaseOperationGit) PushTag(*ReleaseExecutionContext, string, string, string) error {
	return git.effects.run("push-tag")
}

type recordingDispatchJournalPreparation struct {
	effects *recordingReleaseOperationEffects
}

func (store recordingDispatchJournalPreparation) Prepare(*ReleaseDispatchRequest) (*DispatchJournalResolution, error) {
	if err := store.effects.run("prepare-dispatch-journal"); err != nil {
		return nil, err
	}
	return &DispatchJournalResolution{Path: "/tmp/dispatch-journal.json"}, nil
}

type recordingReleaseDispatchRequestBuilder struct{}

func (recordingReleaseDispatchRequestBuilder) Build(*ReleaseExecutionContext, *GitReleaseResult) (*ReleaseDispatchRequest, error) {
	return &ReleaseDispatchRequest{Identity: ReleaseDispatchIdentity{SHA256: "dispatch"}}, nil
}

func assertReleaseOperationTraceContains(t *testing.T, calls []string, want string) {
	t.Helper()
	for _, call := range calls {
		if call == want {
			return
		}
	}
	t.Fatalf("calls %#v do not contain %q", calls, want)
}

func assertReleaseOperationTraceOmits(t *testing.T, calls []string, unwanted string) {
	t.Helper()
	for _, call := range calls {
		if call == unwanted {
			t.Fatalf("calls %#v contain %q", calls, unwanted)
		}
	}
}

func assertReleaseOperationTraceOmitsPrefix(t *testing.T, calls []string, prefix string) {
	t.Helper()
	for _, call := range calls {
		if strings.HasPrefix(call, prefix) {
			t.Fatalf("calls %#v contain prefix %q", calls, prefix)
		}
	}
}

func assertReleaseOperationTraceOrder(t *testing.T, calls []string, ordered ...string) {
	t.Helper()
	next := 0
	for _, call := range calls {
		if next < len(ordered) && call == ordered[next] {
			next++
		}
	}
	if next != len(ordered) {
		t.Fatalf("calls %#v do not contain ordered sequence %#v", calls, ordered)
	}
}
