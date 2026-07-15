//nolint:staticcheck // This file is the explicit application boundary for deprecated V1 compatibility.
package release

import (
	"errors"
	"fmt"
	"path/filepath"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

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

// V1Executor invokes one explicitly composed legacy release system and owns
// the rollback evidence produced by that invocation.
type V1Executor interface {
	Name() string
	Run(V1ExecutorRequest) error
	Rollback() error
	CompensationState() GitReleaseState
}

type v1ReleaseExecutor = V1Executor

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
	previewPlans         v1PreviewPlanBuilder
	executionPlans       v1ExecutionPlanBuilder
	requirements         v1ReleaseRequirements
	preflight            v1ReleasePreflight
	materializer         v1ReleaseConfigMaterializer
	executors            v1ReleaseExecutorCatalog
	reporter             v1ReleaseReporter
	compensationStores   v1CompensationEvidenceStores
	compensationFiles    v1CompensationConfigFiles
	compensationGit      v1CompensationGit
	compensationReleases v1GitHubReleaseRemover
	compensationClock    v1CompensationClock
}

func (useCase v1ReleaseExecutionUseCase) Execute(request V1ReleaseExecutionRequest) (V1ReleaseResult, *V1ReleaseFailure) {
	store := useCase.compensationStores.Open(request.Intent.RepositoryRoot)
	if failure := useCase.continueInterruptedCompensation(store); failure != nil {
		return nil, failure
	}

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

	evidence, failure := useCase.createCompensationEvidence(store, request.Intent, executionPlan)
	if failure != nil {
		return nil, failure
	}
	if failure := useCase.writePlannedConfig(store, request.Intent, executionPlan, evidence); failure != nil {
		return nil, failure
	}
	if failure := useCase.runExecutorWithEvidence(store, executionPlan, executor, evidence); failure != nil {
		return nil, failure
	}

	useCase.reporter.ReleaseCompleted(executionPlan)
	return &V1ReleaseCompleted{
		ReleaseType:     previewPlan.ReleaseType,
		PreviousVersion: previewPlan.CurrentVersion,
		NextVersion:     previewPlan.NextVersion,
		ReleaseSystem:   previewPlan.Executor,
	}, nil
}

func (useCase v1ReleaseExecutionUseCase) createCompensationEvidence(
	store V1CompensationEvidenceStore,
	intent V1ReleaseIntent,
	plan V1ReleasePlan,
) (*V1CompensationEvidence, *V1ReleaseFailure) {
	configPath := filepath.Join(intent.RepositoryRoot, releaseconfig.V1FileName)
	originalConfig, err := useCase.compensationFiles.Read(configPath)
	if err != nil {
		return nil, newV1ReleaseFailure(v1ReleaseExecutionFailure, "RELEASE_FAILED", err)
	}
	evidence, err := newV1CompensationEvidence(plan, configPath, originalConfig, useCase.compensationClock.Now())
	if err != nil {
		return nil, newV1ReleaseFailure(v1ReleaseExecutionFailure, "RELEASE_FAILED", err)
	}
	if err := store.Create(evidence); err != nil {
		return nil, newV1ReleaseFailure(v1ReleaseExecutionFailure, "RELEASE_FAILED", err)
	}
	return &evidence, nil
}

func (useCase v1ReleaseExecutionUseCase) writePlannedConfig(
	store V1CompensationEvidenceStore,
	intent V1ReleaseIntent,
	plan V1ReleasePlan,
	evidence *V1CompensationEvidence,
) *V1ReleaseFailure {
	if err := store.RecordConfigWritePending(evidence); err != nil {
		return newV1ReleaseFailure(v1ReleaseExecutionFailure, "RELEASE_FAILED", err)
	}
	if err := useCase.materializer.WritePlannedVersion(intent, plan); err != nil {
		useCase.reporter.ConfigWriteFailed(err)
		return useCase.recoverConfigMutationFailure(store, evidence, err)
	}
	if err := useCase.compensationFiles.VerifyVersion(evidence.OriginalConfig.Path, plan.NextVersion); err != nil {
		useCase.reporter.ConfigWriteFailed(err)
		return useCase.recoverConfigMutationFailure(store, evidence, err)
	}
	if err := store.ConfirmConfigWrite(evidence); err != nil {
		return useCase.recoverConfigMutationFailure(store, evidence, err)
	}
	return nil
}

