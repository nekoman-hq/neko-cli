package release

import (
	"errors"
	"strings"
	"testing"
)

type recordingV1PreviewPlans struct { //nolint:govet // Test double fields follow configured behavior order.
	events *[]string
	plan   V1ReleasePlan
	err    error
}

func (plans *recordingV1PreviewPlans) BuildPreviewPlan(V1ReleaseIntent) (V1ReleasePlan, error) {
	*plans.events = append(*plans.events, "preview-plan")
	return plans.plan, plans.err
}

type recordingV1ExecutionPlans struct { //nolint:govet // Test double fields follow configured behavior order.
	events *[]string
	plan   V1ReleasePlan
	err    error
}

func (plans *recordingV1ExecutionPlans) BuildExecutionPlan(V1ReleaseIntent) (V1ReleasePlan, error) {
	*plans.events = append(*plans.events, "execution-plan")
	return plans.plan, plans.err
}

type recordingV1Requirements struct {
	events *[]string
	err    error
}

func (requirements *recordingV1Requirements) Validate(V1ReleaseIntent) error {
	*requirements.events = append(*requirements.events, "requirements")
	return requirements.err
}

type recordingV1Preflight struct {
	events  *[]string
	failure *V1ReleaseFailure
}

func (preflight *recordingV1Preflight) Check(V1ReleaseIntent) *V1ReleaseFailure {
	*preflight.events = append(*preflight.events, "preflight")
	return preflight.failure
}

type recordingV1Materializer struct {
	events     *[]string
	writeErr   error
	restoreErr error
}

func (materializer *recordingV1Materializer) WritePlannedVersion(V1ReleaseIntent, V1ReleasePlan) error {
	*materializer.events = append(*materializer.events, "write-config")
	return materializer.writeErr
}

func (materializer *recordingV1Materializer) RestorePreviousVersion(V1ReleaseIntent, V1ReleasePlan) error {
	*materializer.events = append(*materializer.events, "restore-config")
	return materializer.restoreErr
}

type recordingV1ExecutorCatalog struct {
	events   *[]string
	executor v1ReleaseExecutor
	err      error
}

func (catalog *recordingV1ExecutorCatalog) Resolve(string) (v1ReleaseExecutor, error) {
	*catalog.events = append(*catalog.events, "resolve-executor")
	return catalog.executor, catalog.err
}

type recordingV1Executor struct {
	events      *[]string
	executeErr  error
	rollbackErr error
	request     V1ExecutorRequest
}

func (*recordingV1Executor) Name() string { return "goreleaser" }

func (executor *recordingV1Executor) Execute(request V1ExecutorRequest) error {
	*executor.events = append(*executor.events, "execute")
	executor.request = request
	return executor.executeErr
}

func (executor *recordingV1Executor) Rollback() error {
	*executor.events = append(*executor.events, "rollback")
	return executor.rollbackErr
}

type recordingV1Reporter struct {
	events *[]string
}

func (reporter recordingV1Reporter) PlanningStarted() {
	*reporter.events = append(*reporter.events, "report-planning-started")
}
func (reporter recordingV1Reporter) PlanningCompleted(V1ReleasePlan) {
	*reporter.events = append(*reporter.events, "report-planning-completed")
}
func (reporter recordingV1Reporter) PreviewCompleted(V1ReleasePlan) {
	*reporter.events = append(*reporter.events, "report-preview")
}
func (reporter recordingV1Reporter) ExecutionReady(V1ReleasePlan, string) {
	*reporter.events = append(*reporter.events, "report-execution-ready")
}
func (reporter recordingV1Reporter) ConfigWriteFailed(error) {
	*reporter.events = append(*reporter.events, "report-config-write-failed")
}
func (reporter recordingV1Reporter) ConfigRestoreFailed(error) {
	*reporter.events = append(*reporter.events, "report-config-restore-failed")
}
func (reporter recordingV1Reporter) RollbackStarted() {
	*reporter.events = append(*reporter.events, "report-rollback-started")
}
func (reporter recordingV1Reporter) RollbackCompleted() {
	*reporter.events = append(*reporter.events, "report-rollback-completed")
}
func (reporter recordingV1Reporter) ReleaseCompleted(V1ReleasePlan) {
	*reporter.events = append(*reporter.events, "report-release-completed")
}

