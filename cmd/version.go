package cmd

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      17.12.2025
*/

import (
	github "github.com/nekoman-hq/neko-cli/pkg/git"
	"github.com/nekoman-hq/neko-cli/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the installed CLI build and the newest stable CLI release",
	RunE: func(cmd *cobra.Command, args []string) error {
		// The configured CLI repository, not the current working repository, owns
		// CLI release discovery. `neko update` resolves the same repository through
		// the same resolver, so both commands report the same CLI release.
		return version.Latest(cmd.Context(), github.CLIRepository())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
