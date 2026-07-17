package history

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      29.12.2025
*/

import (
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

type historyCommandHandler struct {
	query historyQuerier
	clock historyResponseClock
}

func HandleHistory(req plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleHistoryAt(root, req)
}

// HandleHistoryAt returns release history at an explicit repository root
// without changing process cwd.
func HandleHistoryAt(root workspace.RepositoryRoot, req plugin.Request) (*plugin.Response, error) {
	handler := historyCommandHandler{
		query: newHistoryQueryUseCaseAt(
			root.Path(),
			historyReleaseRepositoryReader{},
			historyGitAdapter{repositoryRoot: root.Path()},
		),
		clock: systemHistoryResponseClock{},
	}
	return handler.Handle(req)
}

func (handler historyCommandHandler) Handle(req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Exec, "Starting release history")

	result, failure := handler.query.Query(parseHistoryQueryRequest(req.Flags))
	if failure == nil && result.SourceFormat == config.SourceFormatV1 {
		log.PluginV(log.Exec, "Found %d tags", len(result.Entries))
		log.PluginPrint(log.Exec, "Release history completed")
	}

	return mapHistoryQueryResponse(result, failure, handler.clock.Now()), nil
}
