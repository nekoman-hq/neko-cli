package release

import (
	"context"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

const releaseContextValidationCommandName = "ci-validate-context"

type releaseContextValidator interface {
	Validate(context.Context, ReleaseContextValidationRequest) (*ValidatedReleaseContext, *CommandFailure)
}

type releaseContextValidationCommandHandler struct {
	validator releaseContextValidator
	clock     ReleaseClock
	root      workspace.RepositoryRoot
}

// ParseReleaseContextValidationRequest isolates the plugin transport's
// untyped flag map and supplies the already validated explicit repository root.
func ParseReleaseContextValidationRequest(root workspace.RepositoryRoot, request plugin.Request) ReleaseContextValidationRequest {
	return ReleaseContextValidationRequest{
		RepositoryRoot: root.Path(),
		UnitID:         commandFlagString(request.Flags, "unit"),
		Version:        commandFlagString(request.Flags, "version"),
		Tag:            commandFlagString(request.Flags, "tag"),
		ReleaseSHA:     commandFlagString(request.Flags, "release-sha"),
	}
}

func (handler releaseContextValidationCommandHandler) Handle(ctx context.Context, request plugin.Request) (*plugin.Response, error) {
	typedRequest := ParseReleaseContextValidationRequest(handler.root, request)
	result, failure := handler.validator.Validate(ctx, typedRequest)
	timestamp := handler.clock.Now()
	if failure != nil {
		response := MapCommandFailure(releaseContextValidationCommandName, failure, timestamp)
		response.ExitCode = 1
		return response, nil
	}
	return MapValidatedReleaseContext(result, timestamp), nil
}

// HandleReleaseContextValidation resolves the repository root from the request
// context and validates one dispatched Release V2 context without mutation.
func HandleReleaseContextValidation(request plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(request.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleReleaseContextValidationAt(root, request)
}

// HandleReleaseContextValidationAt validates one dispatched Release V2 context
// against an explicit repository root without changing process cwd.
func HandleReleaseContextValidationAt(root workspace.RepositoryRoot, request plugin.Request) (*plugin.Response, error) {
	handler := releaseContextValidationCommandHandler{
		validator: newReleaseContextValidationUseCase(),
		clock:     systemReleaseClock{},
		root:      root,
	}
	return handler.Handle(context.Background(), request)
}
