package release

import (
	"path/filepath"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

type terminalGitReleaseDiagnostics struct{}

func newTerminalGitReleaseDiagnostics() gitReleaseDiagnostics {
	return terminalGitReleaseDiagnostics{}
}

func (terminalGitReleaseDiagnostics) GitInputsValidated(unitID, tag string, files []string) {
	log.PluginV(log.Exec, "Validated V2 git inputs for unit=%s tag=%s files=%s", unitID, tag, strings.Join(files, ", "))
}

func (terminalGitReleaseDiagnostics) GitTopLevelResolved(string) {
	log.PluginV(log.Exec, "Git toplevel resolved to the selected repository root")
}

func (terminalGitReleaseDiagnostics) GitUpstreamResolved(branch, remote, upstreamBranch string) {
	log.PluginV(log.Exec, "Resolved upstream for branch %s: remote=%s branch=%s", branch, remote, upstreamBranch)
}

func (terminalGitReleaseDiagnostics) GitStatusClean() {
	log.PluginV(log.Exec, "Git status is clean")
}

func (terminalGitReleaseDiagnostics) GitStagedFilesMatch(files []string) {
	log.PluginV(log.Exec, "Staged file set matches expected release files: %s", strings.Join(files, ", "))
}

func (terminalGitReleaseDiagnostics) GitCommitContainsState(commitSHA, version, unitID string) {
	log.PluginV(log.Exec, "Release commit %s contains expected state version %s for unit %s", commitSHA, version, unitID)
}

func (terminalGitReleaseDiagnostics) GitCommandRunning(string, []string) {
	log.PluginV(log.Exec, "Running repository-local git command")
}

func (terminalGitReleaseDiagnostics) GitCommandFailed(args []string, output string) {
	log.PluginV(log.Exec, "Git command failed: %s (%s)", safeGitOperationForLog(args), summarizeCommandOutput(output))
}

func (terminalGitReleaseDiagnostics) GitCommandSucceeded(args []string, output string) {
	log.PluginV(log.Exec, "Git command succeeded: %s (%s)", safeGitOperationForLog(args), summarizeCommandOutput(output))
}

func safeGitOperationForLog(args []string) string {
	if len(args) == 0 {
		return "unspecified operation"
	}
	for _, arg := range args {
		if filepath.IsAbs(arg) {
			return args[0] + " (repository-local path omitted)"
		}
	}
	return formatCommandArgsForLog(args)
}
