package release

import "testing"

func TestResolveResumeRecoveryCoversEveryStateAndRelevantPendingAction(t *testing.T) {
	states := []struct {
		state     ReleaseExecutionJournalState
		operation resumeReleaseOperationKind
		refusal   resumeRecoveryRefusalKind
	}{
		{state: ReleaseExecutionPrepared, refusal: resumeRecoveryRefusalUnsupportedState},
		{state: ReleaseExecutionPreflightValidated, refusal: resumeRecoveryRefusalUnsupportedState},
		{state: ReleaseExecutionMaterializationApplied, refusal: resumeRecoveryRefusalUnsupportedState},
		{state: ReleaseExecutionStateWritten, refusal: resumeRecoveryRefusalUnsupportedState},
		{state: ReleaseExecutionReleaseFilesStaged, refusal: resumeRecoveryRefusalUnsupportedState},
		{state: ReleaseExecutionCommitCreated, operation: resumeReleaseFromCommitCreated},
		{state: ReleaseExecutionTagCreated, operation: resumeReleaseFromTagCreated},
		{state: ReleaseExecutionDispatchJournalPrepared, refusal: resumeRecoveryRefusalUnprovenCommitPush},
		{state: ReleaseExecutionCommitPushed, refusal: resumeRecoveryRefusalUnprovenTagPush},
		{state: ReleaseExecutionTagPushed, operation: resumeReleaseFromTagPushed},
		{state: ReleaseExecutionHandoffReady, operation: returnCompletedReleaseHandoff},
	}
	pendingActions := []ReleaseExecutionPendingAction{
		ReleaseExecutionPendingNone,
		ReleaseExecutionPendingApplyMaterialization,
		ReleaseExecutionPendingWriteState,
		ReleaseExecutionPendingStageReleaseFiles,
		ReleaseExecutionPendingCreateReleaseCommit,
		ReleaseExecutionPendingCreateUnitTag,
		ReleaseExecutionPendingCreateDispatchJournal,
		ReleaseExecutionPendingPushReleaseCommit,
		ReleaseExecutionPendingPushUnitTag,
	}
	assessment := &ReleaseExecutionRecoveryAssessment{Status: ReleaseExecutionRecoveryInterruptedAfterTag}

	for _, state := range states {
		for _, pending := range pendingActions {
			journal := &ReleaseExecutionJournal{
				State:            state.state,
				PendingAction:    pending,
				ReleaseCommitSHA: "1111111111111111111111111111111111111111",
			}

			resolution := resolveResumeRecovery(journal, assessment)

			if pending == ReleaseExecutionPendingPushReleaseCommit {
				assertResumeRecoveryRefusal(t, resolution, resumeRecoveryRefusalAmbiguousCommitPush)
				continue
			}
			if pending == ReleaseExecutionPendingPushUnitTag {
				assertResumeRecoveryRefusal(t, resolution, resumeRecoveryRefusalAmbiguousTagPush)
				continue
			}
			if state.refusal != resumeRecoveryRefusalUnknown {
				assertResumeRecoveryRefusal(t, resolution, state.refusal)
				continue
			}
			if resolution.Refusal != nil || resolution.Operation != state.operation {
				t.Fatalf("state=%s pending=%s resolution=%#v, want operation=%d", state.state, pending, resolution, state.operation)
			}
		}
	}
}

func TestResolveResumeRecoveryPrioritizesUnsafeAssessment(t *testing.T) {
	tests := []struct {
		status  ReleaseExecutionRecoveryStatus
		refusal resumeRecoveryRefusalKind
	}{
		{status: ReleaseExecutionRecoveryConflicted, refusal: resumeRecoveryRefusalConflicted},
		{status: ReleaseExecutionRecoveryCorrupted, refusal: resumeRecoveryRefusalCorrupted},
	}
	for _, test := range tests {
		resolution := resolveResumeRecovery(
			&ReleaseExecutionJournal{
				State:            ReleaseExecutionTagPushed,
				PendingAction:    ReleaseExecutionPendingPushUnitTag,
				ReleaseCommitSHA: "1111111111111111111111111111111111111111",
			},
			&ReleaseExecutionRecoveryAssessment{Status: test.status, Guidance: "manual inspection"},
		)

		assertResumeRecoveryRefusal(t, resolution, test.refusal)
		if resolution.Refusal.Guidance != "manual inspection" {
			t.Fatalf("guidance = %q", resolution.Refusal.Guidance)
		}
	}
}

func TestResolveResumeRecoveryRequiresConfirmedCommit(t *testing.T) {
	resolution := resolveResumeRecovery(
		&ReleaseExecutionJournal{State: ReleaseExecutionTagPushed, PendingAction: ReleaseExecutionPendingNone},
		&ReleaseExecutionRecoveryAssessment{Status: ReleaseExecutionRecoveryInterruptedAfterTagPush},
	)

	assertResumeRecoveryRefusal(t, resolution, resumeRecoveryRefusalBeforeCommit)
}

func TestResolveResumeDispatchSeparatesFreshAcceptedAndNoRetry(t *testing.T) {
	tests := []struct {
		name      string
		journal   *DispatchJournal
		refusal   DispatchJournalState
		operation resumeDispatchOperationKind
	}{
		{name: "missing is fresh", operation: requestFreshResumeDispatch},
		{name: "prepared is fresh", journal: &DispatchJournal{State: DispatchJournalPrepared}, operation: requestFreshResumeDispatch},
		{name: "accepted is reused", journal: &DispatchJournal{State: DispatchJournalAccepted}, operation: reuseAcceptedResumeDispatch},
		{name: "request started is uncertain", journal: &DispatchJournal{State: DispatchJournalRequestStarted}, refusal: DispatchJournalRequestStarted},
		{name: "rejected is terminal", journal: &DispatchJournal{State: DispatchJournalRejected}, refusal: DispatchJournalRejected},
		{name: "unknown is uncertain", journal: &DispatchJournal{State: DispatchJournalUnknown}, refusal: DispatchJournalUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolution := resolveResumeDispatch(test.journal)
			if test.refusal != "" {
				if resolution.Refusal == nil || resolution.Refusal.State != test.refusal || resolution.Operation != resumeDispatchOperationUnknown {
					t.Fatalf("resolution = %#v, want refusal=%s", resolution, test.refusal)
				}
				return
			}
			if resolution.Refusal != nil || resolution.Operation != test.operation {
				t.Fatalf("resolution = %#v, want operation=%d", resolution, test.operation)
			}
		})
	}
}

func assertResumeRecoveryRefusal(t *testing.T, resolution resumeRecoveryResolution, want resumeRecoveryRefusalKind) {
	t.Helper()
	if resolution.Refusal == nil || resolution.Refusal.Kind != want || resolution.Operation != resumeReleaseOperationUnknown {
		t.Fatalf("resolution = %#v, want refusal=%d", resolution, want)
	}
}
