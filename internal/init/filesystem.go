package init

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      23.12.2025
*/

import (
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/nekoman-hq/neko-cli/internal/errors"
)

func confirmOverwriteIfExists() bool {
	if _, err := os.Stat(".neko.json"); err != nil {
		return true
	}

	var overwrite bool
	confirm := &survey.Confirm{
		Message: ".neko.json already exists. Overwrite it?",
		Default: false,
	}

	if err := survey.AskOne(confirm, &overwrite); err != nil {
		errors.Warning(
			"Initialization cancelled",
			"Configuration wizard was aborted by the user.",
		)
		return false
	}

	if !overwrite {
		errors.Warning(
			"Initialization aborted",
			"Existing .neko.json was not overwritten.",
		)
		return false
	}

	return true
}
