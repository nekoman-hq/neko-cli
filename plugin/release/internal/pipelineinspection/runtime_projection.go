package pipelineinspection

import "sort"

func applyPipelineRuntime(result *pipelineResult, snapshot RuntimeSnapshot) {
	for index := range result.Stages {
		result.Stages[index].RuntimeStatus = RuntimeNotObserved
	}
	result.Execution = pipelineExecution{
		Identity: "", Validity: "not_observed",
		Observations: make([]pipelineExecutionJournal, 0),
	}
	result.Dispatch = pipelineDispatch{
		Identity: "", Correlation: "not_observed",
		Observations: make([]pipelineDispatchJournal, 0),
	}
	result.LocalGit = pipelineLocalGit{Scope: "local_only", RemoteFreshness: "remote_not_inspected"}
	result.Recovery = pipelineRecovery{RetrySafety: "not_evaluated", Reasons: make([]string, 0)}
	result.ManualIntervention = pipelineManualIntervention{Reasons: make([]string, 0)}
	if !snapshot.Inspected {
		return
	}

	result.ProgressInspection.ExecutionProgress = "not_started"
	result.ProgressInspection.JournalsInspected = true
	result.Limitations = []string{
		"Only local execution evidence was inspected; remote Git freshness was not inspected.",
		"Workflow execution and publication state were not inspected remotely.",
		"Runtime inspection is read-only and does not resume, retry, repair, or clean a release.",
	}
	result.Repository.LocalBranch = snapshot.Repository.Branch
	result.Repository.LocalHead = snapshot.Repository.Head
	result.Repository.Tracking = "remote_not_inspected"
	result.LocalGit.Branch = snapshot.Repository.Branch
	result.LocalGit.Head = snapshot.Repository.Head
	result.LocalGit.IndexState = snapshot.Repository.IndexState
	result.LocalGit.WorktreeState = snapshot.Repository.WorktreeState

	relevant := relevantExecutionObservations(result, snapshot)
	result.Execution.JournalCount = len(relevant)
	for _, observation := range relevant {
		result.Execution.Observations = append(result.Execution.Observations, pipelineExecutionJournal{
			Identity: observation.Identity, Reference: observation.Reference,
			State: observation.State, Unresolved: observation.Unresolved,
			Valid: observation.Valid, Problem: observation.Problem,
		})
		if observation.Unresolved {
			result.Execution.UnresolvedCount++
		}
		if !observation.Valid {
			result.InvalidEvidence = true
		}
	}
	for _, problem := range snapshot.Problems {
		if problem.Kind != "execution_journal" && problem.Kind != "local_git" {
			continue
		}
		if problem.UnitID == "" || problem.UnitID == result.Unit.ID {
			result.InvalidEvidence = true
			result.Execution.Observations = append(result.Execution.Observations, pipelineExecutionJournal{
				Identity: "", Reference: problem.Reference, Valid: false, Problem: problem.Reason,
			})
			result.Execution.JournalCount++
		}
	}
	sort.Slice(result.Execution.Observations, func(i, j int) bool {
		left, right := result.Execution.Observations[i], result.Execution.Observations[j]
		if left.Identity == right.Identity {
			return left.Reference < right.Reference
		}
		return left.Identity < right.Identity
	})

	unresolved := validUnresolvedExecutions(relevant)
	if len(unresolved) > 1 {
		result.InvalidEvidence = true
		result.Execution.Validity = "conflict"
		result.Status = pipelineInvalid
		result.ProgressInspection.ExecutionProgress = "invalid"
		finalizePipelineRuntimeStatus(result, nil)
		return
	}

	var selected *RuntimeExecutionObservation
	if len(unresolved) == 1 {
		selected = &unresolved[0]
	} else {
		completed := currentCompletedExecutions(result, relevant)
		if len(completed) > 1 {
			result.InvalidEvidence = true
			result.Execution.Validity = "conflict"
			result.Status = pipelineInvalid
			result.ProgressInspection.ExecutionProgress = "invalid"
			finalizePipelineRuntimeStatus(result, nil)
			return
		}
		if len(completed) == 1 {
			selected = &completed[0]
		}
	}

	if result.InvalidEvidence {
		result.Execution.Validity = "invalid"
		result.Status = pipelineInvalid
		result.ProgressInspection.ExecutionProgress = "invalid"
		finalizePipelineRuntimeStatus(result, nil)
		return
	}
	if selected == nil {
		result.Execution.Validity = "valid"
		applyDispatchRuntime(result, snapshot, nil)
		finalizePipelineRuntimeStatus(result, nil)
		return
	}

	result.Execution.Present = true
	result.Execution.Identity = selected.Identity
	result.Execution.Validity = "valid"
	result.Execution.State = selected.State
	result.Execution.PendingAction = selected.PendingAction
	result.Execution.Terminal = !selected.Unresolved
	result.Execution.CreatedAt = selected.CreatedAt
	result.Execution.UpdatedAt = selected.UpdatedAt
	result.Execution.JournalReference = selected.Reference
	result.ProgressInspection.ExecutionProgress = selected.State
	result.Status = pipelineActive
	applyExecutionStageRuntime(result.Stages, *selected)
	applyLocalGitRuntime(result, selected.LocalGit)
	applyDispatchRuntime(result, snapshot, selected)
	applyRecoveryRuntime(result, selected.Recovery)
	finalizePipelineRuntimeStatus(result, selected)
}

