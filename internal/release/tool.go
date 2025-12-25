package release

import (
	"fmt"
	"os/exec"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/log"
)

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

type Tool interface {
	Name() string
	Init(v *semver.Version, cfg *config.NekoConfig) error
	Release(v *semver.Version) error
	Survey(v *semver.Version) (Type, error)
	SupportsSurvey() bool
}

type ToolBase struct{}

func (tb *ToolBase) RequireBinary(name string) {
	log.V(log.Init,
		fmt.Sprintf("Searching for %s executable: %s",
			name,
			log.ColorText(log.ColorGreen, fmt.Sprintf("which %s", name)),
		),
	)

	path, err := exec.LookPath(name)
	if err != nil {
		errors.Fatal(
			"Required dependency missing",
			fmt.Sprintf(
				"%s is not installed or not available in PATH",
				name,
			),
			errors.ErrDependencyMissing,
		)
	}

	log.Print(
		log.Init,
		fmt.Sprintf(
			"\uF00C Found %s at %s",
			log.ColorText(log.ColorCyan, name),
			log.ColorText(log.ColorGreen, path),
		),
	)
}
