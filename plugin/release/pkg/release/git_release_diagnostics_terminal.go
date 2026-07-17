package release

import (
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

func (terminalGitReleaseDiagnostics) GitTopLevelResolved(path string) {
	log.PluginV(log.Exec, "Git toplevel resolved to %s", path)
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

func (terminalGitReleaseDiagnostics) GitCommandRunning(repositoryRoot string, args []string) {
	log.PluginV(log.Exec, "Running command: git -C %s %s", repositoryRoot, formatCommandArgsForLog(args))
}

func (terminalGitReleaseDiagnostics) GitCommandFailed(args []string, output string) {
	log.PluginV(log.Exec, "Command failed: git %s (%s)", formatCommandArgsForLog(args), summarizeCommandOutput(output))
}

func (terminalGitReleaseDiagnostics) GitCommandSucceeded(args []string, output string) {
	log.PluginV(log.Exec, "Command succeeded: git %s (%s)", formatCommandArgsForLog(args), summarizeCommandOutput(output))
}
