//nolint:staticcheck // This file is the explicit application boundary for deprecated V1 compatibility.
package release

import "fmt"

type v1PreviewPlanBuilder interface {
	BuildPreviewPlan(V1ReleaseIntent) (V1ReleasePlan, error)
}

type v1ExecutionPlanBuilder interface {
	BuildExecutionPlan(V1ReleaseIntent) (V1ReleasePlan, error)
}

type v1ReleaseRequirements interface {
	Validate(V1ReleaseIntent) error
}

type v1ReleasePreflight interface {
	Check(V1ReleaseIntent) *V1ReleaseFailure
}

type v1ReleaseConfigMaterializer interface {
	WritePlannedVersion(V1ReleaseIntent, V1ReleasePlan) error
	RestorePreviousVersion(V1ReleaseIntent, V1ReleasePlan) error
}

type v1ReleaseExecutor interface {
	Name() string
	Execute(V1ExecutorRequest) error
	Rollback() error
}

type v1ReleaseExecutorCatalog interface {
	Resolve(string) (v1ReleaseExecutor, error)
}

type v1ReleaseReporter interface {
	PlanningStarted()
	PlanningCompleted(V1ReleasePlan)
	PreviewCompleted(V1ReleasePlan)
	ExecutionReady(V1ReleasePlan, string)
	ConfigWriteFailed(error)
	ConfigRestoreFailed(error)
	RollbackStarted()
	RollbackCompleted()
	ReleaseCompleted(V1ReleasePlan)
}

type v1ReleasePreviewUseCase struct {
	plans    v1PreviewPlanBuilder
	reporter v1ReleaseReporter
}

func (useCase v1ReleasePreviewUseCase) Preview(request V1ReleasePreviewRequest) (V1ReleaseResult, *V1ReleaseFailure) {
	useCase.reporter.PlanningStarted()
	plan, err := useCase.plans.BuildPreviewPlan(request.Intent)
	if err != nil {
		return nil, newV1ReleaseFailure(v1ReleasePlanningFailure, "VERSION_ERROR", err)
	}
	useCase.reporter.PlanningCompleted(plan)
	useCase.reporter.PreviewCompleted(plan)
	return &V1ReleasePreview{
		ReleaseType:    plan.ReleaseType,
		CurrentVersion: plan.CurrentVersion,
		NextVersion:    plan.NextVersion,
		ReleaseSystem:  plan.Executor,
	}, nil
}

type v1ReleaseExecutionUseCase struct {
	previewPlans   v1PreviewPlanBuilder
	executionPlans v1ExecutionPlanBuilder
	requirements   v1ReleaseRequirements
	preflight      v1ReleasePreflight
	materializer   v1ReleaseConfigMaterializer
	executors      v1ReleaseExecutorCatalog
	reporter       v1ReleaseReporter
}

func (useCase v1ReleaseExecutionUseCase) Execute(request V1ReleaseExecutionRequest) (V1ReleaseResult, *V1ReleaseFailure) {
	useCase.reporter.PlanningStarted()
	previewPlan, err := useCase.previewPlans.BuildPreviewPlan(request.Intent)
	if err != nil {
		return nil, newV1ReleaseFailure(v1ReleasePlanningFailure, "VERSION_ERROR", err)
	}
	useCase.reporter.PlanningCompleted(previewPlan)

	if requirementsError := useCase.requirements.Validate(request.Intent); requirementsError != nil {
		return nil, newV1ReleaseFailure(v1ReleaseRequirementsFailure, "VALIDATION_FAILED", requirementsError)
	}
	if failure := useCase.preflight.Check(request.Intent); failure != nil {
		return nil, failure
	}

	useCase.reporter.PlanningStarted()
	executionPlan, err := useCase.executionPlans.BuildExecutionPlan(request.Intent)
	if err != nil {
		return nil, newV1ReleaseFailure(v1ReleasePlanningFailure, "RELEASE_FAILED", err)
	}
	useCase.reporter.PlanningCompleted(executionPlan)

	executor, err := useCase.executors.Resolve(executionPlan.Executor)
	if err != nil {
		return nil, newV1ReleaseFailure(
			v1ReleaseExecutorResolutionFailure,
			"RELEASE_FAILED",
			fmt.Errorf("release System Not Found: %w", err),
		)
	}
	useCase.reporter.ExecutionReady(executionPlan, executor.Name())

	if err := useCase.materializer.WritePlannedVersion(request.Intent, executionPlan); err != nil {
		useCase.reporter.ConfigWriteFailed(err)
	}
	if err := executor.Execute(V1ExecutorRequest{Plan: executionPlan}); err != nil {
		return nil, useCase.recoverExecutorFailure(request.Intent, executionPlan, executor, err)
	}

	useCase.reporter.ReleaseCompleted(executionPlan)
	return &V1ReleaseCompleted{
		ReleaseType:     previewPlan.ReleaseType,
		PreviousVersion: previewPlan.CurrentVersion,
		NextVersion:     previewPlan.NextVersion,
		ReleaseSystem:   previewPlan.Executor,
	}, nil
}

func (useCase v1ReleaseExecutionUseCase) recoverExecutorFailure(
	intent V1ReleaseIntent,
	plan V1ReleasePlan,
	executor v1ReleaseExecutor,
	executorError error,
) *V1ReleaseFailure {
	releaseError := fmt.Errorf("release failed: %w", executorError)
	if err := useCase.materializer.RestorePreviousVersion(intent, plan); err != nil {
		useCase.reporter.ConfigRestoreFailed(err)
	}

	useCase.reporter.RollbackStarted()
	if err := executor.Rollback(); err != nil {
		return newV1ReleaseFailure(
			v1ReleaseRollbackFailure,
			"RELEASE_FAILED",
			fmt.Errorf("%w: Failed undoing changes: %w", releaseError, err),
		)
	}
	useCase.reporter.RollbackCompleted()
	return newV1ReleaseFailure(v1ReleaseExecutionFailure, "RELEASE_FAILED", releaseError)
}
