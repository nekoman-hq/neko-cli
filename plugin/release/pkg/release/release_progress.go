package release

// ReleaseProgress is the narrow user-visible release progress port.
//
// Implementations are observational and infallible: reporting must not choose
// operations, mutate journals, alter command results, or classify recovery.
type ReleaseProgress interface {
	ReportReleaseProgress(event ReleaseProgressEvent)
}

type ReleaseProgressEventKind string

const (
	ReleaseProgressReleaseStarted ReleaseProgressEventKind = "release-started"

	ReleaseProgressRepositoryContext ReleaseProgressEventKind = "repository-context"
	ReleaseProgressReleasePlan       ReleaseProgressEventKind = "release-plan"
	ReleaseProgressDryRunPlan        ReleaseProgressEventKind = "dry-run-plan"

	ReleaseProgressDryRunBoundary     ReleaseProgressEventKind = "dry-run-boundary"
	ReleaseProgressDryRunDispatchPlan ReleaseProgressEventKind = "dry-run-dispatch-plan"

	ReleaseProgressTokenPreflightResolving ReleaseProgressEventKind = "token-preflight-resolving"
	ReleaseProgressTokenPreflightAvailable ReleaseProgressEventKind = "token-preflight-available"

	ReleaseProgressMaterializationPlanningStarted   ReleaseProgressEventKind = "materialization-planning-started"
	ReleaseProgressMaterializationPlanningCompleted ReleaseProgressEventKind = "materialization-planning-completed"
	ReleaseProgressKnownFiles                       ReleaseProgressEventKind = "known-files"

	ReleaseProgressGitPreflightUnitStarted        ReleaseProgressEventKind = "git-preflight-unit-started"
	ReleaseProgressGitPreflightStarted            ReleaseProgressEventKind = "git-preflight-started"
	ReleaseProgressGitPreflightRepositoryVerified ReleaseProgressEventKind = "git-preflight-repository-verified"
	ReleaseProgressGitPreflightBranch             ReleaseProgressEventKind = "git-preflight-branch"
	ReleaseProgressGitPreflightUpstream           ReleaseProgressEventKind = "git-preflight-upstream"
	ReleaseProgressGitPreflightClean              ReleaseProgressEventKind = "git-preflight-clean"
	ReleaseProgressGitPreflightTagAvailable       ReleaseProgressEventKind = "git-preflight-tag-available"
	ReleaseProgressGitPreflightSummary            ReleaseProgressEventKind = "git-preflight-summary"
	ReleaseProgressGitPreflightRemoteURL          ReleaseProgressEventKind = "git-preflight-remote-url"
	ReleaseProgressGitPreflightWorkflowValidated  ReleaseProgressEventKind = "git-preflight-workflow-validated"
	ReleaseProgressGitPreflightUnresolvedJournals ReleaseProgressEventKind = "git-preflight-unresolved-journals"
	ReleaseProgressGitPreflightBaseCommit         ReleaseProgressEventKind = "git-preflight-base-commit"

	ReleaseProgressExecutionJournalPreparing ReleaseProgressEventKind = "execution-journal-preparing"
	ReleaseProgressExecutionJournalPrepared  ReleaseProgressEventKind = "execution-journal-prepared"
	ReleaseProgressExecutionPhase            ReleaseProgressEventKind = "execution-phase"
	ReleaseProgressExecutionPhaseConfirmed   ReleaseProgressEventKind = "execution-phase-confirmed"
	ReleaseProgressExecutionState            ReleaseProgressEventKind = "execution-state"
	ReleaseProgressRecoveryGuidance          ReleaseProgressEventKind = "recovery-guidance"

	ReleaseProgressMaterializationSnapshotCapturing ReleaseProgressEventKind = "materialization-snapshot-capturing"
	ReleaseProgressMaterializationSnapshotCaptured  ReleaseProgressEventKind = "materialization-snapshot-captured"
	ReleaseProgressMaterializedFilesApplied         ReleaseProgressEventKind = "materialized-files-applied"

	ReleaseProgressStateSnapshotCapturing ReleaseProgressEventKind = "state-snapshot-capturing"
	ReleaseProgressStateSnapshotCaptured  ReleaseProgressEventKind = "state-snapshot-captured"
	ReleaseProgressStateUpdateWriting     ReleaseProgressEventKind = "state-update-writing"
	ReleaseProgressStateUpdateWritten     ReleaseProgressEventKind = "state-update-written"

	ReleaseProgressPendingActionStarting ReleaseProgressEventKind = "pending-action-starting"
	ReleaseProgressPendingActionRecorded ReleaseProgressEventKind = "pending-action-recorded"
	ReleaseProgressPendingActionFinished ReleaseProgressEventKind = "pending-action-finished"

	ReleaseProgressStagingTargetedFiles ReleaseProgressEventKind = "staging-targeted-files"
	ReleaseProgressTargetedFilesStaged  ReleaseProgressEventKind = "targeted-files-staged"

	ReleaseProgressReleaseCommitCreating  ReleaseProgressEventKind = "release-commit-creating"
	ReleaseProgressReleaseCommitCreated   ReleaseProgressEventKind = "release-commit-created"
	ReleaseProgressReleaseCommitPushing   ReleaseProgressEventKind = "release-commit-pushing"
	ReleaseProgressReleaseCommitPushDone  ReleaseProgressEventKind = "release-commit-push-done"
	ReleaseProgressUnitTagCreating        ReleaseProgressEventKind = "unit-tag-creating"
	ReleaseProgressUnitTagPushPreparing   ReleaseProgressEventKind = "unit-tag-push-preparing"
	ReleaseProgressUnitTagPushDone        ReleaseProgressEventKind = "unit-tag-push-done"
	ReleaseProgressDispatchJournalPrepare ReleaseProgressEventKind = "dispatch-journal-prepare"
	ReleaseProgressDispatchJournalReady   ReleaseProgressEventKind = "dispatch-journal-ready"
	ReleaseProgressDispatchInputs         ReleaseProgressEventKind = "dispatch-inputs"
	ReleaseProgressWorkflowDispatching    ReleaseProgressEventKind = "workflow-dispatching"
	ReleaseProgressDispatchState          ReleaseProgressEventKind = "dispatch-state"
	ReleaseProgressDispatchRun            ReleaseProgressEventKind = "dispatch-run"

	ReleaseProgressGitWorktreeChecking           ReleaseProgressEventKind = "git-worktree-checking"
	ReleaseProgressGitStagingFiles               ReleaseProgressEventKind = "git-staging-files"
	ReleaseProgressGitStagedFilesVerified        ReleaseProgressEventKind = "git-staged-files-verified"
	ReleaseProgressGitReleaseCommitCreating      ReleaseProgressEventKind = "git-release-commit-creating"
	ReleaseProgressGitReleaseCommitVerified      ReleaseProgressEventKind = "git-release-commit-verified"
	ReleaseProgressGitUnitTagAlreadyExists       ReleaseProgressEventKind = "git-unit-tag-already-exists"
	ReleaseProgressGitUnitTagCreating            ReleaseProgressEventKind = "git-unit-tag-creating"
	ReleaseProgressGitUnitTagVerified            ReleaseProgressEventKind = "git-unit-tag-verified"
	ReleaseProgressGitReleaseCommitPushing       ReleaseProgressEventKind = "git-release-commit-pushing"
	ReleaseProgressGitReleaseCommitPushed        ReleaseProgressEventKind = "git-release-commit-pushed"
	ReleaseProgressGitUnitTagPushing             ReleaseProgressEventKind = "git-unit-tag-pushing"
	ReleaseProgressGitUnitTagPushed              ReleaseProgressEventKind = "git-unit-tag-pushed"
	ReleaseProgressGitKnownFilesUnstaging        ReleaseProgressEventKind = "git-known-files-unstaging"
	ReleaseProgressGitHubActionsTarget           ReleaseProgressEventKind = "github-actions-target"
	ReleaseProgressGitHubActionsJournal          ReleaseProgressEventKind = "github-actions-journal"
	ReleaseProgressGitHubActionsJournalPath      ReleaseProgressEventKind = "github-actions-journal-path"
	ReleaseProgressGitHubActionsBlocked          ReleaseProgressEventKind = "github-actions-blocked"
	ReleaseProgressGitHubActionsTokenResolving   ReleaseProgressEventKind = "github-actions-token-resolving"
	ReleaseProgressGitHubActionsTokenAvailable   ReleaseProgressEventKind = "github-actions-token-available"
	ReleaseProgressGitHubActionsRequestStarting  ReleaseProgressEventKind = "github-actions-request-starting"
	ReleaseProgressGitHubActionsRequestSending   ReleaseProgressEventKind = "github-actions-request-sending"
	ReleaseProgressGitHubActionsResponse         ReleaseProgressEventKind = "github-actions-response"
	ReleaseProgressGitHubActionsJournalFinalized ReleaseProgressEventKind = "github-actions-journal-finalized"
)

