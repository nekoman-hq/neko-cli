package release

import (
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

func resumeProgress(message string, args ...any) {
	log.PluginV(log.Exec, message, args...)
}

func resumeSelectedIdentity(execution *resumableExecution) string {
	if execution == nil || execution.resolution.Journal == nil {
		return "unresolved"
	}
	return lifecycleFallback(execution.resolution.Journal.Identity.SHA256, "unresolved")
}

func resumeSelectedUnit(execution *resumableExecution) string {
	if execution == nil || execution.resolution.Journal == nil {
		return "unresolved"
	}
	return lifecycleFallback(execution.resolution.Journal.UnitID, "unresolved")
}

func resumeSelectedState(execution *resumableExecution) string {
	if execution == nil || execution.resolution.Journal == nil {
		return "unresolved"
	}
	return lifecycleFallback(string(execution.resolution.Journal.State), "unresolved")
}

func resumeSelectedPendingAction(execution *resumableExecution) string {
	if execution == nil || execution.resolution.Journal == nil {
		return "unresolved"
	}
	return lifecycleFallback(string(execution.resolution.Journal.PendingAction), string(ReleaseExecutionPendingNone))
}

func resumeAssessmentStatus(assessment *ReleaseExecutionRecoveryAssessment) string {
	if assessment == nil {
		return "unresolved"
	}
	return lifecycleFallback(string(assessment.Status), "unresolved")
}

func resumeOperationName(kind resumeReleaseOperationKind) string {
	switch kind {
	case resumeReleaseFromCommitCreated:
		return "continue after release commit"
	case resumeReleaseFromTagCreated:
		return "continue after unit tag"
	case resumeReleaseFromTagPushed:
		return "continue after tag push"
	case returnCompletedReleaseHandoff:
		return "return completed handoff"
	default:
		return "unsupported continuation"
	}
}

func resumeRefusalName(refusal *resumeRecoveryRefusal) string {
	if refusal == nil {
		return "none"
	}
	switch refusal.Kind {
	case resumeRecoveryRefusalConflicted:
		return "conflicting local evidence"
	case resumeRecoveryRefusalCorrupted:
		return "corrupted recovery evidence"
	case resumeRecoveryRefusalAmbiguousCommitPush:
		return "ambiguous release commit push"
	case resumeRecoveryRefusalAmbiguousTagPush:
		return "ambiguous unit tag push"
	case resumeRecoveryRefusalBeforeCommit:
		return "release commit not confirmed"
	case resumeRecoveryRefusalUnprovenCommitPush:
		return "release commit push not proven"
	case resumeRecoveryRefusalUnprovenTagPush:
		return "unit tag push not proven"
	case resumeRecoveryRefusalUnsupportedState:
		return "unsupported journal state"
	default:
		return "unsupported recovery refusal"
	}
}

func resumeFailureCode(failure *CommandFailure) string {
	if failure == nil || strings.TrimSpace(failure.Code) == "" {
		return "unknown"
	}
	return failure.Code
}
