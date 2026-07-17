package release

type gitReleaseDiagnostics interface {
	GitInputsValidated(unitID, tag string, files []string)
	GitTopLevelResolved(path string)
	GitUpstreamResolved(branch, remote, upstreamBranch string)
	GitStatusClean()
	GitStagedFilesMatch(files []string)
	GitCommitContainsState(commitSHA, version, unitID string)
	GitCommandRunning(repositoryRoot string, args []string)
	GitCommandFailed(args []string, output string)
	GitCommandSucceeded(args []string, output string)
}

type noopGitReleaseDiagnostics struct{}

func (noopGitReleaseDiagnostics) GitInputsValidated(string, string, []string)   {}
func (noopGitReleaseDiagnostics) GitTopLevelResolved(string)                    {}
func (noopGitReleaseDiagnostics) GitUpstreamResolved(string, string, string)    {}
func (noopGitReleaseDiagnostics) GitStatusClean()                               {}
func (noopGitReleaseDiagnostics) GitStagedFilesMatch([]string)                  {}
func (noopGitReleaseDiagnostics) GitCommitContainsState(string, string, string) {}
func (noopGitReleaseDiagnostics) GitCommandRunning(string, []string)            {}
func (noopGitReleaseDiagnostics) GitCommandFailed([]string, string)             {}
func (noopGitReleaseDiagnostics) GitCommandSucceeded([]string, string)          {}

func gitReleaseDiagnosticsOrNoop(diagnostics gitReleaseDiagnostics) gitReleaseDiagnostics {
	if diagnostics == nil {
		return noopGitReleaseDiagnostics{}
	}
	return diagnostics
}
