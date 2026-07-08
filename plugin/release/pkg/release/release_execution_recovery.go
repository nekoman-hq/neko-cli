package release

import (
	"fmt"
	"path/filepath"
)

type ReleaseExecutionRecoveryStatus string

const (
	ReleaseExecutionRecoveryNotStarted                 ReleaseExecutionRecoveryStatus = "not-started"
	ReleaseExecutionRecoveryInterruptedBeforeCommit    ReleaseExecutionRecoveryStatus = "interrupted-before-commit"
	ReleaseExecutionRecoveryInterruptedAfterCommit     ReleaseExecutionRecoveryStatus = "interrupted-after-commit"
	ReleaseExecutionRecoveryInterruptedAfterTag        ReleaseExecutionRecoveryStatus = "interrupted-after-tag"
	ReleaseExecutionRecoveryInterruptedBeforePush      ReleaseExecutionRecoveryStatus = "interrupted-before-push"
	ReleaseExecutionRecoveryInterruptedAfterCommitPush ReleaseExecutionRecoveryStatus = "interrupted-after-commit-push"
	ReleaseExecutionRecoveryInterruptedAfterTagPush    ReleaseExecutionRecoveryStatus = "interrupted-after-tag-push"
	ReleaseExecutionRecoveryReadyForDispatch           ReleaseExecutionRecoveryStatus = "ready-for-dispatch"
	ReleaseExecutionRecoveryAlreadyHandedOff           ReleaseExecutionRecoveryStatus = "already-handed-off"
	ReleaseExecutionRecoveryConflicted                 ReleaseExecutionRecoveryStatus = "conflicted"
	ReleaseExecutionRecoveryCorrupted                  ReleaseExecutionRecoveryStatus = "corrupted"
)

// ReleaseExecutionRecoveryAssessment is a read-only local assessment of an
// interrupted V2 release execution journal.
//
//nolint:govet // Fields follow report order.
type ReleaseExecutionRecoveryAssessment struct {
	Status                     ReleaseExecutionRecoveryStatus
	LastConfirmedPhase         ReleaseExecutionJournalState
	PendingAction              ReleaseExecutionPendingAction
	SafeToContinue             bool
	RequiresManualIntervention bool
	AlreadyHandedOff           bool
	Conflicts                  []string
	Guidance                   string
}

// AssessReleaseExecutionRecovery inspects only local Git and file state. It
// never resumes, retries, restores, resets, cleans, pushes, tags, commits, or
// dispatches.
func AssessReleaseExecutionRecovery(repositoryRoot string, journal *ReleaseExecutionJournal) (*ReleaseExecutionRecoveryAssessment, error) {
	if journal == nil {
		return &ReleaseExecutionRecoveryAssessment{
			Status:                     ReleaseExecutionRecoveryCorrupted,
			RequiresManualIntervention: true,
			Guidance:                   "Release execution journal is missing or corrupted. Manual inspection is required.",
		}, nil
	}
	assessment := &ReleaseExecutionRecoveryAssessment{
		LastConfirmedPhase: journal.State,
		PendingAction:      journal.PendingAction,
	}
	if err := validateJournalForRecovery(journal); err != nil {
		assessment.Status = ReleaseExecutionRecoveryCorrupted
		assessment.RequiresManualIntervention = true
		assessment.Conflicts = append(assessment.Conflicts, err.Error())
		assessment.Guidance = "Release execution journal is structurally invalid. Manual inspection is required."
		return assessment, nil
	}
	if conflicts := assessKnownReleaseFileHashes(repositoryRoot, journal); len(conflicts) > 0 {
		assessment.Status = ReleaseExecutionRecoveryConflicted
		assessment.RequiresManualIntervention = true
		assessment.Conflicts = append(assessment.Conflicts, conflicts...)
		assessment.Guidance = "Known release files do not match expected journal hashes. Do not resume automatically."
		return assessment, nil
	}
	if conflict := assessLocalTag(repositoryRoot, journal); conflict != "" {
		assessment.Status = ReleaseExecutionRecoveryConflicted
		assessment.RequiresManualIntervention = true
		assessment.Conflicts = append(assessment.Conflicts, conflict)
		assessment.Guidance = "Local unit tag does not match the journal. Manual inspection is required."
		return assessment, nil
	}
	assessment.Status = recoveryStatusForJournal(journal)
	assessment.SafeToContinue = assessment.Status == ReleaseExecutionRecoveryNotStarted ||
		assessment.Status == ReleaseExecutionRecoveryInterruptedBeforeCommit ||
		assessment.Status == ReleaseExecutionRecoveryReadyForDispatch
	assessment.AlreadyHandedOff = assessment.Status == ReleaseExecutionRecoveryAlreadyHandedOff
	assessment.RequiresManualIntervention = !assessment.SafeToContinue && !assessment.AlreadyHandedOff
	assessment.Guidance = recoveryGuidanceForStatus(assessment.Status)
	return assessment, nil
}

func validateJournalForRecovery(journal *ReleaseExecutionJournal) error {
	if journal.SchemaVersion != releaseExecutionJournalSchemaVersion {
		return fmt.Errorf("schemaVersion mismatch")
	}
	if !journal.State.Valid() {
		return fmt.Errorf("invalid state %q", journal.State)
	}
	if !journal.PendingAction.Valid() {
		return fmt.Errorf("invalid pending action %q", journal.PendingAction)
	}
	if journal.Identity.SHA256 == "" || !isSafeDispatchIdentityHash(journal.Identity.SHA256) {
		return fmt.Errorf("unsafe identity hash")
	}
	return nil
}

