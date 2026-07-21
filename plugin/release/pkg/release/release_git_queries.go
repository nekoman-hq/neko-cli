package release

import (
	"fmt"
	"sort"
	"strings"
)

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

func (coordinator *GitReleaseCoordinator) commitExists(repositoryRoot, commitSHA string) (bool, error) {
	_, err := coordinator.gitOutput(repositoryRoot, "rev-parse", "--verify", "--quiet", commitSHA+"^{commit}")
	if err != nil {
		if isGitNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspect local release commit %s: %w", commitSHA, err)
	}
	return true, nil
}

func (coordinator *GitReleaseCoordinator) headContainsCommit(repositoryRoot, commitSHA string) (bool, error) {
	_, err := coordinator.gitOutput(repositoryRoot, "merge-base", "--is-ancestor", commitSHA, "HEAD")
	if err != nil {
		if isGitNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspect whether HEAD contains release commit %s: %w", commitSHA, err)
	}
	return true, nil
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
