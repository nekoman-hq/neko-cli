package release

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/pipelineinspection"
)

func inspectPipelineRuntime(repositoryRoot string) pipelineinspection.RuntimeSnapshot {
	snapshot := pipelineinspection.RuntimeSnapshot{
		Inspected:  true,
		Executions: make([]pipelineinspection.RuntimeExecutionObservation, 0),
		Dispatches: make([]pipelineinspection.RuntimeDispatchObservation, 0),
		Problems:   make([]pipelineinspection.RuntimeProblem, 0),
	}
	coordinator := NewGitReleaseCoordinator()
	inspectPipelineRepositoryGit(repositoryRoot, coordinator, &snapshot)
	inspectPipelineExecutionJournals(repositoryRoot, coordinator, &snapshot)
	inspectPipelineDispatchJournals(repositoryRoot, &snapshot)
	return snapshot
}

func inspectPipelineRepositoryGit(repositoryRoot string, coordinator *GitReleaseCoordinator, snapshot *pipelineinspection.RuntimeSnapshot) {
	inspectPipelineRepositoryIdentity(repositoryRoot, coordinator, snapshot)
	inspectPipelineRepositoryHead(repositoryRoot, coordinator, snapshot)
	inspectPipelineRepositoryStatus(repositoryRoot, coordinator, snapshot)
	snapshot.Repository.Inspected = len(snapshot.Problems) == 0
}

func inspectPipelineRepositoryIdentity(repositoryRoot string, coordinator *GitReleaseCoordinator, snapshot *pipelineinspection.RuntimeSnapshot) {
	branch, err := coordinator.currentBranch(repositoryRoot)
	if err != nil {
		snapshot.Problems = append(snapshot.Problems, pipelineRuntimeProblem("local_git", "local Git branch identity could not be inspected"))
		return
	}
	snapshot.Repository.Branch = branch
	remoteName, upstreamBranch, err := coordinator.upstream(repositoryRoot, branch)
	if err != nil {
		snapshot.Problems = append(snapshot.Problems, pipelineRuntimeProblem("local_git", "local Git upstream identity could not be inspected"))
		return
	}
	snapshot.Repository.RemoteName = remoteName
	snapshot.Repository.Tracking = remoteName + "/" + upstreamBranch
	remoteURL, err := coordinator.gitOutput(repositoryRoot, "remote", "get-url", remoteName)
	if err != nil || strings.TrimSpace(remoteURL) == "" {
		snapshot.Problems = append(snapshot.Problems, pipelineRuntimeProblem("local_git", "local Git remote identity could not be inspected"))
		return
	}
	snapshot.RepositoryRemote = strings.TrimSpace(remoteURL)
	snapshot.Repository.RemoteURL = snapshot.RepositoryRemote
}

func inspectPipelineRepositoryHead(repositoryRoot string, coordinator *GitReleaseCoordinator, snapshot *pipelineinspection.RuntimeSnapshot) {
	head, headErr := coordinator.headCommit(repositoryRoot)
	if headErr != nil {
		snapshot.Problems = append(snapshot.Problems, pipelineRuntimeProblem("local_git", "local Git HEAD could not be inspected"))
	} else {
		snapshot.Repository.Head = head
	}
}

func inspectPipelineRepositoryStatus(repositoryRoot string, coordinator *GitReleaseCoordinator, snapshot *pipelineinspection.RuntimeSnapshot) {
	index, indexErr := coordinator.gitOutput(repositoryRoot, "diff", "--cached", "--name-only")
	if indexErr != nil {
		snapshot.Problems = append(snapshot.Problems, pipelineRuntimeProblem("local_git", "local Git index could not be inspected"))
	} else if strings.TrimSpace(index) == "" {
		snapshot.Repository.IndexState = "clean"
	} else {
		snapshot.Repository.IndexState = "changes_present"
	}
	worktree, worktreeErr := coordinator.gitOutput(repositoryRoot, "diff", "--name-only")
	untracked, untrackedErr := coordinator.gitOutput(repositoryRoot, "ls-files", "--others", "--exclude-standard")
	if worktreeErr != nil || untrackedErr != nil {
		snapshot.Problems = append(snapshot.Problems, pipelineRuntimeProblem("local_git", "local Git worktree could not be inspected"))
	} else if strings.TrimSpace(worktree) == "" && strings.TrimSpace(untracked) == "" {
		snapshot.Repository.WorktreeState = "clean"
	} else {
		snapshot.Repository.WorktreeState = "changes_present"
	}
}

