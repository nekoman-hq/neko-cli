package release

import (
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/pipelineinspection"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

// HandlePipeline returns the configured Release V2 pipeline and read-only
// local runtime evidence without executing or planning a release.
func HandlePipeline(request plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveInspectionRepositoryRoot(request.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandlePipelineAt(root, request)
}

// HandlePipelineAt returns the configured pipeline and local runtime evidence
// at an explicit root.
func HandlePipelineAt(root workspace.RepositoryRoot, request plugin.Request) (*plugin.Response, error) {
	verification := inspectLocalPipelineVerification(root, request)
	runtime := inspectPipelineRuntime(root.Path())
	return pipelineinspection.HandlePipelineRuntimeVerificationAt(
		root, request, configuredReleaseLifecycleStages(), runtime, verification,
	)
}
