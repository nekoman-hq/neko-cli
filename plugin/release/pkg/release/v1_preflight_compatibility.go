//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package release

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// Legacy: Preflight preserves the fatal V1 preflight compatibility behavior.
func Preflight(cfg *config.V1ReleaseConfig) {
	failure := checkV1ReleasePreflight(cfg)
	if failure != nil {
		errors.WriteError(failure.Code, failure.Cause.Error())
	}
}

// Legacy: checkV1ReleasePreflight forwards the former package-level check to
// the explicit V1 preflight owner.
func checkV1ReleasePreflight(cfg *config.V1ReleaseConfig) *V1ReleaseFailure {
	root := currentV1RepositoryRoot()
	return legacyV1Preflight{
		requirements: newSystemV1ReleaseRequirements(),
		repository:   systemV1PreflightRepository{},
	}.Check(V1ReleaseIntent{RepositoryRoot: root, Config: cfg})
}
