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
	version := VersionGuard(rs.cfg)

	releaser, err := Get(string(rs.cfg.ReleaseSystem))
	if err != nil {
		errors.Fatal(
			"Release System Not Found",
			err.Error(),
			errors.ErrInvalidReleaseSystem,
		)
	}

	log.Print(log.Release,
		fmt.Sprintf("Release system detected: %s",
			log.ColorText(log.ColorPurple, releaser.Name()),
		),
	)

	log.Print(log.Release,
		fmt.Sprintf("Latest version tag extracted successfully \uF178 %s",
			log.ColorText(log.ColorCyan, version.String()),
		),
	)

	rt, err := ResolveReleaseType(version, args, releaser)
	if err != nil {
		errors.Fatal(
			"Invalid Release Type",
			err.Error(),
			errors.ErrInvalidReleaseType,
		)
	}

	log.Print(log.VersionGuard, fmt.Sprintf("\uF00C All checks have succeeded. %s", log.ColorText(log.ColorGreen, "Starting release now!")))

	newVersion := NextVersion(version, rt)

	if err := rs.updateConfig(&newVersion); err != nil {
		errors.Warning(
			"Failed to update local config",
			fmt.Sprintf("Updating version in .neko.json failed. Attempting to proceed with release: %s", err.Error()))
	}

	if err := releaser.Release(&newVersion, rt); err != nil {
		errors.Fatal(
			"Release failed",
			err.Error(),
			errors.ErrReleaseFailed,
		)
	}

	log.Print(log.Release, fmt.Sprintf("\uF00C Successfully released version %s",
		log.ColorText(log.ColorCyan, newVersion.String())))

	return nil
}

func (rs *Service) updateConfig(newVersion *semver.Version) error {
	rs.cfg.Version = newVersion.String()
	return config.SaveConfig(*rs.cfg)
}
