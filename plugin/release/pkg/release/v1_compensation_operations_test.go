package release

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestV1ConfigRestorationPersistsPendingBeforeSideEffect(t *testing.T) {
	store, evidence := newV1CompensationOperationFixture(t)
	files := &observingV1CompensationFiles{store: store}

	if err := restoreOriginalV1Config(store, files, &evidence); err != nil {
		t.Fatalf("restoreOriginalV1Config: %v", err)
	}
	if files.calls != 1 || files.observedStatus != V1CompensationActionPending {
		t.Fatalf("restore calls=%d observed status=%q", files.calls, files.observedStatus)
	}
	loaded, err := store.LoadCurrent()
	if err != nil {
		t.Fatalf("LoadCurrent: %v", err)
	}
	if loaded.Compensation.Actions.RestoreConfig.Status != V1CompensationActionConfirmed {
		t.Fatalf("restore status = %q", loaded.Compensation.Actions.RestoreConfig.Status)
	}
}

func TestV1CompensationDoesNotActWhenPendingPersistenceFails(t *testing.T) {
	store, evidence := newV1CompensationOperationFixture(t)
	store.files.replaceFile = func(string, []byte, os.FileMode) error {
		return errors.New("pending persistence failed")
	}
	files := &observingV1CompensationFiles{store: store}

	err := restoreOriginalV1Config(store, files, &evidence)

	if err == nil || !strings.Contains(err.Error(), "pending persistence failed") {
		t.Fatalf("restore error = %v", err)
	}
	if files.calls != 0 {
		t.Fatalf("restore calls = %d, want 0", files.calls)
	}
}

func TestV1PendingLocalRestorationCanContinueAfterConfirmationInterruption(t *testing.T) {
	store, evidence := newV1CompensationOperationFixture(t)
	originalWriter := store.files.replaceFile
	writes := 0
	store.files.replaceFile = func(path string, data []byte, mode os.FileMode) error {
		writes++
		if writes == 2 {
			return errors.New("confirmation interrupted")
		}
		return originalWriter(path, data, mode)
	}
	files := &observingV1CompensationFiles{store: store}

	if err := restoreOriginalV1Config(store, files, &evidence); err == nil {
		t.Fatal("restore succeeded despite confirmation interruption")
	}
	loaded, err := store.LoadCurrent()
	if err != nil {
		t.Fatalf("LoadCurrent: %v", err)
	}
	if loaded.Compensation.PendingAction != V1CompensationRestoreConfig || loaded.Compensation.Actions.RestoreConfig.Status != V1CompensationActionPending {
		t.Fatalf("interrupted evidence = %#v", loaded.Compensation)
	}
	decision := SelectV1CompensationOperation(loaded)
	if decision.Kind != V1CompensationPerformOperation || decision.Operation != V1CompensationRestoreConfig {
		t.Fatalf("restart decision = %#v", decision)
	}

	store.files.replaceFile = originalWriter
	files.store = store
	if err := restoreOriginalV1Config(store, files, loaded); err != nil {
		t.Fatalf("continued restoration: %v", err)
	}
	if files.calls != 2 {
		t.Fatalf("repeatable local restore calls = %d, want 2", files.calls)
	}
}

func TestV1PendingRemoteDeletionRequiresManualRecoveryWithoutReplay(t *testing.T) {
	store, evidence := newV1CompensationOperationFixture(t)
	evidence.Compensation.Actions.RestoreConfig.Status = V1CompensationActionConfirmed
	evidence.Compensation.Actions.DeleteGitHubRelease.Status = V1CompensationActionPlanned
	evidence.Release.Git.GitHubReleaseTag = "v1.2.4"
	if err := store.Create(evidence); err != nil {
		t.Fatalf("replace fixture evidence: %v", err)
	}
	originalWriter := store.files.replaceFile
	writes := 0
	store.files.replaceFile = func(path string, data []byte, mode os.FileMode) error {
		writes++
		if writes == 2 {
			return errors.New("remote confirmation interrupted")
		}
		return originalWriter(path, data, mode)
	}
	remover := &countingV1CompensationReleaseRemover{}

	if err := deleteConfirmedV1GitHubRelease(store, remover, &evidence); err == nil {
		t.Fatal("remote deletion succeeded despite confirmation interruption")
	}
	loaded, err := store.LoadCurrent()
	if err != nil {
		t.Fatalf("LoadCurrent: %v", err)
	}
	decision := SelectV1CompensationOperation(loaded)
	if decision.Kind != V1CompensationRequireManual || decision.Reason != V1CompensationReasonPendingExternal {
		t.Fatalf("restart decision = %#v", decision)
	}
	if remover.calls != 1 {
		t.Fatalf("remote deletion calls = %d, want 1", remover.calls)
	}
}