func inspectPipelineExecutionJournals(repositoryRoot string, coordinator *GitReleaseCoordinator, snapshot *pipelineinspection.RuntimeSnapshot) {
	store := NewReleaseExecutionJournalStore(repositoryRoot)
	directory, err := store.JournalDirectory()
	if err != nil {
		snapshot.Problems = append(snapshot.Problems, pipelineRuntimeProblem("execution_journal", "execution journal location could not be inspected"))
		return
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		snapshot.Problems = append(snapshot.Problems, pipelineRuntimeProblem("execution_journal", "execution journal directory could not be read"))
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		reference := path.Join("executions", entry.Name())
		journal, loadErr := store.loadAt(filepath.Join(directory, entry.Name()))
		if loadErr != nil || journal == nil {
			snapshot.Problems = append(snapshot.Problems, pipelineinspection.RuntimeProblem{
				Kind: "execution_journal", Reference: reference,
				Reason: "execution journal is malformed",
			})
			continue
		}
		observation := observePipelineExecutionJournal(entry.Name(), reference, journal)
		if observation.Valid {
			observation.LocalGit = inspectPipelineExecutionGit(repositoryRoot, coordinator, journal)
			observation.Recovery = observePipelineRecovery(repositoryRoot, journal)
		}
		snapshot.Executions = append(snapshot.Executions, observation)
	}
}

func observePipelineExecutionJournal(filename, reference string, journal *ReleaseExecutionJournal) pipelineinspection.RuntimeExecutionObservation {
	observation := pipelineinspection.RuntimeExecutionObservation{
		Reference: reference, Identity: journal.Identity.SHA256,
		RepositoryRemote: journal.RepositoryRemote, UnitID: journal.UnitID,
		CurrentVersion: journal.CurrentVersion, NextVersion: journal.NextVersion,
		Tag: journal.Tag, Executor: journal.Executor, Delivery: journal.Delivery,
		WorkflowPath: journal.WorkflowPath, State: string(journal.State),
		PendingAction: string(journal.PendingAction), ReleaseCommitSHA: journal.ReleaseCommitSHA,
		DispatchJournalIdentity: journal.DispatchJournalIdentity,
		CreatedAt:               journal.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:               journal.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		Unresolved:              journal.State != ReleaseExecutionHandoffReady,
	}
	if err := validatePipelineExecutionJournal(filename, journal); err != nil {
		observation.Problem = err.Error()
		return observation
	}
	observation.Valid = true
	observation.ConfirmedStageIDs = confirmedPipelineStageIDs(journal.State)
	observation.CurrentStageIDs = pipelineStageIDsForPhase(journal.State)
	observation.PendingStageID = pipelineStageIDForPendingAction(journal.PendingAction)
	return observation
}

func validatePipelineExecutionJournal(filename string, journal *ReleaseExecutionJournal) error {
	if err := validateJournalForRecovery(journal); err != nil {
		return fmt.Errorf("execution journal is structurally invalid")
	}
	if err := validatePipelineExecutionIdentity(filename, journal); err != nil {
		return err
	}
	if journal.CreatedAt.IsZero() || journal.UpdatedAt.Before(journal.CreatedAt) {
		return fmt.Errorf("execution journal timestamps are invalid")
	}
	if err := validatePipelineKnownReleaseFiles(journal.KnownReleaseFiles); err != nil {
		return err
	}
	return validatePipelineExecutionPhaseEvidence(journal)
}

func validatePipelineExecutionIdentity(filename string, journal *ReleaseExecutionJournal) error {
	identity, err := newReleaseExecutionIdentity(
		journal.RepositoryRemote, journal.BaseCommitSHA, journal.UnitID,
		journal.CurrentVersion, journal.NextVersion, journal.Tag,
		journal.Executor, journal.Delivery, journal.WorkflowPath,
	)
	if err != nil || identity.SHA256 != journal.Identity.SHA256 || identity != journal.Identity {
		return fmt.Errorf("execution identity does not match immutable journal fields")
	}
	if filename != journal.Identity.SHA256+".json" {
		return fmt.Errorf("execution journal reference does not match its exact identity")
	}
	if journal.RepositoryRemote != journal.Identity.RepositoryRemote ||
		journal.BaseCommitSHA != journal.Identity.BaseCommitSHA ||
		journal.UnitID != journal.Identity.UnitID ||
		journal.CurrentVersion != journal.Identity.CurrentVersion ||
		journal.NextVersion != journal.Identity.NextVersion ||
		journal.Tag != journal.Identity.Tag ||
		journal.Executor != journal.Identity.Executor ||
		journal.Delivery != journal.Identity.Delivery ||
		journal.WorkflowPath != journal.Identity.WorkflowPath {
		return fmt.Errorf("execution identity conflicts with journal fields")
	}
	return nil
}

