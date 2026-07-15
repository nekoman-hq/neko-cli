package release

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

var ErrV1CompensationEvidenceNotFound = errors.New("V1 compensation evidence not found")

type v1CompensationClock interface {
	Now() time.Time
}

type systemV1CompensationClock struct{}

func (systemV1CompensationClock) Now() time.Time { return time.Now().UTC() }

// V1CompensationEvidenceStore is the canonical owner of V1 compensation
// persistence. Its methods name the exact durable fact they record.
type V1CompensationEvidenceStore struct {
	clock v1CompensationClock
	files releaseJournalFiles
}

func NewV1CompensationEvidenceStore(repositoryRoot string, git gitCommandRunner) V1CompensationEvidenceStore {
	return V1CompensationEvidenceStore{
		files: newReleaseJournalFiles(repositoryRoot, git),
		clock: systemV1CompensationClock{},
	}
}

func (store V1CompensationEvidenceStore) CurrentPath() (string, error) {
	releaseDirectory, err := store.files.releaseDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(releaseDirectory, "v1-compensation", "current.json"), nil
}

func (store V1CompensationEvidenceStore) Create(evidence V1CompensationEvidence) error {
	if err := evidence.Validate(); err != nil {
		return fmt.Errorf("create V1 compensation evidence: %w", err)
	}
	return store.write(evidence)
}

func (store V1CompensationEvidenceStore) LoadCurrent() (*V1CompensationEvidence, error) {
	path, err := store.CurrentPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrV1CompensationEvidenceNotFound
		}
		return nil, fmt.Errorf("read V1 compensation evidence: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var evidence V1CompensationEvidence
	if err := decoder.Decode(&evidence); err != nil {
		return nil, fmt.Errorf("decode V1 compensation evidence: %w", err)
	}
	if err := requireV1CompensationJSONEnd(decoder); err != nil {
		return nil, err
	}
	if err := evidence.Validate(); err != nil {
		return nil, fmt.Errorf("validate V1 compensation evidence: %w", err)
	}
	return &evidence, nil
}

