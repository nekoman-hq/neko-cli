package release

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestGitHubActionsReleaseUseCaseRunsNamedOperationsInOrder(t *testing.T) {
	recording := newRecordingGitHubActionsReleaseUseCase(true)
	result, err := recording.useCase.Run(context.Background(), releaseUseCaseTestContext())
	if err != nil {
		t.Fatalf("run release use case: %v", err)
	}
	want := []string{
		"resolve-token",
		"plan",
		"validate-preflight",
		"prepare-execution",
		"apply-materialization",
		"write-state",
		"stage-files",
		"create-commit",
		"create-tag",
		"prepare-dispatch",
		"push-commit",
		"push-tag",
		"dispatch-workflow",
		"confirm-handoff",
	}
	if !reflect.DeepEqual(recording.calls, want) {
		t.Fatalf("operation order = %#v, want %#v", recording.calls, want)
	}
	if recording.dispatchedToken != "test-token" {
		t.Fatalf("dispatch token = %q", recording.dispatchedToken)
	}
	if result.ExecutionState != ReleaseExecutionHandoffReady || result.DispatchState != DispatchJournalAccepted {
		t.Fatalf("unexpected result states: execution=%s dispatch=%s", result.ExecutionState, result.DispatchState)
	}
}

func TestGitHubActionsReleaseUseCaseStopsAtEveryFailedDependency(t *testing.T) {
	steps := []string{
		"resolve-token",
		"plan",
		"validate-preflight",
		"prepare-execution",
		"apply-materialization",
		"write-state",
		"stage-files",
		"create-commit",
		"create-tag",
		"prepare-dispatch",
		"push-commit",
		"push-tag",
		"dispatch-workflow",
		"confirm-handoff",
	}
	for index, failedStep := range steps {
		t.Run(failedStep, func(t *testing.T) {
			recording := newRecordingGitHubActionsReleaseUseCase(true)
			recording.failAt = failedStep
			result, err := recording.useCase.Run(context.Background(), releaseUseCaseTestContext())
			if !errors.Is(err, errReleaseUseCaseDependency) {
				t.Fatalf("error = %v, want dependency failure", err)
			}
			if result != nil {
				t.Fatalf("result = %#v, want nil", result)
			}
			want := append([]string(nil), steps[:index+1]...)
			if failedStep == "write-state" {
				want = append(want, "restore-materialization")
			}
			if failedStep == "stage-files" {
				want = append(want, "restore-state", "restore-materialization")
			}
			if !reflect.DeepEqual(recording.calls, want) {
				t.Fatalf("calls = %#v, want %#v", recording.calls, want)
			}
		})
	}
}

func TestGitHubActionsReleaseUseCaseDoesNotConfirmRejectedDispatch(t *testing.T) {
	recording := newRecordingGitHubActionsReleaseUseCase(false)
	result, err := recording.useCase.Run(context.Background(), releaseUseCaseTestContext())
	if err == nil {
		t.Fatal("expected rejected dispatch error")
	}
	if result == nil {
		t.Fatal("expected safe rejected-dispatch result")
		return
	}
	if result.ExecutionState != ReleaseExecutionTagPushed || result.DispatchState != DispatchJournalRejected {
		t.Fatalf("unexpected result states: execution=%s dispatch=%s", result.ExecutionState, result.DispatchState)
	}
	if result.DispatchJournalPath != "/tmp/dispatch-journal.json" {
		t.Fatalf("dispatch journal path = %q", result.DispatchJournalPath)
	}
	for _, call := range recording.calls {
		if call == "confirm-handoff" {
			t.Fatalf("handoff confirmed after rejected dispatch: %#v", recording.calls)
		}
	}
}

func TestGitHubActionsReleaseUseCaseDoesNotExposeToken(t *testing.T) {
	const token = "stage-three-super-secret-token"
	recording := newRecordingGitHubActionsReleaseUseCase(true)
	recording.token = token
	stderr := captureReleaseUseCaseStderr(t, func() {
		result, err := recording.useCase.Run(context.Background(), releaseUseCaseTestContext())
		if err != nil {
			t.Fatalf("run release use case: %v", err)
		}
		if strings.Contains(fmt.Sprintf("%+v", result), token) {
			t.Fatal("release result exposed token")
		}
	})
	if strings.Contains(stderr, token) {
		t.Fatalf("release logs exposed token: %s", stderr)
	}
}

var errReleaseUseCaseDependency = errors.New("release use case dependency failed")

type recordingGitHubActionsReleaseUseCase struct {
	failAt           string
	dispatchedToken  string
	token            string
	useCase          *githubActionsReleaseUseCase
	calls            []string
	dispatchAccepted bool
}

