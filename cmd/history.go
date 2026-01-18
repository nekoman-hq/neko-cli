package cmd

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      29.12.2025
*/

import (
	"github.com/spf13/cobra"

	"github.com/nekoman-hq/neko-cli/internal/history"
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show repository history and statistics",
	Long:  `Display a formatted overview of your repository's history including branch, commits, tags, and contributors.`,
	Run: func(cmd *cobra.Command, args []string) {
		history.ShowHistory()
	},
}

func init() {
	rootCmd.AddCommand(historyCmd)
}
