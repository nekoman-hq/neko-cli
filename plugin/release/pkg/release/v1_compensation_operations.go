package release

import (
	"errors"
	"fmt"
)

type V1CompensationContinuationStatus string

const (
	V1CompensationContinuationNotNeeded V1CompensationContinuationStatus = "not-needed"
	V1CompensationContinuationCompleted V1CompensationContinuationStatus = "completed"
	V1CompensationContinuationManual    V1CompensationContinuationStatus = "manual-recovery-required"
)

type v1LocalTagCompensation interface {
	DeleteLocalTag(string, string) error
}

type v1RemoteTagCompensation interface {
	DeleteRemoteTag(string, string) error
}

type v1ReleaseCommitReverter interface {
	RevertCommit(string, string) error
}

type v1RevertCommitPusher interface {
	PushCommits(string) error
}

type v1ReleaseCommitResetter interface {
	HardResetTo(string, string) error
}

type v1ReleaseFileCleaner interface {
	CleanUntracked(string) error
}

type v1CompensationGit interface {
	v1LocalTagCompensation
	v1RemoteTagCompensation
	v1ReleaseCommitReverter
	v1RevertCommitPusher
	v1ReleaseCommitResetter
	v1ReleaseFileCleaner
}

func continueV1Compensation(
	store V1CompensationEvidenceStore,
	configFiles v1CompensationConfigFiles,
	git v1CompensationGit,
	releases v1GitHubReleaseRemover,
	evidence *V1CompensationEvidence,
) (V1CompensationContinuationStatus, error) {
	for {
		decision := SelectV1CompensationOperation(evidence)
		switch decision.Kind {
		case V1CompensationNoRecoveryNeeded:
			return V1CompensationContinuationNotNeeded, nil
		case V1CompensationAlreadyComplete:
			return V1CompensationContinuationCompleted, nil
		case V1CompensationMarkComplete:
			if err := store.MarkCompensationCompleted(evidence); err != nil {
				return "", err
			}
			return V1CompensationContinuationCompleted, nil
		case V1CompensationRequireManual:
			if err := store.MarkManualRecoveryRequired(evidence); err != nil {
				return "", err
			}
			return V1CompensationContinuationManual, nil
		case V1CompensationPerformOperation:
			if err := performSelectedV1CompensationOperation(decision.Operation, store, configFiles, git, releases, evidence); err != nil {
				return "", err
			}
		default:
			return "", fmt.Errorf("unsupported V1 compensation decision %q", decision.Kind)
		}
	}
}

func performSelectedV1CompensationOperation(
	operation V1CompensationAction,
	store V1CompensationEvidenceStore,
	configFiles v1CompensationConfigFiles,
	git v1CompensationGit,
	releases v1GitHubReleaseRemover,
	evidence *V1CompensationEvidence,
) error {
	switch operation {
	case V1CompensationRestoreConfig:
		return restoreOriginalV1Config(store, configFiles, evidence)
	case V1CompensationDeleteGitHubRelease:
		return deleteConfirmedV1GitHubRelease(store, releases, evidence)
	case V1CompensationDeleteLocalTag:
		return deleteConfirmedV1LocalTag(store, git, evidence)
	case V1CompensationDeleteRemoteTag:
		return deleteConfirmedV1RemoteTag(store, git, evidence)
	case V1CompensationRevertReleaseCommit:
		return revertConfirmedV1ReleaseCommit(store, git, evidence)
	case V1CompensationPushRevertCommit:
		return pushConfirmedV1RevertCommit(store, git, evidence)
	case V1CompensationResetReleaseCommit:
		return resetConfirmedV1ReleaseCommit(store, git, evidence)
	case V1CompensationCleanUntrackedReleaseFiles:
		return cleanConfirmedV1ReleaseFiles(store, git, evidence)
	default:
		return fmt.Errorf("unsupported V1 compensation operation %q", operation)
	}
}

func restoreOriginalV1Config(store V1CompensationEvidenceStore, files v1CompensationConfigFiles, evidence *V1CompensationEvidence) error {
	if err := store.RecordConfigRestorationPending(evidence); err != nil {
		return err
	}
	if err := files.Restore(evidence.OriginalConfig.Path, []byte(evidence.OriginalConfig.Content)); err != nil {
		return recordLocalV1CompensationFailure(err, store.RecordConfigRestorationFailure(evidence))
	}
	return store.ConfirmConfigRestored(evidence)
}

