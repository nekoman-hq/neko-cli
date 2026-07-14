package release

type resumeReleaseOperationKind uint8

const (
	resumeReleaseOperationUnknown resumeReleaseOperationKind = iota
	resumeReleaseFromCommitCreated
	resumeReleaseFromTagCreated
	resumeReleaseFromTagPushed
	returnCompletedReleaseHandoff
)

type resumeRecoveryRefusalKind uint8

const (
	resumeRecoveryRefusalUnknown resumeRecoveryRefusalKind = iota
	resumeRecoveryRefusalConflicted
	resumeRecoveryRefusalCorrupted
	resumeRecoveryRefusalAmbiguousCommitPush
	resumeRecoveryRefusalAmbiguousTagPush
	resumeRecoveryRefusalBeforeCommit
	resumeRecoveryRefusalUnprovenCommitPush
	resumeRecoveryRefusalUnprovenTagPush
	resumeRecoveryRefusalUnsupportedState
)

type resumeRecoveryRefusal struct {
	Guidance string
	State    ReleaseExecutionJournalState
	Kind     resumeRecoveryRefusalKind
}

type resumeRecoveryResolution struct {
	Refusal   *resumeRecoveryRefusal
	Operation resumeReleaseOperationKind
}

// resolveResumeRecovery is the complete pure policy for selecting one
// supported release-execution continuation. It does not execute transitions.
func resolveResumeRecovery(journal *ReleaseExecutionJournal, assessment *ReleaseExecutionRecoveryAssessment) resumeRecoveryResolution {
	if assessment == nil || assessment.Status == ReleaseExecutionRecoveryCorrupted {
		return refusedResumeRecovery(resumeRecoveryRefusalCorrupted, journalState(journal), assessmentGuidance(assessment))
	}
	if assessment.Status == ReleaseExecutionRecoveryConflicted {
		return refusedResumeRecovery(resumeRecoveryRefusalConflicted, journalState(journal), assessment.Guidance)
	}
	if journal == nil {
		return refusedResumeRecovery(resumeRecoveryRefusalCorrupted, "", assessment.Guidance)
	}
	if journal.PendingAction == ReleaseExecutionPendingPushReleaseCommit {
		return refusedResumeRecovery(resumeRecoveryRefusalAmbiguousCommitPush, journal.State, "")
	}
	if journal.PendingAction == ReleaseExecutionPendingPushUnitTag {
		return refusedResumeRecovery(resumeRecoveryRefusalAmbiguousTagPush, journal.State, "")
	}
	if journal.ReleaseCommitSHA == "" {
		return refusedResumeRecovery(resumeRecoveryRefusalBeforeCommit, journal.State, "")
	}

	switch journal.State {
	case ReleaseExecutionCommitCreated:
		return resumeRecoveryResolution{Operation: resumeReleaseFromCommitCreated}
	case ReleaseExecutionTagCreated:
		return resumeRecoveryResolution{Operation: resumeReleaseFromTagCreated}
	case ReleaseExecutionTagPushed:
		return resumeRecoveryResolution{Operation: resumeReleaseFromTagPushed}
	case ReleaseExecutionHandoffReady:
		return resumeRecoveryResolution{Operation: returnCompletedReleaseHandoff}
	case ReleaseExecutionDispatchJournalPrepared:
		return refusedResumeRecovery(resumeRecoveryRefusalUnprovenCommitPush, journal.State, "")
	case ReleaseExecutionCommitPushed:
		return refusedResumeRecovery(resumeRecoveryRefusalUnprovenTagPush, journal.State, "")
	default:
		return refusedResumeRecovery(resumeRecoveryRefusalUnsupportedState, journal.State, "")
	}
}

func refusedResumeRecovery(kind resumeRecoveryRefusalKind, state ReleaseExecutionJournalState, guidance string) resumeRecoveryResolution {
	return resumeRecoveryResolution{Refusal: &resumeRecoveryRefusal{Kind: kind, State: state, Guidance: guidance}}
}

func journalState(journal *ReleaseExecutionJournal) ReleaseExecutionJournalState {
	if journal == nil {
		return ""
	}
	return journal.State
}

func assessmentGuidance(assessment *ReleaseExecutionRecoveryAssessment) string {
	if assessment == nil {
		return ""
	}
	return assessment.Guidance
}

type resumeDispatchOperationKind uint8

const (
	resumeDispatchOperationUnknown resumeDispatchOperationKind = iota
	requestFreshResumeDispatch
	reuseAcceptedResumeDispatch
)

type resumeDispatchResolution struct {
	Refusal   *resumeDispatchRefusal
	Operation resumeDispatchOperationKind
}

type resumeDispatchRefusal struct {
	State DispatchJournalState
}

// resolveResumeDispatch is deliberately separate from release-execution
// policy. It decides only whether one immutable dispatch request is fresh,
// already accepted, or prohibited from automatic retry.
func resolveResumeDispatch(journal *DispatchJournal) resumeDispatchResolution {
	if journal == nil || journal.State == DispatchJournalPrepared {
		return resumeDispatchResolution{Operation: requestFreshResumeDispatch}
	}
	if journal.State == DispatchJournalAccepted {
		return resumeDispatchResolution{Operation: reuseAcceptedResumeDispatch}
	}
	return resumeDispatchResolution{Refusal: &resumeDispatchRefusal{State: journal.State}}
}