func applyLocalGitRuntime(result *pipelineResult, observation RuntimeLocalGitObservation) {
	branch, head := result.LocalGit.Branch, result.LocalGit.Head
	indexState, worktreeState := result.LocalGit.IndexState, result.LocalGit.WorktreeState
	result.LocalGit = pipelineLocalGit{
		Scope: "local_only", RemoteFreshness: "remote_not_inspected",
		Branch: branch, Head: head, IndexState: indexState, WorktreeState: worktreeState,
		ExpectedCommit: observation.ExpectedCommit, CommitExists: observation.CommitExists,
		CommitContentVerified: observation.CommitContentVerified, ExpectedTag: observation.ExpectedTag,
		TagExists: observation.TagExists, TagTarget: observation.TagTarget,
		TagMatchesExpectedCommit:         observation.TagMatchesExpectedCommit,
		HeadContainsExpectedCommit:       observation.HeadContainsExpectedCommit,
		IndexContainsRecoveryEvidence:    observation.IndexContainsRecoveryEvidence,
		WorktreeContainsRecoveryEvidence: observation.WorktreeContainsRecoveryEvidence,
		Consistent:                       observation.Consistent, Problem: observation.Problem,
	}
	if observation.Inspected && !observation.Consistent {
		result.InvalidEvidence = true
		result.Status = pipelineInvalid
		result.ProgressInspection.ExecutionProgress = "invalid"
	}
}

func applyDispatchRuntime(result *pipelineResult, snapshot RuntimeSnapshot, selected *RuntimeExecutionObservation) {
	relevant := relevantDispatchObservations(result, snapshot)
	result.Dispatch.JournalCount = len(relevant)
	referenced := make(map[string]bool)
	for _, execution := range snapshot.Executions {
		if execution.Valid && execution.DispatchJournalIdentity != "" {
			referenced[execution.DispatchJournalIdentity] = true
		}
	}
	linked := make([]RuntimeDispatchObservation, 0, 1)
	for _, observation := range relevant {
		correlation := "unlinked"
		switch {
		case !observation.Valid:
			correlation = "invalid"
			result.InvalidEvidence = true
		case selected != nil && selected.DispatchJournalIdentity != "" && observation.Identity == selected.DispatchJournalIdentity:
			correlation = "exact"
			linked = append(linked, observation)
		case referenced[observation.Identity]:
			correlation = "other_execution"
		default:
			result.Dispatch.UnlinkedCount++
			result.InvalidEvidence = true
		}
		result.Dispatch.Observations = append(result.Dispatch.Observations, pipelineDispatchJournal{
			Identity: observation.Identity, Reference: observation.Reference,
			State: observation.State, Correlation: correlation,
			Valid: observation.Valid, Problem: observation.Problem,
		})
	}
	sort.Slice(result.Dispatch.Observations, func(i, j int) bool {
		left, right := result.Dispatch.Observations[i], result.Dispatch.Observations[j]
		if left.Identity == right.Identity {
			return left.Reference < right.Reference
		}
		return left.Identity < right.Identity
	})
	if selected != nil && selected.DispatchJournalIdentity != "" && len(linked) != 1 {
		result.InvalidEvidence = true
	}
	if len(linked) > 1 {
		result.InvalidEvidence = true
	}
	if result.InvalidEvidence {
		result.Dispatch.Correlation = "invalid"
		result.Status = pipelineInvalid
		result.ProgressInspection.ExecutionProgress = "invalid"
		return
	}
	if len(linked) == 0 {
		result.Dispatch.Correlation = "none"
		return
	}

	dispatch := linked[0]
	result.Dispatch.Present = true
	result.Dispatch.Identity = dispatch.Identity
	result.Dispatch.Correlation = "exact"
	result.Dispatch.State = dispatch.State
	result.Dispatch.WorkflowPath = dispatch.WorkflowPath
	result.Dispatch.RunID = dispatch.RunID
	result.Recovery.RetrySafety = dispatch.RetrySafety
	applyDispatchStageRuntime(result.Stages, dispatch)
}