func newRecordingGitHubActionsReleaseUseCase(dispatchAccepted bool) *recordingGitHubActionsReleaseUseCase {
	recording := &recordingGitHubActionsReleaseUseCase{dispatchAccepted: dispatchAccepted, token: "test-token"}
	recording.useCase = &githubActionsReleaseUseCase{
		tokenResolver:      recordingReleaseTokenResolver{recording},
		planner:            recordingReleasePlanner{recording},
		preflightValidator: recordingReleasePreflight{recording},
		executionPreparer:  recordingReleaseExecutionPreparer{recording},
		materialization:    recordingReleaseMaterialization{recording},
		stateWriter:        recordingReleaseStateWriter{recording},
		fileStager:         recordingReleaseFileStager{recording},
		commitCreator:      recordingReleaseCommitCreator{recording},
		tagCreator:         recordingReleaseTagCreator{recording},
		dispatchPreparer:   recordingReleaseDispatchPreparer{recording},
		commitPusher:       recordingReleaseCommitPusher{recording},
		tagPusher:          recordingReleaseTagPusher{recording},
		workflowDispatcher: recordingReleaseWorkflowDispatcher{recording},
		handoffConfirmer:   recordingReleaseHandoffConfirmer{recording},
	}
	return recording
}

func (recording *recordingGitHubActionsReleaseUseCase) call(name string) error {
	recording.calls = append(recording.calls, name)
	if recording.failAt == name {
		return errReleaseUseCaseDependency
	}
	return nil
}

type recordingReleaseTokenResolver struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (resolver recordingReleaseTokenResolver) ResolveGitHubActionsDispatchToken(context.Context) (GitHubActionsDispatchToken, error) {
	if err := resolver.recording.call("resolve-token"); err != nil {
		return GitHubActionsDispatchToken{}, err
	}
	return NewGitHubActionsDispatchToken(resolver.recording.token)
}

type recordingReleasePlanner struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (planner recordingReleasePlanner) Plan(*ReleaseExecutionContext) (plannedGitHubActionsRelease, error) {
	if err := planner.recording.call("plan"); err != nil {
		return plannedGitHubActionsRelease{}, err
	}
	return plannedGitHubActionsRelease{MaterializationPlan: &MaterializationPlan{}}, nil
}

type recordingReleasePreflight struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (preflight recordingReleasePreflight) Validate(*ReleaseExecutionContext, KnownReleaseFiles) (validatedGitHubActionsReleasePreflight, error) {
	if err := preflight.recording.call("validate-preflight"); err != nil {
		return validatedGitHubActionsReleasePreflight{}, err
	}
	return validatedGitHubActionsReleasePreflight{
		Git:           GitReleasePreflight{Remote: "origin", UpstreamBranch: "main"},
		RemoteURL:     "https://github.com/nekoman-hq/neko-cli.git",
		BaseCommitSHA: "1111111111111111111111111111111111111111",
	}, nil
}

type recordingReleaseExecutionPreparer struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (preparer recordingReleaseExecutionPreparer) Prepare(*ReleaseExecutionContext, KnownReleaseFiles, validatedGitHubActionsReleasePreflight) (preparedGitHubActionsReleaseExecution, error) {
	if err := preparer.recording.call("prepare-execution"); err != nil {
		return preparedGitHubActionsReleaseExecution{}, err
	}
	return preparedGitHubActionsReleaseExecution{
		Identity: ReleaseExecutionIdentity{SHA256: "execution-identity"},
		Path:     "/tmp/execution-journal.json",
	}, nil
}

type recordingReleaseMaterialization struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (operation recordingReleaseMaterialization) Apply(preparedGitHubActionsReleaseExecution, *MaterializationPlan) (releaseMaterializationRollback, error) {
	if err := operation.recording.call("apply-materialization"); err != nil {
		return nil, err
	}
	return recordingMaterializationRollback(operation), nil
}

type recordingMaterializationRollback struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (rollback recordingMaterializationRollback) Restore() error {
	rollback.recording.calls = append(rollback.recording.calls, "restore-materialization")
	return nil
}

type recordingReleaseStateWriter struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (operation recordingReleaseStateWriter) Write(_ *ReleaseExecutionContext, _ preparedGitHubActionsReleaseExecution, materialization releaseMaterializationRollback) (releaseStateRollback, error) {
	if err := operation.recording.call("write-state"); err != nil {
		_ = materialization.Restore()
		return nil, err
	}
	return recordingStateRollback(operation), nil
}

