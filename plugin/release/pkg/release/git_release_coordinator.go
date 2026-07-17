package release

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type gitCommandRunner interface {
	Run(repositoryRoot string, args ...string) (string, error)
}

type execGitRunner struct{}

func (execGitRunner) Run(repositoryRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repositoryRoot}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(output)), err)
	}
	return string(output), nil
}

type GitReleaseCoordinator struct {
	runner      gitCommandRunner
	progress    ReleaseProgress
	diagnostics gitReleaseDiagnostics
}

type GitReleaseCoordinatorOption func(*GitReleaseCoordinator)

func NewGitReleaseCoordinator(options ...GitReleaseCoordinatorOption) *GitReleaseCoordinator {
	coordinator := &GitReleaseCoordinator{
		runner:      execGitRunner{},
		progress:    noopReleaseProgress{},
		diagnostics: noopGitReleaseDiagnostics{},
	}
	for _, option := range options {
		if option != nil {
			option(coordinator)
		}
	}
	return coordinator
}

func WithGitReleaseProgress(progress ReleaseProgress) GitReleaseCoordinatorOption {
	return func(coordinator *GitReleaseCoordinator) {
		coordinator.progress = releaseProgressOrNoop(progress)
	}
}

func WithGitReleaseDiagnostics(diagnostics gitReleaseDiagnostics) GitReleaseCoordinatorOption {
	return func(coordinator *GitReleaseCoordinator) {
		coordinator.diagnostics = gitReleaseDiagnosticsOrNoop(diagnostics)
	}
}

func newGitReleaseCoordinatorWithRunner(runner gitCommandRunner) *GitReleaseCoordinator {
	return &GitReleaseCoordinator{
		runner:      runner,
		progress:    noopReleaseProgress{},
		diagnostics: noopGitReleaseDiagnostics{},
	}
}

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

func (coordinator *GitReleaseCoordinator) UnstageKnown(files KnownReleaseFiles) error {
	paths := files.RelativePaths()
	if len(paths) == 0 {
		return nil
	}
	reportReleaseProgress(coordinator.progress, ReleaseProgressEvent{Kind: ReleaseProgressGitKnownFilesUnstaging, Files: paths})
	if _, err := coordinator.gitOutput(files.RepositoryRoot, append([]string{"restore", "--staged", "--"}, paths...)...); err != nil {
		return fmt.Errorf("unstage V2 release files %s: %w", strings.Join(paths, ", "), err)
	}
	return nil
}

func ReleaseCommitMessage(ctx *ReleaseExecutionContext) string {
	return fmt.Sprintf("chore(release): %s %s", ctx.Unit.ID, ctx.Tag)
}

func (coordinator *GitReleaseCoordinator) ensureOnlyKnownStatus(repositoryRoot string, files KnownReleaseFiles) error {
	status, err := coordinator.gitOutput(repositoryRoot, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("inspect V2 release status: %w", err)
	}
	known := files.RelativePathSet()
	for _, line := range strings.Split(status, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		path := statusPath(line)
		if _, ok := known[path]; !ok {
			return fmt.Errorf("%s Foreign change %q is present during V2 release staging", v2CleanlinessMessage, path)
		}
	}
	return nil
}

func statusPath(line string) string {
	if len(line) < 4 {
		return strings.TrimSpace(line)
	}
	path := strings.TrimSpace(line[3:])
	if strings.Contains(path, " -> ") {
		parts := strings.Split(path, " -> ")
		path = parts[len(parts)-1]
	}
	return strings.Trim(path, `"`)
}

func (coordinator *GitReleaseCoordinator) tagCommit(repositoryRoot, tag string) (string, error) {
	output, err := coordinator.gitOutput(repositoryRoot, "rev-parse", "--verify", "--quiet", "refs/tags/"+tag+"^{}")
	if err != nil {
		if isGitNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("inspect V2 unit tag %q: %w", tag, err)
	}
	return strings.TrimSpace(output), nil
}

func (coordinator *GitReleaseCoordinator) headCommit(repositoryRoot string) (string, error) {
	output, err := coordinator.gitOutput(repositoryRoot, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve HEAD commit: %w", err)
	}
	return strings.TrimSpace(output), nil
}

func (coordinator *GitReleaseCoordinator) gitOutput(repositoryRoot string, args ...string) (string, error) {
	coordinator.diagnostics.GitCommandRunning(repositoryRoot, args)
	output, err := coordinator.runner.Run(repositoryRoot, args...)
	if err != nil {
		coordinator.diagnostics.GitCommandFailed(args, output)
		return output, err
	}
	coordinator.diagnostics.GitCommandSucceeded(args, output)
	return output, nil
}

func sortedNonEmptyLines(output string) []string {
	lines := strings.Split(output, "\n")
	values := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			values = append(values, line)
		}
	}
	sort.Strings(values)
	return values
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func isGitNotFound(err error) bool {
	return strings.Contains(err.Error(), "exit status 1")
}

func formatCommandArgsForLog(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, quoteCommandArgForLog(arg))
	}
	return strings.Join(quoted, " ")
}

func quoteCommandArgForLog(arg string) string {
	if arg == "" {
		return "''"
	}
	if strings.ContainsAny(arg, " \t\n\"'\\$") {
		return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}
	return arg
}

func summarizeCommandOutput(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "no output"
	}
	lineCount := len(strings.Split(trimmed, "\n"))
	return fmt.Sprintf("%d bytes, %d lines", len(output), lineCount)
}
