package release

import (
	"errors"
	"strings"
	"testing"
)

func TestV1RollbackRepeatedInvocationRepeatsDestructiveEffects(t *testing.T) {
	operations := &v1CompensationOperationRecorder{}
	rollback := &V1ReleaseRollback{
		git:      recordingV1CompensationGit{operations: operations},
		releases: recordingV1CompensationReleaseRemover{operations: operations},
	}
	state := GitReleaseState{
		PreHead:              "before",
		ReleaseHead:          "release",
		TagName:              "v1.2.4",
		GitHubReleaseTag:     "v1.2.4",
		PushedCommit:         true,
		PushedTag:            true,
		CreatedGitHubRelease: true,
	}

	if err := rollback.Rollback("/repo", state); err != nil {
		t.Fatalf("first Rollback: %v", err)
	}
	if err := rollback.Rollback("/repo", state); err != nil {
		t.Fatalf("second Rollback: %v", err)
	}

	onePass := []string{
		"delete GitHub release v1.2.4",
		"delete local tag v1.2.4",
		"delete remote tag v1.2.4",
		"revert commit release",
		"push commits",
		"clean untracked",
	}
	want := append(append([]string(nil), onePass...), onePass...)
	if got := strings.Join(operations.values, "\n"); got != strings.Join(want, "\n") {
		t.Fatalf("repeated rollback operations:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
}

func TestV1RollbackRepeatedAfterPartialFailureReplaysCompletedEffects(t *testing.T) {
	operations := &v1CompensationOperationRecorder{failOnce: "delete remote tag v1.2.4"}
	rollback := &V1ReleaseRollback{
		git:      recordingV1CompensationGit{operations: operations},
		releases: recordingV1CompensationReleaseRemover{operations: operations},
	}
	state := GitReleaseState{
		TagName:              "v1.2.4",
		GitHubReleaseTag:     "v1.2.4",
		PushedTag:            true,
		CreatedGitHubRelease: true,
	}

	if err := rollback.Rollback("/repo", state); err == nil {
		t.Fatal("first Rollback succeeded at injected remote-tag failure")
	}
	if err := rollback.Rollback("/repo", state); err != nil {
		t.Fatalf("second Rollback: %v", err)
	}

	want := []string{
		"delete GitHub release v1.2.4",
		"delete local tag v1.2.4",
		"delete remote tag v1.2.4",
		"delete GitHub release v1.2.4",
		"delete local tag v1.2.4",
		"delete remote tag v1.2.4",
		"clean untracked",
	}
	if got := strings.Join(operations.values, "\n"); got != strings.Join(want, "\n") {
		t.Fatalf("partial retry operations:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
}

func TestV1RollbackCannotResumeWhenExecutorEvidenceIsLost(t *testing.T) {
	operations := &v1CompensationOperationRecorder{failOnce: "delete remote tag v1.2.4"}
	rollback := &V1ReleaseRollback{
		git:      recordingV1CompensationGit{operations: operations},
		releases: recordingV1CompensationReleaseRemover{operations: operations},
	}
	state := GitReleaseState{TagName: "v1.2.4", PushedTag: true}

	if err := rollback.Rollback("/repo", state); err == nil {
		t.Fatal("Rollback succeeded at injected interruption boundary")
	}
	beforeRestart := append([]string(nil), operations.values...)

	if err := rollback.Rollback("/repo", GitReleaseState{}); err != nil {
		t.Fatalf("Rollback after state loss: %v", err)
	}
	if got := strings.Join(operations.values, "\n"); got != strings.Join(beforeRestart, "\n") {
		t.Fatalf("state-free restart unexpectedly resumed compensation:\n%s", got)
	}
}

type v1CompensationOperationRecorder struct {
	values   []string
	failOnce string
}

func (recorder *v1CompensationOperationRecorder) record(operation string) error {
	recorder.values = append(recorder.values, operation)
	if operation == recorder.failOnce {
		recorder.failOnce = ""
		return errors.New("injected compensation interruption")
	}
	return nil
}

type recordingV1CompensationGit struct {
	operations *v1CompensationOperationRecorder
}

func (git recordingV1CompensationGit) DeleteLocalTag(_ string, tag string) error {
	return git.operations.record("delete local tag " + tag)
}

func (git recordingV1CompensationGit) DeleteRemoteTag(_ string, tag string) error {
	return git.operations.record("delete remote tag " + tag)
}

func (git recordingV1CompensationGit) RevertCommit(_ string, hash string) error {
	return git.operations.record("revert commit " + hash)
}

func (git recordingV1CompensationGit) CreateFallbackCommit(_ string, message string) error {
	return git.operations.record("create fallback commit " + message)
}

func (git recordingV1CompensationGit) PushCommits(string) error {
	return git.operations.record("push commits")
}

func (git recordingV1CompensationGit) HardResetTo(_ string, hash string) error {
	return git.operations.record("hard reset " + hash)
}

func (git recordingV1CompensationGit) CleanUntracked(string) error {
	return git.operations.record("clean untracked")
}

type recordingV1CompensationReleaseRemover struct {
	operations *v1CompensationOperationRecorder
}

func (remover recordingV1CompensationReleaseRemover) Delete(_ string, tag string) error {
	return remover.operations.record("delete GitHub release " + tag)
}
