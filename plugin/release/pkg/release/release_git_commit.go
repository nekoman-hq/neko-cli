package release

import (
	"encoding/json"
	"fmt"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func (coordinator *GitReleaseCoordinator) Commit(ctx *ReleaseExecutionContext, files KnownReleaseFiles) (string, error) {
	if ctx.DryRun {
		return "", fmt.Errorf("dry run does not create V2 release commits")
	}
	if err := coordinator.VerifyStagedFiles(ctx, files); err != nil {
		return "", err
	}
	message := ReleaseCommitMessage(ctx)
	reportReleaseProgress(coordinator.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitReleaseCommitCreating, CommitMessage: message})
	if _, err := coordinator.gitOutput(ctx.RepositoryRoot, "commit", "-m", message); err != nil {
		return "", fmt.Errorf("create V2 release commit %q: %w", message, err)
	}
	commitSHA, err := coordinator.headCommit(ctx.RepositoryRoot)
	if err != nil {
		return "", err
	}
	if err := coordinator.VerifyCommit(ctx, files, commitSHA); err != nil {
		return "", err
	}
	reportReleaseProgress(coordinator.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitReleaseCommitVerified, CommitSHA: commitSHA})
	return commitSHA, nil
}

func (coordinator *GitReleaseCoordinator) VerifyCommit(ctx *ReleaseExecutionContext, files KnownReleaseFiles, commitSHA string) error {
	head, err := coordinator.headCommit(ctx.RepositoryRoot)
	if err != nil {
		return err
	}
	if head != commitSHA {
		return fmt.Errorf("HEAD %s does not match release commit %s", head, commitSHA)
	}
	changed, err := coordinator.gitOutput(ctx.RepositoryRoot, "diff-tree", "--no-commit-id", "--name-only", "-r", commitSHA)
	if err != nil {
		return fmt.Errorf("inspect V2 release commit files: %w", err)
	}
	actual := sortedNonEmptyLines(changed)
	expected := files.RelativePaths()
	if !sameStringSet(actual, expected) {
		return fmt.Errorf("V2 release commit contains unexpected files; expected [%s], got [%s]", strings.Join(expected, ", "), strings.Join(actual, ", "))
	}
	stateContent, err := coordinator.gitOutput(ctx.RepositoryRoot, "show", commitSHA+":"+releaseconfig.V2Directory+"/"+releaseconfig.V2StateFileName)
	if err != nil {
		return fmt.Errorf("inspect V2 state in release commit %s: %w", commitSHA, err)
	}
	var state releaseconfig.V2ReleaseState
	if err := json.Unmarshal([]byte(stateContent), &state); err != nil {
		return fmt.Errorf("decode V2 state in release commit %s: %w", commitSHA, err)
	}
	unitState, ok := state.Units[ctx.Unit.ID]
	if !ok {
		return fmt.Errorf("release commit %s state is missing unit %q", commitSHA, ctx.Unit.ID)
	}
	if unitState.Version != ctx.NextVersion {
		return fmt.Errorf("release commit %s state unit %q version = %q, expected %q", commitSHA, ctx.Unit.ID, unitState.Version, ctx.NextVersion)
	}
	coordinator.diagnostics.GitCommitContainsState(commitSHA, ctx.NextVersion, ctx.Unit.ID)
	return nil
}
