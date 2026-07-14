package release

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/log"
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

type versionMaterializationReleasePlanner struct{}

func (versionMaterializationReleasePlanner) Plan(execCtx *ReleaseExecutionContext) (plannedGitHubActionsRelease, error) {
	materializer, err := ResolveVersionMaterializer(execCtx.Executor)
	if err != nil {
		return plannedGitHubActionsRelease{}, err
	}
	log.PluginPrint(log.Exec, "Planning materialized files")
	plan, err := materializer.Plan(execCtx)
	if err != nil {
		return plannedGitHubActionsRelease{}, err
	}
	if validationErr := materializer.Validate(plan); validationErr != nil {
		return plannedGitHubActionsRelease{}, validationErr
	}
	log.PluginPrint(log.Exec, "Planned materialized files: %s", materializedFilesValue(plan))
	knownFiles, err := NewKnownReleaseFiles(execCtx, plan)
	if err != nil {
		return plannedGitHubActionsRelease{}, err
	}
	log.PluginPrint(log.Exec, "Known release files: %s", strings.Join(knownFiles.RelativePaths(), ", "))
	return plannedGitHubActionsRelease{MaterializationPlan: plan, KnownFiles: knownFiles}, nil
}

type validateGitHubActionsReleasePreflight struct {
	git                  githubActionsReleasePreflightGit
	unresolvedExecutions unresolvedReleaseExecutionFinder
}

func (operation validateGitHubActionsReleasePreflight) Validate(execCtx *ReleaseExecutionContext, files KnownReleaseFiles) (validatedGitHubActionsReleasePreflight, error) {
	log.PluginPrint(log.Exec, "Running git preflight checks")
	preflight, err := operation.git.Preflight(execCtx, files)
	if err != nil {
		return validatedGitHubActionsReleasePreflight{}, err
	}
	log.PluginPrint(log.Exec, "Git preflight: branch=%s remote=%s upstream=%s", preflight.Branch, preflight.Remote, preflight.UpstreamBranch)
	remoteURL, err := operation.git.RemoteURL(execCtx.RepositoryRoot, preflight.Remote)
	if err != nil {
		return validatedGitHubActionsReleasePreflight{}, err
	}
	log.PluginPrint(log.Exec, "Git preflight: remote URL=%s", sanitizeRemoteForLog(remoteURL))
	if _, targetErr := ResolveGitHubRepositoryTarget(preflight.Remote, remoteURL); targetErr != nil {
		return validatedGitHubActionsReleasePreflight{}, targetErr
	}
	log.PluginPrint(log.Exec, "Git preflight: workflow validation passed for %s", execCtx.Workflow)
	unresolved, err := operation.unresolvedExecutions.FindUnresolved(remoteURL, execCtx.Unit.ID)
	if err != nil {
		return validatedGitHubActionsReleasePreflight{}, err
	}
	log.PluginPrint(log.Exec, "Execution journal preflight: unresolved journals=%d", len(unresolved))
	if len(unresolved) > 0 {
		return validatedGitHubActionsReleasePreflight{}, fmt.Errorf("unresolved V2 release execution journal exists for unit %q; use neko release resume --unit %s", execCtx.Unit.ID, execCtx.Unit.ID)
	}
	baseSHA, err := operation.git.HeadCommit(execCtx.RepositoryRoot)
	if err != nil {
		return validatedGitHubActionsReleasePreflight{}, err
	}
	log.PluginPrint(log.Exec, "Base commit before release: %s", baseSHA)
	return validatedGitHubActionsReleasePreflight{Git: preflight, RemoteURL: remoteURL, BaseCommitSHA: baseSHA}, nil
}

type prepareGitHubActionsReleaseExecution struct {
	journal releaseExecutionJournalPreparation
	clock   ReleaseClock
}

func (operation prepareGitHubActionsReleaseExecution) Prepare(execCtx *ReleaseExecutionContext, files KnownReleaseFiles, preflight validatedGitHubActionsReleasePreflight) (preparedGitHubActionsReleaseExecution, error) {
	journal, err := BuildReleaseExecutionJournal(execCtx, BuildReleasePlan(execCtx), files, preflight.BaseCommitSHA, preflight.RemoteURL, operation.clock.Now())
	if err != nil {
		return preparedGitHubActionsReleaseExecution{}, err
	}
	log.PluginPrint(log.Exec, "Preparing execution journal")
	resolution, err := operation.journal.Prepare(journal)
	if err != nil {
		return preparedGitHubActionsReleaseExecution{}, err
	}
	log.PluginPrint(log.Exec, "Execution journal path: %s", resolution.Path)
	log.PluginPrint(log.Exec, "Execution journal identity: %s", journal.Identity.SHA256)
	journal = resolution.Journal
	if _, err := operation.journal.ConfirmPhase(journal.Identity, ReleaseExecutionPreflightValidated, ReleaseExecutionJournalUpdate{}); err != nil {
		return preparedGitHubActionsReleaseExecution{}, err
	}
	log.PluginPrint(log.Exec, "Execution phase: %s", ReleaseExecutionPreflightValidated)
	return preparedGitHubActionsReleaseExecution{Identity: journal.Identity, Path: resolution.Path}, nil
}

