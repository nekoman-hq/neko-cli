package init

import (
	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
)

func Run() {
	if !confirmOverwriteIfExists() {
		return
	}

	cfg := runWizard()

	if err := config.SaveConfig(cfg); err != nil {
		errors.Fatal(
			"Configuration write failed",
			err.Error(),
			errors.ErrConfigWrite,
		)
		return
	}

	printSetupInstructions(cfg)
}
