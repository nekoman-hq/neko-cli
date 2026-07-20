package contextvalidation

import (
	"fmt"
	"os/exec"
	"strings"
)

type contextGitCommandRunner interface {
	Run(repositoryRoot string, args ...string) (string, error)
}

type execContextGitRunner struct{}

func (execContextGitRunner) Run(repositoryRoot string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", repositoryRoot}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(output)), err)
	}
	return string(output), nil
}

func isContextGitNotFound(err error) bool {
	return strings.Contains(err.Error(), "exit status 1")
}
