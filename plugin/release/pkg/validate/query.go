//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package validate

//lint:file-ignore SA1019 V1 validation compatibility intentionally uses deprecated V1 APIs during migration

import (
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

const missingConfigurationHint = "Run 'neko release init' to create V2 config/state, or 'neko release migrate' to convert an existing V1 config"

type validationQueryRequest struct {
	Unit string
	Show bool
}

type legacyValidationDetails struct {
	ProjectName   string
	ProjectOwner  string
	ProjectType   string
	ReleaseSystem string
	Version       string
	UnitID        string
}

//nolint:govet // Logical query-result order keeps source and mode ahead of format-specific data.
type validationQueryResult struct {
	SourceFormat config.SourceFormat
	Legacy       legacyValidationDetails
	Units        []config.ReleaseUnit
	Show         bool
}

type validationQueryFailure struct {
	Cause   error
	Code    string
	Message string
	Hint    string
}

type validationQuerier interface {
	Query(validationQueryRequest) (validationQueryResult, *validationQueryFailure)
}

type validationRepositoryReader interface {
	Read(string) (*config.ReleaseRepository, bool, error)
}

type legacyRequirementsValidator interface {
	Validate(*config.V1ReleaseConfig) error
}

type validationQueryUseCase struct {
	repositoryRoot string
	repositories   validationRepositoryReader
	requirements   legacyRequirementsValidator
}

func newValidationQueryUseCase(repositories validationRepositoryReader, requirements legacyRequirementsValidator) validationQueryUseCase {
	return newValidationQueryUseCaseAt(".", repositories, requirements)
}

func newValidationQueryUseCaseAt(root string, repositories validationRepositoryReader, requirements legacyRequirementsValidator) validationQueryUseCase {
	return validationQueryUseCase{repositoryRoot: root, repositories: repositories, requirements: requirements}
}

func (useCase validationQueryUseCase) Query(request validationQueryRequest) (validationQueryResult, *validationQueryFailure) {
	repository, configurationPresent, err := useCase.repositories.Read(useCase.repositoryRoot)
	if err != nil {
		if configurationPresent {
			return validationQueryResult{}, validationFailure("CONFIG_INVALID", err)
		}
		return validationQueryResult{}, &validationQueryFailure{
			Code:    "CONFIG_NOT_FOUND",
			Message: "No release configuration found",
			Hint:    missingConfigurationHint,
		}
	}
	if repository.SourceFormat == config.SourceFormatV2 {
		return queryV2Validation(repository, request)
	}
	return useCase.queryV1Validation(repository, request)
}

func queryV2Validation(repository *config.ReleaseRepository, request validationQueryRequest) (validationQueryResult, *validationQueryFailure) {
	result := validationQueryResult{SourceFormat: config.SourceFormatV2, Show: request.Show}
	units := repository.Units
	if request.Unit != "" {
		unit, err := config.ResolveReleaseUnit(repository, request.Unit, config.UnitResolutionOptions{})
		if err != nil {
			return result, validationFailure("UNIT_RESOLUTION_FAILED", err)
		}
		units = []config.ReleaseUnit{*unit}
	}
	result.Units = cloneValidationUnits(units)
	return result, nil
}

func cloneValidationUnits(units []config.ReleaseUnit) []config.ReleaseUnit {
	cloned := make([]config.ReleaseUnit, len(units))
	for i, unit := range units {
		cloned[i] = unit
		cloned[i].Paths = append([]string(nil), unit.Paths...)
	}
	return cloned
}

func (useCase validationQueryUseCase) queryV1Validation(repository *config.ReleaseRepository, request validationQueryRequest) (validationQueryResult, *validationQueryFailure) {
	result := validationQueryResult{SourceFormat: config.SourceFormatV1, Show: request.Show}
	unit, err := config.ResolveReleaseUnit(repository, request.Unit, config.UnitResolutionOptions{})
	if err != nil {
		return result, validationFailure("UNIT_RESOLUTION_FAILED", err)
	}
	legacy := repository.Legacy
	if err := config.V1Validate(legacy); err != nil {
		return result, validationFailure("VALIDATION_FAILED", err)
	}
	if err := useCase.requirements.Validate(legacy); err != nil {
		return result, validationFailure("VALIDATION_FAILED", err)
	}
	result.Legacy = legacyValidationDetails{
		ProjectName:   legacy.ProjectName,
		ProjectOwner:  legacy.ProjectOwner,
		ProjectType:   string(legacy.ProjectType),
		ReleaseSystem: string(legacy.ReleaseSystem),
		Version:       legacy.Version,
		UnitID:        unit.ID,
	}
	return result, nil
}

func validationFailure(code string, err error) *validationQueryFailure {
	return &validationQueryFailure{Cause: err, Code: code, Message: err.Error()}
}

type validationReleaseRepositoryReader struct{}

func (validationReleaseRepositoryReader) Read(root string) (*config.ReleaseRepository, bool, error) {
	repository, err := config.LoadReleaseRepository(root)
	if err == nil {
		return repository, true, nil
	}
	present := config.V2ConfigExists(root) || config.V1ConfigExistsAt(root)
	return nil, present, err
}

type legacyReleaseRequirementsValidator struct {
	repositoryRoot string
}

func (validator legacyReleaseRequirementsValidator) Validate(cfg *config.V1ReleaseConfig) error {
	return release.ValidateRequirementsAt(validator.repositoryRoot, cfg)
}
