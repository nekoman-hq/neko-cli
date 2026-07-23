package release

//nolint:govet // Planning facts follow the canonical V2 release-planning sequence.
type v2ReleasePlanningFacts struct {
	ExecutionContext    *ReleaseExecutionContext
	ReleasePlan         ReleasePlan
	MaterializationPlan *MaterializationPlan
	KnownFiles          KnownReleaseFiles
	Dispatch            *ReleaseDispatchDryRunSummary
}

func planV2ReleaseFacts(execCtx *ReleaseExecutionContext) (v2ReleasePlanningFacts, *CommandFailure) {
	return planV2ReleaseFactsWithRequirements(execCtx, ValidateRequirementsForContext)
}

func planV2ReleaseFactsForInspection(execCtx *ReleaseExecutionContext) (v2ReleasePlanningFacts, *CommandFailure) {
	return planV2ReleaseFactsWithRequirements(execCtx, validateRequirementsForContextInspection)
}

func planV2ReleaseFactsWithRequirements(
	execCtx *ReleaseExecutionContext,
	validateRequirements func(*ReleaseExecutionContext) error,
) (v2ReleasePlanningFacts, *CommandFailure) {
	if execCtx == nil {
		return v2ReleasePlanningFacts{}, failureFromMessage("EXECUTION_CONTEXT_FAILED", "release execution context is missing")
	}
	if err := validateRequirements(execCtx); err != nil {
		return v2ReleasePlanningFacts{}, failureFromError("VALIDATION_FAILED", err)
	}
	plan := BuildReleasePlan(execCtx)
	materializer, err := ResolveVersionMaterializer(execCtx.Executor)
	if err != nil {
		return v2ReleasePlanningFacts{}, failureFromError("MATERIALIZATION_FAILED", err)
	}
	materializationPlan, err := materializer.Plan(execCtx)
	if err != nil {
		return v2ReleasePlanningFacts{}, failureFromError("MATERIALIZATION_FAILED", err)
	}
	if validationErr := materializer.Validate(materializationPlan); validationErr != nil {
		return v2ReleasePlanningFacts{}, failureFromError("MATERIALIZATION_FAILED", validationErr)
	}
	knownFiles, err := NewKnownReleaseFiles(execCtx, materializationPlan)
	if err != nil {
		return v2ReleasePlanningFacts{}, failureFromError("GIT_COORDINATION_FAILED", err)
	}
	dispatchSummary, err := BuildReleaseDispatchDryRunSummary(execCtx)
	if err != nil {
		return v2ReleasePlanningFacts{}, failureFromError("DISPATCH_CONTRACT_FAILED", err)
	}
	return v2ReleasePlanningFacts{
		ExecutionContext:    execCtx,
		ReleasePlan:         plan,
		MaterializationPlan: materializationPlan,
		KnownFiles:          knownFiles,
		Dispatch:            dispatchSummary,
	}, nil
}

func (facts v2ReleasePlanningFacts) ReleasePreview() *V2ReleasePreview {
	return newV2ReleasePreview(
		facts.ExecutionContext,
		facts.ReleasePlan,
		facts.MaterializationPlan,
		facts.KnownFiles,
		facts.Dispatch,
	)
}
