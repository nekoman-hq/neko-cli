package release

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/git"
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
	VersionGuard(rs.cfg)

	releaser, err := Get(string(rs.cfg.ReleaseSystem))
	if err != nil {
		errors.Fatal(
			"Release System Not Found",
			err.Error(),
			errors.ErrInvalidReleaseSystem,
		)
	}

	rt, err := ResolveReleaseType(args, releaser)
	if err != nil {
		errors.Fatal(
			"Invalid Release Type",
			err.Error(),
			errors.ErrInvalidReleaseType,
		)
	}

	// Later:
	// VersionHandling(...) - global version check - to ensure neko.json is the single source of truth
	// Finally - Execute release based on given tool

	return releaser.Release(rt)
}
