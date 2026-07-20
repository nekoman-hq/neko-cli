package release

import (
	"fmt"
	"strings"
)

func (coordinator *GitReleaseCoordinator) Stage(ctx *ReleaseExecutionContext, files KnownReleaseFiles) error {
	if ctx.DryRun {
		return fmt.Errorf("dry run does not stage V2 release files")
	}
	if err := validateGitReleaseInputs(ctx, files, coordinator.diagnostics); err != nil {
		return err
	}
	reportReleaseProgress(coordinator.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitWorktreeChecking})
	if err := coordinator.ensureOnlyKnownStatus(ctx.RepositoryRoot, files); err != nil {
		return err
	}
	paths := files.RelativePaths()
	if len(paths) == 0 {
		return fmt.Errorf("known release files are missing")
	}
	reportReleaseProgress(coordinator.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitStagingFiles, Files: paths})
	if _, err := coordinator.gitOutput(ctx.RepositoryRoot, append([]string{"add", "--"}, paths...)...); err != nil {
		_ = coordinator.UnstageKnown(files)
		return fmt.Errorf("stage V2 release files %s: %w", strings.Join(paths, ", "), err)
	}
	if err := coordinator.VerifyStagedFiles(ctx, files); err != nil {
		_ = coordinator.UnstageKnown(files)
		return err
	}
	reportReleaseProgress(coordinator.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitStagedFilesVerified})
	return nil
}

func (coordinator *GitReleaseCoordinator) VerifyStagedFiles(ctx *ReleaseExecutionContext, files KnownReleaseFiles) error {
	if err := validateGitReleaseInputs(ctx, files, coordinator.diagnostics); err != nil {
		return err
	}
	staged, err := coordinator.gitOutput(ctx.RepositoryRoot, "diff", "--cached", "--name-only")
	if err != nil {
		return fmt.Errorf("inspect staged V2 release files: %w", err)
	}
	actual := sortedNonEmptyLines(staged)
	expected := files.RelativePaths()
	if !sameStringSet(actual, expected) {
		return fmt.Errorf("staged V2 release files differ from expected set; expected [%s], got [%s]", strings.Join(expected, ", "), strings.Join(actual, ", "))
	}
	coordinator.diagnostics.GitStagedFilesMatch(expected)
	return nil
}
