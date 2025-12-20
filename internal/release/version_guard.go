package release

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/git"
	"github.com/nekoman-hq/neko-cli/internal/log"
)

func VersionGuard(cfg *config.NekoConfig) {
	git.Fetch()

	latestTag := git.LatestTag()

	assertVersion(cfg, latestTag)
}

func assertVersion(cfg *config.NekoConfig, latestTag string) {
	localVer, err := semver.NewVersion(cfg.Version)
	if err != nil {
		errors.Fatal(
			"Invalid local version",
			fmt.Sprintf("Version %s in .neko.json is not a valid semantic version", cfg.Version),
			errors.ErrVersionViolation,
		)
	}

	remoteVer, err := semver.NewVersion(latestTag)
	if err != nil {
		errors.Warning("Latest Git tag %s is not a valid semver, skipping comparison", latestTag)
		return
	}

	if localVer.LessThan(remoteVer) {
		errors.Fatal(
			"Version violation",
			fmt.Sprintf("Local version %s is smaller than latest tag %s", localVer, remoteVer),
			errors.ErrVersionViolation,
		)
	}

	log.V(fmt.Sprintf("Local version %s is >= latest tag %s, proceeding.", localVer, remoteVer))
}
