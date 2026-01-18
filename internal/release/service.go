// Package release includes all neko cli release logic
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

type Service struct {
	cfg *config.NekoConfig
}

func NewReleaseService(cfg *config.NekoConfig) *Service {
	return &Service{cfg: cfg}
}

func (rs *Service) Run(args []string) error {
	_, _ = git.Current()

	Preflight()
	version, err := VersionGuard(rs.cfg)
	if err != nil {
		return err
	}

	releaser, err := Get(string(rs.cfg.ReleaseSystem))
	if err != nil {
		return fmt.Errorf(
			"release System Not Found: %w", err,
		)
	}

	log.Print(log.Release,
		"Release system detected: %s",
		log.ColorText(log.ColorPurple, releaser.Name()),
	)

	log.Print(log.Release,
		"Latest version tag extracted successfully \uF178 %s",
		log.ColorText(log.ColorCyan, version.String()),
	)

	rt, err := ResolveReleaseType(version, args, releaser)
	if err != nil {
		return fmt.Errorf(
			"invalid Release Type: %w", err,
		)
	}

	log.Print(log.VersionGuard, "\uF00C All checks have succeeded. %s", log.ColorText(log.ColorGreen, "Starting release now!"))

	newVersion := NextVersion(version, rt)

	if err := releaser.Release(&newVersion); err != nil {
		releaseError := fmt.Errorf("release failed: %w", err)

		log.Print(log.VersionGuard, "Encountered error while releasing. Trying to undo changes...")
		if err := releaser.RevertRelease(); err != nil {
			return fmt.Errorf("%w: Failed undoing changes: %w", releaseError, err)
		}
		log.Print(log.VersionGuard, "Successfully undid changes.")

		return releaseError
	}

	if err := rs.updateConfig(&newVersion); err != nil {
		errors.Warning(
			"Failed to update local config",
			fmt.Sprintf("Updating version in .neko.json failed. Attempting to proceed with release: %s", err.Error()))
	}

	log.Print(log.Release, "\uF00C Successfully released version %s",
		log.ColorText(log.ColorCyan, newVersion.String()))

	return nil
}

func (rs *Service) updateConfig(newVersion *semver.Version) error {
	rs.cfg.Version = newVersion.String()
	return config.SaveConfig(*rs.cfg)
}
