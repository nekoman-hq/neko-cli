package init

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      23.12.2025
*/

import (
	"github.com/AlecAivazis/survey/v2"

	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
)

func askInitialVersion(cfg *config.NekoConfig) {
	err := survey.AskOne(&survey.Input{
		Message: "Initial version:",
		Default: "0.1.0",
		Help:    "Semantic Versioning (MAJOR.MINOR.PATCH)",
	}, &cfg.Version)
	if err != nil {
		errors.Error(
			"Version input failed",
			"Could not read version input.",
			errors.ErrSurveyFailed,
		)
		return
	}
}
