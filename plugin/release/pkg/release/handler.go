// Package release includes all neko cli release logic
package release

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      03.02.2026
*/

import (
	"context"
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

type releaseRepositoryReader interface {
	Load(string) (*config.ReleaseRepository, error)
}

type releaseConfigRepositoryReader struct{}

func (releaseConfigRepositoryReader) Load(root string) (*config.ReleaseRepository, error) {
	repository, err := config.LoadReleaseRepository(root)
	if err != nil {
		return nil, err
	}
	absoluteRoot, err := absoluteExistingDir(repository.RepositoryRoot, "repository root")
	if err != nil {
		return nil, err
	}
	repository.RepositoryRoot = absoluteRoot
	return repository, nil
}

type v1ReleaseApplication interface {
	Start(context.Context, *config.ReleaseRepository, ReleaseCommandRequest) (ReleaseCommandOutcome, *CommandFailure)
}

type v2ReleaseApplication interface {
	Start(context.Context, *config.ReleaseRepository, ReleaseCommandRequest) (ReleaseCommandOutcome, *CommandFailure)
}

type releaseStartOperation struct {
	repositories   releaseRepositoryReader
	v1             v1ReleaseApplication
	v2             v2ReleaseApplication
	progress       ReleaseProgress
	repositoryRoot string
}

func newReleaseStartOperationAt(root workspace.RepositoryRoot) releaseStartOperation {
	return newReleaseStartOperationWithV1ExecutorsAt(root, registeredV1ReleaseExecutorCatalog{})
}

func newReleaseStartOperationWithV1ExecutorsAt(root workspace.RepositoryRoot, executors v1ReleaseExecutorCatalog) releaseStartOperation {
	return newReleaseStartOperationWithRepositoryRoot(root.Path(), executors)
}

func newReleaseStartOperationWithRepositoryRoot(repositoryRoot string, executors v1ReleaseExecutorCatalog) releaseStartOperation {
	progress := newTerminalReleaseProgress()
	return releaseStartOperation{
		repositoryRoot: repositoryRoot,
		repositories:   releaseConfigRepositoryReader{},
		v1:             composeV1ReleaseCommandApplication(executors),
		v2:             v2ReleaseCommandApplication{progress: progress},
		progress:       progress,
	}
}

func (operation releaseStartOperation) Start(ctx context.Context, request ReleaseCommandRequest) (ReleaseCommandOutcome, *CommandFailure) {
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{
		Kind:        ReleaseProgressReleaseStarted,
		ReleaseType: string(request.ReleaseType),
	})

	repository, err := operation.repositories.Load(operation.repositoryRoot)
	if err != nil {
		return nil, &CommandFailure{
			Code:  "CONFIG_NOT_FOUND",
			Cause: err,
			Details: map[string]any{
				"hint": "Run 'neko release init' to create V2 config/state, or 'neko release migrate' to convert an existing V1 config",
			},
		}
	}

	path, err := selectReleaseApplicationPath(repository.SourceFormat)
	if err != nil {
		return nil, failureFromError("SOURCE_FORMAT_UNSUPPORTED", err)
	}

	switch path {
	case config.SourceFormatV1:
		return operation.v1.Start(ctx, repository, request)
	case config.SourceFormatV2:
		return operation.v2.Start(ctx, repository, request)
	default:
		return nil, failureFromMessage("SOURCE_FORMAT_UNSUPPORTED", "release source selection returned no application path")
	}
}

type v2ReleaseCommandApplication struct {
	progress ReleaseProgress
}

func (application v2ReleaseCommandApplication) Start(
	ctx context.Context,
	repository *config.ReleaseRepository,
	request ReleaseCommandRequest,
) (ReleaseCommandOutcome, *CommandFailure) {
	unit, err := config.ResolveReleaseUnit(repository, request.UnitID, config.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		return nil, failureFromError("UNIT_RESOLUTION_FAILED", err)
	}
	execCtx, err := BuildV2ReleaseExecutionContext(repository.RepositoryRoot, *unit, request.ReleaseType, request.DryRun)
	if err != nil {
		return nil, failureFromError("EXECUTION_CONTEXT_FAILED", err)
	}
	return startV2Release(ctx, execCtx, application.progress)
}

func startV2Release(ctx context.Context, execCtx *ReleaseExecutionContext, progress ReleaseProgress) (ReleaseCommandOutcome, *CommandFailure) {
	if execCtx.DryRun {
		return planV2Release(execCtx, progress)
	}
	if execCtx.Delivery == string(config.DeliveryGitHubActions) {
		result, err := NewGitHubActionsReleaseRunner(
			WithGitHubActionsReleaseProgress(progress),
			withGitHubActionsReleaseGitDiagnostics(newTerminalGitReleaseDiagnostics()),
		).Run(ctx, execCtx)
		if err != nil {
			return nil, failureFromError("V2_GITHUB_ACTIONS_RELEASE_FAILED", err)
		}
		return result, nil
	}
	if execCtx.Delivery == string(config.DeliveryLocal) {
		return nil, failureFromMessage("V2_LOCAL_DELIVERY_BLOCKED", "V2 local release execution is not available yet.")
	}
	return nil, failureFromMessage("V2_PUBLICATION_ADAPTERS_UNAVAILABLE", v2GitCoordinationUnavailableMessage)
}

