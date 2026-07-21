package release

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/pipelineinspection"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func inspectPipelineDispatchJournals(repositoryRoot string, snapshot *pipelineinspection.RuntimeSnapshot) {
	store := NewDispatchJournalStore(repositoryRoot)
	directory, err := store.JournalDirectory()
	if err != nil {
		snapshot.Problems = append(snapshot.Problems, pipelineRuntimeProblem("dispatch_journal", "dispatch journal location could not be inspected"))
		return
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		snapshot.Problems = append(snapshot.Problems, pipelineRuntimeProblem("dispatch_journal", "dispatch journal directory could not be read"))
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		reference := path.Join("dispatches", entry.Name())
		journal, loadErr := store.loadAt(filepath.Join(directory, entry.Name()))
		if loadErr != nil || journal == nil {
			identity := strings.TrimSuffix(entry.Name(), ".json")
			if !isSafeDispatchIdentityHash(identity) {
				identity = ""
			}
			snapshot.Dispatches = append(snapshot.Dispatches, pipelineinspection.RuntimeDispatchObservation{
				Reference: reference, Identity: identity, Problem: "dispatch journal is malformed",
			})
			continue
		}
		snapshot.Dispatches = append(snapshot.Dispatches, observePipelineDispatchJournal(entry.Name(), reference, journal))
	}
}

