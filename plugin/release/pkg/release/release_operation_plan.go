package release

import (
	"fmt"
	"strings"
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

type knownReleaseFileUnstager interface {
	UnstageKnown(files KnownReleaseFiles) error
}

type githubActionsReleaseCommitGit interface {
	Commit(execCtx *ReleaseExecutionContext, files KnownReleaseFiles) (string, error)
	UnstageKnown(files KnownReleaseFiles) error
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
