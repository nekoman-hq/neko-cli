package release

import (
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/workflowinit"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

// HandleGitHubWorkflowInit resolves the repository root and creates or
// previews the configured canonical GitHub Actions release workflow.
func HandleGitHubWorkflowInit(request plugin.Request) (*plugin.Response, error) {
	return workflowinit.HandleGitHubWorkflowInit(request)
}

// HandleGitHubWorkflowInitAt creates or previews the configured canonical
// GitHub Actions release workflow at an explicit repository root.
func HandleGitHubWorkflowInitAt(root workspace.RepositoryRoot, request plugin.Request) (*plugin.Response, error) {
	return workflowinit.HandleGitHubWorkflowInitAt(root, request)
}
