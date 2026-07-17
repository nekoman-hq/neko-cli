package release

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type releaseExecutionPendingRecorder interface {
	BeginPending(identity ReleaseExecutionIdentity, action ReleaseExecutionPendingAction) (*ReleaseExecutionJournalResolution, error)
}

type releaseExecutionPhaseConfirmer interface {
	ConfirmPhase(identity ReleaseExecutionIdentity, next ReleaseExecutionJournalState, update ReleaseExecutionJournalUpdate) (*ReleaseExecutionJournalResolution, error)
}

type releaseExecutionErrorRecorder interface {
	RecordLastError(identity ReleaseExecutionIdentity, message string) (*ReleaseExecutionJournalResolution, error)
}

type releaseExecutionJournalMutations interface {
	releaseExecutionPendingRecorder
	releaseExecutionPhaseConfirmer
	releaseExecutionErrorRecorder
}

type releaseExecutionJournalPreparation interface {
	Prepare(expected *ReleaseExecutionJournal) (*ReleaseExecutionJournalResolution, error)
	releaseExecutionPhaseConfirmer
}

type unresolvedReleaseExecutionFinder interface {
	FindUnresolved(repositoryRemote, unitID string) ([]ReleaseExecutionJournalResolution, error)
}

type releaseDispatchJournalPreparation interface {
	Prepare(request *ReleaseDispatchRequest) (*DispatchJournalResolution, error)
}

type releaseMaterializationTransaction interface {
	CaptureSnapshots() error
	Apply() (*AppliedMaterialization, error)
	Restore() error
}

type releaseMaterializationTransactionFactory interface {
	New(plan *MaterializationPlan) releaseMaterializationTransaction
}

type defaultReleaseMaterializationTransactionFactory struct{}

func (defaultReleaseMaterializationTransactionFactory) New(plan *MaterializationPlan) releaseMaterializationTransaction {
	return NewMaterializationTransaction(plan)
}

type releaseStateTransaction interface {
	CaptureSnapshot() error
	WriteUnitVersion(unitID, nextVersion string) error
	RestoreSnapshot() error
}

type releaseStateTransactionFactory interface {
	New(repositoryRoot string) releaseStateTransaction
}

type defaultReleaseStateTransactionFactory struct{}

func (defaultReleaseStateTransactionFactory) New(repositoryRoot string) releaseStateTransaction {
	return NewStateTransaction(repositoryRoot)
}

type githubActionsReleasePreflightGit interface {
	Preflight(execCtx *ReleaseExecutionContext, files KnownReleaseFiles) (GitReleasePreflight, error)
	RemoteURL(repositoryRoot, remote string) (string, error)
	HeadCommit(repositoryRoot string) (string, error)
}

type githubActionsReleaseStagingGit interface {
	Stage(execCtx *ReleaseExecutionContext, files KnownReleaseFiles) error
	UnstageKnown(files KnownReleaseFiles) error
}

type githubActionsReleaseCommitGit interface {
	Commit(execCtx *ReleaseExecutionContext, files KnownReleaseFiles) (string, error)
}

type githubActionsReleaseTagGit interface {
	CreateTag(execCtx *ReleaseExecutionContext, commitSHA string) (bool, error)
}

type githubActionsReleaseCommitPushGit interface {
	PushCommit(execCtx *ReleaseExecutionContext, remote, upstreamBranch, commitSHA string) error
}

type githubActionsReleaseTagPushGit interface {
	PushTag(execCtx *ReleaseExecutionContext, remote, tag, commitSHA string) error
}

type githubActionsReleaseGitAdapter struct {
	coordinator *GitReleaseCoordinator
}

func (adapter githubActionsReleaseGitAdapter) Preflight(execCtx *ReleaseExecutionContext, files KnownReleaseFiles) (GitReleasePreflight, error) {
	return adapter.coordinator.Preflight(execCtx, files)
}

func (adapter githubActionsReleaseGitAdapter) RemoteURL(repositoryRoot, remote string) (string, error) {
	remoteURL, err := adapter.coordinator.gitOutput(repositoryRoot, "remote", "get-url", remote)
	if err != nil {
		return "", fmt.Errorf("resolve V2 release remote %q: %w", remote, err)
	}
	return strings.TrimSpace(remoteURL), nil
}

func (adapter githubActionsReleaseGitAdapter) HeadCommit(repositoryRoot string) (string, error) {
	return adapter.coordinator.headCommit(repositoryRoot)
}

func (adapter githubActionsReleaseGitAdapter) Stage(execCtx *ReleaseExecutionContext, files KnownReleaseFiles) error {
	return adapter.coordinator.Stage(execCtx, files)
}

func (adapter githubActionsReleaseGitAdapter) UnstageKnown(files KnownReleaseFiles) error {
	return adapter.coordinator.UnstageKnown(files)
}

func (adapter githubActionsReleaseGitAdapter) Commit(execCtx *ReleaseExecutionContext, files KnownReleaseFiles) (string, error) {
	return adapter.coordinator.Commit(execCtx, files)
}

func (adapter githubActionsReleaseGitAdapter) CreateTag(execCtx *ReleaseExecutionContext, commitSHA string) (bool, error) {
	return adapter.coordinator.CreateTag(execCtx, commitSHA)
}

func (adapter githubActionsReleaseGitAdapter) PushCommit(execCtx *ReleaseExecutionContext, remote, upstreamBranch, commitSHA string) error {
	return adapter.coordinator.PushCommit(execCtx, remote, upstreamBranch, commitSHA)
}

func (adapter githubActionsReleaseGitAdapter) PushTag(execCtx *ReleaseExecutionContext, remote, tag, commitSHA string) error {
	return adapter.coordinator.PushTag(execCtx, remote, tag, commitSHA)
}

type versionMaterializationReleasePlanner struct {
	progress ReleaseProgress
}

func (operation versionMaterializationReleasePlanner) Plan(execCtx *ReleaseExecutionContext) (plannedGitHubActionsRelease, error) {
	materializer, err := ResolveVersionMaterializer(execCtx.Executor)
	if err != nil {
		return plannedGitHubActionsRelease{}, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressMaterializationPlanningStarted})
	plan, err := materializer.Plan(execCtx)
	if err != nil {
		return plannedGitHubActionsRelease{}, err
	}
	if validationErr := materializer.Validate(plan); validationErr != nil {
		return plannedGitHubActionsRelease{}, validationErr
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressMaterializationPlanningCompleted, Files: []string{materializedFilesValue(plan)}})
	knownFiles, err := NewKnownReleaseFiles(execCtx, plan)
	if err != nil {
		return plannedGitHubActionsRelease{}, err
	}
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{Kind: ReleaseProgressKnownFiles, Files: knownFiles.RelativePaths()})
	return plannedGitHubActionsRelease{MaterializationPlan: plan, KnownFiles: knownFiles}, nil
}

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
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return raw
	}
	parsed.User = nil
	return parsed.String()
}
