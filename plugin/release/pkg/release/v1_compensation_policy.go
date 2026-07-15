package release

type V1CompensationDecisionKind string

const (
	V1CompensationNoRecoveryNeeded V1CompensationDecisionKind = "no-recovery-needed"
	V1CompensationPerformOperation V1CompensationDecisionKind = "perform-operation"
	V1CompensationRequireManual    V1CompensationDecisionKind = "require-manual-recovery"
	V1CompensationAlreadyComplete  V1CompensationDecisionKind = "already-complete"
	V1CompensationMarkComplete     V1CompensationDecisionKind = "mark-complete"
)

type V1CompensationDecisionReason string

const (
	V1CompensationReasonNoEvidence           V1CompensationDecisionReason = "no-evidence"
	V1CompensationReasonNoUnsafeEffect       V1CompensationDecisionReason = "no-unsafe-effect"
	V1CompensationReasonActionReady          V1CompensationDecisionReason = "action-ready"
	V1CompensationReasonPendingExternal      V1CompensationDecisionReason = "pending-external-action"
	V1CompensationReasonPendingNonRepeatable V1CompensationDecisionReason = "pending-non-repeatable-action"
	V1CompensationReasonUncertainExecution   V1CompensationDecisionReason = "uncertain-release-execution"
	V1CompensationReasonUncertainAction      V1CompensationDecisionReason = "uncertain-compensation-action"
	V1CompensationReasonInvalidEvidence      V1CompensationDecisionReason = "invalid-evidence"
	V1CompensationReasonCompleted            V1CompensationDecisionReason = "compensation-completed"
	V1CompensationReasonAllActionsConfirmed  V1CompensationDecisionReason = "all-actions-confirmed"
)

type V1CompensationDecision struct {
	Kind      V1CompensationDecisionKind
	Operation V1CompensationAction
	Reason    V1CompensationDecisionReason
}

type V1ExecutorFailureClassification string

const (
	V1ExecutorFailureKnownEffects        V1ExecutorFailureClassification = "known-effects"
	V1ExecutorFailureExternalUncertainty V1ExecutorFailureClassification = "external-outcome-uncertain"
)

func ClassifyV1ExecutorFailure(executor V1CompensationExecutor, state GitReleaseState) V1ExecutorFailureClassification {
	switch executor {
	case V1CompensationExecutorGoReleaser:
		if state.CreatedGitHubRelease || (state.TagName != "" && !state.PushedCommit) ||
			(state.PushedCommit && !state.PushedTag) || (state.PushedTag && !state.CreatedGitHubRelease) {
			return V1ExecutorFailureExternalUncertainty
		}
		return V1ExecutorFailureKnownEffects
	case V1CompensationExecutorJReleaser:
		if (state.ReleaseHead != "" && !state.PushedCommit) || state.PushedCommit || state.CreatedGitHubRelease {
			return V1ExecutorFailureExternalUncertainty
		}
		return V1ExecutorFailureKnownEffects
	case V1CompensationExecutorReleaseIt:
		return V1ExecutorFailureExternalUncertainty
	default:
		return V1ExecutorFailureExternalUncertainty
	}
}

