package cmd

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

import (
	"github.com/spf13/cobra"

	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/release"
)

// releaseCmd represents the release command
var releaseCmd = &cobra.Command{
	Use:       "release [type]",
	Short:     "Create a new release for your project",
	ValidArgs: []string{"major", "minor", "patch"},
	Args:      cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			errors.Fatal(
				"Loading Configuration Failed",
				err.Error(),
				errors.ErrConfigExists,
			)
		}

		service := release.NewReleaseService(cfg)

		if err := service.Run(args); err != nil {
			errors.Fatal(
				"Release failed",
				err.Error(),
				errors.ErrReleaseFailed,
			)
		}
	},
}

func init() {
	rootCmd.AddCommand(releaseCmd)
}