func applyRecoveryRuntime(result *pipelineResult, observation RuntimeRecoveryObservation) {
	result.ProgressInspection.ResumeEligibilityEvaluated = observation.Evaluated
	result.Recovery.Evaluated = observation.Evaluated
	result.Recovery.Classification = observation.Classification
	result.Recovery.SafeToContinue = observation.SafeToContinue
	result.Recovery.ResumeEligible = observation.ResumeEligible
	result.Recovery.ResumeOperation = observation.ResumeOperation
	result.Recovery.ResumeRefusal = observation.ResumeRefusal
	result.Recovery.ManualInterventionRequired = observation.ManualIntervention
	result.Recovery.Guidance = observation.Guidance
	if observation.ResumeRefusal != "" {
		result.Recovery.Reasons = append(result.Recovery.Reasons, "resume policy: "+observation.ResumeRefusal)
	}
	if observation.Invalid {
		result.InvalidEvidence = true
	}
}

func finalizePipelineRuntimeStatus(result *pipelineResult, selected *RuntimeExecutionObservation) {
	result.ManualIntervention = pipelineManualIntervention{Reasons: make([]string, 0)}
	switch {
	case result.InvalidEvidence:
		result.Status = pipelineInvalid
		result.ManualIntervention.Required = true
		result.ManualIntervention.Reasons = append(result.ManualIntervention.Reasons, "Local journal or Git evidence is invalid or contradictory.")
	case result.Dispatch.State == "rejected":
		result.Status = pipelineRejected
		result.ManualIntervention.Required = true
		result.ManualIntervention.Reasons = append(result.ManualIntervention.Reasons, "The exactly correlated workflow dispatch was rejected.")
	case result.Dispatch.State == "request-started" || result.Dispatch.State == "unknown" || selected != nil && selected.Recovery.Uncertain:
		result.Status = pipelineUncertain
		result.ManualIntervention.Required = true
		result.ManualIntervention.Reasons = append(result.ManualIntervention.Reasons, "A durable external-effect boundary is uncertain; automatic retry is prohibited.")
	case selected != nil && selected.Recovery.ManualIntervention:
		result.Status = pipelineBlocked
		result.ManualIntervention.Required = true
		result.ManualIntervention.Reasons = append(result.ManualIntervention.Reasons, "Existing recovery or resume policy requires manual inspection.")
	case selected != nil && selected.Recovery.ResumeEligible:
		result.Status = pipelineResumable
	case selected != nil && selected.Unresolved:
		result.Status = pipelineActive
	case selected != nil && !selected.Unresolved && result.Dispatch.State == "accepted":
		result.Status = pipelineCompleted
	case selected != nil:
		result.Status = pipelineBlocked
		result.ManualIntervention.Required = true
		result.ManualIntervention.Reasons = append(result.ManualIntervention.Reasons, "Completed local execution evidence lacks an exactly accepted workflow handoff.")
	default:
		result.Status = pipelineReady
	}
	result.Recovery.ManualInterventionRequired = result.ManualIntervention.Required
}