func observePipelineDispatchJournal(filename, reference string, journal *DispatchJournal) pipelineinspection.RuntimeDispatchObservation {
	observation := pipelineinspection.RuntimeDispatchObservation{
		Reference: reference, Identity: journal.Identity.SHA256,
		RepositoryRemote: journal.RepositoryRemote, RepositoryRemoteName: journal.RepositoryRemoteName,
		UnitID: journal.UnitID, Version: journal.Version, Tag: journal.Tag,
		ReleaseCommitSHA: journal.ReleaseCommitSHA, WorkflowPath: journal.WorkflowPath,
		Executor: journal.Executor, Delivery: journal.Delivery, State: string(journal.State),
		RunID:     journal.DispatchMetadata.RunID,
		CreatedAt: journal.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: journal.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if err := validatePipelineDispatchJournal(filename, journal); err != nil {
		observation.Problem = err.Error()
		return observation
	}
	observation.Valid = true
	observation.RetrySafety, observation.ManualIntervention = observePipelineDispatchRetry(journal)
	return observation
}

func validatePipelineDispatchJournal(filename string, journal *DispatchJournal) error {
	if journal.SchemaVersion != dispatchJournalSchemaVersion || !journal.State.Valid() {
		return fmt.Errorf("dispatch journal is structurally invalid")
	}
	identity, err := newReleaseDispatchIdentity(
		journal.RepositoryRemoteName, journal.RepositoryRemote, journal.UnitID,
		journal.Version, journal.Tag, journal.ReleaseCommitSHA,
		journal.WorkflowPath, journal.Executor, journal.Delivery,
	)
	if err != nil || identity.SHA256 != journal.Identity.SHA256 || identity != journal.Identity {
		return fmt.Errorf("dispatch identity does not match immutable journal fields")
	}
	if filename != journal.Identity.SHA256+".json" {
		return fmt.Errorf("dispatch journal reference does not match its exact identity")
	}
	if journal.RepositoryRemoteName != journal.Identity.RepositoryRemoteName ||
		journal.RepositoryRemote != journal.Identity.RepositoryRemote ||
		journal.UnitID != journal.Identity.UnitID || journal.Version != journal.Identity.Version ||
		journal.Tag != journal.Identity.Tag || journal.ReleaseCommitSHA != journal.Identity.ReleaseCommitSHA ||
		journal.WorkflowPath != journal.Identity.WorkflowPath || journal.Executor != journal.Identity.Executor ||
		journal.Delivery != journal.Identity.Delivery {
		return fmt.Errorf("dispatch identity conflicts with journal fields")
	}
	if journal.WorkflowFileName != path.Base(journal.WorkflowPath) {
		return fmt.Errorf("dispatch workflow filename conflicts with the exact workflow path")
	}
	expectedInputs := map[string]string{
		"unit": journal.UnitID, "version": journal.Version,
		"tag": journal.Tag, "release_sha": journal.ReleaseCommitSHA,
	}
	if !sameStringMap(journal.Inputs, expectedInputs) {
		return fmt.Errorf("dispatch inputs conflict with immutable dispatch identity")
	}
	if journal.CreatedAt.IsZero() || journal.UpdatedAt.Before(journal.CreatedAt) {
		return fmt.Errorf("dispatch journal timestamps are invalid")
	}
	return nil
}

func inspectPipelineExecutionGit(repositoryRoot string, coordinator *GitReleaseCoordinator, journal *ReleaseExecutionJournal) pipelineinspection.RuntimeLocalGitObservation {
	observation := pipelineinspection.RuntimeLocalGitObservation{
		Inspected: true, ExpectedCommit: journal.ReleaseCommitSHA,
		ExpectedTag: journal.Tag, Consistent: true,
	}
	known := knownReleaseFilesFromJournal(repositoryRoot, journal)
	index, indexErr := coordinator.gitOutput(repositoryRoot, "diff", "--cached", "--name-only")
	worktree, worktreeErr := coordinator.gitOutput(repositoryRoot, "diff", "--name-only")
	untracked, untrackedErr := coordinator.gitOutput(repositoryRoot, "ls-files", "--others", "--exclude-standard")
	if indexErr != nil || worktreeErr != nil || untrackedErr != nil {
		observation.Consistent = false
		observation.Problem = "local Git recovery evidence could not be inspected"
		return observation
	}
	observation.IndexContainsRecoveryEvidence = outputContainsKnownReleaseFile(index, known)
	observation.WorktreeContainsRecoveryEvidence = outputContainsKnownReleaseFile(worktree+"\n"+untracked, known)

	if journal.ReleaseCommitSHA != "" {
		exists, err := coordinator.commitExists(repositoryRoot, journal.ReleaseCommitSHA)
		if err != nil {
			return inconsistentPipelineGit(observation, "expected local release commit could not be inspected")
		}
		observation.CommitExists = exists
		if exists {
			ctx := &ReleaseExecutionContext{
				RepositoryRoot: repositoryRoot, Unit: releaseconfig.ReleaseUnit{ID: journal.UnitID},
				NextVersion: journal.NextVersion,
			}
			if err := coordinator.verifyCommitObject(ctx, known, journal.ReleaseCommitSHA); err == nil {
				observation.CommitContentVerified = true
			} else {
				observation.Problem = "local release commit content does not match journal evidence"
				observation.Consistent = false
			}
			contains, containsErr := coordinator.headContainsCommit(repositoryRoot, journal.ReleaseCommitSHA)
			if containsErr != nil {
				return inconsistentPipelineGit(observation, "HEAD containment of the release commit could not be inspected")
			}
			observation.HeadContainsExpectedCommit = contains
		}
		if releaseExecutionStateRank(journal.State) >= releaseExecutionStateRank(ReleaseExecutionCommitCreated) &&
			(!observation.CommitExists || !observation.CommitContentVerified || !observation.HeadContainsExpectedCommit) {
			if observation.Problem == "" {
				observation.Problem = "confirmed release commit is missing or inconsistent in local Git"
			}
			observation.Consistent = false
		}
	}

	tagTarget, tagErr := coordinator.tagCommit(repositoryRoot, journal.Tag)
	if tagErr != nil {
		return inconsistentPipelineGit(observation, "expected local unit tag could not be inspected")
	}
	observation.TagTarget = tagTarget
	observation.TagExists = tagTarget != ""
	observation.TagMatchesExpectedCommit = tagTarget != "" && journal.ReleaseCommitSHA != "" && tagTarget == journal.ReleaseCommitSHA
	if releaseExecutionStateRank(journal.State) >= releaseExecutionStateRank(ReleaseExecutionTagCreated) &&
		(!observation.TagExists || !observation.TagMatchesExpectedCommit) {
		observation.Consistent = false
		if observation.Problem == "" {
			observation.Problem = "confirmed unit tag is missing or points to a different local commit"
		}
	}
	return observation
}

func outputContainsKnownReleaseFile(output string, files KnownReleaseFiles) bool {
	known := files.RelativePathSet()
	for _, line := range strings.Split(output, "\n") {
		if _, present := known[strings.TrimSpace(line)]; present {
			return true
		}
	}
	return false
}

func inconsistentPipelineGit(observation pipelineinspection.RuntimeLocalGitObservation, problem string) pipelineinspection.RuntimeLocalGitObservation {
	observation.Consistent = false
	observation.Problem = problem
	return observation
}