func TestV1CompensationFailureStopsLaterEffectsAndKeepsSafeFailureEvidence(t *testing.T) {
	store, evidence := newV1CompensationOperationFixture(t)
	evidence.Compensation.Actions.DeleteGitHubRelease.Status = V1CompensationActionPlanned
	evidence.Release.Git.GitHubReleaseTag = "v1.2.4"
	if err := store.Create(evidence); err != nil {
		t.Fatalf("replace fixture evidence: %v", err)
	}
	files := &observingV1CompensationFiles{store: store, restoreErr: errors.New("restore failed")}
	remover := &countingV1CompensationReleaseRemover{}

	_, err := continueV1Compensation(store, files, successfulV1CompensationGit{}, remover, &evidence)

	if err == nil || !strings.Contains(err.Error(), "restore failed") {
		t.Fatalf("continuation error = %v", err)
	}
	if remover.calls != 0 {
		t.Fatalf("later remote deletion calls = %d, want 0", remover.calls)
	}
	loaded, loadErr := store.LoadCurrent()
	if loadErr != nil {
		t.Fatalf("LoadCurrent: %v", loadErr)
	}
	if loaded.Compensation.Failure == nil || loaded.Compensation.Failure.Kind != V1CompensationLocalFailure || loaded.Compensation.Failure.Action != V1CompensationRestoreConfig {
		t.Fatalf("safe failure evidence = %#v", loaded.Compensation.Failure)
	}
}