type applyGitHubActionsReleaseMaterialization struct {
	journal      releaseExecutionJournalMutations
	transactions releaseMaterializationTransactionFactory
}

func (operation applyGitHubActionsReleaseMaterialization) Apply(execution preparedGitHubActionsReleaseExecution, plan *MaterializationPlan) (releaseMaterializationRollback, error) {
	transaction := operation.transactions.New(plan)
	log.PluginPrint(log.Exec, "Capturing materialization snapshots")
	if err := transaction.CaptureSnapshots(); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	log.PluginPrint(log.Exec, "Materialization snapshots captured")
	log.PluginV(log.Exec, "Starting release action: %s", ReleaseExecutionPendingApplyMaterialization)
	if _, err := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingApplyMaterialization); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	log.PluginV(log.Exec, "Execution journal pending action recorded: %s", ReleaseExecutionPendingApplyMaterialization)
	if _, err := transaction.Apply(); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	log.PluginV(log.Exec, "Release action completed: %s", ReleaseExecutionPendingApplyMaterialization)
	if _, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionMaterializationApplied, ReleaseExecutionJournalUpdate{}); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	log.PluginV(log.Exec, "Execution phase confirmed: %s", ReleaseExecutionMaterializationApplied)
	log.PluginPrint(log.Exec, "Applied materialized files: %s", materializedFilesValue(plan))
	return transaction, nil
}

type writeGitHubActionsReleaseState struct {
	journal      releaseExecutionJournalMutations
	transactions releaseStateTransactionFactory
}

func (operation writeGitHubActionsReleaseState) Write(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, materialization releaseMaterializationRollback) (releaseStateRollback, error) {
	transaction := operation.transactions.New(execCtx.RepositoryRoot)
	log.PluginPrint(log.Exec, "Capturing V2 state snapshot")
	if err := transaction.CaptureSnapshot(); err != nil {
		_ = materialization.Restore()
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	log.PluginPrint(log.Exec, "V2 state snapshot captured: %s", releaseconfig.V2StatePath(execCtx.RepositoryRoot))
	log.PluginPrint(log.Exec, "Writing V2 state update: %s -> %s", execCtx.Unit.ID, execCtx.NextVersion)
	log.PluginV(log.Exec, "Starting release action: %s", ReleaseExecutionPendingWriteState)
	if _, err := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingWriteState); err != nil {
		_ = materialization.Restore()
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	log.PluginV(log.Exec, "Execution journal pending action recorded: %s", ReleaseExecutionPendingWriteState)
	if err := transaction.WriteUnitVersion(execCtx.Unit.ID, execCtx.NextVersion); err != nil {
		_ = materialization.Restore()
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	log.PluginV(log.Exec, "Release action completed: %s", ReleaseExecutionPendingWriteState)
	if _, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionStateWritten, ReleaseExecutionJournalUpdate{}); err != nil {
		_ = materialization.Restore()
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	log.PluginV(log.Exec, "Execution phase confirmed: %s", ReleaseExecutionStateWritten)
	log.PluginPrint(log.Exec, "State update written")
	return transaction, nil
}

type stageGitHubActionsReleaseFiles struct {
	journal releaseExecutionJournalMutations
	git     githubActionsReleaseStagingGit
}

func (operation stageGitHubActionsReleaseFiles) Stage(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, files KnownReleaseFiles, state releaseStateRollback, materialization releaseMaterializationRollback) error {
	log.PluginPrint(log.Exec, "Staging targeted release files: %s", strings.Join(files.RelativePaths(), ", "))
	log.PluginV(log.Exec, "Starting release action: %s", ReleaseExecutionPendingStageReleaseFiles)
	if _, err := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingStageReleaseFiles); err != nil {
		_ = state.RestoreSnapshot()
		_ = materialization.Restore()
		_ = operation.git.UnstageKnown(files)
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	log.PluginV(log.Exec, "Execution journal pending action recorded: %s", ReleaseExecutionPendingStageReleaseFiles)
	if err := operation.git.Stage(execCtx, files); err != nil {
		_ = state.RestoreSnapshot()
		_ = materialization.Restore()
		_ = operation.git.UnstageKnown(files)
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	log.PluginV(log.Exec, "Release action completed: %s", ReleaseExecutionPendingStageReleaseFiles)
	if _, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionReleaseFilesStaged, ReleaseExecutionJournalUpdate{}); err != nil {
		_ = state.RestoreSnapshot()
		_ = materialization.Restore()
		_ = operation.git.UnstageKnown(files)
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	log.PluginV(log.Exec, "Execution phase confirmed: %s", ReleaseExecutionReleaseFilesStaged)
	log.PluginPrint(log.Exec, "Targeted release files staged")
	return nil
}

type createGitHubActionsReleaseCommit struct {
	journal releaseExecutionJournalMutations
	git     githubActionsReleaseCommitGit
}

func (operation createGitHubActionsReleaseCommit) Create(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, files KnownReleaseFiles) (string, error) {
	if _, err := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingCreateReleaseCommit); err != nil {
		return "", err
	}
	log.PluginV(log.Exec, "Execution journal pending action recorded: %s", ReleaseExecutionPendingCreateReleaseCommit)
	log.PluginPrint(log.Exec, "Creating release commit: %s", ReleaseCommitMessage(execCtx))
	commitSHA, err := operation.git.Commit(execCtx, files)
	if err == nil {
		_, err = operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionCommitCreated, ReleaseExecutionJournalUpdate{ReleaseCommitSHA: commitSHA})
	}
	if err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return "", err
	}
	log.PluginV(log.Exec, "Execution phase confirmed: %s", ReleaseExecutionCommitCreated)
	log.PluginPrint(log.Exec, "Release commit created: %s", commitSHA)
	return commitSHA, nil
}