func validatePipelineKnownReleaseFiles(files []ReleaseExecutionFileMetadata) error {
	for _, file := range files {
		clean := path.Clean(file.RepositoryRelativePath)
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(file.RepositoryRelativePath) {
			return fmt.Errorf("execution journal contains an unsafe release file reference")
		}
	}
	return nil
}

func validatePipelineExecutionPhaseEvidence(journal *ReleaseExecutionJournal) error {
	rank := releaseExecutionStateRank(journal.State)
	if journal.PendingAction != ReleaseExecutionPendingNone &&
		(rank+1 >= len(releaseExecutionStateOrder) || pendingActionForConfirmedPhase(releaseExecutionStateOrder[rank+1]) != journal.PendingAction) {
		return fmt.Errorf("execution journal pending action conflicts with confirmed phase")
	}
	if rank >= releaseExecutionStateRank(ReleaseExecutionCommitCreated) && !fullGitSHARegexp.MatchString(journal.ReleaseCommitSHA) {
		return fmt.Errorf("execution journal is missing its confirmed release commit identity")
	}
	if rank >= releaseExecutionStateRank(ReleaseExecutionTagCreated) && journal.TagTargetSHA != journal.ReleaseCommitSHA {
		return fmt.Errorf("execution journal tag target conflicts with the release commit")
	}
	if rank >= releaseExecutionStateRank(ReleaseExecutionDispatchJournalPrepared) && !isSafeDispatchIdentityHash(journal.DispatchJournalIdentity) {
		return fmt.Errorf("execution journal is missing its exact dispatch identity")
	}
	if rank >= releaseExecutionStateRank(ReleaseExecutionCommitPushed) && journal.CommitPushStatus == "" {
		return fmt.Errorf("execution journal is missing commit-push confirmation")
	}
	if rank >= releaseExecutionStateRank(ReleaseExecutionTagPushed) && journal.TagPushStatus == "" {
		return fmt.Errorf("execution journal is missing tag-push confirmation")
	}
	return nil
}

func confirmedPipelineStageIDs(state ReleaseExecutionJournalState) []string {
	rank := releaseExecutionStateRank(state)
	if rank < 0 {
		return nil
	}
	stages := make([]string, 0)
	for _, phase := range releaseExecutionStateOrder[:rank+1] {
		stages = append(stages, pipelineStageIDsForPhase(phase)...)
	}
	return stages
}

func pipelineStageIDsForPhase(phase ReleaseExecutionJournalState) []string {
	switch phase {
	case ReleaseExecutionPrepared:
		return []string{
			"source-unit-resolution", "release-context-planning", "dispatch-token-resolution",
			"release-file-planning", "execution-journal-preparation",
		}
	case ReleaseExecutionPreflightValidated:
		return []string{"release-preflight"}
	case ReleaseExecutionMaterializationApplied:
		return []string{"release-file-materialization"}
	case ReleaseExecutionStateWritten:
		return []string{"selected-unit-state-write"}
	case ReleaseExecutionReleaseFilesStaged:
		return []string{"known-release-file-staging"}
	case ReleaseExecutionCommitCreated:
		return []string{"release-commit-creation"}
	case ReleaseExecutionTagCreated:
		return []string{"unit-tag-creation"}
	case ReleaseExecutionDispatchJournalPrepared:
		return []string{"workflow-request-preparation"}
	case ReleaseExecutionCommitPushed:
		return []string{"release-commit-push"}
	case ReleaseExecutionTagPushed:
		return []string{"unit-tag-push"}
	case ReleaseExecutionHandoffReady:
		return []string{"handoff-confirmation"}
	default:
		return nil
	}
}

func pipelineStageIDForPendingAction(action ReleaseExecutionPendingAction) string {
	switch action {
	case ReleaseExecutionPendingApplyMaterialization:
		return "release-file-materialization"
	case ReleaseExecutionPendingWriteState:
		return "selected-unit-state-write"
	case ReleaseExecutionPendingStageReleaseFiles:
		return "known-release-file-staging"
	case ReleaseExecutionPendingCreateReleaseCommit:
		return "release-commit-creation"
	case ReleaseExecutionPendingCreateUnitTag:
		return "unit-tag-creation"
	case ReleaseExecutionPendingCreateDispatchJournal:
		return "workflow-request-preparation"
	case ReleaseExecutionPendingPushReleaseCommit:
		return "release-commit-push"
	case ReleaseExecutionPendingPushUnitTag:
		return "unit-tag-push"
	default:
		return ""
	}
}

func pipelineRuntimeProblem(kind, reason string) pipelineinspection.RuntimeProblem {
	return pipelineinspection.RuntimeProblem{Kind: kind, Reason: reason}
}