func (useCase v1ReleaseExecutionUseCase) runExecutorWithEvidence(
	store V1CompensationEvidenceStore,
	plan V1ReleasePlan,
	executor v1ReleaseExecutor,
	evidence *V1CompensationEvidence,
) *V1ReleaseFailure {
	if err := store.RecordExecutorPending(evidence); err != nil {
		planningErr := store.PlanFailedExecution(evidence, GitReleaseState{})
		return useCase.continueFailedCompensation(store, evidence, errors.Join(err, planningErr))
	}
	if err := executor.Run(V1ExecutorRequest{Plan: plan}); err != nil {
		return useCase.recoverExecutorFailure(store, evidence, executor, err)
	}
	if err := store.ConfirmReleaseExecution(evidence, executor.CompensationState()); err != nil {
		manualErr := store.MarkManualRecoveryRequired(evidence)
		return newV1ReleaseFailure(
			v1ReleaseRollbackFailure,
			"RELEASE_FAILED",
			errors.Join(fmt.Errorf("release completed but durable confirmation failed: %w", err), manualErr),
		)
	}
	return nil
}

func (useCase v1ReleaseExecutionUseCase) recoverExecutorFailure(
	store V1CompensationEvidenceStore,
	evidence *V1CompensationEvidence,
	executor v1ReleaseExecutor,
	executorError error,
) *V1ReleaseFailure {
	state := executor.CompensationState()
	var evidenceError error
	if ClassifyV1ExecutorFailure(evidence.Identity.Executor, state) == V1ExecutorFailureExternalUncertainty {
		evidenceError = store.RetainUncertainExecution(evidence, state)
	} else {
		evidenceError = store.PlanFailedExecution(evidence, state)
	}
	return useCase.continueFailedCompensation(store, evidence, errors.Join(executorError, evidenceError))
}

func (useCase v1ReleaseExecutionUseCase) recoverConfigMutationFailure(
	store V1CompensationEvidenceStore,
	evidence *V1CompensationEvidence,
	configError error,
) *V1ReleaseFailure {
	evidenceError := store.RecordConfigWriteFailure(evidence)
	return useCase.continueFailedCompensation(store, evidence, errors.Join(configError, evidenceError))
}

func (useCase v1ReleaseExecutionUseCase) continueFailedCompensation(
	store V1CompensationEvidenceStore,
	evidence *V1CompensationEvidence,
	releaseCause error,
) *V1ReleaseFailure {
	releaseError := fmt.Errorf("release failed: %w", releaseCause)
	useCase.reporter.RollbackStarted()
	status, err := continueV1Compensation(
		store,
		useCase.compensationFiles,
		useCase.compensationGit,
		useCase.compensationReleases,
		evidence,
	)
	if err != nil {
		if evidence.Compensation.Failure != nil && evidence.Compensation.Failure.Action == V1CompensationRestoreConfig {
			useCase.reporter.ConfigRestoreFailed(err)
		}
		return newV1ReleaseFailure(
			v1ReleaseRollbackFailure,
			"RELEASE_FAILED",
			fmt.Errorf("%w: Failed undoing changes: %w", releaseError, err),
		)
	}
	if status == V1CompensationContinuationManual {
		path, pathErr := store.CurrentPath()
		return newV1ReleaseFailure(
			v1ReleaseRollbackFailure,
			"RELEASE_FAILED",
			errors.Join(fmt.Errorf("%w: manual recovery required; inspect V1 compensation evidence: %s", releaseError, path), pathErr),
		)
	}
	useCase.reporter.RollbackCompleted()
	return newV1ReleaseFailure(v1ReleaseExecutionFailure, "RELEASE_FAILED", releaseError)
}

func (useCase v1ReleaseExecutionUseCase) continueInterruptedCompensation(store V1CompensationEvidenceStore) *V1ReleaseFailure {
	evidence, found, err := store.FindUnresolved()
	if err != nil {
		return newV1ReleaseFailure(v1ReleaseRollbackFailure, "RELEASE_FAILED", fmt.Errorf("load V1 compensation evidence: %w", err))
	}
	if !found {
		return nil
	}
	useCase.reporter.RollbackStarted()
	status, err := continueV1Compensation(
		store,
		useCase.compensationFiles,
		useCase.compensationGit,
		useCase.compensationReleases,
		evidence,
	)
	if err != nil {
		return newV1ReleaseFailure(v1ReleaseRollbackFailure, "RELEASE_FAILED", fmt.Errorf("continue interrupted V1 compensation: %w", err))
	}
	if status == V1CompensationContinuationManual {
		path, pathErr := store.CurrentPath()
		return newV1ReleaseFailure(
			v1ReleaseRollbackFailure,
			"RELEASE_FAILED",
			errors.Join(fmt.Errorf("manual recovery required for interrupted V1 release; evidence: %s", path), pathErr),
		)
	}
	useCase.reporter.RollbackCompleted()
	return nil
}
