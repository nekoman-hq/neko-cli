package release

import (
	"fmt"
	"path/filepath"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const v2CleanlinessMessage = "V2 releases require a clean worktree and index because Nekocli commits only the generated release state and declared materialized files."

type GitReleasePreflight struct {
	Branch         string
	Remote         string
	UpstreamBranch string
}

func (coordinator *GitReleaseCoordinator) Preflight(ctx *ReleaseExecutionContext, files KnownReleaseFiles) (GitReleasePreflight, error) {
	if err := validateGitReleaseInputs(ctx, files); err != nil {
		return GitReleasePreflight{}, err
	}
	if err := coordinator.ensureRepositoryRoot(ctx.RepositoryRoot); err != nil {
		return GitReleasePreflight{}, err
	}
	branch, err := coordinator.currentBranch(ctx.RepositoryRoot)
	if err != nil {
		return GitReleasePreflight{}, err
	}
	remote, upstreamBranch, err := coordinator.upstream(ctx.RepositoryRoot, branch)
	if err != nil {
		return GitReleasePreflight{}, err
	}
	if cleanErr := coordinator.ensureCleanWorktreeAndIndex(ctx.RepositoryRoot); cleanErr != nil {
		return GitReleasePreflight{}, cleanErr
	}
	existingCommit, err := coordinator.tagCommit(ctx.RepositoryRoot, ctx.Tag)
	if err != nil {
		return GitReleasePreflight{}, err
	}
	if existingCommit != "" {
		return GitReleasePreflight{}, fmt.Errorf("tag %q already exists before V2 release commit creation; expected a new unit tag", ctx.Tag)
	}
	return GitReleasePreflight{
		Branch:         branch,
		Remote:         remote,
		UpstreamBranch: upstreamBranch,
	}, nil
}

func validateGitReleaseInputs(ctx *ReleaseExecutionContext, files KnownReleaseFiles) error {
	if ctx == nil {
		return fmt.Errorf("release execution context is missing")
	}
	if ctx.SourceFormat != releaseconfig.SourceFormatV2 {
		return fmt.Errorf("git release coordination supports V2 repositories only")
	}
	if !ctx.TagSpec.Matches(ctx.Tag) {
		return fmt.Errorf("tag %q does not match unit %q tag prefix %q", ctx.Tag, ctx.Unit.ID, ctx.TagSpec.Prefix)
	}
	if parsedVersion, ok := ctx.TagSpec.Parse(ctx.Tag); !ok || parsedVersion != ctx.NextVersion {
		return fmt.Errorf("tag %q does not encode next version %q for unit %q", ctx.Tag, ctx.NextVersion, ctx.Unit.ID)
	}
	if files.RepositoryRoot == "" {
		files.RepositoryRoot = ctx.RepositoryRoot
	}
	if err := files.Validate(); err != nil {
		return err
	}
	absoluteContextRoot, err := filepath.Abs(ctx.RepositoryRoot)
	if err != nil {
		return fmt.Errorf("repository root %q cannot be resolved: %w", ctx.RepositoryRoot, err)
	}
	absoluteFilesRoot, err := filepath.Abs(files.RepositoryRoot)
	if err != nil {
		return fmt.Errorf("known release files repository root %q cannot be resolved: %w", files.RepositoryRoot, err)
	}
	if absoluteContextRoot != absoluteFilesRoot {
		return fmt.Errorf("known release files root %s does not match context repository root %s", absoluteFilesRoot, absoluteContextRoot)
	}
	return nil
}

func (coordinator *GitReleaseCoordinator) ensureRepositoryRoot(repositoryRoot string) error {
	inside, err := coordinator.gitOutput(repositoryRoot, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return fmt.Errorf("repository root %s is not a git repository: %w", repositoryRoot, err)
	}
	if strings.TrimSpace(inside) != "true" {
		return fmt.Errorf("repository root %s is not inside a git worktree", repositoryRoot)
	}
	topLevel, err := coordinator.gitOutput(repositoryRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("unable to resolve git toplevel: %w", err)
	}
	absoluteRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return fmt.Errorf("repository root %q cannot be resolved: %w", repositoryRoot, err)
	}
	physicalRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return fmt.Errorf("repository root %q cannot be resolved physically: %w", repositoryRoot, err)
	}
	physicalTopLevel, err := filepath.EvalSymlinks(strings.TrimSpace(topLevel))
	if err != nil {
		return fmt.Errorf("git toplevel %q cannot be resolved physically: %w", strings.TrimSpace(topLevel), err)
	}
	if physicalTopLevel != physicalRoot {
		return fmt.Errorf("repository root %s does not match git toplevel %s", physicalRoot, physicalTopLevel)
	}
	return nil
}

func (coordinator *GitReleaseCoordinator) currentBranch(repositoryRoot string) (string, error) {
	branch, err := coordinator.gitOutput(repositoryRoot, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("current branch is not resolvable: %w", err)
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "", fmt.Errorf("current branch is empty")
	}
	return branch, nil
}

func (coordinator *GitReleaseCoordinator) upstream(repositoryRoot, branch string) (string, string, error) {
	remote, err := coordinator.gitOutput(repositoryRoot, "config", "--get", "branch."+branch+".remote")
	if err != nil || strings.TrimSpace(remote) == "" {
		return "", "", fmt.Errorf("branch %q has no configured upstream remote", branch)
	}
	remote = strings.TrimSpace(remote)
	upstreamRef, err := coordinator.gitOutput(repositoryRoot, "config", "--get", "branch."+branch+".merge")
	if err != nil || strings.TrimSpace(upstreamRef) == "" {
		return "", "", fmt.Errorf("branch %q has no configured upstream branch", branch)
	}
	upstreamBranch := strings.TrimPrefix(strings.TrimSpace(upstreamRef), "refs/heads/")
	if upstreamBranch == "" {
		return "", "", fmt.Errorf("branch %q upstream branch is empty", branch)
	}
	if _, err := coordinator.gitOutput(repositoryRoot, "remote", "get-url", remote); err != nil {
		return "", "", fmt.Errorf("branch %q upstream remote %q is not resolvable: %w", branch, remote, err)
	}
	return remote, upstreamBranch, nil
}

func (coordinator *GitReleaseCoordinator) ensureCleanWorktreeAndIndex(repositoryRoot string) error {
	status, err := coordinator.gitOutput(repositoryRoot, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("unable to inspect git status: %w", err)
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("%s Current status:\n%s", v2CleanlinessMessage, strings.TrimSpace(status))
	}
	return nil
}
