//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package release

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/git"
)

func Preflight(cfg *config.V1ReleaseConfig) {
	failure := checkV1ReleasePreflight(cfg)
	if failure != nil {
		errors.WriteError(failure.Code, failure.Cause.Error())
	}
}

func checkV1ReleasePreflight(cfg *config.V1ReleaseConfig) *V1ReleaseFailure {
	log.PluginV(log.Preflight, "Running pre-flight checks")

	if err := ValidateRequirements(cfg); err != nil {
		return newFatalV1ReleaseFailure("RELEASE_REQUIREMENTS_INVALID", err)
	}

	if err := git.IsClean(); err != nil {
		return newFatalV1ReleaseFailure("UNCOMMITTED_CHANGES", err)
	}

	if err := git.EnsureNotDetached(); err != nil {
		return newFatalV1ReleaseFailure("DETACHED_HEAD", err)
	}

	if err := git.OnMainBranch(); err != nil {
		return newFatalV1ReleaseFailure("INCORRECT_BRANCH", err)
	}

	if err := git.HasUpstream(); err != nil {
		return newFatalV1ReleaseFailure("NO_UPSTREAM_BRANCH", err)
	}

	if err := git.IsUpToDate(); err != nil {
		return newFatalV1ReleaseFailure("BRANCH_OUT_OF_DATE", err)
	}

	log.PluginV(log.Preflight, "\uF00C Preflight checks succeeded!")
	return nil
}
