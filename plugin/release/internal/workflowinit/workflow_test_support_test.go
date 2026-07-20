package workflowinit

import "github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"

const GitHubActionsReleaseWorkflowContractVersion = releaseworkflow.GitHubActionsReleaseWorkflowContractVersion

func RenderCanonicalGitHubActionsReleaseWorkflow() ([]byte, error) {
	return releaseworkflow.RenderCanonicalGitHubActionsReleaseWorkflow()
}
