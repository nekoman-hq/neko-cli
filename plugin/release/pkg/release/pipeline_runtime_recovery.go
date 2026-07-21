package release

import "github.com/nekoman-hq/neko-cli/plugin/release/internal/pipelineinspection"

func observePipelineRecovery(repositoryRoot string, journal *ReleaseExecutionJournal) pipelineinspection.RuntimeRecoveryObservation {
	assessment, err := AssessReleaseExecutionRecovery(
		repositoryRoot, journal,
		resumeGitAdapter{coordinator: NewGitReleaseCoordinator()},
	)
	if err != nil || assessment == nil {
		return pipelineinspection.RuntimeRecoveryObservation{
			Evaluated: true, Invalid: true, ManualIntervention: true,
			Classification: "corrupted", Guidance: "Local recovery evidence could not be assessed safely.",
		}
	}
	resolution := resolveResumeRecovery(journal, assessment)
	operation := pipelineResumeOperationName(resolution.Operation)
	refusal := ""
	if resolution.Refusal != nil {
		refusal = pipelineResumeRefusalName(resolution.Refusal.Kind)
	}
	resumeEligible := resolution.Refusal == nil &&
		(resolution.Operation == resumeReleaseFromCommitCreated ||
			resolution.Operation == resumeReleaseFromTagCreated ||
			resolution.Operation == resumeReleaseFromTagPushed)
	uncertain := resolution.Refusal != nil &&
		(resolution.Refusal.Kind == resumeRecoveryRefusalAmbiguousCommitPush ||
			resolution.Refusal.Kind == resumeRecoveryRefusalAmbiguousTagPush)
	invalid := assessment.Status == ReleaseExecutionRecoveryCorrupted
	manual := !resumeEligible && !assessment.AlreadyHandedOff && (assessment.RequiresManualIntervention || pipelineResumeRefusalNeedsManualIntervention(resolution.Refusal))
	return pipelineinspection.RuntimeRecoveryObservation{
		Evaluated: assessment.Status != "", Classification: string(assessment.Status),
		SafeToContinue: assessment.SafeToContinue, ResumeEligible: resumeEligible,
		ResumeOperation: operation, ResumeRefusal: refusal,
		ManualIntervention: manual, Uncertain: uncertain, Invalid: invalid,
		Guidance: assessment.Guidance,
	}
}

func pipelineResumeOperationName(operation resumeReleaseOperationKind) string {
	switch operation {
	case resumeReleaseFromCommitCreated:
		return "resume_from_commit_created"
	case resumeReleaseFromTagCreated:
		return "resume_from_tag_created"
	case resumeReleaseFromTagPushed:
		return "resume_from_tag_pushed"
	case returnCompletedReleaseHandoff:
		return "reuse_completed_handoff"
	default:
		return ""
	}
}

func pipelineResumeRefusalName(kind resumeRecoveryRefusalKind) string {
	switch kind {
	case resumeRecoveryRefusalConflicted:
		return "conflicted"
	case resumeRecoveryRefusalCorrupted:
		return "corrupted"
	case resumeRecoveryRefusalAmbiguousCommitPush:
		return "ambiguous_commit_push"
	case resumeRecoveryRefusalAmbiguousTagPush:
		return "ambiguous_tag_push"
	case resumeRecoveryRefusalBeforeCommit:
		return "before_confirmed_commit"
	case resumeRecoveryRefusalUnprovenCommitPush:
		return "unproven_commit_push"
	case resumeRecoveryRefusalUnprovenTagPush:
		return "unproven_tag_push"
	case resumeRecoveryRefusalUnsupportedState:
		return "unsupported_state"
	default:
		return ""
	}
}

func pipelineResumeRefusalNeedsManualIntervention(refusal *resumeRecoveryRefusal) bool {
	if refusal == nil {
		return false
	}
	switch refusal.Kind {
	case resumeRecoveryRefusalConflicted,
		resumeRecoveryRefusalCorrupted,
		resumeRecoveryRefusalAmbiguousCommitPush,
		resumeRecoveryRefusalAmbiguousTagPush,
		resumeRecoveryRefusalUnprovenCommitPush,
		resumeRecoveryRefusalUnprovenTagPush:
		return true
	default:
		return false
	}
}

func observePipelineDispatchRetry(journal *DispatchJournal) (string, bool) {
	resolution := resolveResumeDispatch(journal)
	if resolution.Refusal != nil {
		return "automatic_retry_prohibited", true
	}
	switch resolution.Operation {
	case requestFreshResumeDispatch:
		return "fresh_request_permitted", false
	case reuseAcceptedResumeDispatch:
		return "accepted_request_reused", false
	default:
		return "not_evaluated", true
	}
}
