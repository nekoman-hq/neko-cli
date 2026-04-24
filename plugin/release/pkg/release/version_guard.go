package release

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/git"

	"github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/pkg/log"
)

func VersionGuard(cfg *config.NekoConfig) (*semver.Version, error) {
	log.PluginV(log.Guard, "Running Version Guard checks")
	git.Fetch()

	latestTag := git.LatestTag()

	return EnsureVersionIsValid(cfg, latestTag)
}

func EnsureVersionIsValid(cfg *config.NekoConfig, latestTag string) (*semver.Version, error) {
	localVer, err := semver.NewVersion(cfg.Version)
	if err != nil {
		return nil, fmt.Errorf(
			"version %s in .release.neko.json is not a valid semantic version", cfg.Version,
		)
	}

	remoteVer, err := semver.NewVersion(latestTag)
	if err != nil {
		errors.WriteWarning(
			"Latest Git tag %s is not a valid semantic version, skipping comparison",
			latestTag,
		)

		log.PluginV(log.Guard,
			fmt.Sprintf("Using local version %s",
				localVer.String(),
			),
		)

		return localVer, nil
	}

	if localVer.LessThan(remoteVer) {
		return nil, fmt.Errorf(
			"version violation: Local version %s is smaller than latest tag %s",
			localVer,
			remoteVer,
		)
	}

	log.PluginV(log.Guard,
		fmt.Sprintf(
			"Local version %s is >= latest tag %s, proceeding.",
			localVer.String(),
			remoteVer.String(),
		),
	)

	return localVer, nil
}
