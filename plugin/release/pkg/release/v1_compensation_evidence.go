package release

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
)

const v1CompensationEvidenceSchemaVersion = 1

type V1CompensationExecutor string

const (
	V1CompensationExecutorGoReleaser V1CompensationExecutor = V1CompensationExecutor(releasetool.GoReleaser)
	V1CompensationExecutorJReleaser  V1CompensationExecutor = V1CompensationExecutor(releasetool.JReleaser)
	V1CompensationExecutorReleaseIt  V1CompensationExecutor = V1CompensationExecutor(releasetool.ReleaseIt)
)

type V1ReleaseEffectStatus string

const (
	V1ReleaseEffectPrepared  V1ReleaseEffectStatus = "prepared"
	V1ReleaseEffectPending   V1ReleaseEffectStatus = "pending"
	V1ReleaseEffectSucceeded V1ReleaseEffectStatus = "succeeded"
	V1ReleaseEffectFailed    V1ReleaseEffectStatus = "failed"
	V1ReleaseEffectUncertain V1ReleaseEffectStatus = "external-outcome-uncertain"
)

type V1CompensationStatus string

const (
	V1CompensationPlanned        V1CompensationStatus = "planned"
	V1CompensationInProgress     V1CompensationStatus = "in-progress"
	V1CompensationManualRecovery V1CompensationStatus = "manual-recovery-required"
	V1CompensationCompleted      V1CompensationStatus = "completed"
)

type V1CompensationAction string

const (
	V1CompensationActionNone                 V1CompensationAction = "none"
	V1CompensationRestoreConfig              V1CompensationAction = "restore-v1-config"
	V1CompensationDeleteGitHubRelease        V1CompensationAction = "delete-github-release"
	V1CompensationDeleteLocalTag             V1CompensationAction = "delete-local-tag"
	V1CompensationDeleteRemoteTag            V1CompensationAction = "delete-remote-tag"
	V1CompensationRevertReleaseCommit        V1CompensationAction = "revert-release-commit"
	V1CompensationPushRevertCommit           V1CompensationAction = "push-revert-commit"
	V1CompensationResetReleaseCommit         V1CompensationAction = "reset-release-commit"
	V1CompensationCleanUntrackedReleaseFiles V1CompensationAction = "clean-untracked-release-files"
)

type V1CompensationActionStatus string

const (
	V1CompensationActionNotRequired V1CompensationActionStatus = "not-required"
	V1CompensationActionPlanned     V1CompensationActionStatus = "planned"
	V1CompensationActionPending     V1CompensationActionStatus = "pending"
	V1CompensationActionConfirmed   V1CompensationActionStatus = "confirmed"
	V1CompensationActionFailed      V1CompensationActionStatus = "failed"
	V1CompensationActionUncertain   V1CompensationActionStatus = "external-outcome-uncertain"
)

type V1CompensationFailureKind string

const (
	V1CompensationLocalFailure             V1CompensationFailureKind = "local-operation-failed"
	V1CompensationExternalOutcomeUncertain V1CompensationFailureKind = "external-outcome-uncertain"
)

type V1CompensationIdentity struct {
	RepositoryRoot  string                 `json:"repositoryRoot"`
	Executor        V1CompensationExecutor `json:"executor"`
	OriginalVersion string                 `json:"originalVersion"`
	IntendedVersion string                 `json:"intendedVersion"`
	Tag             string                 `json:"tag"`
	SHA256          string                 `json:"sha256"`
}

type V1OriginalConfigEvidence struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
}

type V1ReleaseGitEvidence struct {
	PreHead          string `json:"preHead,omitempty"`
	ReleaseHead      string `json:"releaseHead,omitempty"`
	LocalTag         string `json:"localTag,omitempty"`
	GitHubReleaseTag string `json:"githubReleaseTag,omitempty"`
	CommitPushed     bool   `json:"commitPushed"`
	TagPushed        bool   `json:"tagPushed"`
	ReleasePublished bool   `json:"releasePublished"`
}

