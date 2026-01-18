package cmd

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      17.12.2025
*/

import (
	"github.com/nekoman-hq/neko-cli/internal/git"
	"github.com/nekoman-hq/neko-cli/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show current version of this repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoInfo, _ := git.Current()
		err := version.Latest(repoInfo)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
