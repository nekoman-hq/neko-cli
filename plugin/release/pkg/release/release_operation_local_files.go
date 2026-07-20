package release

import (
	"fmt"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type validateGitHubActionsReleasePreflight struct {
	git                  githubActionsReleasePreflightGit
	unresolvedExecutions unresolvedReleaseExecutionFinder
	progress             ReleaseProgress
}

func (operation validateGitHubActionsReleasePreflight) Validate(execCtx *ReleaseExecutionContext, files KnownReleaseFiles) (validatedGitHubActionsReleasePreflight, error) {
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitPreflightStarted})
	preflight, err := operation.git.Preflight(execCtx, files)
	if err != nil {
		return validatedGitHubActionsReleasePreflight{}, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{
		Kind:           ReleaseProgressGitPreflightSummary,
		Branch:         preflight.Branch,
		Remote:         preflight.Remote,
		UpstreamBranch: preflight.UpstreamBranch,
	})
	remoteURL, err := operation.git.RemoteURL(execCtx.RepositoryRoot, preflight.Remote)
	if err != nil {
		return validatedGitHubActionsReleasePreflight{}, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitPreflightRemoteURL, SafeRemoteURL: sanitizeRemoteForLog(remoteURL)})
	if _, targetErr := ResolveGitHubRepositoryTarget(preflight.Remote, remoteURL); targetErr != nil {
		return validatedGitHubActionsReleasePreflight{}, targetErr
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitPreflightWorkflowValidated, Workflow: execCtx.Workflow})
	unresolved, err := operation.unresolvedExecutions.FindUnresolved(remoteURL, execCtx.Unit.ID)
	if err != nil {
		return validatedGitHubActionsReleasePreflight{}, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitPreflightUnresolvedJournals, Count: len(unresolved)})
	if len(unresolved) > 0 {
		return validatedGitHubActionsReleasePreflight{}, fmt.Errorf("unresolved V2 release execution journal exists for unit %q; use neko release resume --unit %s", execCtx.Unit.ID, execCtx.Unit.ID)
	}
	baseSHA, err := operation.git.HeadCommit(execCtx.RepositoryRoot)
	if err != nil {
		return validatedGitHubActionsReleasePreflight{}, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitPreflightBaseCommit, CommitSHA: baseSHA})
	return validatedGitHubActionsReleasePreflight{Git: preflight, RemoteURL: remoteURL, BaseCommitSHA: baseSHA}, nil
}

type prepareGitHubActionsReleaseExecution struct {
	journal  releaseExecutionJournalPreparation
	clock    ReleaseClock
	progress ReleaseProgress
}

func (operation prepareGitHubActionsReleaseExecution) Prepare(execCtx *ReleaseExecutionContext, files KnownReleaseFiles, preflight validatedGitHubActionsReleasePreflight) (preparedGitHubActionsReleaseExecution, error) {
	journal, err := BuildReleaseExecutionJournal(execCtx, BuildReleasePlan(execCtx), files, preflight.BaseCommitSHA, preflight.RemoteURL, operation.clock.Now())
	if err != nil {
		return preparedGitHubActionsReleaseExecution{}, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressExecutionJournalPreparing})
	resolution, err := operation.journal.Prepare(journal)
	if err != nil {
		return preparedGitHubActionsReleaseExecution{}, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{
		Kind:     ReleaseProgressExecutionJournalPrepared,
		Path:     resolution.Path,
		Identity: journal.Identity.SHA256,
	})
	journal = resolution.Journal
	if _, err := operation.journal.ConfirmPhase(journal.Identity, ReleaseExecutionPreflightValidated, ReleaseExecutionJournalUpdate{}); err != nil {
		return preparedGitHubActionsReleaseExecution{}, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressExecutionPhase, Phase: string(ReleaseExecutionPreflightValidated)})
	return preparedGitHubActionsReleaseExecution{Identity: journal.Identity, Path: resolution.Path}, nil
}

type applyGitHubActionsReleaseMaterialization struct {
	journal      releaseExecutionJournalMutations
	transactions releaseMaterializationTransactionFactory
	progress     ReleaseProgress
}

func (operation applyGitHubActionsReleaseMaterialization) Apply(execution preparedGitHubActionsReleaseExecution, plan *MaterializationPlan) (releaseMaterializationRollback, error) {
	transaction := operation.transactions.New(plan)
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressMaterializationSnapshotCapturing})
	if err := transaction.CaptureSnapshots(); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressMaterializationSnapshotCaptured})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionStarting, PendingAction: string(ReleaseExecutionPendingApplyMaterialization)})
	if _, err := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingApplyMaterialization); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionRecorded, PendingAction: string(ReleaseExecutionPendingApplyMaterialization)})
	if _, err := transaction.Apply(); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionFinished, PendingAction: string(ReleaseExecutionPendingApplyMaterialization)})
	if _, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionMaterializationApplied, ReleaseExecutionJournalUpdate{}); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressExecutionPhaseConfirmed, Phase: string(ReleaseExecutionMaterializationApplied)})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressMaterializedFilesApplied, Files: []string{materializedFilesValue(plan)}})
	return transaction, nil
}

