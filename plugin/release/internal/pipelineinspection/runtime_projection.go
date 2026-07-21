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
		return
	}
	if selected == nil {
		result.Execution.Validity = "valid"
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
