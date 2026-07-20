package release

import "github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"

// GitHubActionsReleaseWorkflowContractVersion identifies the generated
// workflow shape. It is independent from Release V2 config and plugin response
// schema versions.
const GitHubActionsReleaseWorkflowContractVersion = releaseworkflow.GitHubActionsReleaseWorkflowContractVersion

// RenderCanonicalGitHubActionsReleaseWorkflow renders the deterministic
// build-system-neutral workflow shared by documentation and scaffolding.
func RenderCanonicalGitHubActionsReleaseWorkflow() ([]byte, error) {
	return releaseworkflow.RenderCanonicalGitHubActionsReleaseWorkflow()
}
