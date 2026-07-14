package release

import (
	"context"
	"errors"
	"fmt"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type resumeReleaseOperation struct{}

func (resumeReleaseOperation) Resume(ctx context.Context, request ResumeCommandRequest) (ResumeCommandOutcome, *CommandFailure) {
	execution, failure := findResumableExecution(request.UnitID)
	if failure != nil {
		return nil, failure
	}
	assessment, failure := assessResumableExecution(execution)
	if failure != nil {
		return nil, failure
	}
	if request.DryRun {
		result, err := newResumeAssessment(execution.resolution.Path, execution.resolution.Journal, assessment)
		if err != nil {
			return nil, failureFromError("RECOVERY_ASSESSMENT_FAILED", err)
		}
		return result, nil
	}
	return continueResumableExecution(ctx, execution, assessment)
}

// resumableExecution holds the discovered facts needed to assess or continue
// one existing release execution. It does not represent a new workflow state.
type resumableExecution struct {
	repository *releaseconfig.ReleaseRepository
	unit       releaseconfig.ReleaseUnit
	resolution ReleaseExecutionJournalResolution
	remoteName string
	remoteURL  string
}

func findResumableExecution(unitID string) (*resumableExecution, *CommandFailure) {
	repository, err := releaseconfig.LoadReleaseRepository(".")
	if err != nil {
		return nil, failureFromError("CONFIG_NOT_FOUND", err)
	}
	if repository.SourceFormat != releaseconfig.SourceFormatV2 {
		return nil, failureFromMessage("RESUME_UNSUPPORTED", "release resume supports V2 github-actions releases only")
	}
	unit, err := releaseconfig.ResolveReleaseUnit(repository, unitID, releaseconfig.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		return nil, failureFromError("UNIT_RESOLUTION_FAILED", err)
	}
	remoteName, remoteURL, err := selectedReleaseRemote(repository.RepositoryRoot)
	if err != nil {
		return nil, failureFromError("REMOTE_RESOLUTION_FAILED", err)
	}
	store := NewReleaseExecutionJournalStore(repository.RepositoryRoot)
	matches, err := store.FindUnresolved(remoteURL, unit.ID)
	if err != nil {
		return nil, failureFromError("JOURNAL_SCAN_FAILED", err)
	}
	if len(matches) == 0 {
		return nil, failureFromMessage("NO_RESUMABLE_JOURNAL", fmt.Sprintf("no resumable V2 release execution journal found for unit %s", unit.ID))
	}
	if len(matches) > 1 {
		return nil, failureFromMessage("MULTIPLE_RESUMABLE_JOURNALS", multipleJournalMessage(unit.ID, matches))
	}
	resolution := matches[0]
	journal := resolution.Journal
	if journal.Delivery != string(releaseconfig.DeliveryGitHubActions) {
		return nil, failureFromMessage("RESUME_UNSUPPORTED", "release resume supports V2 github-actions releases only")
	}
	return &resumableExecution{
		repository: repository,
		unit:       *unit,
		resolution: resolution,
		remoteName: remoteName,
		remoteURL:  remoteURL,
	}, nil
}

func assessResumableExecution(execution *resumableExecution) (*ReleaseExecutionRecoveryAssessment, *CommandFailure) {
	assessment, err := AssessReleaseExecutionRecovery(execution.repository.RepositoryRoot, execution.resolution.Journal)
	if err != nil {
		return nil, failureFromError("RECOVERY_ASSESSMENT_FAILED", err)
	}
	return assessment, nil
}

func continueResumableExecution(ctx context.Context, execution *resumableExecution, assessment *ReleaseExecutionRecoveryAssessment) (ResumeCommandOutcome, *CommandFailure) {
	journal := execution.resolution.Journal
	if failure := validateResumeEligibility(journal, assessment); failure != nil {
		return nil, failure
	}
	token, err := EnvironmentGitHubActionsDispatchTokenResolver{}.ResolveGitHubActionsDispatchToken(ctx)
	if err != nil {
		return nil, failureFromError("TOKEN_MISSING", err)
	}
	execCtx, failure := resumableExecutionContext(execution)
	if failure != nil {
		return nil, failure
	}
	result, err := resumeJournal(ctx, execCtx, journal, execution.resolution.Path, execution.remoteName, execution.remoteURL, token)
	if err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	return result, nil
}

func validateResumeEligibility(journal *ReleaseExecutionJournal, assessment *ReleaseExecutionRecoveryAssessment) *CommandFailure {
	if assessment.Status == ReleaseExecutionRecoveryConflicted || assessment.Status == ReleaseExecutionRecoveryCorrupted {
		return failureFromMessage("RESUME_BLOCKED", assessment.Guidance)
	}
	if journal.PendingAction == ReleaseExecutionPendingPushReleaseCommit || journal.PendingAction == ReleaseExecutionPendingPushUnitTag {
		return failureFromMessage("RESUME_BLOCKED", "pending push action is ambiguous; manual verification is required before retry")
	}
	return nil
}

func resumableExecutionContext(execution *resumableExecution) (*ReleaseExecutionContext, *CommandFailure) {
	journal := execution.resolution.Journal
	execCtx, err := executionContextFromJournal(execution.repository, execution.unit, journal)
	if err != nil {
		return nil, failureFromError("JOURNAL_CONTEXT_FAILED", err)
	}
	if execCtx.Workflow != execution.unit.Workflow || execCtx.Executor != execution.unit.ExecutorType || execCtx.Delivery != execution.unit.Delivery {
		return nil, failureFromMessage("JOURNAL_CONFLICT", "current V2 config no longer matches the release execution journal")
	}
	return execCtx, nil
}

func resumeJournal(ctx context.Context, execCtx *ReleaseExecutionContext, journal *ReleaseExecutionJournal, executionPath, remoteName, remoteURL, token string) (*GitHubActionsReleaseResult, error) {
	coordinator := NewGitReleaseCoordinator()
	store := NewReleaseExecutionJournalStore(execCtx.RepositoryRoot)
	branch, err := coordinator.currentBranch(execCtx.RepositoryRoot)
	if err != nil {
		return nil, err
	}
	_, upstreamBranch, err := coordinator.upstream(execCtx.RepositoryRoot, branch)
	if err != nil {
		return nil, err
	}
	commitSHA := journal.ReleaseCommitSHA
	if commitSHA == "" {
		return nil, fmt.Errorf("resume before release commit is not yet safe for automatic continuation; use --dry-run and inspect recovery guidance")
	}
	if journal.State == ReleaseExecutionCommitCreated {
		tagCommit, err := coordinator.tagCommit(execCtx.RepositoryRoot, journal.Tag)
		if err != nil {
			return nil, err
		}
		if tagCommit == "" {
			if _, err := store.BeginPending(journal.Identity, ReleaseExecutionPendingCreateUnitTag); err != nil {
				return nil, err
			}
			if _, err := coordinator.CreateTag(execCtx, commitSHA); err != nil {
				_, _ = store.RecordLastError(journal.Identity, err.Error())
				return nil, err
			}
			if _, err := store.ConfirmPhase(journal.Identity, ReleaseExecutionTagCreated, ReleaseExecutionJournalUpdate{TagTargetSHA: commitSHA}); err != nil {
				return nil, err
			}
			journal.State = ReleaseExecutionTagCreated
			journal.TagTargetSHA = commitSHA
		} else if tagCommit != commitSHA {
			return nil, fmt.Errorf("tag %q points to %s, expected %s", journal.Tag, tagCommit, commitSHA)
		}
	}
	if journal.State == ReleaseExecutionTagCreated {
		dispatchPath, _, err := prepareDispatchJournalForResume(execCtx, journal, remoteName, remoteURL, false)
		if err != nil {
			return nil, err
		}
		if journal.DispatchJournalIdentity == "" {
			req, reqErr := dispatchRequestForResume(execCtx, journal, remoteName, remoteURL, false)
			if reqErr != nil {
				return nil, reqErr
			}
			if _, err := store.BeginPending(journal.Identity, ReleaseExecutionPendingCreateDispatchJournal); err != nil {
				return nil, err
			}
			if _, err := store.ConfirmPhase(journal.Identity, ReleaseExecutionDispatchJournalPrepared, ReleaseExecutionJournalUpdate{DispatchJournalIdentity: req.Identity.SHA256}); err != nil {
				return nil, err
			}
		}
		if _, err := store.BeginPending(journal.Identity, ReleaseExecutionPendingPushReleaseCommit); err != nil {
			return nil, err
		}
		if err := coordinator.PushCommit(execCtx, remoteName, upstreamBranch, commitSHA); err != nil {
			_, _ = store.RecordLastError(journal.Identity, err.Error())
			return nil, err
		}
		if _, err := store.ConfirmPhase(journal.Identity, ReleaseExecutionCommitPushed, ReleaseExecutionJournalUpdate{CommitPushStatus: "pushed"}); err != nil {
			return nil, err
		}
		if _, err := store.BeginPending(journal.Identity, ReleaseExecutionPendingPushUnitTag); err != nil {
			return nil, err
		}
		if err := coordinator.PushTag(execCtx, remoteName, journal.Tag, commitSHA); err != nil {
			_, _ = store.RecordLastError(journal.Identity, err.Error())
			return nil, err
		}
		if _, err := store.ConfirmPhase(journal.Identity, ReleaseExecutionTagPushed, ReleaseExecutionJournalUpdate{TagPushStatus: "pushed"}); err != nil {
			return nil, err
		}
		journal.State = ReleaseExecutionTagPushed
		return dispatchFromTagPushed(ctx, execCtx, journal, executionPath, dispatchPath, remoteName, remoteURL, token)
	}
	if journal.State == ReleaseExecutionTagPushed || journal.State == ReleaseExecutionCommitPushed || journal.State == ReleaseExecutionDispatchJournalPrepared {
		dispatchPath, _, err := prepareDispatchJournalForResume(execCtx, journal, remoteName, remoteURL, journal.State != ReleaseExecutionTagPushed)
		if err != nil {
			return nil, err
		}
		if journal.State != ReleaseExecutionTagPushed {
			return nil, fmt.Errorf("resume cannot prove push completion for state %s; manual verification is required", journal.State)
		}
		return dispatchFromTagPushed(ctx, execCtx, journal, executionPath, dispatchPath, remoteName, remoteURL, token)
	}
	if journal.State == ReleaseExecutionHandoffReady {
		return &GitHubActionsReleaseResult{
			Unit:                 journal.UnitID,
			Version:              journal.NextVersion,
			Tag:                  journal.Tag,
			CommitSHA:            journal.ReleaseCommitSHA,
			Workflow:             journal.WorkflowPath,
			ExecutionJournalPath: executionPath,
			ExecutionState:       ReleaseExecutionHandoffReady,
			RecoveryGuidance:     "Release was already handed off.",
		}, nil
	}
	return nil, fmt.Errorf("resume from state %s requires manual inspection before continuing", journal.State)
}

func dispatchFromTagPushed(ctx context.Context, execCtx *ReleaseExecutionContext, journal *ReleaseExecutionJournal, executionPath, dispatchPath, remoteName, remoteURL, token string) (*GitHubActionsReleaseResult, error) {
	req, err := dispatchRequestForResume(execCtx, journal, remoteName, remoteURL, true)
	if err != nil {
		return nil, err
	}
	dispatcher, err := NewGitHubActionsDispatcher(execCtx.RepositoryRoot,
		WithGitHubActionsDispatcherTokenResolver(staticGitHubActionsDispatchTokenResolver{token: token}),
	)
	if err != nil {
		return nil, err
	}
	result, err := dispatcher.Dispatch(ctx, req)
	if err != nil {
		return nil, err
	}
	if !result.Accepted {
		return &GitHubActionsReleaseResult{
			Unit:                 journal.UnitID,
			Version:              journal.NextVersion,
			Tag:                  journal.Tag,
			CommitSHA:            journal.ReleaseCommitSHA,
			Workflow:             journal.WorkflowPath,
			ExecutionJournalPath: executionPath,
			DispatchJournalPath:  dispatchPath,
			ExecutionState:       ReleaseExecutionTagPushed,
			DispatchState:        result.State,
			RecoveryGuidance:     result.RecoveryGuidance,
		}, errors.New(result.RecoveryGuidance)
	}
	if _, err := NewReleaseExecutionJournalStore(execCtx.RepositoryRoot).ConfirmPhase(journal.Identity, ReleaseExecutionHandoffReady, ReleaseExecutionJournalUpdate{}); err != nil {
		return nil, err
	}
	return &GitHubActionsReleaseResult{
		Unit:                 journal.UnitID,
		Version:              journal.NextVersion,
		Tag:                  journal.Tag,
		CommitSHA:            journal.ReleaseCommitSHA,
		Workflow:             journal.WorkflowPath,
		ExecutionJournalPath: executionPath,
		DispatchJournalPath:  result.JournalPath,
		ExecutionState:       ReleaseExecutionHandoffReady,
		DispatchState:        result.State,
		RecoveryGuidance:     "GitHub Actions dispatch accepted. GitHub Actions owns build and publish from the pushed tag.",
		DispatchRunURL:       result.HTMLURL,
	}, nil
}

func prepareDispatchJournalForResume(execCtx *ReleaseExecutionContext, journal *ReleaseExecutionJournal, remoteName, remoteURL string, loadOnly bool) (string, *ReleaseDispatchRequest, error) {
	req, err := dispatchRequestForResume(execCtx, journal, remoteName, remoteURL, true)
	if err != nil {
		return "", nil, err
	}
	store := NewDispatchJournalStore(execCtx.RepositoryRoot)
	var resolution *DispatchJournalResolution
	if loadOnly {
		resolution, err = store.Load(req)
	} else {
		resolution, err = store.Prepare(req)
	}
	if err != nil {
		return "", nil, err
	}
	if resolution.Journal != nil && resolution.Journal.State != DispatchJournalPrepared && resolution.Journal.State != DispatchJournalAccepted {
		return resolution.Path, req, fmt.Errorf("dispatch journal is %s; do not dispatch again automatically", resolution.Journal.State)
	}
	return resolution.Path, req, nil
}

func dispatchRequestForResume(execCtx *ReleaseExecutionContext, journal *ReleaseExecutionJournal, remoteName, remoteURL string, pushed bool) (*ReleaseDispatchRequest, error) {
	return BuildReleaseDispatchRequest(execCtx, &GitReleaseResult{
		Unit:                 journal.UnitID,
		Version:              journal.NextVersion,
		Tag:                  journal.Tag,
		CommitSHA:            journal.ReleaseCommitSHA,
		RepositoryRemoteName: remoteName,
		RepositoryRemote:     remoteURL,
		CommitCreated:        true,
		TagCreated:           true,
		CommitPushed:         pushed,
		TagPushed:            pushed,
	})
}

func executionContextFromJournal(repository *releaseconfig.ReleaseRepository, unit releaseconfig.ReleaseUnit, journal *ReleaseExecutionJournal) (*ReleaseExecutionContext, error) {
	repositoryRoot, err := absoluteExistingDir(repository.RepositoryRoot, "repository root")
	if err != nil {
		return nil, err
	}
	tagSpec, err := releaseconfig.NewTagSpec(unit.TagPrefix)
	if err != nil {
		return nil, err
	}
	unitRoot, err := resolveUnitRoot(repositoryRoot, repository.SourceFormat, unit)
	if err != nil {
		return nil, err
	}
	delivery, err := ResolveDelivery(unit.Delivery)
	if err != nil {
		return nil, err
	}
	capabilities, err := ResolveExecutorCapabilities(unit.ExecutorType)
	if err != nil {
		return nil, err
	}
	return &ReleaseExecutionContext{
		RepositoryRoot: repositoryRoot,
		Unit:           unit,
		UnitRoot:       unitRoot,
		CurrentVersion: journal.CurrentVersion,
		NextVersion:    journal.NextVersion,
		Tag:            journal.Tag,
		TagSpec:        tagSpec,
		ReleaseKind:    Patch,
		Executor:       journal.Executor,
		Delivery:       journal.Delivery,
		Workflow:       journal.WorkflowPath,
		SourceFormat:   releaseconfig.SourceFormatV2,
		Capabilities:   capabilities,
		DeliveryMode:   delivery,
	}, nil
}

func selectedReleaseRemote(repositoryRoot string) (string, string, error) {
	coordinator := NewGitReleaseCoordinator()
	branch, err := coordinator.currentBranch(repositoryRoot)
	if err != nil {
		return "", "", err
	}
	remoteName, _, err := coordinator.upstream(repositoryRoot, branch)
	if err != nil {
		return "", "", err
	}
	remoteURL, err := coordinator.gitOutput(repositoryRoot, "remote", "get-url", remoteName)
	if err != nil {
		return "", "", err
	}
	return remoteName, strings.TrimSpace(remoteURL), nil
}

func multipleJournalMessage(unitID string, matches []ReleaseExecutionJournalResolution) string {
	parts := make([]string, 0, len(matches))
	for _, match := range matches {
		if match.Journal == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s version=%s tag=%s state=%s pending=%s", match.Path, match.Journal.NextVersion, match.Journal.Tag, match.Journal.State, match.Journal.PendingAction))
	}
	return fmt.Sprintf("multiple resumable release journals found for unit %s; manual inspection is required: %s", unitID, strings.Join(parts, "; "))
}
