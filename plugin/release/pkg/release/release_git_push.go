package release

import (
	"fmt"
	"strings"
)

func (coordinator *GitReleaseCoordinator) Push(ctx *ReleaseExecutionContext, commitSHA string, result *GitReleaseResult) error {
	if ctx.DryRun {
		return fmt.Errorf("dry run does not push V2 release commits or tags")
	}
	branch, err := coordinator.currentBranch(ctx.RepositoryRoot)
	if err != nil {
		return err
	}
	remote, upstreamBranch, err := coordinator.upstream(ctx.RepositoryRoot, branch)
	if err != nil {
		return err
	}
	if result != nil {
		result.RepositoryRemoteName = remote
		remoteURL, urlErr := coordinator.gitOutput(ctx.RepositoryRoot, "remote", "get-url", remote)
		if urlErr != nil {
			return fmt.Errorf("resolve V2 release remote %q: %w", remote, urlErr)
		}
		result.RepositoryRemote = strings.TrimSpace(remoteURL)
	}
	if err := coordinator.PushCommit(ctx, remote, upstreamBranch, commitSHA); err != nil {
		if result != nil {
			result.RecoveryGuidance = recoveryGuidanceCommitCreated(commitSHA, ctx.Unit.ID, ctx.Tag)
		}
		return err
	}
	if result != nil {
		result.CommitPushed = true
		result.ReachedPhase = "commit-pushed"
		result.RecoveryGuidance = recoveryGuidanceCommitPushedTagMissing(commitSHA, ctx.Unit.ID, ctx.Tag)
	}
	if err := coordinator.PushTag(ctx, remote, ctx.Tag, commitSHA); err != nil {
		return err
	}
	if result != nil {
		result.TagPushed = true
		result.ReachedPhase = "tag-pushed"
	}
	return nil
}

func (coordinator *GitReleaseCoordinator) PushCommit(ctx *ReleaseExecutionContext, remote, upstreamBranch, commitSHA string) error {
	if ctx.DryRun {
		return fmt.Errorf("dry run does not push V2 release commits")
	}
	reportReleaseProgress(coordinator.progress, ReleaseProgressEvent{
		Kind:           ReleaseProgressGitReleaseCommitPushing,
		CommitSHA:      commitSHA,
		Remote:         remote,
		UpstreamBranch: upstreamBranch,
	})
	if _, err := coordinator.gitOutput(ctx.RepositoryRoot, "push", remote, "HEAD:refs/heads/"+upstreamBranch); err != nil {
		return fmt.Errorf("push V2 release commit %s for unit %q before tag %q failed: %w", commitSHA, ctx.Unit.ID, ctx.Tag, err)
	}
	reportReleaseProgress(coordinator.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitReleaseCommitPushed, CommitSHA: commitSHA})
	return nil
}

func (coordinator *GitReleaseCoordinator) PushTag(ctx *ReleaseExecutionContext, remote, tag, commitSHA string) error {
	if ctx.DryRun {
		return fmt.Errorf("dry run does not push V2 release tags")
	}
	reportReleaseProgress(coordinator.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitUnitTagPushing, Tag: tag, Remote: remote})
	if _, err := coordinator.gitOutput(ctx.RepositoryRoot, "push", remote, "refs/tags/"+tag+":refs/tags/"+tag); err != nil {
		return fmt.Errorf("push V2 unit tag %q failed after release commit %s was pushed for unit %q: %w", tag, commitSHA, ctx.Unit.ID, err)
	}
	reportReleaseProgress(coordinator.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitUnitTagPushed, Tag: tag})
	return nil
}
