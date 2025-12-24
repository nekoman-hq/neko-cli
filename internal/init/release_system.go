package init

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
)

func askReleaseSystem(cfg *config.NekoConfig) {
	var input string

	err := survey.AskOne(&survey.Select{
		Message: "Which release system should be used?",
		Options: releaseOptionsFor(cfg.ProjectType),
	}, &input)

	if err != nil {
		errors.Error(
			"Release system selection failed",
			"Could not read release system input.",
			errors.ErrSurveyFailed,
		)
		return
	}

	cfg.ReleaseSystem = config.ReleaseSystem(input)
	if !cfg.ReleaseSystem.IsValid() {
		errors.Error(
			"Invalid release system",
			"Selected release system is not supported.",
			errors.ErrConfigMarshal,
		)
		return
	}

	return
}
