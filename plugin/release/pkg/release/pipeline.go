package release

import (
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/pipelineinspection"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

// HandlePipeline returns the locally configured Release V2 pipeline without
// executing or planning a release.
func HandlePipeline(request plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveInspectionRepositoryRoot(request.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandlePipelineAt(root, request)
}

// HandlePipelineAt returns the configured pipeline at an explicit root.
func HandlePipelineAt(root workspace.RepositoryRoot, request plugin.Request) (*plugin.Response, error) {
	runtime := inspectPipelineRuntime(root.Path())
	return pipelineinspection.HandlePipelineRuntimeAt(root, request, configuredReleaseLifecycleStages(), runtime)
}