func assessKnownReleaseFileHashes(repositoryRoot string, journal *ReleaseExecutionJournal) []string {
	var conflicts []string
	for _, file := range journal.KnownReleaseFiles {
		path := filepath.Join(repositoryRoot, filepath.FromSlash(file.RepositoryRelativePath))
		hash, exists, err := hashFileIfExists(path)
		if err != nil {
			conflicts = append(conflicts, err.Error())
			continue
		}
		if journal.PendingAction == ReleaseExecutionPendingApplyMaterialization && file.PostimageSHA256 != "" {
			if !exists || hash != file.PostimageSHA256 {
				conflicts = append(conflicts, fmt.Sprintf("%s does not match expected postimage", file.RepositoryRelativePath))
			}
			continue
		}
		if stateBeforeMutation(journal.State) && file.PreimageSHA256 != "" {
			if exists != file.ExpectedExistsBefore || hash != file.PreimageSHA256 {
				conflicts = append(conflicts, fmt.Sprintf("%s does not match expected preimage", file.RepositoryRelativePath))
			}
			continue
		}
		if !stateBeforeMutation(journal.State) && file.PostimageSHA256 != "" {
			if exists != file.ExpectedExistsAfter || hash != file.PostimageSHA256 {
				conflicts = append(conflicts, fmt.Sprintf("%s does not match expected postimage", file.RepositoryRelativePath))
			}
		}
	}
	return conflicts
}

func assessLocalTag(repositoryRoot string, journal *ReleaseExecutionJournal) string {
	if journal.TagTargetSHA == "" {
		return ""
	}
	tagCommit, err := NewGitReleaseCoordinator().tagCommit(repositoryRoot, journal.Tag)
	if err != nil {
		return err.Error()
	}
	if tagCommit != "" && tagCommit != journal.TagTargetSHA {
		return fmt.Sprintf("tag %q points to %s, expected %s", journal.Tag, tagCommit, journal.TagTargetSHA)
	}
	return ""
}

func stateBeforeMutation(state ReleaseExecutionJournalState) bool {
	return state == ReleaseExecutionPrepared || state == ReleaseExecutionPreflightValidated
}

func recoveryStatusForJournal(journal *ReleaseExecutionJournal) ReleaseExecutionRecoveryStatus {
	switch {
	case journal.State == ReleaseExecutionPrepared && journal.PendingAction == ReleaseExecutionPendingNone:
		return ReleaseExecutionRecoveryNotStarted
	case journal.PendingAction == ReleaseExecutionPendingApplyMaterialization:
		return ReleaseExecutionRecoveryInterruptedBeforeCommit
	case journal.State == ReleaseExecutionCommitCreated && journal.TagTargetSHA == "":
		return ReleaseExecutionRecoveryInterruptedAfterCommit
	case journal.State == ReleaseExecutionTagCreated:
		return ReleaseExecutionRecoveryInterruptedAfterTag
	case journal.State == ReleaseExecutionDispatchJournalPrepared:
		return ReleaseExecutionRecoveryReadyForDispatch
	case journal.State == ReleaseExecutionCommitPushed:
		return ReleaseExecutionRecoveryInterruptedAfterCommitPush
	case journal.State == ReleaseExecutionTagPushed:
		return ReleaseExecutionRecoveryInterruptedAfterTagPush
	case journal.State == ReleaseExecutionHandoffReady:
		return ReleaseExecutionRecoveryAlreadyHandedOff
	case journal.State == ReleaseExecutionReleaseFilesStaged:
		return ReleaseExecutionRecoveryInterruptedBeforePush
	case releaseExecutionStateRank(journal.State) >= releaseExecutionStateRank(ReleaseExecutionMaterializationApplied):
		return ReleaseExecutionRecoveryInterruptedBeforeCommit
	default:
		return ReleaseExecutionRecoveryNotStarted
	}
}

func recoveryGuidanceForStatus(status ReleaseExecutionRecoveryStatus) string {
	switch status {
	case ReleaseExecutionRecoveryNotStarted:
		return "No release mutation has been confirmed. Future resume logic may restart from preflight."
	case ReleaseExecutionRecoveryInterruptedBeforeCommit:
		return "Release file mutations may have happened, but no release commit is confirmed. Inspect known files before any future resume."
	case ReleaseExecutionRecoveryInterruptedAfterCommit:
		return "Release commit exists locally, but the expected unit tag is not confirmed. Do not recreate blindly."
	case ReleaseExecutionRecoveryInterruptedAfterTag:
		return "Release commit and local unit tag are confirmed. Future logic may inspect push state before continuing."
	case ReleaseExecutionRecoveryInterruptedBeforePush:
		return "Release files are staged locally. Inspect index and worktree before any future resume."
	case ReleaseExecutionRecoveryInterruptedAfterCommitPush:
		return "Commit push was locally recorded. Tag push is not confirmed; do not infer remote tag state without network."
	case ReleaseExecutionRecoveryInterruptedAfterTagPush:
		return "Tag push was locally recorded. Dispatch handoff is not confirmed."
	case ReleaseExecutionRecoveryReadyForDispatch:
		return "Execution journal reached dispatch-journal preparation. Future dispatch handoff may continue after local inspection."
	case ReleaseExecutionRecoveryAlreadyHandedOff:
		return "Release execution was handed off. Inspect the dispatch journal for HTTP dispatch status."
	case ReleaseExecutionRecoveryConflicted:
		return "Release execution journal conflicts with local repository state. Manual intervention is required."
	case ReleaseExecutionRecoveryCorrupted:
		return "Release execution journal is corrupted. Manual intervention is required."
	default:
		return "Release execution status is unknown. Manual inspection is required."
	}
}