type V1ReleaseEffectEvidence struct {
	Status         V1ReleaseEffectStatus      `json:"status"`
	ConfigMutation V1CompensationActionStatus `json:"configMutation"`
	Git            V1ReleaseGitEvidence       `json:"git"`
}

type V1CompensationActionEvidence struct {
	Status V1CompensationActionStatus `json:"status"`
}

type V1CompensationActions struct {
	RestoreConfig       V1CompensationActionEvidence `json:"restoreConfig"`
	DeleteGitHubRelease V1CompensationActionEvidence `json:"deleteGitHubRelease"`
	DeleteLocalTag      V1CompensationActionEvidence `json:"deleteLocalTag"`
	DeleteRemoteTag     V1CompensationActionEvidence `json:"deleteRemoteTag"`
	RevertReleaseCommit V1CompensationActionEvidence `json:"revertReleaseCommit"`
	PushRevertCommit    V1CompensationActionEvidence `json:"pushRevertCommit"`
	ResetReleaseCommit  V1CompensationActionEvidence `json:"resetReleaseCommit"`
	CleanUntrackedFiles V1CompensationActionEvidence `json:"cleanUntrackedFiles"`
}

type V1CompensationFailureEvidence struct {
	Action V1CompensationAction      `json:"action"`
	Kind   V1CompensationFailureKind `json:"kind"`
}

type V1CompensationProgress struct {
	Failure       *V1CompensationFailureEvidence `json:"failure,omitempty"`
	Actions       V1CompensationActions          `json:"actions"`
	Status        V1CompensationStatus           `json:"status"`
	PendingAction V1CompensationAction           `json:"pendingAction"`
}

// V1CompensationEvidence is the V1-only durable record for one release attempt.
// It deliberately uses fixed action fields instead of an executable step list.
type V1CompensationEvidence struct {
	Compensation   V1CompensationProgress   `json:"compensation"`
	CreatedAt      time.Time                `json:"createdAt"`
	UpdatedAt      time.Time                `json:"updatedAt"`
	Identity       V1CompensationIdentity   `json:"identity"`
	OriginalConfig V1OriginalConfigEvidence `json:"originalConfig"`
	Release        V1ReleaseEffectEvidence  `json:"release"`
	SchemaVersion  int                      `json:"schemaVersion"`
}

