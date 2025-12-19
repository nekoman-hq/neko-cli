package cmd

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

import (
	"fmt"

	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/release"
	"github.com/spf13/cobra"
)

// releaseCmd represents the release command
var releaseCmd = &cobra.Command{
	Use:       "release [type]",
	Short:     "Create a new release for your project",
	ValidArgs: []string{"major", "minor", "patch"},
	Args:      cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		cfg := config.LoadConfig()
		config.Validate(cfg)
		_ = config.GetPAT()

		tool, err := release.Get(string(cfg.ReleaseSystem))
		if err != nil {
			errors.Fatal(
				"Release System Not Found",
				err.Error(),
				errors.ErrInvalidReleaseSystem,
			)
			return
		}

		var rt release.Type

		if len(args) > 0 {
			rt, err = release.ParseReleaseType(args[0])
			if err != nil {
				errors.Fatal(
					"Invalid Release Type",
					err.Error(),
					errors.ErrInvalidReleaseType,
				)
				return
			}
		} else {
			if !tool.SupportsSurvey() {
				errors.Fatal(
					"Interactive mode not supported",
					fmt.Sprintf("%s requires an explicit release type", tool.Name()),
					errors.ErrSurveyFailed,
				)
				return
			}

			fmt.Printf("Found Release System: %s\n", tool.Name())

			rt, err = tool.Survey()
			if err != nil {
				errors.Fatal(
					"Survey cancelled",
					err.Error(),
					errors.ErrSurveyFailed,
				)
				return
			}
		}

		if err := tool.Release(rt); err != nil {
			errors.Fatal(
				"Release Failed",
				err.Error(),
				errors.ErrReleaseFailed,
			)
		}
	},
}

func init() {
	rootCmd.AddCommand(releaseCmd)
}
