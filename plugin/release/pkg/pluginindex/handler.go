package pluginindex

import (
	"context"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

const CommandName = "plugin-index"

type pluginIndexCommandHandler struct {
	useCase pluginIndexCommandRunner
	clock   pluginIndexResponseClock
}

// HandlePluginIndex generates the public plugin registry index.
func HandlePluginIndex(req plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandlePluginIndexAt(root, req)
}

// HandlePluginIndexAt generates the public plugin registry index at an explicit
// repository root without changing process cwd.
func HandlePluginIndexAt(root workspace.RepositoryRoot, req plugin.Request) (*plugin.Response, error) {
	handler := pluginIndexCommandHandler{
		useCase: newGeneratePluginIndexUseCaseAt(
			root.Path(),
			pluginIndexQueryUseCase{sources: pluginIndexDiskSourceReader{}},
			jsonPluginIndexOutputBuilder{},
			newPluginIndexOutputPersister(pluginIndexPersistenceDisk{}),
		),
		clock: systemPluginIndexResponseClock{},
	}
	return handler.Handle(req)
}

func (handler pluginIndexCommandHandler) Handle(req plugin.Request) (*plugin.Response, error) {
	request, err := parsePluginIndexCommandRequest(req.Flags)
	if err != nil {
		return nil, err
	}
	result, err := handler.useCase.Run(context.Background(), request)
	if err != nil {
		return nil, err
	}
	return mapPluginIndexCommandResponse(result, handler.clock.Now()), nil
}