func newV1CompensationEvidence(plan V1ReleasePlan, configPath string, originalConfig []byte, now time.Time) (V1CompensationEvidence, error) {
	executor := V1CompensationExecutor(plan.Executor)
	identity := V1CompensationIdentity{
		RepositoryRoot:  strings.TrimSpace(plan.RepositoryRoot),
		Executor:        executor,
		OriginalVersion: plan.CurrentVersion,
		IntendedVersion: plan.NextVersion,
		Tag:             plan.Tag,
	}
	hash, err := hashV1CompensationIdentity(identity)
	if err != nil {
		return V1CompensationEvidence{}, err
	}
	identity.SHA256 = hash
	configHash := sha256.Sum256(originalConfig)
	notRequired := V1CompensationActionEvidence{Status: V1CompensationActionNotRequired}
	evidence := V1CompensationEvidence{
		SchemaVersion: v1CompensationEvidenceSchemaVersion,
		Identity:      identity,
		OriginalConfig: V1OriginalConfigEvidence{
			Path:    configPath,
			Content: string(originalConfig),
			SHA256:  hex.EncodeToString(configHash[:]),
		},
		Release: V1ReleaseEffectEvidence{
			Status:         V1ReleaseEffectPrepared,
			ConfigMutation: V1CompensationActionNotRequired,
		},
		Compensation: V1CompensationProgress{
			Status:        V1CompensationPlanned,
			PendingAction: V1CompensationActionNone,
			Actions: V1CompensationActions{
				RestoreConfig:       notRequired,
				DeleteGitHubRelease: notRequired,
				DeleteLocalTag:      notRequired,
				DeleteRemoteTag:     notRequired,
				RevertReleaseCommit: notRequired,
				PushRevertCommit:    notRequired,
				ResetReleaseCommit:  notRequired,
				CleanUntrackedFiles: notRequired,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := evidence.Validate(); err != nil {
		return V1CompensationEvidence{}, err
	}
	return evidence, nil
}

func hashV1CompensationIdentity(identity V1CompensationIdentity) (string, error) {
	canonical := struct {
		RepositoryRoot  string                 `json:"repositoryRoot"`
		Executor        V1CompensationExecutor `json:"executor"`
		OriginalVersion string                 `json:"originalVersion"`
		IntendedVersion string                 `json:"intendedVersion"`
		Tag             string                 `json:"tag"`
	}{
		RepositoryRoot:  identity.RepositoryRoot,
		Executor:        identity.Executor,
		OriginalVersion: identity.OriginalVersion,
		IntendedVersion: identity.IntendedVersion,
		Tag:             identity.Tag,
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("marshal V1 compensation identity: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (evidence V1CompensationEvidence) Validate() error {
	if evidence.SchemaVersion != v1CompensationEvidenceSchemaVersion {
		return fmt.Errorf("unsupported V1 compensation evidence schema version %d", evidence.SchemaVersion)
	}
	if evidence.Identity.RepositoryRoot == "" || evidence.Identity.OriginalVersion == "" || evidence.Identity.IntendedVersion == "" || evidence.Identity.Tag == "" {
		return fmt.Errorf("V1 compensation identity is incomplete")
	}
	if !evidence.Identity.Executor.valid() {
		return fmt.Errorf("unknown V1 compensation executor %q", evidence.Identity.Executor)
	}
	wantIdentityHash, err := hashV1CompensationIdentity(evidence.Identity)
	if err != nil {
		return err
	}
	if evidence.Identity.SHA256 != wantIdentityHash {
		return fmt.Errorf("V1 compensation identity hash mismatch")
	}
	if evidence.OriginalConfig.Path == "" || evidence.OriginalConfig.SHA256 == "" {
		return fmt.Errorf("original V1 config evidence is incomplete")
	}
	configHash := sha256.Sum256([]byte(evidence.OriginalConfig.Content))
	if evidence.OriginalConfig.SHA256 != hex.EncodeToString(configHash[:]) {
		return fmt.Errorf("original V1 config evidence hash mismatch")
	}
	if evidence.CreatedAt.IsZero() || evidence.UpdatedAt.IsZero() {
		return fmt.Errorf("V1 compensation evidence timestamps are required")
	}
	if !evidence.Release.Status.valid() {
		return fmt.Errorf("unknown V1 release effect status %q", evidence.Release.Status)
	}
	if !evidence.Release.ConfigMutation.valid() {
		return fmt.Errorf("unknown V1 config mutation status %q", evidence.Release.ConfigMutation)
	}
	if !evidence.Compensation.Status.valid() {
		return fmt.Errorf("unknown V1 compensation status %q", evidence.Compensation.Status)
	}
	if !evidence.Compensation.PendingAction.valid() {
		return fmt.Errorf("unknown pending V1 compensation action %q", evidence.Compensation.PendingAction)
	}
	if err := evidence.Compensation.validateActionStatuses(); err != nil {
		return err
	}
	if evidence.Compensation.PendingAction != V1CompensationActionNone && evidence.Compensation.statusFor(evidence.Compensation.PendingAction) != V1CompensationActionPending {
		return fmt.Errorf("pending V1 compensation action %q is not marked pending", evidence.Compensation.PendingAction)
	}
	if evidence.Compensation.Actions.RevertReleaseCommit.Status != V1CompensationActionNotRequired &&
		evidence.Compensation.Actions.ResetReleaseCommit.Status != V1CompensationActionNotRequired {
		return fmt.Errorf("V1 compensation cannot both revert and reset the release commit")
	}
	if evidence.Compensation.Failure != nil {
		if !evidence.Compensation.Failure.Action.valid() || evidence.Compensation.Failure.Action == V1CompensationActionNone {
			return fmt.Errorf("V1 compensation failure action is invalid")
		}
		if !evidence.Compensation.Failure.Kind.valid() {
			return fmt.Errorf("unknown V1 compensation failure kind %q", evidence.Compensation.Failure.Kind)
		}
	}
	return nil
}

func (executor V1CompensationExecutor) valid() bool {
	switch executor {
	case V1CompensationExecutorGoReleaser, V1CompensationExecutorJReleaser, V1CompensationExecutorReleaseIt:
		return true
	default:
		return false
	}
}

func (status V1ReleaseEffectStatus) valid() bool {
	switch status {
	case V1ReleaseEffectPrepared, V1ReleaseEffectPending, V1ReleaseEffectSucceeded, V1ReleaseEffectFailed, V1ReleaseEffectUncertain:
		return true
	default:
		return false
	}
}

func (status V1CompensationStatus) valid() bool {
	switch status {
	case V1CompensationPlanned, V1CompensationInProgress, V1CompensationManualRecovery, V1CompensationCompleted:
		return true
	default:
		return false
	}
}

func (action V1CompensationAction) valid() bool {
	switch action {
	case V1CompensationActionNone, V1CompensationRestoreConfig, V1CompensationDeleteGitHubRelease,
		V1CompensationDeleteLocalTag, V1CompensationDeleteRemoteTag, V1CompensationRevertReleaseCommit,
		V1CompensationPushRevertCommit, V1CompensationResetReleaseCommit, V1CompensationCleanUntrackedReleaseFiles:
		return true
	default:
		return false
	}
}

func (status V1CompensationActionStatus) valid() bool {
	switch status {
	case V1CompensationActionNotRequired, V1CompensationActionPlanned, V1CompensationActionPending,
		V1CompensationActionConfirmed, V1CompensationActionFailed, V1CompensationActionUncertain:
		return true
	default:
		return false
	}
}

func (kind V1CompensationFailureKind) valid() bool {
	return kind == V1CompensationLocalFailure || kind == V1CompensationExternalOutcomeUncertain
}

func (progress V1CompensationProgress) statusFor(action V1CompensationAction) V1CompensationActionStatus {
	switch action {
	case V1CompensationRestoreConfig:
		return progress.Actions.RestoreConfig.Status
	case V1CompensationDeleteGitHubRelease:
		return progress.Actions.DeleteGitHubRelease.Status
	case V1CompensationDeleteLocalTag:
		return progress.Actions.DeleteLocalTag.Status
	case V1CompensationDeleteRemoteTag:
		return progress.Actions.DeleteRemoteTag.Status
	case V1CompensationRevertReleaseCommit:
		return progress.Actions.RevertReleaseCommit.Status
	case V1CompensationPushRevertCommit:
		return progress.Actions.PushRevertCommit.Status
	case V1CompensationResetReleaseCommit:
		return progress.Actions.ResetReleaseCommit.Status
	case V1CompensationCleanUntrackedReleaseFiles:
		return progress.Actions.CleanUntrackedFiles.Status
	default:
		return ""
	}
}

func (progress V1CompensationProgress) validateActionStatuses() error {
	checks := []struct {
		action V1CompensationAction
		status V1CompensationActionStatus
	}{
		{V1CompensationRestoreConfig, progress.Actions.RestoreConfig.Status},
		{V1CompensationDeleteGitHubRelease, progress.Actions.DeleteGitHubRelease.Status},
		{V1CompensationDeleteLocalTag, progress.Actions.DeleteLocalTag.Status},
		{V1CompensationDeleteRemoteTag, progress.Actions.DeleteRemoteTag.Status},
		{V1CompensationRevertReleaseCommit, progress.Actions.RevertReleaseCommit.Status},
		{V1CompensationPushRevertCommit, progress.Actions.PushRevertCommit.Status},
		{V1CompensationResetReleaseCommit, progress.Actions.ResetReleaseCommit.Status},
		{V1CompensationCleanUntrackedReleaseFiles, progress.Actions.CleanUntrackedFiles.Status},
	}
	for _, check := range checks {
		if !check.status.valid() {
			return fmt.Errorf("unknown status %q for V1 compensation action %q", check.status, check.action)
		}
	}
	return nil
}
