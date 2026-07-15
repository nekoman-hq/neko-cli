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
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type validateCommandHandler struct {
	query validationQuerier
	clock validationResponseClock
}

// HandleValidate validates the release configuration.
func HandleValidate(req plugin.Request) (*plugin.Response, error) {
	handler := validateCommandHandler{
		query: newValidationQueryUseCase(
			validationReleaseRepositoryReader{},
			legacyReleaseRequirementsValidator{},
		),
		clock: systemValidationResponseClock{},
	}
	return handler.Handle(req)
}

func (handler validateCommandHandler) Handle(req plugin.Request) (*plugin.Response, error) {
	log.PluginPrint(log.Config, "Validating release configuration")

	result, failure := handler.query.Query(parseValidationQueryRequest(req.Flags))
	if result.SourceFormat == config.SourceFormatV2 {
		log.PluginPrint(log.Config, "V2 release configuration is valid")
	} else if result.SourceFormat == config.SourceFormatV1 && failure == nil {
		log.PluginPrint(log.Config, "Configuration is valid")
	}

	return mapValidationQueryResponse(result, failure, handler.clock.Now()), nil
}
