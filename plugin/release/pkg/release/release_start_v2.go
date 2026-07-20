package release

import (
	"context"
	"fmt"
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

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
		return nil, failureFromMessage("V2_LOCAL_DELIVERY_UNSUPPORTED", "V2 local delivery is not supported; use github-actions delivery with a workflow.")
	}
	return nil, failureFromMessage("V2_DELIVERY_UNSUPPORTED", fmt.Sprintf("V2 delivery %q is not supported; use github-actions delivery with a workflow.", execCtx.Delivery))
}

func planV2Release(execCtx *ReleaseExecutionContext, progress ReleaseProgress) (ReleaseCommandOutcome, *CommandFailure) {
	facts, failure := planV2ReleaseFacts(execCtx)
	if failure != nil {
		return nil, failure
	}
	reportV2DryRunPlan(progress, facts.ExecutionContext, facts.MaterializationPlan, facts.KnownFiles, facts.Dispatch)
	return facts.ReleasePreview(), nil
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
