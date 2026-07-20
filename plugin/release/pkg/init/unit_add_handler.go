package init

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

// HandleUnitAdd appends one unit to an existing V2 release configuration.
func HandleUnitAdd(req plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleUnitAddAt(root, req)
}

// HandleUnitAddAt appends one unit at an explicit repository root without
// changing process cwd.
func HandleUnitAddAt(root workspace.RepositoryRoot, req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Init, "Starting release unit append")
	repository := newV2Repository(root.Path())
	useCase := addV2ReleaseUnitUseCase{
		presenceReader: repository,
		loader:         repository,
		validator:      repository,
		writer:         repository,
	}
	result, failure := useCase.Execute(parseUnitAddCommandRequest(req.Flags))
	response := mapAddV2UnitResponse(result, failure, time.Now())
	if failure != nil {
		if failure.origin == commandFailureFromPresencePolicy {
			log.PluginV(log.Init, "Existing release configuration prevents unit-add: %s", failure.message)
		}
		return response, nil
	}

	log.PluginPrint(log.Init, "Release unit %s appended to %s", result.Unit.UnitID, config.V2ConfigPath("."))
	log.PluginPrint(log.Init, "State entry saved to %s", config.V2StatePath("."))
	return response, nil
}
