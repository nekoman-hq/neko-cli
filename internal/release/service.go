package release

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"fmt"

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

	rt, err := ResolveReleaseType(args, releaser, version)
	if err != nil {
		errors.Fatal(
			"Invalid Release Type",
			err.Error(),
			errors.ErrInvalidReleaseType,
		)
	}

	log.Print(log.VersionGuard, fmt.Sprintf("\uF00C All checks have succeeded. %s", log.ColorText(log.ColorGreen, "Starting release now!")))
	return releaser.Release(rt)
}