func relevantDispatchObservations(result *pipelineResult, snapshot RuntimeSnapshot) []RuntimeDispatchObservation {
	observations := make([]RuntimeDispatchObservation, 0)
	for _, observation := range snapshot.Dispatches {
		if observation.UnitID != "" && observation.UnitID != result.Unit.ID {
			continue
		}
		if observation.RepositoryRemote != "" && snapshot.RepositoryRemote != "" && observation.RepositoryRemote != snapshot.RepositoryRemote {
			continue
		}
		observations = append(observations, observation)
	}
	sort.Slice(observations, func(i, j int) bool {
		if observations[i].Identity == observations[j].Identity {
			return observations[i].Reference < observations[j].Reference
		}
		return observations[i].Identity < observations[j].Identity
	})
	return observations
}

func applyDispatchStageRuntime(stages []LifecycleStage, dispatch RuntimeDispatchObservation) {
	for index := range stages {
		if stages[index].ID != "workflow-request-submission" {
			continue
		}
		stages[index].RuntimeEvidence = "dispatch_journal"
		stages[index].RuntimeIdentity = dispatch.Identity
		switch dispatch.State {
		case "prepared":
			stages[index].RuntimeStatus = RuntimeNotStarted
		case "request-started", "unknown":
			stages[index].RuntimeStatus = RuntimeUnknown
			stages[index].RuntimeReason = "the durable dispatch outcome is not safe to infer or retry"
		case "accepted":
			stages[index].RuntimeStatus = RuntimeConfirmed
			stages[index].RuntimeConfirmedAt = dispatch.UpdatedAt
		case "rejected":
			stages[index].RuntimeStatus = RuntimeRejected
			stages[index].RuntimeConfirmedAt = dispatch.UpdatedAt
		}
	}
}

func relevantExecutionObservations(result *pipelineResult, snapshot RuntimeSnapshot) []RuntimeExecutionObservation {
	observations := make([]RuntimeExecutionObservation, 0)
	for _, observation := range snapshot.Executions {
		if observation.UnitID != "" && observation.UnitID != result.Unit.ID {
			continue
		}
		if observation.RepositoryRemote != "" && snapshot.RepositoryRemote != "" && observation.RepositoryRemote != snapshot.RepositoryRemote {
			continue
		}
		observations = append(observations, observation)
	}
	sort.Slice(observations, func(i, j int) bool {
		if observations[i].Identity == observations[j].Identity {
			return observations[i].Reference < observations[j].Reference
		}
		return observations[i].Identity < observations[j].Identity
	})
	return observations
}

func validUnresolvedExecutions(observations []RuntimeExecutionObservation) []RuntimeExecutionObservation {
	matches := make([]RuntimeExecutionObservation, 0)
	for _, observation := range observations {
		if observation.Valid && observation.Unresolved {
			matches = append(matches, observation)
		}
	}
	return matches
}

func currentCompletedExecutions(result *pipelineResult, observations []RuntimeExecutionObservation) []RuntimeExecutionObservation {
	matches := make([]RuntimeExecutionObservation, 0)
	for _, observation := range observations {
		if !observation.Valid || observation.Unresolved ||
			observation.NextVersion != result.Unit.ConfiguredVersion ||
			observation.Tag != result.Release.ConfiguredTag ||
			observation.Executor != result.Unit.Executor ||
			observation.Delivery != result.Unit.Delivery ||
			observation.WorkflowPath != result.Workflow.Path {
			continue
		}
		matches = append(matches, observation)
	}
	return matches
}

func applyExecutionStageRuntime(stages []LifecycleStage, observation RuntimeExecutionObservation) {
	for index := range stages {
		stages[index].RuntimeStatus = RuntimeNotStarted
		if containsRuntimeStage(observation.ConfirmedStageIDs, stages[index].ID) {
			stages[index].RuntimeStatus = RuntimeConfirmed
			stages[index].RuntimeEvidence = "execution_journal"
			stages[index].RuntimeIdentity = observation.Identity
		}
		if containsRuntimeStage(observation.CurrentStageIDs, stages[index].ID) {
			stages[index].RuntimeConfirmedAt = observation.UpdatedAt
		}
		if observation.PendingStageID == stages[index].ID {
			stages[index].RuntimeStatus = RuntimePending
			stages[index].RuntimeEvidence = "execution_journal"
			stages[index].RuntimeIdentity = observation.Identity
			stages[index].RuntimeReason = "authoritative execution journal records a pending operation"
		}
	}
}

func containsRuntimeStage(stages []string, target string) bool {
	for _, stage := range stages {
		if stage == target {
			return true
		}
	}
	return false
}
