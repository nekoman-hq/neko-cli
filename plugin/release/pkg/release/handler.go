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

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type releaseStartOperation struct{}

func (releaseStartOperation) Start(ctx context.Context, request ReleaseCommandRequest) (ReleaseCommandOutcome, *CommandFailure) {
	log.PluginPrint(log.Exec, "Starting %s release", string(request.ReleaseType))

	repository, err := config.LoadReleaseRepository(".")
	if err != nil {
		return nil, &CommandFailure{
			Code:  "CONFIG_NOT_FOUND",
			Cause: err,
			Details: map[string]any{
				"hint": "Run 'neko release init' to create V2 config/state, or 'neko release migrate' to convert an existing V1 config",
			},
		}
	}

	unit, err := config.ResolveReleaseUnit(repository, request.UnitID, config.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		return nil, failureFromError("UNIT_RESOLUTION_FAILED", err)
	}
	execCtx, err := BuildReleaseExecutionContext(repository, *unit, request.ReleaseType, request.DryRun)
	if err != nil {
		return nil, failureFromError("EXECUTION_CONTEXT_FAILED", err)
	}

	if repository.SourceFormat == config.SourceFormatV2 {
		return startV2Release(ctx, execCtx)
	}
	return startLegacyRelease(execCtx, repository.Legacy)
}

func startV2Release(ctx context.Context, execCtx *ReleaseExecutionContext) (ReleaseCommandOutcome, *CommandFailure) {
	if execCtx.DryRun {
		return planV2Release(execCtx)
	}
	if execCtx.Delivery == string(config.DeliveryGitHubActions) {
		result, err := NewGitHubActionsReleaseRunner().Run(ctx, execCtx)
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

func planV2Release(execCtx *ReleaseExecutionContext) (ReleaseCommandOutcome, *CommandFailure) {
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
	logV2DryRunPlan(execCtx, materializationPlan, knownFiles, dispatchSummary)
	return newV2ReleasePreview(execCtx, plan, materializationPlan, knownFiles, dispatchSummary), nil
}

func startLegacyRelease(execCtx *ReleaseExecutionContext, cfg *config.V1ReleaseConfig) (ReleaseCommandOutcome, *CommandFailure) { //nolint:staticcheck // V1 compatibility path intentionally uses the deprecated V1 schema.
	svc := NewReleaseServiceWithContext(cfg, execCtx)
	oldVersion, newVersion, err := svc.GetNewVersion(execCtx.ReleaseKind)
	if err != nil {
		return nil, failureFromError("VERSION_ERROR", err)
	}

	if execCtx.DryRun {
		log.PluginPrint(log.Exec, "Dry run mode - no changes will be made")
		return &LegacyReleasePreview{
			ReleaseType:    execCtx.ReleaseKind,
			CurrentVersion: oldVersion.String(),
			NextVersion:    newVersion.String(),
			ReleaseSystem:  string(cfg.ReleaseSystem),
		}, nil
	}

	if err := ValidateRequirementsForContext(execCtx); err != nil {
		return nil, failureFromError("VALIDATION_FAILED", err)
	}
	if err := svc.Run(execCtx.ReleaseKind); err != nil {
		return nil, failureFromError("RELEASE_FAILED", err)
	}
	return &LegacyReleaseCompleted{
		ReleaseType:     execCtx.ReleaseKind,
		PreviousVersion: oldVersion.String(),
		NextVersion:     newVersion.String(),
		ReleaseSystem:   string(cfg.ReleaseSystem),
	}, nil
}

func logV2DryRunPlan(execCtx *ReleaseExecutionContext, materializationPlan *MaterializationPlan, knownFiles KnownReleaseFiles, dispatchSummary *ReleaseDispatchDryRunSummary) {
	log.PluginPrint(log.Config, "Repository root: %s", execCtx.RepositoryRoot)
	log.PluginPrint(log.Config, "Release source format: %s", execCtx.SourceFormat)
	log.PluginPrint(log.Config, "Selected unit: %s", execCtx.Unit.ID)
	log.PluginPrint(log.Config, "Config path: %s", config.V2ConfigPath(execCtx.RepositoryRoot))
	log.PluginPrint(log.Config, "State path: %s", config.V2StatePath(execCtx.RepositoryRoot))
	log.PluginPrint(log.Exec, "Planning V2 dry-run: current=%s next=%s tag=%s", execCtx.CurrentVersion, execCtx.NextVersion, execCtx.Tag)
	log.PluginPrint(log.Exec, "Executor=%s delivery=%s workflow=%s tagPrefix=%s", execCtx.Executor, execCtx.Delivery, workflowValue(execCtx.Workflow), execCtx.TagSpec.Prefix)
	log.PluginPrint(log.Exec, "Planned materialized files: %s", materializedFilesValue(materializationPlan))
	log.PluginPrint(log.Exec, "Known release files: %s", strings.Join(knownFiles.RelativePaths(), ", "))
	log.PluginPrint(log.Exec, "Dry run only: no token required, no journal created, no commit/tag/push/dispatch")
	if dispatchSummary != nil {
		log.PluginPrint(log.Exec, "Planned dispatch ref: %s", dispatchSummary.Ref)
		log.PluginPrint(log.Exec, "Planned dispatch inputs: %s", dispatchInputsValue(dispatchSummary.Inputs))
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
