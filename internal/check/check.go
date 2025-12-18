package check

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since     17.12.2025
*/

import (
	"regexp"

	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
)

var semverRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[\da-zA-Z-]+(?:\.[\da-zA-Z-]+)*)?(?:\+[\da-zA-Z-]+(?:\.[\da-zA-Z-]+)*)?$`)

func ValidateConfig(cfg *config.NekoConfig) {
	if cfg.ProjectType == "" {
		errors.Error(
			"Invalid configuration",
			"ProjectType is missing in .neko.json",
			errors.ErrConfigMarshal,
		)
	}

	if cfg.ReleaseSystem == "" {
		errors.Error(
			"Invalid configuration",
			"ReleaseSystem is missing in .neko.json",
			errors.ErrConfigMarshal,
		)
	}

	if cfg.Version == "" {
		errors.Error(
			"Invalid configuration",
			"Version is missing in .neko.json",
			errors.ErrConfigMarshal,
		)
	} else if !semverRegex.MatchString(cfg.Version) {
		errors.Error(
			"Invalid configuration",
			"Version is not a valid semantic version (SemVer)",
			errors.ErrInvalidVersion,
		)
	}

	println("\n✓ Configuration appears valid")
}
