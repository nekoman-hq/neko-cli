package release

import (
	"fmt"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

type terminalReleaseProgress struct{}

func newTerminalReleaseProgress() ReleaseProgress {
	return terminalReleaseProgress{}
}

func (terminalReleaseProgress) ReportReleaseProgress(event ReleaseProgressEvent) {
	for _, line := range renderReleaseProgressEvent(event) {
		if line.verbose {
			log.PluginV(line.category, line.message, line.args...)
			continue
		}
		log.PluginPrint(line.category, line.message, line.args...)
	}
}

type releaseProgressLine struct {
	category log.Category
	message  string
	args     []any
	verbose  bool
}

func progressLine(category log.Category, message string, args ...any) releaseProgressLine {
	return releaseProgressLine{category: category, message: message, args: args}
}

func verboseProgressLine(category log.Category, message string, args ...any) releaseProgressLine {
	return releaseProgressLine{category: category, message: message, args: args, verbose: true}
}

func renderReleaseProgressEvent(event ReleaseProgressEvent) []releaseProgressLine {
	switch event.Kind {
	case ReleaseProgressReleaseStarted:
		return []releaseProgressLine{progressLine(log.Exec, "Starting %s release", event.ReleaseType)}
	case ReleaseProgressRepositoryContext:
		return []releaseProgressLine{
			progressLine(log.Config, "Repository root: %s", event.RepositoryRoot),
			progressLine(log.Config, "Release source format: %s", event.SourceFormat),
			progressLine(log.Config, "Selected unit: %s", event.UnitID),
			progressLine(log.Config, "Config path: %s", event.ConfigPath),
			progressLine(log.Config, "State path: %s", event.StatePath),
		}
	case ReleaseProgressReleasePlan:
		return []releaseProgressLine{
			progressLine(log.Exec, "Planning V2 release: current=%s next=%s tag=%s", event.CurrentVersion, event.NextVersion, event.Tag),
			progressLine(log.Exec, "Executor=%s delivery=%s workflow=%s tagPrefix=%s", event.Executor, event.Delivery, event.Workflow, event.TagPrefix),
		}
	case ReleaseProgressDryRunPlan:
		return []releaseProgressLine{
			progressLine(log.Exec, "Planning V2 dry-run: current=%s next=%s tag=%s", event.CurrentVersion, event.NextVersion, event.Tag),
			progressLine(log.Exec, "Executor=%s delivery=%s workflow=%s tagPrefix=%s", event.Executor, event.Delivery, workflowValue(event.Workflow), event.TagPrefix),
		}
	case ReleaseProgressDryRunBoundary:
		return []releaseProgressLine{progressLine(log.Exec, "Dry run only: no token required, no journal created, no commit/tag/push/dispatch")}
	case ReleaseProgressDryRunDispatchPlan:
		return []releaseProgressLine{
			progressLine(log.Exec, "Planned dispatch ref: %s", event.Ref),
			progressLine(log.Exec, "Planned dispatch inputs: %s", releaseProgressInputsValue(event.Inputs)),
		}
	case ReleaseProgressTokenPreflightResolving:
		return []releaseProgressLine{progressLine(log.Exec, "GitHub token preflight: resolving token without printing it")}
	case ReleaseProgressTokenPreflightAvailable:
		return []releaseProgressLine{progressLine(log.Exec, "GitHub token preflight: token available")}
	case ReleaseProgressMaterializationPlanningStarted:
		return []releaseProgressLine{progressLine(log.Exec, "Planning materialized files")}
	case ReleaseProgressMaterializationPlanningCompleted:
		return []releaseProgressLine{progressLine(log.Exec, "Planned materialized files: %s", strings.Join(event.Files, ", "))}
	case ReleaseProgressKnownFiles:
		return []releaseProgressLine{progressLine(log.Exec, "Known release files: %s", strings.Join(event.Files, ", "))}
	case ReleaseProgressGitPreflightStarted:
		return []releaseProgressLine{progressLine(log.Exec, "Running git preflight checks")}
	case ReleaseProgressGitPreflightUnitStarted:
		return []releaseProgressLine{progressLine(log.Exec, "Starting V2 git preflight for unit=%s tag=%s", event.UnitID, event.Tag)}
	case ReleaseProgressGitPreflightRepositoryVerified:
		return []releaseProgressLine{progressLine(log.Exec, "Git preflight: repository root verified")}
	case ReleaseProgressGitPreflightBranch:
		return []releaseProgressLine{progressLine(log.Exec, "Git preflight: current branch=%s", event.Branch)}
	case ReleaseProgressGitPreflightUpstream:
		return []releaseProgressLine{progressLine(log.Exec, "Git preflight: upstream=%s/%s", event.Remote, event.UpstreamBranch)}
	case ReleaseProgressGitPreflightClean:
		return []releaseProgressLine{progressLine(log.Exec, "Git preflight: worktree and index are clean")}
	case ReleaseProgressGitPreflightTagAvailable:
		return []releaseProgressLine{progressLine(log.Exec, "Git preflight: unit tag %s is available", event.Tag)}
	case ReleaseProgressGitPreflightSummary:
		return []releaseProgressLine{progressLine(log.Exec, "Git preflight: branch=%s remote=%s upstream=%s", event.Branch, event.Remote, event.UpstreamBranch)}
	case ReleaseProgressGitPreflightRemoteURL:
		return []releaseProgressLine{progressLine(log.Exec, "Git preflight: remote URL=%s", event.SafeRemoteURL)}
	case ReleaseProgressGitPreflightWorkflowValidated:
		return []releaseProgressLine{progressLine(log.Exec, "Git preflight: workflow validation passed for %s", event.Workflow)}
	case ReleaseProgressGitPreflightUnresolvedJournals:
		return []releaseProgressLine{progressLine(log.Exec, "Execution journal preflight: unresolved journals=%d", event.Count)}
	case ReleaseProgressGitPreflightBaseCommit:
		return []releaseProgressLine{progressLine(log.Exec, "Base commit before release: %s", event.CommitSHA)}
	case ReleaseProgressExecutionJournalPreparing:
		return []releaseProgressLine{progressLine(log.Exec, "Preparing execution journal")}
	case ReleaseProgressExecutionJournalPrepared:
		return []releaseProgressLine{
			progressLine(log.Exec, "Execution journal path: %s", event.Path),
			progressLine(log.Exec, "Execution journal identity: %s", event.Identity),
		}
	case ReleaseProgressExecutionPhase:
		return []releaseProgressLine{progressLine(log.Exec, "Execution phase: %s", event.Phase)}
	case ReleaseProgressExecutionPhaseConfirmed:
		return []releaseProgressLine{verboseProgressLine(log.Exec, "Execution phase confirmed: %s", event.Phase)}
	case ReleaseProgressExecutionState:
		return []releaseProgressLine{progressLine(log.Exec, "Execution state: %s", event.Phase)}
	case ReleaseProgressRecoveryGuidance:
		return []releaseProgressLine{progressLine(log.Exec, "Recovery guidance: %s", event.Guidance)}
	case ReleaseProgressMaterializationSnapshotCapturing:
		return []releaseProgressLine{progressLine(log.Exec, "Capturing materialization snapshots")}
	case ReleaseProgressMaterializationSnapshotCaptured:
		return []releaseProgressLine{progressLine(log.Exec, "Materialization snapshots captured")}
	case ReleaseProgressMaterializedFilesApplied:
		return []releaseProgressLine{progressLine(log.Exec, "Applied materialized files: %s", strings.Join(event.Files, ", "))}
	case ReleaseProgressStateSnapshotCapturing:
		return []releaseProgressLine{progressLine(log.Exec, "Capturing V2 state snapshot")}
	case ReleaseProgressStateSnapshotCaptured:
		return []releaseProgressLine{progressLine(log.Exec, "V2 state snapshot captured: %s", event.Path)}
	case ReleaseProgressStateUpdateWriting:
		return []releaseProgressLine{progressLine(log.Exec, "Writing V2 state update: %s -> %s", event.UnitID, event.NextVersion)}
	case ReleaseProgressStateUpdateWritten:
		return []releaseProgressLine{progressLine(log.Exec, "State update written")}
	case ReleaseProgressPendingActionStarting:
		return []releaseProgressLine{verboseProgressLine(log.Exec, "Starting release action: %s", event.PendingAction)}
	case ReleaseProgressPendingActionRecorded:
		return []releaseProgressLine{verboseProgressLine(log.Exec, "Execution journal pending action recorded: %s", event.PendingAction)}
	case ReleaseProgressPendingActionFinished:
		return []releaseProgressLine{verboseProgressLine(log.Exec, "Release action completed: %s", event.PendingAction)}
	case ReleaseProgressStagingTargetedFiles:
		return []releaseProgressLine{progressLine(log.Exec, "Staging targeted release files: %s", strings.Join(event.Files, ", "))}
	case ReleaseProgressTargetedFilesStaged:
		return []releaseProgressLine{progressLine(log.Exec, "Targeted release files staged")}
	case ReleaseProgressReleaseCommitCreating:
		return []releaseProgressLine{progressLine(log.Exec, "Creating release commit: %s", event.CommitMessage)}
	case ReleaseProgressReleaseCommitCreated:
		return []releaseProgressLine{progressLine(log.Exec, "Release commit created: %s", event.CommitSHA)}
	case ReleaseProgressUnitTagCreating:
		return []releaseProgressLine{progressLine(log.Exec, "Creating unit tag: %s", event.Tag)}
	case ReleaseProgressDispatchJournalPrepare:
		return []releaseProgressLine{progressLine(log.Exec, "Preparing dispatch journal")}
	case ReleaseProgressDispatchJournalReady:
		return []releaseProgressLine{progressLine(log.Exec, "Dispatch journal path: %s", event.Path)}
	case ReleaseProgressDispatchInputs:
		return []releaseProgressLine{progressLine(log.Exec, "Dispatch inputs: %s", releaseProgressInputsValue(event.Inputs))}
	case ReleaseProgressReleaseCommitPushing:
		return []releaseProgressLine{progressLine(log.Exec, "Pushing release commit %s to %s/%s", event.CommitSHA, event.Remote, event.UpstreamBranch)}
	case ReleaseProgressReleaseCommitPushDone:
		return []releaseProgressLine{progressLine(log.Exec, "Release commit push succeeded")}
	case ReleaseProgressUnitTagPushPreparing:
		return []releaseProgressLine{progressLine(log.Exec, "Pushing unit tag %s", event.Tag)}
	case ReleaseProgressUnitTagPushDone:
		return []releaseProgressLine{progressLine(log.Exec, "Unit tag push succeeded")}
	case ReleaseProgressWorkflowDispatching:
		return []releaseProgressLine{progressLine(log.Exec, "Dispatching workflow %s for ref %s", event.Workflow, event.Ref)}
	case ReleaseProgressDispatchState:
		return []releaseProgressLine{progressLine(log.Exec, "Dispatch state: %s", event.DispatchState)}
	case ReleaseProgressDispatchRun:
		return []releaseProgressLine{progressLine(log.Exec, "Dispatch run: %s", emptyFallback(event.DispatchRunURL, "not resolved"))}
	case ReleaseProgressGitWorktreeChecking:
		return []releaseProgressLine{progressLine(log.Exec, "Checking V2 release worktree before staging")}
	case ReleaseProgressGitStagingFiles:
		return []releaseProgressLine{progressLine(log.Exec, "Staging V2 release files: %s", strings.Join(event.Files, ", "))}
	case ReleaseProgressGitStagedFilesVerified:
		return []releaseProgressLine{progressLine(log.Exec, "Verified staged V2 release files")}
	case ReleaseProgressGitReleaseCommitCreating:
		return []releaseProgressLine{progressLine(log.Exec, "Creating V2 release commit: %s", event.CommitMessage)}
	case ReleaseProgressGitReleaseCommitVerified:
		return []releaseProgressLine{progressLine(log.Exec, "Verified V2 release commit: %s", event.CommitSHA)}
	case ReleaseProgressGitUnitTagAlreadyExists:
		return []releaseProgressLine{progressLine(log.Exec, "Unit tag already points to release commit: %s -> %s", event.Tag, event.CommitSHA)}
	case ReleaseProgressGitUnitTagCreating:
		return []releaseProgressLine{progressLine(log.Exec, "Creating V2 unit tag: %s -> %s", event.Tag, event.CommitSHA)}
	case ReleaseProgressGitUnitTagVerified:
		return []releaseProgressLine{progressLine(log.Exec, "Verified V2 unit tag: %s", event.Tag)}
	case ReleaseProgressGitReleaseCommitPushing:
		return []releaseProgressLine{progressLine(log.Exec, "Pushing V2 release commit %s to %s/%s", event.CommitSHA, event.Remote, event.UpstreamBranch)}
	case ReleaseProgressGitReleaseCommitPushed:
		return []releaseProgressLine{progressLine(log.Exec, "Pushed V2 release commit %s", event.CommitSHA)}
	case ReleaseProgressGitUnitTagPushing:
		return []releaseProgressLine{progressLine(log.Exec, "Pushing V2 unit tag %s to %s", event.Tag, event.Remote)}
	case ReleaseProgressGitUnitTagPushed:
		return []releaseProgressLine{progressLine(log.Exec, "Pushed V2 unit tag %s", event.Tag)}
	case ReleaseProgressGitKnownFilesUnstaging:
		return []releaseProgressLine{progressLine(log.Exec, "Unstaging V2 release files: %s", strings.Join(event.Files, ", "))}
	case ReleaseProgressGitHubActionsTarget:
		return []releaseProgressLine{progressLine(log.Exec, "GitHub Actions target resolved: %s/%s workflow=%s ref=%s", event.Owner, event.Repository, event.Workflow, event.Ref)}
	case ReleaseProgressGitHubActionsJournal:
		return []releaseProgressLine{progressLine(log.Exec, "Preparing GitHub Actions dispatch journal")}
	case ReleaseProgressGitHubActionsJournalPath:
		return []releaseProgressLine{progressLine(log.Exec, "Dispatch journal path: %s", event.Path)}
	case ReleaseProgressGitHubActionsBlocked:
		return []releaseProgressLine{progressLine(log.Exec, "Dispatch blocked by existing journal state: %s", event.DispatchState)}
	case ReleaseProgressGitHubActionsTokenResolving:
		return []releaseProgressLine{progressLine(log.Exec, "Resolving GitHub Actions dispatch token")}
	case ReleaseProgressGitHubActionsTokenAvailable:
		return []releaseProgressLine{progressLine(log.Exec, "GitHub Actions dispatch token available")}
	case ReleaseProgressGitHubActionsRequestStarting:
		return []releaseProgressLine{progressLine(log.Exec, "Recording dispatch request-started before HTTP call")}
	case ReleaseProgressGitHubActionsRequestSending:
		return []releaseProgressLine{progressLine(log.Exec, "Sending workflow_dispatch request")}
	case ReleaseProgressGitHubActionsResponse:
		return []releaseProgressLine{progressLine(log.Exec, "workflow_dispatch response state=%s status=%d", event.DispatchState, event.HTTPStatus)}
	case ReleaseProgressGitHubActionsJournalFinalized:
		return []releaseProgressLine{progressLine(log.Exec, "Dispatch journal finalized with state: %s", event.DispatchState)}
	default:
		return nil
	}
}

func releaseProgressInputsValue(inputs []ReleaseProgressInput) string {
	parts := make([]string, 0, len(inputs))
	for _, input := range inputs {
		parts = append(parts, fmt.Sprintf("%s=%s", input.Name, input.Value))
	}
	return strings.Join(parts, " ")
}