func TestV1ReleasePreviewUseCaseReturnsTypedReadOnlyResult(t *testing.T) {
	events := []string{}
	plan := testV1ReleasePlan("1.2.3", "1.2.4")
	useCase := v1ReleasePreviewUseCase{
		plans:    &recordingV1PreviewPlans{events: &events, plan: plan},
		reporter: recordingV1Reporter{events: &events},
	}

	result, failure := useCase.Preview(V1ReleasePreviewRequest{})

	if failure != nil {
		t.Fatalf("Preview failure: %#v", failure)
	}
	preview, ok := result.(*V1ReleasePreview)
	if !ok || preview.CurrentVersion != "1.2.3" || preview.NextVersion != "1.2.4" {
		t.Fatalf("preview = %#v", result)
	}
	assertV1UseCaseEvents(t, events, []string{
		"report-planning-started",
		"preview-plan",
		"report-planning-completed",
		"report-preview",
	})
}

func TestV1ReleaseExecutionUseCasePreservesOperationOrderAndInitialResult(t *testing.T) {
	events := []string{}
	previewPlan := testV1ReleasePlan("1.2.3", "1.2.4")
	executionPlan := testV1ReleasePlan("1.2.3", "1.2.4")
	executor := &recordingV1Executor{events: &events}
	useCase := v1ReleaseExecutionUseCase{
		previewPlans:   &recordingV1PreviewPlans{events: &events, plan: previewPlan},
		executionPlans: &recordingV1ExecutionPlans{events: &events, plan: executionPlan},
		requirements:   &recordingV1Requirements{events: &events},
		preflight:      &recordingV1Preflight{events: &events},
		materializer:   &recordingV1Materializer{events: &events},
		executors:      &recordingV1ExecutorCatalog{events: &events, executor: executor},
		reporter:       recordingV1Reporter{events: &events},
	}

	result, failure := useCase.Execute(V1ReleaseExecutionRequest{})

	if failure != nil {
		t.Fatalf("Execute failure: %#v", failure)
	}
	completed, ok := result.(*V1ReleaseCompleted)
	if !ok || completed.PreviousVersion != previewPlan.CurrentVersion || completed.NextVersion != previewPlan.NextVersion {
		t.Fatalf("completed result = %#v", result)
	}
	if executor.request.Plan.NextVersion != executionPlan.NextVersion {
		t.Fatalf("executor request = %#v", executor.request)
	}
	assertV1UseCaseEvents(t, events, []string{
		"report-planning-started", "preview-plan", "report-planning-completed",
		"requirements", "preflight",
		"report-planning-started", "execution-plan", "report-planning-completed",
		"resolve-executor", "report-execution-ready", "write-config", "execute", "report-release-completed",
	})
}

func TestV1ReleaseExecutionStopsBeforeMutationOnRequirementsFailure(t *testing.T) {
	events := []string{}
	useCase := v1ReleaseExecutionUseCase{
		previewPlans: &recordingV1PreviewPlans{events: &events, plan: testV1ReleasePlan("1.2.3", "1.2.4")},
		requirements: &recordingV1Requirements{events: &events, err: errors.New("token missing")},
		reporter:     recordingV1Reporter{events: &events},
	}

	_, failure := useCase.Execute(V1ReleaseExecutionRequest{})

	if failure == nil || failure.Code != "VALIDATION_FAILED" || failure.Class != v1ReleaseRequirementsFailure {
		t.Fatalf("failure = %#v", failure)
	}
	assertV1UseCaseEvents(t, events, []string{
		"report-planning-started", "preview-plan", "report-planning-completed", "requirements",
	})
}