func deleteConfirmedV1GitHubRelease(store V1CompensationEvidenceStore, releases v1GitHubReleaseRemover, evidence *V1CompensationEvidence) error {
	if err := store.RecordGitHubReleaseDeletionPending(evidence); err != nil {
		return err
	}
	if err := releases.Delete(evidence.Identity.RepositoryRoot, evidence.Release.Git.GitHubReleaseTag); err != nil {
		return recordUncertainV1CompensationFailure(err, store.RetainUncertainGitHubReleaseDeletion(evidence))
	}
	return store.ConfirmGitHubReleaseDeleted(evidence)
}

func deleteConfirmedV1LocalTag(store V1CompensationEvidenceStore, git v1LocalTagCompensation, evidence *V1CompensationEvidence) error {
	if err := store.RecordLocalTagDeletionPending(evidence); err != nil {
		return err
	}
	if err := git.DeleteLocalTag(evidence.Identity.RepositoryRoot, evidence.Release.Git.LocalTag); err != nil {
		return recordLocalV1CompensationFailure(err, store.RecordLocalTagDeletionFailure(evidence))
	}
	return store.ConfirmLocalTagDeleted(evidence)
}

func deleteConfirmedV1RemoteTag(store V1CompensationEvidenceStore, git v1RemoteTagCompensation, evidence *V1CompensationEvidence) error {
	if err := store.RecordRemoteTagDeletionPending(evidence); err != nil {
		return err
	}
	if err := git.DeleteRemoteTag(evidence.Identity.RepositoryRoot, evidence.Release.Git.LocalTag); err != nil {
		return recordUncertainV1CompensationFailure(err, store.RetainUncertainRemoteTagDeletion(evidence))
	}
	return store.ConfirmRemoteTagDeleted(evidence)
}

func revertConfirmedV1ReleaseCommit(store V1CompensationEvidenceStore, git v1ReleaseCommitReverter, evidence *V1CompensationEvidence) error {
	if err := store.RecordReleaseCommitRevertPending(evidence); err != nil {
		return err
	}
	if err := git.RevertCommit(evidence.Identity.RepositoryRoot, evidence.Release.Git.ReleaseHead); err != nil {
		return recordLocalV1CompensationFailure(err, store.RecordReleaseCommitRevertFailure(evidence))
	}
	return store.ConfirmReleaseCommitReverted(evidence)
}

func pushConfirmedV1RevertCommit(store V1CompensationEvidenceStore, git v1RevertCommitPusher, evidence *V1CompensationEvidence) error {
	if err := store.RecordRevertPushPending(evidence); err != nil {
		return err
	}
	if err := git.PushCommits(evidence.Identity.RepositoryRoot); err != nil {
		return recordUncertainV1CompensationFailure(err, store.RetainUncertainRevertPush(evidence))
	}
	return store.ConfirmRevertPushed(evidence)
}

func resetConfirmedV1ReleaseCommit(store V1CompensationEvidenceStore, git v1ReleaseCommitResetter, evidence *V1CompensationEvidence) error {
	if err := store.RecordReleaseCommitResetPending(evidence); err != nil {
		return err
	}
	if err := git.HardResetTo(evidence.Identity.RepositoryRoot, evidence.Release.Git.PreHead); err != nil {
		return recordLocalV1CompensationFailure(err, store.RecordReleaseCommitResetFailure(evidence))
	}
	return store.ConfirmReleaseCommitReset(evidence)
}

func cleanConfirmedV1ReleaseFiles(store V1CompensationEvidenceStore, git v1ReleaseFileCleaner, evidence *V1CompensationEvidence) error {
	if err := store.RecordReleaseFileCleanupPending(evidence); err != nil {
		return err
	}
	if err := git.CleanUntracked(evidence.Identity.RepositoryRoot); err != nil {
		return recordLocalV1CompensationFailure(err, store.RecordReleaseFileCleanupFailure(evidence))
	}
	return store.ConfirmReleaseFilesCleaned(evidence)
}

func recordLocalV1CompensationFailure(operationError, persistenceError error) error {
	if persistenceError != nil {
		return errors.Join(operationError, fmt.Errorf("persist local V1 compensation failure: %w", persistenceError))
	}
	return operationError
}

func recordUncertainV1CompensationFailure(operationError, persistenceError error) error {
	if persistenceError != nil {
		return errors.Join(operationError, fmt.Errorf("persist uncertain V1 compensation outcome: %w", persistenceError))
	}
	return operationError
}
