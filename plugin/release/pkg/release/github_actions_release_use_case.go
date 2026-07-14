package release

import (
	"context"
	"fmt"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

type plannedGitHubActionsRelease struct {
	MaterializationPlan *MaterializationPlan
	KnownFiles          KnownReleaseFiles
}

type validatedGitHubActionsReleasePreflight struct {
	Git           GitReleasePreflight
	RemoteURL     string
	BaseCommitSHA string
}

type preparedGitHubActionsReleaseExecution struct {
	Identity ReleaseExecutionIdentity
	Path     string
}

type preparedGitHubActionsReleaseDispatch struct {
	Request *ReleaseDispatchRequest
	Path    string
}

type githubActionsReleasePlanner interface {
	Plan(execCtx *ReleaseExecutionContext) (plannedGitHubActionsRelease, error)
}

type githubActionsReleasePreflightValidator interface {
	Validate(execCtx *ReleaseExecutionContext, files KnownReleaseFiles) (validatedGitHubActionsReleasePreflight, error)
}

type githubActionsReleaseExecutionPreparer interface {
	Prepare(execCtx *ReleaseExecutionContext, files KnownReleaseFiles, preflight validatedGitHubActionsReleasePreflight) (preparedGitHubActionsReleaseExecution, error)
}

type releaseMaterializationRollback interface {
	Restore() error
}

type githubActionsReleaseMaterializationApplier interface {
	Apply(execution preparedGitHubActionsReleaseExecution, plan *MaterializationPlan) (releaseMaterializationRollback, error)
}

type releaseStateRollback interface {
	RestoreSnapshot() error
}

type githubActionsReleaseStateWriter interface {
	Write(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, materialization releaseMaterializationRollback) (releaseStateRollback, error)
}

type githubActionsReleaseFileStager interface {
	Stage(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, files KnownReleaseFiles, state releaseStateRollback, materialization releaseMaterializationRollback) error
}

type githubActionsReleaseCommitCreator interface {
	Create(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, files KnownReleaseFiles) (string, error)
}

type githubActionsReleaseTagCreator interface {
	Create(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, commitSHA string) error
}

type githubActionsReleaseDispatchPreparer interface {
	Prepare(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, preflight validatedGitHubActionsReleasePreflight, files KnownReleaseFiles, commitSHA string) (preparedGitHubActionsReleaseDispatch, error)
}

type githubActionsReleaseCommitPusher interface {
	Push(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, preflight validatedGitHubActionsReleasePreflight, commitSHA string) error
}

type githubActionsReleaseTagPusher interface {
	Push(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, preflight validatedGitHubActionsReleasePreflight, commitSHA string) error
}

type githubActionsReleaseWorkflowDispatcher interface {
	Dispatch(ctx context.Context, execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, dispatch preparedGitHubActionsReleaseDispatch, token string) (*GitHubActionsDispatchResult, error)
}

type githubActionsReleaseHandoffConfirmer interface {
	Confirm(execution preparedGitHubActionsReleaseExecution) error
}

// githubActionsReleaseUseCase executes one active V2 GitHub Actions release.
// Its fields are the ordered release operations, not a generic dependency bag.
type githubActionsReleaseUseCase struct {
	tokenResolver      GitHubActionsDispatchTokenResolver
	planner            githubActionsReleasePlanner
	preflightValidator githubActionsReleasePreflightValidator
	executionPreparer  githubActionsReleaseExecutionPreparer
	materialization    githubActionsReleaseMaterializationApplier
	stateWriter        githubActionsReleaseStateWriter
	fileStager         githubActionsReleaseFileStager
	commitCreator      githubActionsReleaseCommitCreator
	tagCreator         githubActionsReleaseTagCreator
	dispatchPreparer   githubActionsReleaseDispatchPreparer
	commitPusher       githubActionsReleaseCommitPusher
	tagPusher          githubActionsReleaseTagPusher
	workflowDispatcher githubActionsReleaseWorkflowDispatcher
	handoffConfirmer   githubActionsReleaseHandoffConfirmer
}

func (useCase *githubActionsReleaseUseCase) Run(ctx context.Context, execCtx *ReleaseExecutionContext) (*GitHubActionsReleaseResult, error) {
	log.PluginPrint(log.Exec, "GitHub token preflight: resolving token without printing it")
	token, err := useCase.tokenResolver.ResolveGitHubActionsDispatchToken(ctx)
	if err != nil {
		return nil, err
	}
	log.PluginPrint(log.Exec, "GitHub token preflight: token available")

	planned, err := useCase.planner.Plan(execCtx)
	if err != nil {
		return nil, err
	}
	preflight, err := useCase.preflightValidator.Validate(execCtx, planned.KnownFiles)
	if err != nil {
		return nil, err
	}
	execution, err := useCase.executionPreparer.Prepare(execCtx, planned.KnownFiles, preflight)
	if err != nil {
		return nil, err
	}

	materialization, err := useCase.materialization.Apply(execution, planned.MaterializationPlan)
	if err != nil {
		return nil, err
	}
	state, err := useCase.stateWriter.Write(execCtx, execution, materialization)
	if err != nil {
		return nil, err
	}
	if stageErr := useCase.fileStager.Stage(execCtx, execution, planned.KnownFiles, state, materialization); stageErr != nil {
		return nil, stageErr
	}
	commitSHA, err := useCase.commitCreator.Create(execCtx, execution, planned.KnownFiles)
	if err != nil {
		return nil, err
	}
	if tagErr := useCase.tagCreator.Create(execCtx, execution, commitSHA); tagErr != nil {
		return nil, tagErr
	}
	dispatch, err := useCase.dispatchPreparer.Prepare(execCtx, execution, preflight, planned.KnownFiles, commitSHA)
	if err != nil {
		return nil, err
	}
	if pushErr := useCase.commitPusher.Push(execCtx, execution, preflight, commitSHA); pushErr != nil {
		return nil, pushErr
	}
	if pushErr := useCase.tagPusher.Push(execCtx, execution, preflight, commitSHA); pushErr != nil {
		return nil, pushErr
	}
	dispatchResult, err := useCase.workflowDispatcher.Dispatch(ctx, execCtx, execution, dispatch, token)
	if err != nil {
		return nil, err
	}
	if !dispatchResult.Accepted {
		return rejectedGitHubActionsReleaseResult(execCtx, execution, dispatch, commitSHA, dispatchResult), fmt.Errorf("GitHub Actions dispatch was not accepted: %s", dispatchResult.RecoveryGuidance)
	}
	if err := useCase.handoffConfirmer.Confirm(execution); err != nil {
		return nil, err
	}

	log.PluginPrint(log.Exec, "Execution state: %s", ReleaseExecutionHandoffReady)
	log.PluginPrint(log.Exec, "Recovery guidance: GitHub Actions dispatch accepted. GitHub Actions owns build and publish from the pushed tag.")
	return acceptedGitHubActionsReleaseResult(execCtx, execution, commitSHA, dispatchResult), nil
}

func rejectedGitHubActionsReleaseResult(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, dispatch preparedGitHubActionsReleaseDispatch, commitSHA string, dispatchResult *GitHubActionsDispatchResult) *GitHubActionsReleaseResult {
	return &GitHubActionsReleaseResult{
		Unit:                 execCtx.Unit.ID,
		Version:              execCtx.NextVersion,
		Tag:                  execCtx.Tag,
		CommitSHA:            commitSHA,
		Workflow:             execCtx.Workflow,
		ExecutionJournalPath: execution.Path,
		DispatchJournalPath:  dispatch.Path,
		ExecutionState:       ReleaseExecutionTagPushed,
		DispatchState:        dispatchResult.State,
		RecoveryGuidance:     dispatchResult.RecoveryGuidance,
		DispatchRunURL:       dispatchResult.HTMLURL,
	}
}

func acceptedGitHubActionsReleaseResult(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, commitSHA string, dispatchResult *GitHubActionsDispatchResult) *GitHubActionsReleaseResult {
	return &GitHubActionsReleaseResult{
		Unit:                 execCtx.Unit.ID,
		Version:              execCtx.NextVersion,
		Tag:                  execCtx.Tag,
		CommitSHA:            commitSHA,
		Workflow:             execCtx.Workflow,
		ExecutionJournalPath: execution.Path,
		DispatchJournalPath:  dispatchResult.JournalPath,
		ExecutionState:       ReleaseExecutionHandoffReady,
		DispatchState:        dispatchResult.State,
		RecoveryGuidance:     "GitHub Actions dispatch accepted. GitHub Actions owns build and publish from the pushed tag.",
		DispatchRunURL:       dispatchResult.HTMLURL,
	}
}