type createGitHubActionsReleaseTag struct {
	journal releaseExecutionJournalMutations
	git     githubActionsReleaseTagGit
}

func (operation createGitHubActionsReleaseTag) Create(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, commitSHA string) error {
	if _, err := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingCreateUnitTag); err != nil {
		return err
	}
	log.PluginV(log.Exec, "Execution journal pending action recorded: %s", ReleaseExecutionPendingCreateUnitTag)
	log.PluginPrint(log.Exec, "Creating unit tag: %s", execCtx.Tag)
	if _, err := operation.git.CreateTag(execCtx, commitSHA); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	if _, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionTagCreated, ReleaseExecutionJournalUpdate{TagTargetSHA: commitSHA}); err != nil {
		return err
	}
	log.PluginV(log.Exec, "Execution phase confirmed: %s", ReleaseExecutionTagCreated)
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
	log.PluginV(log.Exec, "Execution journal pending action recorded: %s", ReleaseExecutionPendingCreateDispatchJournal)
	log.PluginPrint(log.Exec, "Preparing dispatch journal")
	resolution, err := operation.dispatch.Prepare(request)
	if err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return preparedGitHubActionsReleaseDispatch{}, err
	}
	log.PluginPrint(log.Exec, "Dispatch journal path: %s", resolution.Path)
	log.PluginPrint(log.Exec, "Dispatch inputs: %s", dispatchInputsValue(request.Inputs))
	if _, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionDispatchJournalPrepared, ReleaseExecutionJournalUpdate{DispatchJournalIdentity: request.Identity.SHA256}); err != nil {
		return preparedGitHubActionsReleaseDispatch{}, err
	}
	log.PluginV(log.Exec, "Execution phase confirmed: %s", ReleaseExecutionDispatchJournalPrepared)
	return preparedGitHubActionsReleaseDispatch{Request: request, Path: resolution.Path}, nil
}

type pushGitHubActionsReleaseCommit struct {
	journal releaseExecutionJournalMutations
	git     githubActionsReleaseCommitPushGit
}

func (operation pushGitHubActionsReleaseCommit) Push(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, preflight validatedGitHubActionsReleasePreflight, commitSHA string) error {
	log.PluginPrint(log.Exec, "Pushing release commit %s to %s/%s", commitSHA, preflight.Git.Remote, preflight.Git.UpstreamBranch)
	log.PluginV(log.Exec, "Starting release action: %s", ReleaseExecutionPendingPushReleaseCommit)
	if _, err := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingPushReleaseCommit); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	log.PluginV(log.Exec, "Execution journal pending action recorded: %s", ReleaseExecutionPendingPushReleaseCommit)
	if err := operation.git.PushCommit(execCtx, preflight.Git.Remote, preflight.Git.UpstreamBranch, commitSHA); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	log.PluginV(log.Exec, "Release action completed: %s", ReleaseExecutionPendingPushReleaseCommit)
	if _, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionCommitPushed, ReleaseExecutionJournalUpdate{CommitPushStatus: "pushed"}); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	log.PluginV(log.Exec, "Execution phase confirmed: %s", ReleaseExecutionCommitPushed)
	log.PluginPrint(log.Exec, "Release commit push succeeded")
	return nil
}

