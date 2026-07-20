// Package init includes the init handlers for plugin-based execution.
package init

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

// HandleInit handles the init command in plugin mode.
func HandleInit(req plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleInitAt(root, req)
}

// HandleInitAt handles the init command at an explicit repository root without
// changing process cwd.
func HandleInitAt(root workspace.RepositoryRoot, req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Init, "Starting release initialization")
	repository := newV2Repository(root.Path())
	useCase := initializeV2RepositoryUseCase{
		presenceReader: repository,
		validator:      repository,
		writer:         repository,
	}
	result, failure := useCase.Execute(parseInitCommandRequest(req.Flags))
	response := mapInitializeV2Response(result, failure, time.Now())
	if failure != nil {
		if failure.origin == commandFailureFromPresencePolicy {
			log.PluginV(log.Init, "Existing release configuration prevents init: %s", failure.message)
		}
		return response, nil
	}

	log.PluginPrint(log.Init, "Configuration saved to %s", config.V2ConfigPath("."))
	log.PluginPrint(log.Init, "State saved to %s", config.V2StatePath("."))
	log.PluginPrint(log.Init, "Initialization completed successfully")
	return response, nil
}
