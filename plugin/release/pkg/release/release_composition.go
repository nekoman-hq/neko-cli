package release

import "github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"

func (runner *GitHubActionsReleaseRunner) newUseCase(repositoryRoot string) *githubActionsReleaseUseCase {
	executionJournal := newReleaseExecutionJournalStore(repositoryRoot, runner.coordinator.runner, runner.clock)
	dispatchJournal := newDispatchJournalStore(repositoryRoot, runner.coordinator.runner, runner.clock)
	git := githubActionsReleaseGitAdapter{coordinator: runner.coordinator}
	dispatchVerifier := gitReleaseDispatchVerifier{coordinator: runner.coordinator}
	return &githubActionsReleaseUseCase{
		tokenResolver:      runner.tokenResolver,
		planner:            versionMaterializationReleasePlanner{progress: runner.progress},
		preflightValidator: validateGitHubActionsReleasePreflight{git: git, unresolvedExecutions: executionJournal, progress: runner.progress},
		executionPreparer:  prepareGitHubActionsReleaseExecution{journal: executionJournal, clock: runner.clock, progress: runner.progress},
		materialization: applyGitHubActionsReleaseMaterialization{
			journal:      executionJournal,
			transactions: defaultReleaseMaterializationTransactionFactory{},
			progress:     runner.progress,
		},
		stateWriter: writeGitHubActionsReleaseState{
			journal:      executionJournal,
			transactions: defaultReleaseStateTransactionFactory{},
			progress:     runner.progress,
		},
		fileStager:       stageGitHubActionsReleaseFiles{journal: executionJournal, git: git, progress: runner.progress},
		commitCreator:    createGitHubActionsReleaseCommit{journal: executionJournal, git: git, progress: runner.progress},
		tagCreator:       createGitHubActionsReleaseTag{journal: executionJournal, git: git, progress: runner.progress},
		dispatchPreparer: prepareGitHubActionsReleaseDispatch{journal: executionJournal, dispatch: dispatchJournal, requests: verifiedReleaseDispatchRequestBuilder{git: dispatchVerifier}, progress: runner.progress},
		commitPusher:     pushGitHubActionsReleaseCommit{journal: executionJournal, git: git, progress: runner.progress},
		tagPusher:        pushGitHubActionsReleaseTag{journal: executionJournal, git: git, progress: runner.progress},
		workflowDispatcher: dispatchGitHubActionsReleaseWorkflow{
			client:   runner.dispatchClient,
			journal:  executionJournal,
			store:    dispatchJournal,
			clock:    runner.clock,
			progress: runner.progress,
		},
		handoffConfirmer: confirmGitHubActionsReleaseHandoff{journal: executionJournal},
		progress:         runner.progress,
	}
}

func sanitizeRemoteForLog(raw string) string {
	return releaseworkflow.SanitizeRemoteForLog(raw)
}
