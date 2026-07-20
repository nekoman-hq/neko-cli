package release

import "context"

type releaseDispatchRequestBuilder interface {
	Build(execCtx *ReleaseExecutionContext, result *GitReleaseResult) (*ReleaseDispatchRequest, error)
}

type verifiedReleaseDispatchRequestBuilder struct {
	git releaseDispatchGitVerifier
}

func (builder verifiedReleaseDispatchRequestBuilder) Build(execCtx *ReleaseExecutionContext, result *GitReleaseResult) (*ReleaseDispatchRequest, error) {
	return BuildReleaseDispatchRequest(execCtx, result, builder.git)
}

func createdGitHubActionsReleaseResult(execCtx *ReleaseExecutionContext, preflight validatedGitHubActionsReleasePreflight, files KnownReleaseFiles, commitSHA string) *GitReleaseResult {
	return &GitReleaseResult{
		Unit:                 execCtx.Unit.ID,
		Version:              execCtx.NextVersion,
		Tag:                  execCtx.Tag,
		CommitSHA:            commitSHA,
		RepositoryRemoteName: preflight.Git.Remote,
		RepositoryRemote:     preflight.RemoteURL,
		CommitCreated:        true,
		TagCreated:           true,
		ReachedPhase:         "tag-created",
		KnownReleaseFiles:    files.RelativePaths(),
	}
}

type prepareGitHubActionsReleaseDispatch struct {
	journal  releaseExecutionJournalMutations
	dispatch releaseDispatchJournalPreparation
	requests releaseDispatchRequestBuilder
	progress ReleaseProgress
}

func (operation prepareGitHubActionsReleaseDispatch) Prepare(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, preflight validatedGitHubActionsReleasePreflight, files KnownReleaseFiles, commitSHA string) (preparedGitHubActionsReleaseDispatch, error) {
	request, err := operation.requests.Build(execCtx, createdGitHubActionsReleaseResult(execCtx, preflight, files, commitSHA))
	if err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return preparedGitHubActionsReleaseDispatch{}, err
	}
	if _, pendingErr := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingCreateDispatchJournal); pendingErr != nil {
		return preparedGitHubActionsReleaseDispatch{}, pendingErr
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionRecorded, PendingAction: string(ReleaseExecutionPendingCreateDispatchJournal)})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressDispatchJournalPrepare})
	resolution, err := operation.dispatch.Prepare(request)
	if err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return preparedGitHubActionsReleaseDispatch{}, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressDispatchJournalReady, Path: resolution.Path})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressDispatchInputs, Inputs: releaseProgressInputs(request.Inputs)})
	if _, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionDispatchJournalPrepared, ReleaseExecutionJournalUpdate{DispatchJournalIdentity: request.Identity.SHA256}); err != nil {
		return preparedGitHubActionsReleaseDispatch{}, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressExecutionPhaseConfirmed, Phase: string(ReleaseExecutionDispatchJournalPrepared)})
	return preparedGitHubActionsReleaseDispatch{Request: request, Path: resolution.Path}, nil
}

type pushGitHubActionsReleaseCommit struct {
	journal  releaseExecutionJournalMutations
	git      githubActionsReleaseCommitPushGit
	progress ReleaseProgress
}

func (operation pushGitHubActionsReleaseCommit) Push(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, preflight validatedGitHubActionsReleasePreflight, commitSHA string) error {
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{
		Kind:           ReleaseProgressReleaseCommitPushing,
		CommitSHA:      commitSHA,
		Remote:         preflight.Git.Remote,
		UpstreamBranch: preflight.Git.UpstreamBranch,
	})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionStarting, PendingAction: string(ReleaseExecutionPendingPushReleaseCommit)})
	if _, err := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingPushReleaseCommit); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionRecorded, PendingAction: string(ReleaseExecutionPendingPushReleaseCommit)})
	if err := operation.git.PushCommit(execCtx, preflight.Git.Remote, preflight.Git.UpstreamBranch, commitSHA); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionFinished, PendingAction: string(ReleaseExecutionPendingPushReleaseCommit)})
	if _, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionCommitPushed, ReleaseExecutionJournalUpdate{CommitPushStatus: "pushed"}); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressExecutionPhaseConfirmed, Phase: string(ReleaseExecutionCommitPushed)})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressReleaseCommitPushDone})
	return nil
}

type pushGitHubActionsReleaseTag struct {
	journal  releaseExecutionJournalMutations
	git      githubActionsReleaseTagPushGit
	progress ReleaseProgress
}

func (operation pushGitHubActionsReleaseTag) Push(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, preflight validatedGitHubActionsReleasePreflight, commitSHA string) error {
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressUnitTagPushPreparing, Tag: execCtx.Tag})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionStarting, PendingAction: string(ReleaseExecutionPendingPushUnitTag)})
	if _, err := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingPushUnitTag); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionRecorded, PendingAction: string(ReleaseExecutionPendingPushUnitTag)})
	if err := operation.git.PushTag(execCtx, preflight.Git.Remote, execCtx.Tag, commitSHA); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionFinished, PendingAction: string(ReleaseExecutionPendingPushUnitTag)})
	if _, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionTagPushed, ReleaseExecutionJournalUpdate{TagPushStatus: "pushed"}); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressExecutionPhaseConfirmed, Phase: string(ReleaseExecutionTagPushed)})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressUnitTagPushDone})
	return nil
}

type dispatchGitHubActionsReleaseWorkflow struct {
	client   GitHubActionsWorkflowDispatchClient
	journal  releaseExecutionErrorRecorder
	store    *DispatchJournalStore
	clock    ReleaseClock
	progress ReleaseProgress
}

func (operation dispatchGitHubActionsReleaseWorkflow) Dispatch(ctx context.Context, execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, dispatch preparedGitHubActionsReleaseDispatch, token GitHubActionsDispatchToken) (*GitHubActionsDispatchResult, error) {
	dispatcher, err := NewGitHubActionsDispatcher(operation.store.RepositoryRoot,
		WithGitHubActionsDispatcherClient(operation.client),
		WithGitHubActionsDispatcherStore(operation.store),
		WithGitHubActionsDispatcherClock(operation.clock),
		WithGitHubActionsDispatcherProgress(operation.progress),
	)
	if err != nil {
		return nil, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressWorkflowDispatching, Workflow: execCtx.Workflow, Ref: execCtx.Tag})
	result, err := dispatcher.dispatchWithToken(ctx, dispatch.Request, token)
	if err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressDispatchState, DispatchState: string(result.State)})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressDispatchRun, DispatchRunURL: result.HTMLURL})
	if !result.Accepted {
		_, _ = operation.journal.RecordLastError(execution.Identity, result.RecoveryGuidance)
	}
	return result, nil
}

type confirmGitHubActionsReleaseHandoff struct {
	journal releaseExecutionPhaseConfirmer
}

func (operation confirmGitHubActionsReleaseHandoff) Confirm(execution preparedGitHubActionsReleaseExecution) error {
	_, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionHandoffReady, ReleaseExecutionJournalUpdate{})
	return err
}
