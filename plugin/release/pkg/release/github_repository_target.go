package release

import "github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"

// GitHubRepositoryTarget identifies the GitHub.com repository that receives a
// workflow_dispatch request.
type GitHubRepositoryTarget = releaseworkflow.GitHubRepositoryTarget

// ResolveGitHubRepositoryTarget derives the dispatch target from the verified
// Git remote selected by V2 Git coordination.
func ResolveGitHubRepositoryTarget(remoteName, remoteURL string) (GitHubRepositoryTarget, error) {
	return releaseworkflow.ResolveGitHubRepositoryTarget(remoteName, remoteURL)
}