func (store V1CompensationEvidenceStore) FindUnresolved() (*V1CompensationEvidence, bool, error) {
	evidence, err := store.LoadCurrent()
	if errors.Is(err, ErrV1CompensationEvidenceNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if evidence.Compensation.Status == V1CompensationCompleted || evidence.Release.Status == V1ReleaseEffectSucceeded {
		return nil, false, nil
	}
	return evidence, true, nil
}

func (store V1CompensationEvidenceStore) RecordConfigWritePending(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Release.ConfigMutation = V1CompensationActionPending
	next.Compensation.Actions.RestoreConfig.Status = V1CompensationActionPlanned
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) ConfirmConfigWrite(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Release.ConfigMutation = V1CompensationActionConfirmed
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordConfigWriteFailure(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Release.ConfigMutation = V1CompensationActionUncertain
	next.Release.Status = V1ReleaseEffectFailed
	next.Compensation.Actions.RestoreConfig.Status = V1CompensationActionPlanned
	next.Compensation.Failure = nil
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordExecutorPending(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Release.Status = V1ReleaseEffectPending
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) PlanFailedExecution(evidence *V1CompensationEvidence, state GitReleaseState) error {
	if state.ReleaseHead != "" && !state.PushedCommit && state.PreHead == "" {
		return fmt.Errorf("cannot plan V1 compensation for release commit without pre-head")
	}
	next := *evidence
	next.Release.Status = V1ReleaseEffectFailed
	next.Release.Git = v1ReleaseGitEvidence(state)
	next.Compensation.Status = V1CompensationPlanned
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Failure = nil
	if state.CreatedGitHubRelease && state.GitHubReleaseTag != "" {
		next.Compensation.Actions.DeleteGitHubRelease.Status = V1CompensationActionPlanned
	}
	if state.TagName != "" {
		next.Compensation.Actions.DeleteLocalTag.Status = V1CompensationActionPlanned
	}
	if state.PushedTag && state.TagName != "" {
		next.Compensation.Actions.DeleteRemoteTag.Status = V1CompensationActionPlanned
	}
	if state.ReleaseHead != "" && state.PushedCommit {
		next.Compensation.Actions.RevertReleaseCommit.Status = V1CompensationActionPlanned
		next.Compensation.Actions.PushRevertCommit.Status = V1CompensationActionPlanned
	}
	if state.ReleaseHead != "" && !state.PushedCommit {
		next.Compensation.Actions.ResetReleaseCommit.Status = V1CompensationActionPlanned
	}
	if state.hasMutatingStep() {
		next.Compensation.Actions.CleanUntrackedFiles.Status = V1CompensationActionPlanned
	}
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RetainUncertainExecution(evidence *V1CompensationEvidence, state GitReleaseState) error {
	next := *evidence
	next.Release.Status = V1ReleaseEffectUncertain
	next.Release.Git = v1ReleaseGitEvidence(state)
	next.Compensation.Status = V1CompensationPlanned
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Failure = nil
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) ConfirmReleaseExecution(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Release.Status = V1ReleaseEffectSucceeded
	next.Compensation.Status = V1CompensationCompleted
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.RestoreConfig.Status = V1CompensationActionNotRequired
	next.Compensation.Failure = nil
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordConfigRestorationPending(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.Status = V1CompensationInProgress
	next.Compensation.PendingAction = V1CompensationRestoreConfig
	next.Compensation.Actions.RestoreConfig.Status = V1CompensationActionPending
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) ConfirmConfigRestored(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.RestoreConfig.Status = V1CompensationActionConfirmed
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordConfigRestorationFailure(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.RestoreConfig.Status = V1CompensationActionFailed
	next.Compensation.Failure = localV1CompensationFailure(V1CompensationRestoreConfig)
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordGitHubReleaseDeletionPending(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.Status = V1CompensationInProgress
	next.Compensation.PendingAction = V1CompensationDeleteGitHubRelease
	next.Compensation.Actions.DeleteGitHubRelease.Status = V1CompensationActionPending
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) ConfirmGitHubReleaseDeleted(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.DeleteGitHubRelease.Status = V1CompensationActionConfirmed
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RetainUncertainGitHubReleaseDeletion(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.Status = V1CompensationManualRecovery
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.DeleteGitHubRelease.Status = V1CompensationActionUncertain
	next.Compensation.Failure = uncertainV1CompensationFailure(V1CompensationDeleteGitHubRelease)
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordLocalTagDeletionPending(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.Status = V1CompensationInProgress
	next.Compensation.PendingAction = V1CompensationDeleteLocalTag
	next.Compensation.Actions.DeleteLocalTag.Status = V1CompensationActionPending
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) ConfirmLocalTagDeleted(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.DeleteLocalTag.Status = V1CompensationActionConfirmed
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordLocalTagDeletionFailure(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.DeleteLocalTag.Status = V1CompensationActionFailed
	next.Compensation.Failure = localV1CompensationFailure(V1CompensationDeleteLocalTag)
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordRemoteTagDeletionPending(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.Status = V1CompensationInProgress
	next.Compensation.PendingAction = V1CompensationDeleteRemoteTag
	next.Compensation.Actions.DeleteRemoteTag.Status = V1CompensationActionPending
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) ConfirmRemoteTagDeleted(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.DeleteRemoteTag.Status = V1CompensationActionConfirmed
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RetainUncertainRemoteTagDeletion(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.Status = V1CompensationManualRecovery
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.DeleteRemoteTag.Status = V1CompensationActionUncertain
	next.Compensation.Failure = uncertainV1CompensationFailure(V1CompensationDeleteRemoteTag)
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordReleaseCommitRevertPending(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.Status = V1CompensationInProgress
	next.Compensation.PendingAction = V1CompensationRevertReleaseCommit
	next.Compensation.Actions.RevertReleaseCommit.Status = V1CompensationActionPending
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) ConfirmReleaseCommitReverted(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.RevertReleaseCommit.Status = V1CompensationActionConfirmed
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordReleaseCommitRevertFailure(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.RevertReleaseCommit.Status = V1CompensationActionFailed
	next.Compensation.Failure = localV1CompensationFailure(V1CompensationRevertReleaseCommit)
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordRevertPushPending(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.Status = V1CompensationInProgress
	next.Compensation.PendingAction = V1CompensationPushRevertCommit
	next.Compensation.Actions.PushRevertCommit.Status = V1CompensationActionPending
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) ConfirmRevertPushed(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.PushRevertCommit.Status = V1CompensationActionConfirmed
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RetainUncertainRevertPush(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.Status = V1CompensationManualRecovery
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.PushRevertCommit.Status = V1CompensationActionUncertain
	next.Compensation.Failure = uncertainV1CompensationFailure(V1CompensationPushRevertCommit)
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordReleaseCommitResetPending(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.Status = V1CompensationInProgress
	next.Compensation.PendingAction = V1CompensationResetReleaseCommit
	next.Compensation.Actions.ResetReleaseCommit.Status = V1CompensationActionPending
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) ConfirmReleaseCommitReset(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.ResetReleaseCommit.Status = V1CompensationActionConfirmed
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordReleaseCommitResetFailure(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.ResetReleaseCommit.Status = V1CompensationActionFailed
	next.Compensation.Failure = localV1CompensationFailure(V1CompensationResetReleaseCommit)
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordReleaseFileCleanupPending(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.Status = V1CompensationInProgress
	next.Compensation.PendingAction = V1CompensationCleanUntrackedReleaseFiles
	next.Compensation.Actions.CleanUntrackedFiles.Status = V1CompensationActionPending
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) ConfirmReleaseFilesCleaned(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.CleanUntrackedFiles.Status = V1CompensationActionConfirmed
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) RecordReleaseFileCleanupFailure(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Actions.CleanUntrackedFiles.Status = V1CompensationActionFailed
	next.Compensation.Failure = localV1CompensationFailure(V1CompensationCleanUntrackedReleaseFiles)
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) MarkManualRecoveryRequired(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.Status = V1CompensationManualRecovery
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) MarkCompensationCompleted(evidence *V1CompensationEvidence) error {
	next := *evidence
	next.Compensation.Status = V1CompensationCompleted
	next.Compensation.PendingAction = V1CompensationActionNone
	next.Compensation.Failure = nil
	return store.replace(evidence, next)
}

func (store V1CompensationEvidenceStore) replace(current *V1CompensationEvidence, next V1CompensationEvidence) error {
	next.UpdatedAt = store.clock.Now()
	if err := next.Validate(); err != nil {
		return fmt.Errorf("record V1 compensation evidence: %w", err)
	}
	if err := store.write(next); err != nil {
		return err
	}
	*current = next
	return nil
}

func (store V1CompensationEvidenceStore) write(evidence V1CompensationEvidence) error {
	path, err := store.CurrentPath()
	if err != nil {
		return err
	}
	if directoryErr := store.files.createPrivateDirectory(filepath.Dir(path)); directoryErr != nil {
		return fmt.Errorf("create V1 compensation evidence directory: %w", directoryErr)
	}
	data, err := marshalCanonicalReleaseJournal(evidence)
	if err != nil {
		return fmt.Errorf("marshal V1 compensation evidence: %w", err)
	}
	if err := store.files.writePrivateAtomic(path, data); err != nil {
		return fmt.Errorf("persist V1 compensation evidence: %w", err)
	}
	return nil
}

func requireV1CompensationJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("decode V1 compensation evidence: trailing JSON value")
		}
		return fmt.Errorf("decode V1 compensation evidence: %w", err)
	}
	return nil
}

func v1ReleaseGitEvidence(state GitReleaseState) V1ReleaseGitEvidence {
	return V1ReleaseGitEvidence{
		PreHead:          state.PreHead,
		ReleaseHead:      state.ReleaseHead,
		LocalTag:         state.TagName,
		GitHubReleaseTag: state.GitHubReleaseTag,
		CommitPushed:     state.PushedCommit,
		TagPushed:        state.PushedTag,
		ReleasePublished: state.CreatedGitHubRelease,
	}
}

func localV1CompensationFailure(action V1CompensationAction) *V1CompensationFailureEvidence {
	return &V1CompensationFailureEvidence{Action: action, Kind: V1CompensationLocalFailure}
}

func uncertainV1CompensationFailure(action V1CompensationAction) *V1CompensationFailureEvidence {
	return &V1CompensationFailureEvidence{Action: action, Kind: V1CompensationExternalOutcomeUncertain}
}
