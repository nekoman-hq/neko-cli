package contributors

import (
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

type contributorsCommandHandler struct {
	query contributorsQuerier
	clock contributorsResponseClock
}

func HandleContributors(req plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleContributorsAt(root, req)
}

// HandleContributorsAt returns contributors at an explicit repository root
// without changing process cwd.
func HandleContributorsAt(root workspace.RepositoryRoot, req plugin.Request) (*plugin.Response, error) {
	handler := contributorsCommandHandler{
		query: newContributorsQueryUseCaseAt(
			root.Path(),
			contributorsReleaseRepositoryReader{},
			contributorsGitAdapter{repositoryRoot: root.Path()},
		),
		clock: systemContributorsResponseClock{},
	}
	return handler.Handle(req)
}

func (handler contributorsCommandHandler) Handle(req plugin.Request) (*plugin.Response, error) {
	result, failure := handler.query.Query(parseContributorsQueryRequest(req.Flags))
	return mapContributorsQueryResponse(result, failure, handler.clock.Now()), nil
}