type pushGitHubActionsReleaseTag struct {
	journal releaseExecutionJournalMutations
	git     githubActionsReleaseTagPushGit
}

func (operation pushGitHubActionsReleaseTag) Push(execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, preflight validatedGitHubActionsReleasePreflight, commitSHA string) error {
	log.PluginPrint(log.Exec, "Pushing unit tag %s", execCtx.Tag)
	log.PluginV(log.Exec, "Starting release action: %s", ReleaseExecutionPendingPushUnitTag)
	if _, err := operation.journal.BeginPending(execution.Identity, ReleaseExecutionPendingPushUnitTag); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	log.PluginV(log.Exec, "Execution journal pending action recorded: %s", ReleaseExecutionPendingPushUnitTag)
	if err := operation.git.PushTag(execCtx, preflight.Git.Remote, execCtx.Tag, commitSHA); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	log.PluginV(log.Exec, "Release action completed: %s", ReleaseExecutionPendingPushUnitTag)
	if _, err := operation.journal.ConfirmPhase(execution.Identity, ReleaseExecutionTagPushed, ReleaseExecutionJournalUpdate{TagPushStatus: "pushed"}); err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return err
	}
	log.PluginV(log.Exec, "Execution phase confirmed: %s", ReleaseExecutionTagPushed)
	log.PluginPrint(log.Exec, "Unit tag push succeeded")
	return nil
}

type dispatchGitHubActionsReleaseWorkflow struct {
	client  GitHubActionsWorkflowDispatchClient
	journal releaseExecutionErrorRecorder
	store   *DispatchJournalStore
	clock   ReleaseClock
}

func (operation dispatchGitHubActionsReleaseWorkflow) Dispatch(ctx context.Context, execCtx *ReleaseExecutionContext, execution preparedGitHubActionsReleaseExecution, dispatch preparedGitHubActionsReleaseDispatch, token GitHubActionsDispatchToken) (*GitHubActionsDispatchResult, error) {
	dispatcher, err := NewGitHubActionsDispatcher(operation.store.RepositoryRoot,
		WithGitHubActionsDispatcherClient(operation.client),
		WithGitHubActionsDispatcherStore(operation.store),
		WithGitHubActionsDispatcherClock(operation.clock),
	)
	if err != nil {
		return nil, err
	}
	log.PluginPrint(log.Exec, "Dispatching workflow %s for ref %s", execCtx.Workflow, execCtx.Tag)
	result, err := dispatcher.dispatchWithToken(ctx, dispatch.Request, token)
	if err != nil {
		_, _ = operation.journal.RecordLastError(execution.Identity, err.Error())
		return nil, err
	}
	log.PluginPrint(log.Exec, "Dispatch state: %s", result.State)
	log.PluginPrint(log.Exec, "Dispatch run: %s", emptyFallback(result.HTMLURL, "not resolved"))
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
		planner:            versionMaterializationReleasePlanner{},
		preflightValidator: validateGitHubActionsReleasePreflight{git: git, unresolvedExecutions: executionJournal},
		executionPreparer:  prepareGitHubActionsReleaseExecution{journal: executionJournal, clock: runner.clock},
		materialization: applyGitHubActionsReleaseMaterialization{
			journal:      executionJournal,
			transactions: defaultReleaseMaterializationTransactionFactory{},
		},
		stateWriter: writeGitHubActionsReleaseState{
			journal:      executionJournal,
			transactions: defaultReleaseStateTransactionFactory{},
		},
		fileStager:       stageGitHubActionsReleaseFiles{journal: executionJournal, git: git},
		commitCreator:    createGitHubActionsReleaseCommit{journal: executionJournal, git: git},
		tagCreator:       createGitHubActionsReleaseTag{journal: executionJournal, git: git},
		dispatchPreparer: prepareGitHubActionsReleaseDispatch{journal: executionJournal, dispatch: dispatchJournal, requests: verifiedReleaseDispatchRequestBuilder{git: dispatchVerifier}},
		commitPusher:     pushGitHubActionsReleaseCommit{journal: executionJournal, git: git},
		tagPusher:        pushGitHubActionsReleaseTag{journal: executionJournal, git: git},
		workflowDispatcher: dispatchGitHubActionsReleaseWorkflow{
			client:  runner.dispatchClient,
			journal: executionJournal,
			store:   dispatchJournal,
			clock:   runner.clock,
		},
		handoffConfirmer: confirmGitHubActionsReleaseHandoff{journal: executionJournal},
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
