package release

import "testing"

func TestV1CompensationPolicySelectsOneOperationInLegacyOrder(t *testing.T) {
	evidence := newV1CompensationEvidenceFixture(t, V1CompensationExecutorGoReleaser)
	evidence.Release.Status = V1ReleaseEffectFailed
	evidence.Compensation.Actions.RestoreConfig.Status = V1CompensationActionConfirmed
	evidence.Compensation.Actions.DeleteGitHubRelease.Status = V1CompensationActionPlanned
	evidence.Compensation.Actions.DeleteLocalTag.Status = V1CompensationActionPlanned
	evidence.Compensation.Actions.DeleteRemoteTag.Status = V1CompensationActionPlanned
	evidence.Compensation.Actions.ResetReleaseCommit.Status = V1CompensationActionPlanned
	evidence.Compensation.Actions.CleanUntrackedFiles.Status = V1CompensationActionPlanned

	decision := SelectV1CompensationOperation(&evidence)

	if decision.Kind != V1CompensationPerformOperation || decision.Operation != V1CompensationDeleteGitHubRelease {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestV1CompensationPolicyHandlesMissingPreparedConfirmedAndCompletedEvidence(t *testing.T) {
	missing := SelectV1CompensationOperation(nil)
	if missing.Kind != V1CompensationNoRecoveryNeeded || missing.Reason != V1CompensationReasonNoEvidence {
		t.Fatalf("missing decision = %#v", missing)
	}

	preparedEvidence := newV1CompensationEvidenceFixture(t, V1CompensationExecutorGoReleaser)
	prepared := SelectV1CompensationOperation(&preparedEvidence)
	if prepared.Kind != V1CompensationNoRecoveryNeeded || prepared.Reason != V1CompensationReasonNoUnsafeEffect {
		t.Fatalf("prepared decision = %#v", prepared)
	}

	confirmedEvidence := newV1CompensationEvidenceFixture(t, V1CompensationExecutorGoReleaser)
	confirmedEvidence.Release.Status = V1ReleaseEffectFailed
	confirmed := SelectV1CompensationOperation(&confirmedEvidence)
	if confirmed.Kind != V1CompensationMarkComplete || confirmed.Reason != V1CompensationReasonAllActionsConfirmed {
		t.Fatalf("confirmed decision = %#v", confirmed)
	}

	completedEvidence := newV1CompensationEvidenceFixture(t, V1CompensationExecutorGoReleaser)
	completedEvidence.Compensation.Status = V1CompensationCompleted
	completed := SelectV1CompensationOperation(&completedEvidence)
	if completed.Kind != V1CompensationAlreadyComplete {
		t.Fatalf("completed decision = %#v", completed)
	}
}

func TestV1CompensationPolicyRetriesRepeatableLocalPendingAndFailedActions(t *testing.T) {
	pending := newV1CompensationEvidenceFixture(t, V1CompensationExecutorJReleaser)
	pending.Release.Status = V1ReleaseEffectFailed
	pending.Compensation.Status = V1CompensationInProgress
	pending.Compensation.PendingAction = V1CompensationRestoreConfig
	pending.Compensation.Actions.RestoreConfig.Status = V1CompensationActionPending
	decision := SelectV1CompensationOperation(&pending)
	if decision.Kind != V1CompensationPerformOperation || decision.Operation != V1CompensationRestoreConfig {
		t.Fatalf("pending local decision = %#v", decision)
	}

	failed := newV1CompensationEvidenceFixture(t, V1CompensationExecutorJReleaser)
	failed.Release.Status = V1ReleaseEffectFailed
	failed.Compensation.Actions.DeleteLocalTag.Status = V1CompensationActionFailed
	decision = SelectV1CompensationOperation(&failed)
	if decision.Kind != V1CompensationPerformOperation || decision.Operation != V1CompensationDeleteLocalTag {
		t.Fatalf("failed local decision = %#v", decision)
	}
}

func TestV1CompensationPolicyRefusesPendingOrUncertainExternalEffects(t *testing.T) {
	pending := newV1CompensationEvidenceFixture(t, V1CompensationExecutorReleaseIt)
	pending.Release.Status = V1ReleaseEffectFailed
	pending.Compensation.Status = V1CompensationInProgress
	pending.Compensation.PendingAction = V1CompensationDeleteRemoteTag
	pending.Compensation.Actions.DeleteRemoteTag.Status = V1CompensationActionPending
	decision := SelectV1CompensationOperation(&pending)
	if decision.Kind != V1CompensationRequireManual || decision.Reason != V1CompensationReasonPendingExternal {
		t.Fatalf("pending external decision = %#v", decision)
	}

	uncertain := newV1CompensationEvidenceFixture(t, V1CompensationExecutorReleaseIt)
	uncertain.Release.Status = V1ReleaseEffectFailed
	uncertain.Compensation.Actions.DeleteGitHubRelease.Status = V1CompensationActionUncertain
	decision = SelectV1CompensationOperation(&uncertain)
	if decision.Kind != V1CompensationRequireManual || decision.Reason != V1CompensationReasonUncertainAction {
		t.Fatalf("uncertain external decision = %#v", decision)
	}
}

func TestV1CompensationPolicyRefusesPendingRevertAndUncertainExecutorRuns(t *testing.T) {
	pendingRevert := newV1CompensationEvidenceFixture(t, V1CompensationExecutorGoReleaser)
	pendingRevert.Release.Status = V1ReleaseEffectFailed
	pendingRevert.Compensation.Status = V1CompensationInProgress
	pendingRevert.Compensation.PendingAction = V1CompensationRevertReleaseCommit
	pendingRevert.Compensation.Actions.RevertReleaseCommit.Status = V1CompensationActionPending
	decision := SelectV1CompensationOperation(&pendingRevert)
	if decision.Kind != V1CompensationRequireManual || decision.Reason != V1CompensationReasonPendingNonRepeatable {
		t.Fatalf("pending revert decision = %#v", decision)
	}

	for _, executor := range []V1CompensationExecutor{
		V1CompensationExecutorGoReleaser,
		V1CompensationExecutorJReleaser,
		V1CompensationExecutorReleaseIt,
	} {
		t.Run(string(executor), func(t *testing.T) {
			evidence := newV1CompensationEvidenceFixture(t, executor)
			evidence.Release.Status = V1ReleaseEffectUncertain
			decision := SelectV1CompensationOperation(&evidence)
			if decision.Kind != V1CompensationRequireManual || decision.Reason != V1CompensationReasonUncertainExecution {
				t.Fatalf("uncertain execution decision = %#v", decision)
			}
		})
	}
}

func TestV1CompensationPolicyRejectsCorruptEvidence(t *testing.T) {
	evidence := newV1CompensationEvidenceFixture(t, V1CompensationExecutorGoReleaser)
	evidence.Compensation.Actions.CleanUntrackedFiles.Status = "mystery"

	decision := SelectV1CompensationOperation(&evidence)

	if decision.Kind != V1CompensationRequireManual || decision.Reason != V1CompensationReasonInvalidEvidence {
		t.Fatalf("corrupt evidence decision = %#v", decision)
	}
}

func TestV1ExecutorFailureClassificationPreservesExecutorSpecificBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		executor V1CompensationExecutor
		want     V1ExecutorFailureClassification
		state    GitReleaseState
	}{
		{
			name:     "goreleaser local commit failure",
			executor: V1CompensationExecutorGoReleaser,
			state:    GitReleaseState{PreHead: "before"},
			want:     V1ExecutorFailureKnownEffects,
		},
		{
			name:     "goreleaser local tag failure",
			executor: V1CompensationExecutorGoReleaser,
			state:    GitReleaseState{PreHead: "before", ReleaseHead: "release"},
			want:     V1ExecutorFailureKnownEffects,
		},
		{
			name:     "goreleaser commit push failure",
			executor: V1CompensationExecutorGoReleaser,
			state:    GitReleaseState{ReleaseHead: "release", TagName: "v1.2.4"},
			want:     V1ExecutorFailureExternalUncertainty,
		},
		{
			name:     "jreleaser local commit failure",
			executor: V1CompensationExecutorJReleaser,
			state:    GitReleaseState{PreHead: "before"},
			want:     V1ExecutorFailureKnownEffects,
		},
		{
			name:     "jreleaser commit push failure",
			executor: V1CompensationExecutorJReleaser,
			state:    GitReleaseState{PreHead: "before", ReleaseHead: "release"},
			want:     V1ExecutorFailureExternalUncertainty,
		},
		{
			name:     "release-it process failure",
			executor: V1CompensationExecutorReleaseIt,
			state:    GitReleaseState{PreHead: "before"},
			want:     V1ExecutorFailureExternalUncertainty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyV1ExecutorFailure(tt.executor, tt.state); got != tt.want {
				t.Fatalf("ClassifyV1ExecutorFailure = %q, want %q", got, tt.want)
			}
		})
	}
}