func TestV1ReleaseExecutionPropagatesFatalPreflightWithoutMutation(t *testing.T) {
	events := []string{}
	preflightFailure := newFatalV1ReleaseFailure("UNCOMMITTED_CHANGES", errors.New("dirty"))
	useCase := v1ReleaseExecutionUseCase{
		previewPlans: &recordingV1PreviewPlans{events: &events, plan: testV1ReleasePlan("1.2.3", "1.2.4")},
		requirements: &recordingV1Requirements{events: &events},
		preflight:    &recordingV1Preflight{events: &events, failure: preflightFailure},
		reporter:     recordingV1Reporter{events: &events},
	}

	_, failure := useCase.Execute(V1ReleaseExecutionRequest{})

	if failure != preflightFailure || failure.Boundary != v1ReleaseFatalFailure {
		t.Fatalf("failure = %#v", failure)
	}
	assertV1UseCaseEvents(t, events, []string{
		"report-planning-started", "preview-plan", "report-planning-completed", "requirements", "preflight",
	})
}

func TestV1ReleaseExecutionRestoresThenRollsBackExecutorFailure(t *testing.T) {
	events := []string{}
	executor := &recordingV1Executor{events: &events, executeErr: errors.New("publish failed")}
	useCase := v1ReleaseExecutionUseCase{
		previewPlans:   &recordingV1PreviewPlans{events: &events, plan: testV1ReleasePlan("1.2.3", "1.2.4")},
		executionPlans: &recordingV1ExecutionPlans{events: &events, plan: testV1ReleasePlan("1.2.3", "1.2.4")},
		requirements:   &recordingV1Requirements{events: &events},
		preflight:      &recordingV1Preflight{events: &events},
		materializer:   &recordingV1Materializer{events: &events, restoreErr: errors.New("restore warning")},
		executors:      &recordingV1ExecutorCatalog{events: &events, executor: executor},
		reporter:       recordingV1Reporter{events: &events},
	}

	_, failure := useCase.Execute(V1ReleaseExecutionRequest{})

	if failure == nil || failure.Class != v1ReleaseExecutionFailure || !strings.Contains(failure.Cause.Error(), "release failed: publish failed") {
		t.Fatalf("failure = %#v", failure)
	}
	assertV1UseCaseEvents(t, events, []string{
		"report-planning-started", "preview-plan", "report-planning-completed", "requirements", "preflight",
		"report-planning-started", "execution-plan", "report-planning-completed", "resolve-executor",
		"report-execution-ready", "write-config", "execute", "restore-config", "report-config-restore-failed",
		"report-rollback-started", "rollback", "report-rollback-completed",
	})
}

func TestV1ReleaseExecutionClassifiesRollbackFailureWithBothCauses(t *testing.T) {
	events := []string{}
	executor := &recordingV1Executor{
		events:      &events,
		executeErr:  errors.New("publish failed"),
		rollbackErr: errors.New("cleanup failed"),
	}
	useCase := v1ReleaseExecutionUseCase{
		previewPlans:   &recordingV1PreviewPlans{events: &events, plan: testV1ReleasePlan("1.2.3", "1.2.4")},
		executionPlans: &recordingV1ExecutionPlans{events: &events, plan: testV1ReleasePlan("1.2.3", "1.2.4")},
		requirements:   &recordingV1Requirements{events: &events},
		preflight:      &recordingV1Preflight{events: &events},
		materializer:   &recordingV1Materializer{events: &events},
		executors:      &recordingV1ExecutorCatalog{events: &events, executor: executor},
		reporter:       recordingV1Reporter{events: &events},
	}

	_, failure := useCase.Execute(V1ReleaseExecutionRequest{})

	if failure == nil || failure.Class != v1ReleaseRollbackFailure ||
		!strings.Contains(failure.Cause.Error(), "release failed: publish failed") ||
		!strings.Contains(failure.Cause.Error(), "Failed undoing changes: cleanup failed") {
		t.Fatalf("failure = %#v", failure)
	}
}

func testV1ReleasePlan(current, next string) V1ReleasePlan {
	return V1ReleasePlan{
		RepositoryRoot: "/repo",
		UnitID:         "default",
		CurrentVersion: current,
		NextVersion:    next,
		Tag:            "v" + next,
		Executor:       "goreleaser",
		ReleaseType:    Patch,
	}
}

func assertV1UseCaseEvents(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("events:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