func planV2Release(execCtx *ReleaseExecutionContext, progress ReleaseProgress) (ReleaseCommandOutcome, *CommandFailure) {
	if execCtx == nil {
		return nil, failureFromMessage("EXECUTION_CONTEXT_FAILED", "release execution context is missing")
	}
	if err := ValidateRequirementsForContext(execCtx); err != nil {
		return nil, failureFromError("VALIDATION_FAILED", err)
	}
	plan := BuildReleasePlan(execCtx)
	materializer, err := ResolveVersionMaterializer(execCtx.Executor)
	if err != nil {
		return nil, failureFromError("MATERIALIZATION_FAILED", err)
	}
	materializationPlan, err := materializer.Plan(execCtx)
	if err != nil {
		return nil, failureFromError("MATERIALIZATION_FAILED", err)
	}
	if validationErr := materializer.Validate(materializationPlan); validationErr != nil {
		return nil, failureFromError("MATERIALIZATION_FAILED", validationErr)
	}
	knownFiles, err := NewKnownReleaseFiles(execCtx, materializationPlan)
	if err != nil {
		return nil, failureFromError("GIT_COORDINATION_FAILED", err)
	}
	dispatchSummary, err := BuildReleaseDispatchDryRunSummary(execCtx)
	if err != nil {
		return nil, failureFromError("DISPATCH_CONTRACT_FAILED", err)
	}
	reportV2DryRunPlan(progress, execCtx, materializationPlan, knownFiles, dispatchSummary)
	return newV2ReleasePreview(execCtx, plan, materializationPlan, knownFiles, dispatchSummary), nil
}

func reportV2DryRunPlan(progress ReleaseProgress, execCtx *ReleaseExecutionContext, materializationPlan *MaterializationPlan, knownFiles KnownReleaseFiles, dispatchSummary *ReleaseDispatchDryRunSummary) {
	reportReleaseProgress(progress, ReleaseProgressEvent{
		Kind:           ReleaseProgressRepositoryContext,
		RepositoryRoot: execCtx.RepositoryRoot,
		SourceFormat:   string(execCtx.SourceFormat),
		UnitID:         execCtx.Unit.ID,
		ConfigPath:     config.V2ConfigPath(execCtx.RepositoryRoot),
		StatePath:      config.V2StatePath(execCtx.RepositoryRoot),
	})
	reportReleaseProgress(progress, ReleaseProgressEvent{
		Kind:           ReleaseProgressDryRunPlan,
		CurrentVersion: execCtx.CurrentVersion,
		NextVersion:    execCtx.NextVersion,
		Tag:            execCtx.Tag,
		Executor:       execCtx.Executor,
		Delivery:       execCtx.Delivery,
		Workflow:       execCtx.Workflow,
		TagPrefix:      execCtx.TagSpec.Prefix,
	})
	reportReleaseProgress(progress, ReleaseProgressEvent{Kind: ReleaseProgressMaterializationPlanningCompleted, Files: []string{materializedFilesValue(materializationPlan)}})
	reportReleaseProgress(progress, ReleaseProgressEvent{Kind: ReleaseProgressKnownFiles, Files: knownFiles.RelativePaths()})
	reportReleaseProgress(progress, ReleaseProgressEvent{Kind: ReleaseProgressDryRunBoundary})
	if dispatchSummary != nil {
		reportReleaseProgress(progress, ReleaseProgressEvent{
			Kind:   ReleaseProgressDryRunDispatchPlan,
			Ref:    dispatchSummary.Ref,
			Inputs: releaseProgressInputs(dispatchSummary.Inputs),
		})
	}
}

func materializedFilesValue(plan *MaterializationPlan) string {
	if plan == nil || len(plan.Changes) == 0 {
		if plan != nil && plan.BlockedReason != "" {
			return "blocked: " + plan.BlockedReason
		}
		return "none"
	}
	values := make([]string, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		values = append(values, change.RepositoryRelativePath)
	}
	return strings.Join(values, ", ")
}

func workflowValue(workflow string) string {
	if workflow == "" {
		return "not applicable"
	}
	return workflow
}

func dispatchInputsValue(inputs map[string]string) string {
	keys := sortedDispatchInputKeys(inputs)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+inputs[key])
	}
	return strings.Join(parts, " ")
}