type writeGitHubActionsReleaseState struct {
	journal      releaseExecutionJournalMutations
	transactions releaseStateTransactionFactory
	progress     ReleaseProgress
}

func (operation writeGitHubActionsReleaseState) Write(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, materialization releaseMaterializationRollback) (releaseStateRollback, error) {
	transaction := operation.transactions.New(execCtx.RepositoryRoot)
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressStateSnapshotCapturing})
	if err := transaction.CaptureSnapshot(); err != nil {
		_ = materialization.Restore()
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressStateSnapshotCaptured, Path: releaseconfig.V2StatePath(execCtx.RepositoryRoot)})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressStateUpdateWriting, UnitID: execCtx.Unit.ID, NextVersion: execCtx.NextVersion})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionStarting, PendingAction: string(ReleaseExecutionPendingWriteState)})
	if _, err := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingWriteState); err != nil {
		_ = materialization.Restore()
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionRecorded, PendingAction: string(ReleaseExecutionPendingWriteState)})
	if err := transaction.WriteUnitVersion(execCtx.Unit.ID, execCtx.NextVersion); err != nil {
		_ = materialization.Restore()
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionFinished, PendingAction: string(ReleaseExecutionPendingWriteState)})
	if _, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionStateWritten, ReleaseExecutionJournalUpdate{}); err != nil {
		_ = materialization.Restore()
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressExecutionPhaseConfirmed, Phase: string(ReleaseExecutionStateWritten)})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressStateUpdateWritten})
	return transaction, nil
}

type stageGitHubActionsReleaseFiles struct {
	journal  releaseExecutionJournalMutations
	git      githubActionsReleaseStagingGit
	progress ReleaseProgress
}

func (operation stageGitHubActionsReleaseFiles) Stage(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, files KnownReleaseFiles, state releaseStateRollback, materialization releaseMaterializationRollback) error {
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressStagingTargetedFiles, Files: files.RelativePaths()})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionStarting, PendingAction: string(ReleaseExecutionPendingStageReleaseFiles)})
	if _, err := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingStageReleaseFiles); err != nil {
		_ = state.RestoreSnapshot()
		_ = materialization.Restore()
		_ = operation.git.UnstageKnown(files)
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionRecorded, PendingAction: string(ReleaseExecutionPendingStageReleaseFiles)})
	if err := operation.git.Stage(execCtx, files); err != nil {
		_ = state.RestoreSnapshot()
		_ = materialization.Restore()
		_ = operation.git.UnstageKnown(files)
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionFinished, PendingAction: string(ReleaseExecutionPendingStageReleaseFiles)})
	if _, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionReleaseFilesStaged, ReleaseExecutionJournalUpdate{}); err != nil {
		_ = state.RestoreSnapshot()
		_ = materialization.Restore()
		_ = operation.git.UnstageKnown(files)
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressExecutionPhaseConfirmed, Phase: string(ReleaseExecutionReleaseFilesStaged)})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressTargetedFilesStaged})
	return nil
}

type createGitHubActionsReleaseCommit struct {
	journal  releaseExecutionJournalMutations
	git      githubActionsReleaseCommitGit
	progress ReleaseProgress
}

func (operation createGitHubActionsReleaseCommit) Create(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, files KnownReleaseFiles) (string, error) {
	if _, err := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingCreateReleaseCommit); err != nil {
		return "", err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionRecorded, PendingAction: string(ReleaseExecutionPendingCreateReleaseCommit)})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressReleaseCommitCreating, CommitMessage: ReleaseCommitMessage(execCtx)})
	commitSHA, err := operation.git.Commit(execCtx, files)
	if err == nil {
		_, err = operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionCommitCreated, ReleaseExecutionJournalUpdate{ReleaseCommitSHA: commitSHA})
	}
	if err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return "", err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressExecutionPhaseConfirmed, Phase: string(ReleaseExecutionCommitCreated)})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressReleaseCommitCreated, CommitSHA: commitSHA})
	return commitSHA, nil
}

type createGitHubActionsReleaseTag struct {
	journal  releaseExecutionJournalMutations
	git      githubActionsReleaseTagGit
	progress ReleaseProgress
}

func (operation createGitHubActionsReleaseTag) Create(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, commitSHA string) error {
	if _, err := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingCreateUnitTag); err != nil {
		return err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressPendingActionRecorded, PendingAction: string(ReleaseExecutionPendingCreateUnitTag)})
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressUnitTagCreating, Tag: execCtx.Tag})
	if _, err := operation.git.CreateTag(execCtx, commitSHA); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	if _, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionTagCreated, ReleaseExecutionJournalUpdate{TagTargetSHA: commitSHA}); err != nil {
		return err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressExecutionPhaseConfirmed, Phase: string(ReleaseExecutionTagCreated)})
	return nil
}
