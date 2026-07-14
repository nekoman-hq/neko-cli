package release

import (
	"context"
	"errors"
	"fmt"
)

//nolint:govet // Fields follow the continuation contract rather than memory layout.
type reconstructedResumeRelease struct {
	Context   *ReleaseExecutionContext
	Execution preparedGitHubActionsReleaseExecution
	Preflight validatedGitHubActionsReleasePreflight
	Files     KnownReleaseFiles
	CommitSHA string
}

type resumableReleaseExecution struct {
	Discovered *resumableExecution
	Context    *ReleaseExecutionContext
}

type resumeReleasePreparer interface {
	Prepare(*resumableReleaseExecution) (reconstructedResumeRelease, *CommandFailure)
}

type resumeReleaseExecutionOperation interface {
	Resume(context.Context, *resumableReleaseExecution) (*GitHubActionsReleaseResult, *CommandFailure)
}

type resumeReleaseTagInspector interface {
	TagCommit(repositoryRoot, tag string) (string, error)
}

type resumeFromTagCreatedContinuation interface {
	ResumePrepared(context.Context, reconstructedResumeRelease) (*GitHubActionsReleaseResult, *CommandFailure)
}

type resumeFromTagPushedContinuation interface {
	ResumePrepared(context.Context, reconstructedResumeRelease) (*GitHubActionsReleaseResult, *CommandFailure)
}

type resumeFromCommitCreatedOperation struct {
	preparer     resumeReleasePreparer
	tags         resumeReleaseTagInspector
	creator      githubActionsReleaseTagCreator
	continuation resumeFromTagCreatedContinuation
}

func (operation resumeFromCommitCreatedOperation) Resume(ctx context.Context, execution *resumableReleaseExecution) (*GitHubActionsReleaseResult, *CommandFailure) {
	release, failure := operation.preparer.Prepare(execution)
	if failure != nil {
		return nil, failure
	}
	tagCommit, err := operation.tags.TagCommit(release.Context.RepositoryRoot, release.Context.Tag)
	if err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	if tagCommit != "" {
		if tagCommit != release.CommitSHA {
			return nil, failureFromError("RESUME_FAILED", fmt.Errorf("tag %q points to %s, expected %s", release.Context.Tag, tagCommit, release.CommitSHA))
		}
		return nil, failureFromError("RESUME_FAILED", fmt.Errorf("resume from state %s requires manual inspection before continuing", ReleaseExecutionCommitCreated))
	}
	if err := operation.creator.Create(release.Context, release.Execution, release.CommitSHA); err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	return operation.continuation.ResumePrepared(ctx, release)
}

type resumeDispatchJournalLoader interface {
	Load(*ReleaseDispatchRequest) (*DispatchJournalResolution, error)
}

type assessedGitHubActionsResumeDispatch struct {
	Journal  *DispatchJournal
	Dispatch preparedGitHubActionsReleaseDispatch
}

type githubActionsResumeDispatchAssessor interface {
	Assess(reconstructedResumeRelease) (assessedGitHubActionsResumeDispatch, error)
}

type assessGitHubActionsResumeDispatch struct {
	journal  resumeDispatchJournalLoader
	requests releaseDispatchRequestBuilder
}

func (assessor assessGitHubActionsResumeDispatch) Assess(release reconstructedResumeRelease) (assessedGitHubActionsResumeDispatch, error) {
	request, err := assessor.requests.Build(release.Context, createdGitHubActionsReleaseResult(release.Context, release.Preflight, release.Files, release.CommitSHA))
	if err != nil {
		return assessedGitHubActionsResumeDispatch{}, err
	}
	resolution, err := assessor.journal.Load(request)
	if err != nil {
		return assessedGitHubActionsResumeDispatch{}, err
	}
	return assessedGitHubActionsResumeDispatch{
		Dispatch: preparedGitHubActionsReleaseDispatch{Request: request, Path: resolution.Path},
		Journal:  resolution.Journal,
	}, nil
}

type resumeFromTagCreatedOperation struct {
	preparer         resumeReleasePreparer
	dispatches       githubActionsResumeDispatchAssessor
	dispatchPreparer githubActionsReleaseDispatchPreparer
	commitPusher     githubActionsReleaseCommitPusher
	tagPusher        githubActionsReleaseTagPusher
	continuation     resumeFromTagPushedContinuation
}

func (operation resumeFromTagCreatedOperation) Resume(ctx context.Context, execution *resumableReleaseExecution) (*GitHubActionsReleaseResult, *CommandFailure) {
	release, failure := operation.preparer.Prepare(execution)
	if failure != nil {
		return nil, failure
	}
	return operation.ResumePrepared(ctx, release)
}

