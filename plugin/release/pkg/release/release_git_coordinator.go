package release

import (
	"fmt"
	"os/exec"
	"strings"
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