type ReleaseProgressEvent struct {
	Kind ReleaseProgressEventKind

	ReleaseType    string
	RepositoryRoot string
	SourceFormat   string
	UnitID         string
	ConfigPath     string
	StatePath      string

	CurrentVersion string
	NextVersion    string
	Tag            string
	Executor       string
	Delivery       string
	Workflow       string
	TagPrefix      string

	Branch           string
	Remote           string
	UpstreamBranch   string
	SafeRemoteURL    string
	CommitSHA        string
	CommitMessage    string
	TargetSHA        string
	Path             string
	Identity         string
	Phase            string
	PendingAction    string
	DispatchState    string
	DispatchRunURL   string
	Guidance         string
	Owner            string
	Repository       string
	Ref              string
	UnitStateVersion string
	Files            []string
	Inputs           []ReleaseProgressInput
	Count            int
	HTTPStatus       int
}

type ReleaseProgressInput struct {
	Name  string
	Value string
}

type noopReleaseProgress struct{}

func (noopReleaseProgress) ReportReleaseProgress(ReleaseProgressEvent) {}

func releaseProgressOrNoop(progress ReleaseProgress) ReleaseProgress {
	if progress == nil {
		return noopReleaseProgress{}
	}
	return progress
}

func reportReleaseProgress(progress ReleaseProgress, event ReleaseProgressEvent) {
	releaseProgressOrNoop(progress).ReportReleaseProgress(event)
}

func releaseProgressInputs(inputs map[string]string) []ReleaseProgressInput {
	keys := sortedDispatchInputKeys(inputs)
	progressInputs := make([]ReleaseProgressInput, 0, len(keys))
	for _, key := range keys {
		progressInputs = append(progressInputs, ReleaseProgressInput{Name: key, Value: inputs[key]})
	}
	return progressInputs
}
