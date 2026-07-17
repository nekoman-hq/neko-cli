package migrate

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

type migrationCommandUseCase interface {
	Migrate(request migrationCommandRequest) (migrationCommandResult, *migrationFailure)
}

type migrationCommandHandler struct {
	useCase migrationCommandUseCase
	now     func() time.Time
}

// HandleMigrate handles the V1-to-V2 migration command.
func HandleMigrate(req plugin.Request) (*plugin.Response, error) {
	handler := migrationCommandHandler{
		useCase: newMigrationUseCase(),
		now:     time.Now,
	}
	return handler.Handle(req)
}

// HandleMigrateAt handles the V1-to-V2 migration command at an explicit
// migration root without changing process cwd.
func HandleMigrateAt(root workspace.RepositoryRoot, req plugin.Request) (*plugin.Response, error) {
	req.Context.WorkingDir = root.Path()
	handler := migrationCommandHandler{
		useCase: newMigrationUseCaseAt(root.Path()),
		now:     time.Now,
	}
	return handler.Handle(req)
}

func (handler migrationCommandHandler) Handle(req plugin.Request) (*plugin.Response, error) {
	request := parseMigrationCommandRequest(req.Flags, req.Context.WorkingDir)
	result, failure := handler.useCase.Migrate(request)
	return mapMigrationCommandResponse(result, failure, handler.now()), nil
}