type recordingStateRollback struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (rollback recordingStateRollback) RestoreSnapshot() error {
	rollback.recording.calls = append(rollback.recording.calls, "restore-state")
	return nil
}

type recordingReleaseFileStager struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (operation recordingReleaseFileStager) Stage(_ *ReleaseExecutionContext, _ preparedGitHubActionsReleaseExecution, _ KnownReleaseFiles, state releaseStateRollback, materialization releaseMaterializationRollback) error {
	if err := operation.recording.call("stage-files"); err != nil {
		_ = state.RestoreSnapshot()
		_ = materialization.Restore()
		return err
	}
	return nil
}

type recordingReleaseCommitCreator struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (operation recordingReleaseCommitCreator) Create(*ReleaseExecutionContext, preparedGitHubActionsReleaseExecution, KnownReleaseFiles, releaseStateRollback, releaseMaterializationRollback) (string, error) {
	if err := operation.recording.call("create-commit"); err != nil {
		return "", err
	}
	return "2222222222222222222222222222222222222222", nil
}

type recordingReleaseTagCreator struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (operation recordingReleaseTagCreator) Create(*ReleaseExecutionContext, preparedGitHubActionsReleaseExecution, string) error {
	return operation.recording.call("create-tag")
}

type recordingReleaseDispatchPreparer struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (operation recordingReleaseDispatchPreparer) Prepare(*ReleaseExecutionContext, preparedGitHubActionsReleaseExecution, validatedGitHubActionsReleasePreflight, KnownReleaseFiles, string) (preparedGitHubActionsReleaseDispatch, error) {
	if err := operation.recording.call("prepare-dispatch"); err != nil {
		return preparedGitHubActionsReleaseDispatch{}, err
	}
	return preparedGitHubActionsReleaseDispatch{Request: &ReleaseDispatchRequest{}, Path: "/tmp/dispatch-journal.json"}, nil
}

type recordingReleaseCommitPusher struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (operation recordingReleaseCommitPusher) Push(*ReleaseExecutionContext, preparedGitHubActionsReleaseExecution, validatedGitHubActionsReleasePreflight, string) error {
	return operation.recording.call("push-commit")
}

type recordingReleaseTagPusher struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (operation recordingReleaseTagPusher) Push(*ReleaseExecutionContext, preparedGitHubActionsReleaseExecution, validatedGitHubActionsReleasePreflight, string) error {
	return operation.recording.call("push-tag")
}

type recordingReleaseWorkflowDispatcher struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (operation recordingReleaseWorkflowDispatcher) Dispatch(_ context.Context, _ *ReleaseExecutionContext, _ preparedGitHubActionsReleaseExecution, _ preparedGitHubActionsReleaseDispatch, token GitHubActionsDispatchToken) (*GitHubActionsDispatchResult, error) {
	operation.recording.dispatchedToken = token.secretValue()
	if err := operation.recording.call("dispatch-workflow"); err != nil {
		return nil, err
	}
	if !operation.recording.dispatchAccepted {
		return &GitHubActionsDispatchResult{
			JournalPath:      "/tmp/dispatch-journal.json",
			State:            DispatchJournalRejected,
			RecoveryGuidance: "inspect rejected dispatch",
		}, nil
	}
	return &GitHubActionsDispatchResult{
		JournalPath:      "/tmp/dispatch-journal.json",
		State:            DispatchJournalAccepted,
		Accepted:         true,
		RecoveryGuidance: "dispatch accepted",
	}, nil
}

type recordingReleaseHandoffConfirmer struct {
	recording *recordingGitHubActionsReleaseUseCase
}

func (operation recordingReleaseHandoffConfirmer) Confirm(preparedGitHubActionsReleaseExecution) error {
	return operation.recording.call("confirm-handoff")
}

func releaseUseCaseTestContext() *ReleaseExecutionContext {
	return &ReleaseExecutionContext{
		Unit:        releaseconfig.ReleaseUnit{ID: "neko"},
		NextVersion: "1.2.3",
		Tag:         "neko/v1.2.3",
		Workflow:    ".github/workflows/release.yml",
	}
}

func captureReleaseUseCaseStderr(t *testing.T, run func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	original := os.Stderr
	os.Stderr = writer
	defer func() { os.Stderr = original }()

	run()
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close stderr writer: %v", closeErr)
	}
	os.Stderr = original
	output, readErr := io.ReadAll(reader)
	if readErr != nil {
		t.Fatalf("read stderr: %v", readErr)
	}
	if closeErr := reader.Close(); closeErr != nil {
		t.Fatalf("close stderr reader: %v", closeErr)
	}
	return string(output)
}