func TestV1CompensationContinuesConfirmedEffectsInLegacyOrder(t *testing.T) {
	store, evidence := newV1CompensationOperationFixture(t)
	if err := store.RecordConfigWritePending(&evidence); err != nil {
		t.Fatalf("RecordConfigWritePending: %v", err)
	}
	if err := store.ConfirmConfigWrite(&evidence); err != nil {
		t.Fatalf("ConfirmConfigWrite: %v", err)
	}
	if err := store.PlanFailedExecution(&evidence, GitReleaseState{
		PreHead:              "before",
		ReleaseHead:          "release",
		TagName:              "v1.2.4",
		GitHubReleaseTag:     "v1.2.4",
		PushedCommit:         true,
		PushedTag:            true,
		CreatedGitHubRelease: true,
	}); err != nil {
		t.Fatalf("PlanFailedExecution: %v", err)
	}
	operations := &v1CompensationOperationRecorder{}
	files := recordingOrderedV1ConfigRestorer{operations: operations}
	git := observingOrderedV1CompensationGit{store: store, operations: operations}
	releases := observingOrderedV1ReleaseRemover{store: store, operations: operations}

	status, err := continueV1Compensation(store, files, git, releases, &evidence)

	if err != nil || status != V1CompensationContinuationCompleted {
		t.Fatalf("continuation status=%q error=%v", status, err)
	}
	want := []string{
		"restore V1 config",
		"delete GitHub release v1.2.4",
		"delete local tag v1.2.4",
		"delete remote tag v1.2.4",
		"revert commit release",
		"push commits",
		"clean untracked",
	}
	if got := strings.Join(operations.values, "\n"); got != strings.Join(want, "\n") {
		t.Fatalf("compensation operations:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
	loaded, loadErr := store.LoadCurrent()
	if loadErr != nil {
		t.Fatalf("LoadCurrent: %v", loadErr)
	}
	if loaded.Compensation.Actions.RestoreConfig.Status != V1CompensationActionConfirmed ||
		loaded.Compensation.Actions.DeleteGitHubRelease.Status != V1CompensationActionConfirmed ||
		loaded.Compensation.Actions.DeleteLocalTag.Status != V1CompensationActionConfirmed ||
		loaded.Compensation.Actions.DeleteRemoteTag.Status != V1CompensationActionConfirmed ||
		loaded.Compensation.Actions.RevertReleaseCommit.Status != V1CompensationActionConfirmed ||
		loaded.Compensation.Actions.PushRevertCommit.Status != V1CompensationActionConfirmed ||
		loaded.Compensation.Actions.CleanUntrackedFiles.Status != V1CompensationActionConfirmed {
		t.Fatalf("confirmed compensation actions = %#v", loaded.Compensation.Actions)
	}
	before := len(operations.values)
	status, err = continueV1Compensation(store, files, git, releases, &evidence)
	if err != nil || status != V1CompensationContinuationCompleted || len(operations.values) != before {
		t.Fatalf("completed compensation replayed effects: status=%q error=%v operations=%v", status, err, operations.values)
	}
}

func TestV1CompensationResetsConfirmedUnpushedCommitBeforeCleanup(t *testing.T) {
	store, evidence := newV1CompensationOperationFixture(t)
	if err := store.RecordConfigWritePending(&evidence); err != nil {
		t.Fatalf("RecordConfigWritePending: %v", err)
	}
	if err := store.ConfirmConfigWrite(&evidence); err != nil {
		t.Fatalf("ConfirmConfigWrite: %v", err)
	}
	if err := store.PlanFailedExecution(&evidence, GitReleaseState{
		PreHead:     "before",
		ReleaseHead: "release",
	}); err != nil {
		t.Fatalf("PlanFailedExecution: %v", err)
	}
	operations := &v1CompensationOperationRecorder{}
	files := recordingOrderedV1ConfigRestorer{operations: operations}
	git := observingOrderedV1CompensationGit{store: store, operations: operations}

	status, err := continueV1Compensation(store, files, git, observingOrderedV1ReleaseRemover{store: store, operations: operations}, &evidence)

	if err != nil || status != V1CompensationContinuationCompleted {
		t.Fatalf("continuation status=%q error=%v", status, err)
	}
	want := []string{"restore V1 config", "hard reset before", "clean untracked"}
	if got := strings.Join(operations.values, "\n"); got != strings.Join(want, "\n") {
		t.Fatalf("reset compensation operations:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
}

func newV1CompensationOperationFixture(t *testing.T) (V1CompensationEvidenceStore, V1CompensationEvidence) {
	t.Helper()
	root := t.TempDir()
	commonDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(commonDir, 0700); err != nil {
		t.Fatalf("create git common dir: %v", err)
	}
	store := NewV1CompensationEvidenceStore(root, fixedV1CompensationGitRunner{commonDir: commonDir})
	store.clock = fixedV1CompensationClock{now: v1CompensationTestTime}
	evidence := newV1CompensationEvidenceFixtureAt(t, root, V1CompensationExecutorGoReleaser)
	evidence.Release.Status = V1ReleaseEffectFailed
	evidence.Compensation.Actions.RestoreConfig.Status = V1CompensationActionPlanned
	if err := store.Create(evidence); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return store, evidence
}

type observingV1CompensationFiles struct {
	store          V1CompensationEvidenceStore
	restoreErr     error
	observedStatus V1CompensationActionStatus
	calls          int
}

func (*observingV1CompensationFiles) Read(string) ([]byte, error) { return nil, nil }

func (files *observingV1CompensationFiles) Restore(string, []byte) error {
	files.calls++
	evidence, err := files.store.LoadCurrent()
	if err != nil {
		return err
	}
	files.observedStatus = evidence.Compensation.Actions.RestoreConfig.Status
	return files.restoreErr
}

func (*observingV1CompensationFiles) VerifyVersion(string, string) error { return nil }

type countingV1CompensationReleaseRemover struct {
	calls int
}

func (remover *countingV1CompensationReleaseRemover) Delete(string, string) error {
	remover.calls++
	return nil
}

type recordingOrderedV1ConfigRestorer struct {
	operations *v1CompensationOperationRecorder
}

func (recordingOrderedV1ConfigRestorer) Read(string) ([]byte, error) { return nil, nil }

func (restorer recordingOrderedV1ConfigRestorer) Restore(string, []byte) error {
	return restorer.operations.record("restore V1 config")
}

func (recordingOrderedV1ConfigRestorer) VerifyVersion(string, string) error { return nil }

type observingOrderedV1CompensationGit struct {
	operations *v1CompensationOperationRecorder
	store      V1CompensationEvidenceStore
}

func (git observingOrderedV1CompensationGit) DeleteLocalTag(_ string, tag string) error {
	return git.recordPending(V1CompensationDeleteLocalTag, "delete local tag "+tag)
}

func (git observingOrderedV1CompensationGit) DeleteRemoteTag(_ string, tag string) error {
	return git.recordPending(V1CompensationDeleteRemoteTag, "delete remote tag "+tag)
}

func (git observingOrderedV1CompensationGit) RevertCommit(_ string, hash string) error {
	return git.recordPending(V1CompensationRevertReleaseCommit, "revert commit "+hash)
}

func (git observingOrderedV1CompensationGit) PushCommits(string) error {
	return git.recordPending(V1CompensationPushRevertCommit, "push commits")
}

func (git observingOrderedV1CompensationGit) HardResetTo(_ string, hash string) error {
	return git.recordPending(V1CompensationResetReleaseCommit, "hard reset "+hash)
}

func (git observingOrderedV1CompensationGit) CleanUntracked(string) error {
	return git.recordPending(V1CompensationCleanUntrackedReleaseFiles, "clean untracked")
}

func (git observingOrderedV1CompensationGit) recordPending(action V1CompensationAction, operation string) error {
	evidence, err := git.store.LoadCurrent()
	if err != nil {
		return err
	}
	if evidence.Compensation.PendingAction != action || evidence.Compensation.statusFor(action) != V1CompensationActionPending {
		return errors.New("compensation side effect ran without pending evidence")
	}
	return git.operations.record(operation)
}

type observingOrderedV1ReleaseRemover struct {
	operations *v1CompensationOperationRecorder
	store      V1CompensationEvidenceStore
}

func (remover observingOrderedV1ReleaseRemover) Delete(_ string, tag string) error {
	evidence, err := remover.store.LoadCurrent()
	if err != nil {
		return err
	}
	if evidence.Compensation.PendingAction != V1CompensationDeleteGitHubRelease ||
		evidence.Compensation.Actions.DeleteGitHubRelease.Status != V1CompensationActionPending {
		return errors.New("GitHub release deletion ran without pending evidence")
	}
	return remover.operations.record("delete GitHub release " + tag)
}
