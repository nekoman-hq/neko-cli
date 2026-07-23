// Package validate includes the validate command handler
//
//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package validate

//lint:file-ignore SA1019 V1 validation compatibility intentionally uses deprecated V1 APIs during migration

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      03.02.2026
*/

import (
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

type validateCommandHandler struct {
	query validationQuerier
	clock validationResponseClock
}

// HandleValidate validates the release configuration.
func HandleValidate(req plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleValidateAt(root, req)
}

// HandleValidateAt validates the release configuration at an explicit
// repository root without changing process cwd.
func HandleValidateAt(root workspace.RepositoryRoot, req plugin.Request) (*plugin.Response, error) {
	handler := validateCommandHandler{
		query: newValidationQueryUseCaseAt(
			root.Path(),
			validationReleaseRepositoryReader{},
			legacyReleaseRequirementsValidator{repositoryRoot: root.Path()},
		),
		clock: systemValidationResponseClock{},
	}
	return handler.Handle(req)
}

func (handler validateCommandHandler) Handle(req plugin.Request) (*plugin.Response, error) {
	result, failure := handler.query.Query(parseValidationQueryRequest(req.Flags))
	return mapValidationQueryResponse(result, failure, handler.clock.Now()), nil
}
