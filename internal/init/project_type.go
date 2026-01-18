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

func askProjectType(cfg *config.NekoConfig) {
	var input string

	err := survey.AskOne(&survey.Select{
		Message: "What kind of project is this?",
		Options: []string{
			string(config.ProjectTypeFrontend),
			string(config.ProjectTypeBackend),
			string(config.ProjectTypeOther),
		},
	}, &input)
	if err != nil {
		errors.Error(
			"Project type selection failed",
			"Could not read project type input.",
			errors.ErrSurveyFailed,
		)
		return
	}

	cfg.ProjectType = config.ProjectType(input)
	if !cfg.ProjectType.IsValid() {
		errors.Error(
			"Invalid project type",
			"Selected project type is not supported.",
			errors.ErrConfigMarshal,
		)
		return
	}
}
