package release

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestV1ReleaseInvocationContinuesPendingLocalCompensationBeforeNewExecution(t *testing.T) {
	events := []string{}
	executor := &recordingV1Executor{events: &events}
	useCase := configuredV1CompensationIntegrationUseCase(t, &events, executor)
	stores, ok := useCase.compensationStores.(fixedV1CompensationEvidenceStores)
	if !ok {
		t.Fatalf("compensation stores = %T", useCase.compensationStores)
	}
	store := stores.store
	evidence := newV1CompensationEvidenceFixture(t, V1CompensationExecutorGoReleaser)
	evidence.Release.Status = V1ReleaseEffectFailed
	evidence.Compensation.Status = V1CompensationInProgress
	evidence.Compensation.PendingAction = V1CompensationRestoreConfig
	evidence.Compensation.Actions.RestoreConfig.Status = V1CompensationActionPending
	if err := store.Create(evidence); err != nil {
		t.Fatalf("Create interrupted evidence: %v", err)
	}

	result, failure := useCase.Execute(V1ReleaseExecutionRequest{})

	if failure != nil || result == nil {
		t.Fatalf("Execute result=%#v failure=%#v", result, failure)
	}
	wantPrefix := []string{"report-rollback-started", "restore-config", "report-rollback-completed", "report-planning-started"}
	if len(events) < len(wantPrefix) || strings.Join(events[:len(wantPrefix)], "\n") != strings.Join(wantPrefix, "\n") {
		t.Fatalf("recovery prefix = %v, want %v", events, wantPrefix)
	}
	loaded, err := store.LoadCurrent()
	if err != nil {
		t.Fatalf("LoadCurrent: %v", err)
	}
	if loaded.Release.Status != V1ReleaseEffectSucceeded || loaded.Compensation.Status != V1CompensationCompleted {
		t.Fatalf("completed release evidence = %#v", loaded)
	}
}

func TestV1ReleaseInvocationRefusesPendingRemoteCompensationBeforePlanning(t *testing.T) {
	events := []string{}
	executor := &recordingV1Executor{events: &events}
	useCase := configuredV1CompensationIntegrationUseCase(t, &events, executor)
	stores, ok := useCase.compensationStores.(fixedV1CompensationEvidenceStores)
	if !ok {
		t.Fatalf("compensation stores = %T", useCase.compensationStores)
	}
	store := stores.store
	evidence := newV1CompensationEvidenceFixture(t, V1CompensationExecutorReleaseIt)
	evidence.Release.Status = V1ReleaseEffectFailed
	evidence.Release.Git.GitHubReleaseTag = "v1.2.4"
	evidence.Compensation.Status = V1CompensationInProgress
	evidence.Compensation.PendingAction = V1CompensationDeleteGitHubRelease
	evidence.Compensation.Actions.DeleteGitHubRelease.Status = V1CompensationActionPending
	if err := store.Create(evidence); err != nil {
		t.Fatalf("Create interrupted evidence: %v", err)
	}
	remover := &countingV1CompensationReleaseRemover{}
	useCase.compensationReleases = remover

	result, failure := useCase.Execute(V1ReleaseExecutionRequest{})

	if result != nil || failure == nil || failure.Code != "RELEASE_FAILED" || failure.Class != v1ReleaseRollbackFailure {
		t.Fatalf("Execute result=%#v failure=%#v", result, failure)
	}
	if !strings.Contains(failure.Cause.Error(), "manual recovery required") ||
		!strings.Contains(failure.Cause.Error(), filepath.Join("neko", "release", "v1-compensation", "current.json")) {
		t.Fatalf("manual recovery cause = %v", failure.Cause)
	}
	if remover.calls != 0 {
		t.Fatalf("ambiguous remote deletion calls = %d, want 0", remover.calls)
	}
	if got := strings.Join(events, "\n"); got != "report-rollback-started" {
		t.Fatalf("events before refusal = %v", events)
	}
}

func configuredV1CompensationIntegrationUseCase(
	t *testing.T,
	events *[]string,
	executor *recordingV1Executor,
) v1ReleaseExecutionUseCase {
	t.Helper()
	useCase := v1ReleaseExecutionUseCase{
		previewPlans:   &recordingV1PreviewPlans{events: events, plan: testV1ReleasePlan("1.2.3", "1.2.4")},
		executionPlans: &recordingV1ExecutionPlans{events: events, plan: testV1ReleasePlan("1.2.3", "1.2.4")},
		requirements:   &recordingV1Requirements{events: events},
		preflight:      &recordingV1Preflight{events: events},
		materializer:   &recordingV1Materializer{events: events},
		executors:      &recordingV1ExecutorCatalog{events: events, executor: executor},
		reporter:       recordingV1Reporter{events: events},
	}
	configureV1CompensationUseCase(t, &useCase, events)
	return useCase
}
