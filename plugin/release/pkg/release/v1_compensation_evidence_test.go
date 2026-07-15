package release

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestV1CompensationEvidenceRejectsUnknownPersistedClassifications(t *testing.T) {
	tests := []struct {
		name   string
		change func(*V1CompensationEvidence)
		want   string
	}{
		{
			name: "schema",
			change: func(evidence *V1CompensationEvidence) {
				evidence.SchemaVersion = 99
			},
			want: "schema version",
		},
		{
			name: "executor",
			change: func(evidence *V1CompensationEvidence) {
				evidence.Identity.Executor = "unknown"
			},
			want: "unknown V1 compensation executor",
		},
		{
			name: "release status",
			change: func(evidence *V1CompensationEvidence) {
				evidence.Release.Status = "unknown"
			},
			want: "unknown V1 release effect status",
		},
		{
			name: "action",
			change: func(evidence *V1CompensationEvidence) {
				evidence.Compensation.PendingAction = "unknown"
			},
			want: "unknown pending V1 compensation action",
		},
		{
			name: "action status",
			change: func(evidence *V1CompensationEvidence) {
				evidence.Compensation.Actions.DeleteRemoteTag.Status = "unknown"
			},
			want: "unknown status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evidence := newV1CompensationEvidenceFixture(t, V1CompensationExecutorGoReleaser)
			tt.change(&evidence)
			if err := evidence.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestV1CompensationEvidenceRejectsTamperedIdentityAndConfig(t *testing.T) {
	evidence := newV1CompensationEvidenceFixture(t, V1CompensationExecutorJReleaser)
	evidence.Identity.IntendedVersion = "9.9.9"
	if err := evidence.Validate(); err == nil || !strings.Contains(err.Error(), "identity hash mismatch") {
		t.Fatalf("identity validation error = %v", err)
	}

	evidence = newV1CompensationEvidenceFixture(t, V1CompensationExecutorJReleaser)
	evidence.OriginalConfig.Content = `{"version":"9.9.9"}`
	if err := evidence.Validate(); err == nil || !strings.Contains(err.Error(), "config evidence hash mismatch") {
		t.Fatalf("config validation error = %v", err)
	}
}

func TestV1CompensationStoreUsesPrivateGitCommonDirectoryFile(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(commonDir, 0700); err != nil {
		t.Fatalf("create git common dir: %v", err)
	}
	store := NewV1CompensationEvidenceStore(root, fixedV1CompensationGitRunner{commonDir: commonDir})
	store.clock = fixedV1CompensationClock{now: v1CompensationTestTime.Add(time.Minute)}
	evidence := newV1CompensationEvidenceFixtureAt(t, root, V1CompensationExecutorReleaseIt)

	if err := store.Create(evidence); err != nil {
		t.Fatalf("Create: %v", err)
	}
	path, err := store.CurrentPath()
	if err != nil {
		t.Fatalf("CurrentPath: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat evidence: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("evidence mode = %o, want 600", got)
	}
	directoryInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat evidence directory: %v", err)
	}
	if got := directoryInfo.Mode().Perm(); got != 0700 {
		t.Fatalf("evidence directory mode = %o, want 700", got)
	}
	loaded, err := store.LoadCurrent()
	if err != nil {
		t.Fatalf("LoadCurrent: %v", err)
	}
	if loaded.Identity != evidence.Identity || loaded.OriginalConfig != evidence.OriginalConfig {
		t.Fatalf("loaded identity/config changed: %#v", loaded)
	}
}

func TestV1CompensationStoreHandlesMissingCorruptAndUnknownEvidenceConservatively(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(commonDir, 0700); err != nil {
		t.Fatalf("create git common dir: %v", err)
	}
	store := NewV1CompensationEvidenceStore(root, fixedV1CompensationGitRunner{commonDir: commonDir})
	if _, err := store.LoadCurrent(); !errors.Is(err, ErrV1CompensationEvidenceNotFound) {
		t.Fatalf("missing LoadCurrent error = %v", err)
	}
	if evidence, found, err := store.FindUnresolved(); err != nil || found || evidence != nil {
		t.Fatalf("missing FindUnresolved = evidence=%#v found=%t err=%v", evidence, found, err)
	}

	path, err := store.CurrentPath()
	if err != nil {
		t.Fatalf("CurrentPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("create evidence directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"schemaVersion":`), 0600); err != nil {
		t.Fatalf("write corrupt evidence: %v", err)
	}
	if _, err := store.LoadCurrent(); err == nil || !strings.Contains(err.Error(), "decode V1 compensation evidence") {
		t.Fatalf("corrupt LoadCurrent error = %v", err)
	}

	data := []byte(`{"schemaVersion":1,"unexpected":true}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write unknown-field evidence: %v", err)
	}
	if _, err := store.LoadCurrent(); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown-field LoadCurrent error = %v", err)
	}
}

func TestV1CompensationEvidenceNeverPersistsProcessSecret(t *testing.T) {
	const secret = "NEKO_V1_COMPENSATION_SECRET_MUST_NOT_APPEAR"
	t.Setenv("GITHUB_TOKEN", secret)
	root := t.TempDir()
	commonDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(commonDir, 0700); err != nil {
		t.Fatalf("create git common dir: %v", err)
	}
	store := NewV1CompensationEvidenceStore(root, fixedV1CompensationGitRunner{commonDir: commonDir})
	evidence := newV1CompensationEvidenceFixtureAt(t, root, V1CompensationExecutorGoReleaser)
	if err := store.Create(evidence); err != nil {
		t.Fatalf("Create: %v", err)
	}
	path, err := store.CurrentPath()
	if err != nil {
		t.Fatalf("CurrentPath: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read evidence: %v", err)
	}
	if strings.Contains(string(data), secret) {
		t.Fatal("process secret appeared in V1 compensation evidence")
	}
}

func TestV1CompensationStorePlansFixedActionsFromConfirmedExecutorEvidence(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(commonDir, 0700); err != nil {
		t.Fatalf("create git common dir: %v", err)
	}
	store := NewV1CompensationEvidenceStore(root, fixedV1CompensationGitRunner{commonDir: commonDir})
	store.clock = fixedV1CompensationClock{now: v1CompensationTestTime.Add(time.Minute)}
	evidence := newV1CompensationEvidenceFixtureAt(t, root, V1CompensationExecutorGoReleaser)
	if err := store.Create(evidence); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.RecordConfigWritePending(&evidence); err != nil {
		t.Fatalf("RecordConfigWritePending: %v", err)
	}
	if err := store.ConfirmConfigWrite(&evidence); err != nil {
		t.Fatalf("ConfirmConfigWrite: %v", err)
	}
	if err := store.RecordExecutorPending(&evidence); err != nil {
		t.Fatalf("RecordExecutorPending: %v", err)
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

	loaded, err := store.LoadCurrent()
	if err != nil {
		t.Fatalf("LoadCurrent: %v", err)
	}
	if loaded.Release.Status != V1ReleaseEffectFailed || loaded.Release.Git.ReleaseHead != "release" {
		t.Fatalf("release evidence = %#v", loaded.Release)
	}
	if loaded.Compensation.Actions.RestoreConfig.Status != V1CompensationActionPlanned ||
		loaded.Compensation.Actions.DeleteGitHubRelease.Status != V1CompensationActionPlanned ||
		loaded.Compensation.Actions.DeleteLocalTag.Status != V1CompensationActionPlanned ||
		loaded.Compensation.Actions.DeleteRemoteTag.Status != V1CompensationActionPlanned ||
		loaded.Compensation.Actions.RevertReleaseCommit.Status != V1CompensationActionPlanned ||
		loaded.Compensation.Actions.PushRevertCommit.Status != V1CompensationActionPlanned ||
		loaded.Compensation.Actions.ResetReleaseCommit.Status != V1CompensationActionNotRequired ||
		loaded.Compensation.Actions.CleanUntrackedFiles.Status != V1CompensationActionPlanned {
		t.Fatalf("planned compensation actions = %#v", loaded.Compensation.Actions)
	}
}

var v1CompensationTestTime = time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

func newV1CompensationEvidenceFixture(t *testing.T, executor V1CompensationExecutor) V1CompensationEvidence {
	t.Helper()
	return newV1CompensationEvidenceFixtureAt(t, "/repo", executor)
}

func newV1CompensationEvidenceFixtureAt(t *testing.T, root string, executor V1CompensationExecutor) V1CompensationEvidence {
	t.Helper()
	evidence, err := newV1CompensationEvidence(V1ReleasePlan{
		RepositoryRoot: root,
		CurrentVersion: "1.2.3",
		NextVersion:    "1.2.4",
		Tag:            "v1.2.4",
		Executor:       string(executor),
	}, filepath.Join(root, ".release.neko.json"), []byte(`{"version":"1.2.3"}`), v1CompensationTestTime)
	if err != nil {
		t.Fatalf("newV1CompensationEvidence: %v", err)
	}
	return evidence
}

type fixedV1CompensationGitRunner struct {
	commonDir string
}

func (runner fixedV1CompensationGitRunner) Run(_ string, args ...string) (string, error) {
	if strings.Join(args, " ") != "rev-parse --git-common-dir" {
		return "", errors.New("unexpected git command")
	}
	return runner.commonDir, nil
}

type fixedV1CompensationClock struct {
	now time.Time
}

func (clock fixedV1CompensationClock) Now() time.Time { return clock.now }
