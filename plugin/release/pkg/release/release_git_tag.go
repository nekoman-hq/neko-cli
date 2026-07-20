package release

import "fmt"

func (coordinator *GitReleaseCoordinator) CreateTag(ctx *ReleaseExecutionContext, commitSHA string) (bool, error) {
	if ctx.DryRun {
		return false, fmt.Errorf("dry run does not create V2 release tags")
	}
	if parsedVersion, ok := ctx.TagSpec.Parse(ctx.Tag); !ok || parsedVersion != ctx.NextVersion {
		return false, fmt.Errorf("tag %q does not encode next version %q for unit %q", ctx.Tag, ctx.NextVersion, ctx.Unit.ID)
	}
	existingCommit, err := coordinator.tagCommit(ctx.RepositoryRoot, ctx.Tag)
	if err != nil {
		return false, err
	}
	if existingCommit != "" {
		if existingCommit == commitSHA {
			reportReleaseProgress(coordinator.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitUnitTagAlreadyExists, Tag: ctx.Tag, CommitSHA: commitSHA})
			return false, nil
		}
		return false, fmt.Errorf("tag %q already points to %s, expected release commit %s", ctx.Tag, existingCommit, commitSHA)
	}
	reportReleaseProgress(coordinator.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitUnitTagCreating, Tag: ctx.Tag, CommitSHA: commitSHA})
	if _, tagErr := coordinator.gitOutput(ctx.RepositoryRoot, "tag", ctx.Tag, commitSHA); tagErr != nil {
		return false, fmt.Errorf("create lightweight V2 unit tag %q on %s: %w", ctx.Tag, commitSHA, tagErr)
	}
	tagCommit, err := coordinator.tagCommit(ctx.RepositoryRoot, ctx.Tag)
	if err != nil {
		return false, err
	}
	if tagCommit != commitSHA {
		return false, fmt.Errorf("tag %q points to %s after creation, expected %s", ctx.Tag, tagCommit, commitSHA)
	}
	reportReleaseProgress(coordinator.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitUnitTagVerified, Tag: ctx.Tag})
	return true, nil
}