func (operation resumeFromTagCreatedOperation) ResumePrepared(ctx context.Context, release reconstructedResumeRelease) (*GitHubActionsReleaseResult, *CommandFailure) {
	dispatch, err := operation.dispatches.Assess(release)
	if err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	if resolution := resolveResumeDispatch(dispatch.Journal); resolution.Refusal != nil {
		return nil, failureForResumeDispatchRefusal(resolution.Refusal)
	}
	if _, err := operation.dispatchPreparer.Prepare(release.Context, release.Execution, release.Preflight, release.Files, release.CommitSHA); err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	if err := operation.commitPusher.Push(release.Context, release.Execution, release.Preflight, release.CommitSHA); err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	if err := operation.tagPusher.Push(release.Context, release.Execution, release.Preflight, release.CommitSHA); err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	return operation.continuation.ResumePrepared(ctx, release)
}

type resumeDispatchExecutionOperation interface {
	Complete(context.Context, reconstructedResumeRelease, assessedGitHubActionsResumeDispatch) (*GitHubActionsReleaseResult, *CommandFailure)
}

type resumeDispatchOperationSelector struct {
	fresh    resumeDispatchExecutionOperation
	accepted resumeDispatchExecutionOperation
}

func (selector resumeDispatchOperationSelector) Select(kind resumeDispatchOperationKind) (resumeDispatchExecutionOperation, error) {
	switch kind {
	case requestFreshResumeDispatch:
		return selector.fresh, nil
	case reuseAcceptedResumeDispatch:
		return selector.accepted, nil
	default:
		return nil, fmt.Errorf("resume dispatch operation %d is unsupported", kind)
	}
}

type resumeFromTagPushedOperation struct {
	preparer   resumeReleasePreparer
	dispatches githubActionsResumeDispatchAssessor
	selector   resumeDispatchOperationSelector
}

func (operation resumeFromTagPushedOperation) Resume(ctx context.Context, execution *resumableReleaseExecution) (*GitHubActionsReleaseResult, *CommandFailure) {
	release, failure := operation.preparer.Prepare(execution)
	if failure != nil {
		return nil, failure
	}
	return operation.ResumePrepared(ctx, release)
}

func (operation resumeFromTagPushedOperation) ResumePrepared(ctx context.Context, release reconstructedResumeRelease) (*GitHubActionsReleaseResult, *CommandFailure) {
	dispatch, err := operation.dispatches.Assess(release)
	if err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	resolution := resolveResumeDispatch(dispatch.Journal)
	if resolution.Refusal != nil {
		return nil, failureForResumeDispatchRefusal(resolution.Refusal)
	}
	selected, err := operation.selector.Select(resolution.Operation)
	if err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	return selected.Complete(ctx, release, dispatch)
}

type requestFreshGitHubActionsResumeDispatch struct {
	tokens     GitHubActionsDispatchTokenResolver
	dispatcher githubActionsReleaseWorkflowDispatcher
	handoff    githubActionsReleaseHandoffConfirmer
}

func (operation requestFreshGitHubActionsResumeDispatch) Complete(ctx context.Context, release reconstructedResumeRelease, dispatch assessedGitHubActionsResumeDispatch) (*GitHubActionsReleaseResult, *CommandFailure) {
	token, err := operation.tokens.ResolveGitHubActionsDispatchToken(ctx)
	if err != nil {
		return nil, failureFromError("TOKEN_MISSING", err)
	}
	result, err := operation.dispatcher.Dispatch(ctx, release.Context, release.Execution, dispatch.Dispatch, token)
	if err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	if !result.Accepted {
		return nil, failureFromError("RESUME_FAILED", errors.New(result.RecoveryGuidance))
	}
	if err := operation.handoff.Confirm(release.Execution); err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	return acceptedGitHubActionsReleaseResult(release.Context, release.Execution, release.CommitSHA, result), nil
}

type reuseAcceptedGitHubActionsResumeDispatch struct {
	handoff githubActionsReleaseHandoffConfirmer
}

func (operation reuseAcceptedGitHubActionsResumeDispatch) Complete(_ context.Context, release reconstructedResumeRelease, dispatch assessedGitHubActionsResumeDispatch) (*GitHubActionsReleaseResult, *CommandFailure) {
	result := dispatchResultFromJournal(dispatch.Dispatch.Path, dispatch.Journal, false)
	if err := operation.handoff.Confirm(release.Execution); err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	return acceptedGitHubActionsReleaseResult(release.Context, release.Execution, release.CommitSHA, result), nil
}

type returnCompletedReleaseHandoffOperation struct{}

func (returnCompletedReleaseHandoffOperation) Resume(_ context.Context, execution *resumableReleaseExecution) (*GitHubActionsReleaseResult, *CommandFailure) {
	journal := execution.Discovered.resolution.Journal
	return &GitHubActionsReleaseResult{
		Unit:                 journal.UnitID,
		Version:              journal.NextVersion,
		Tag:                  journal.Tag,
		CommitSHA:            journal.ReleaseCommitSHA,
		Workflow:             journal.WorkflowPath,
		ExecutionJournalPath: execution.Discovered.resolution.Path,
		ExecutionState:       ReleaseExecutionHandoffReady,
		RecoveryGuidance:     "Release was already handed off.",
	}, nil
}

func failureForResumeDispatchRefusal(refusal *resumeDispatchRefusal) *CommandFailure {
	return failureFromError("RESUME_FAILED", fmt.Errorf("dispatch journal is %s; do not dispatch again automatically", refusal.State))
}
