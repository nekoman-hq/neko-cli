package cmd

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      17.12.2025
*/

import (
	"os"

	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/repository"
	"github.com/nekoman-hq/neko-cli/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show current version of this repository",
	RunE: func(cmd *cobra.Command, args []string) error {

		repoInfo, _ := repository.Current()

		token, ok := os.LookupEnv("GITHUB_TOKEN")
		if !ok {
			errors.Fatal(
				"Environment Variable Missing",
				"A Github Access Token (GITHUB_TOKEN) is required.\nSet it with: export GITHUB_TOKEN=your_token_here",
				errors.ErrMissingEnvVar,
			)
		}

		version.Latest(repoInfo, token)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
