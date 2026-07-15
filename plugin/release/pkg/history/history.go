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
)

type historyCommandHandler struct {
	query historyQuerier
	clock historyResponseClock
}

func HandleHistory(req plugin.Request) (*plugin.Response, error) {
	handler := historyCommandHandler{
		query: newHistoryQueryUseCase(
			historyReleaseRepositoryReader{},
			historyGitAdapter{},
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