// SelectV1CompensationOperation is the side-effect-free V1 recovery policy.
// It returns exactly one named operation or a typed refusal.
func SelectV1CompensationOperation(evidence *V1CompensationEvidence) V1CompensationDecision {
	if evidence == nil {
		return V1CompensationDecision{Kind: V1CompensationNoRecoveryNeeded, Operation: V1CompensationActionNone, Reason: V1CompensationReasonNoEvidence}
	}
	if err := evidence.Validate(); err != nil {
		return V1CompensationDecision{Kind: V1CompensationRequireManual, Operation: V1CompensationActionNone, Reason: V1CompensationReasonInvalidEvidence}
	}
	if evidence.Compensation.Status == V1CompensationCompleted || evidence.Release.Status == V1ReleaseEffectSucceeded {
		return V1CompensationDecision{Kind: V1CompensationAlreadyComplete, Operation: V1CompensationActionNone, Reason: V1CompensationReasonCompleted}
	}
	if evidence.Compensation.Status == V1CompensationManualRecovery {
		return V1CompensationDecision{Kind: V1CompensationRequireManual, Operation: V1CompensationActionNone, Reason: V1CompensationReasonUncertainAction}
	}

	if pending := evidence.Compensation.PendingAction; pending != V1CompensationActionNone {
		if pending.isExternal() {
			return V1CompensationDecision{Kind: V1CompensationRequireManual, Operation: V1CompensationActionNone, Reason: V1CompensationReasonPendingExternal}
		}
		if pending == V1CompensationRevertReleaseCommit {
			return V1CompensationDecision{Kind: V1CompensationRequireManual, Operation: V1CompensationActionNone, Reason: V1CompensationReasonPendingNonRepeatable}
		}
		return V1CompensationDecision{Kind: V1CompensationPerformOperation, Operation: pending, Reason: V1CompensationReasonActionReady}
	}

	if decision := selectV1CompensationAction(V1CompensationRestoreConfig, evidence.Compensation.Actions.RestoreConfig.Status); decision.Kind != "" {
		return decision
	}
	if decision := selectV1CompensationAction(V1CompensationDeleteGitHubRelease, evidence.Compensation.Actions.DeleteGitHubRelease.Status); decision.Kind != "" {
		return decision
	}
	if decision := selectV1CompensationAction(V1CompensationDeleteLocalTag, evidence.Compensation.Actions.DeleteLocalTag.Status); decision.Kind != "" {
		return decision
	}
	if decision := selectV1CompensationAction(V1CompensationDeleteRemoteTag, evidence.Compensation.Actions.DeleteRemoteTag.Status); decision.Kind != "" {
		return decision
	}
	if decision := selectV1CompensationAction(V1CompensationRevertReleaseCommit, evidence.Compensation.Actions.RevertReleaseCommit.Status); decision.Kind != "" {
		return decision
	}
	if decision := selectV1CompensationAction(V1CompensationPushRevertCommit, evidence.Compensation.Actions.PushRevertCommit.Status); decision.Kind != "" {
		return decision
	}
	if decision := selectV1CompensationAction(V1CompensationResetReleaseCommit, evidence.Compensation.Actions.ResetReleaseCommit.Status); decision.Kind != "" {
		return decision
	}
	if decision := selectV1CompensationAction(V1CompensationCleanUntrackedReleaseFiles, evidence.Compensation.Actions.CleanUntrackedFiles.Status); decision.Kind != "" {
		return decision
	}

	switch evidence.Release.Status {
	case V1ReleaseEffectPrepared:
		return V1CompensationDecision{Kind: V1CompensationNoRecoveryNeeded, Operation: V1CompensationActionNone, Reason: V1CompensationReasonNoUnsafeEffect}
	case V1ReleaseEffectPending, V1ReleaseEffectUncertain:
		return V1CompensationDecision{Kind: V1CompensationRequireManual, Operation: V1CompensationActionNone, Reason: V1CompensationReasonUncertainExecution}
	case V1ReleaseEffectFailed:
		return V1CompensationDecision{Kind: V1CompensationMarkComplete, Operation: V1CompensationActionNone, Reason: V1CompensationReasonAllActionsConfirmed}
	default:
		return V1CompensationDecision{Kind: V1CompensationRequireManual, Operation: V1CompensationActionNone, Reason: V1CompensationReasonInvalidEvidence}
	}
}

func selectV1CompensationAction(action V1CompensationAction, status V1CompensationActionStatus) V1CompensationDecision {
	switch status {
	case V1CompensationActionPlanned:
		return V1CompensationDecision{Kind: V1CompensationPerformOperation, Operation: action, Reason: V1CompensationReasonActionReady}
	case V1CompensationActionFailed:
		if action.isExternal() || action == V1CompensationRevertReleaseCommit {
			return V1CompensationDecision{Kind: V1CompensationRequireManual, Operation: V1CompensationActionNone, Reason: V1CompensationReasonUncertainAction}
		}
		return V1CompensationDecision{Kind: V1CompensationPerformOperation, Operation: action, Reason: V1CompensationReasonActionReady}
	case V1CompensationActionUncertain:
		return V1CompensationDecision{Kind: V1CompensationRequireManual, Operation: V1CompensationActionNone, Reason: V1CompensationReasonUncertainAction}
	default:
		return V1CompensationDecision{}
	}
}

func (action V1CompensationAction) isExternal() bool {
	switch action {
	case V1CompensationDeleteGitHubRelease, V1CompensationDeleteRemoteTag, V1CompensationPushRevertCommit:
		return true
	default:
		return false
	}
}
