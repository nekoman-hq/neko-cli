package contributors

import (
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

type contributorsCommandHandler struct {
	query contributorsQuerier
	clock contributorsResponseClock
}

func HandleContributors(req plugin.Request) (*plugin.Response, error) {
	handler := contributorsCommandHandler{
		query: newContributorsQueryUseCase(
			contributorsReleaseRepositoryReader{},
			contributorsGitAdapter{},
		),
		clock: systemContributorsResponseClock{},
	}
	return handler.Handle(req)
}

func (handler contributorsCommandHandler) Handle(req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Exec, "Collecting contributors")

	result, failure := handler.query.Query(parseContributorsQueryRequest(req.Flags))
	if failure == nil {
		log.PluginPrint(log.Exec, "Successfully collected contributors")
	}

	return mapContributorsQueryResponse(result, failure, handler.clock.Now()), nil
}
