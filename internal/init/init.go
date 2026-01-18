// Package init includes the init wizard of each release system
package init

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      23.12.2025
*/

import (
	"errors"
	"fmt"

	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/git"
	"github.com/nekoman-hq/neko-cli/internal/release"
)

func Run(info *git.RepoInfo) error {
	if !confirmOverwriteIfExists() {
		return nil
	}

	cfg := runWizard()

	if info != nil {
		cfg.ProjectOwner = info.Owner
		cfg.ProjectName = info.Repo
	}

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf(
			"Configuration write failed: %w", err,
		)
	}

	releaser, err := release.Get(string(cfg.ReleaseSystem))
	if err != nil {
		return fmt.Errorf(
			"Release System Not Found: %w", err,
		)
	}

	err = releaser.Init(&cfg)
	if err != nil {
		return errors.New(
			"Release system initialization failed",
		)
	}

	printSetupInstructions(cfg)

	return nil
}
