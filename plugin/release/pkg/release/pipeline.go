package release

import (
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/pipelineinspection"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

// HandlePipeline returns the locally configured Release V2 pipeline without
// executing or planning a release.
func HandlePipeline(request plugin.Request) (*plugin.Response, error) {
	return pipelineinspection.HandlePipeline(request, configuredReleaseLifecycleStages())
}

// HandlePipelineAt returns the configured pipeline at an explicit root.
func HandlePipelineAt(root workspace.RepositoryRoot, request plugin.Request) (*plugin.Response, error) {
	return pipelineinspection.HandlePipelineAt(root, request, configuredReleaseLifecycleStages())
}
