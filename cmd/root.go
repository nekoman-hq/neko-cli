package cmd

/*
This is the plugin command implementation for the neko-cli.

@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      03.02.2026 *(updated)*
*/

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose          bool
	outputFormat     string
	githubOutputFile string
	PluginDir        string
	describe         bool
)

var rootCmd = &cobra.Command{
	Use:   "neko",
	Short: "Neko CLI - Plugin-based release and deployment tool",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Include execution and debug logs in plugin output")

	// Load plugins during initialization
	if err := InitializePlugins(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: Failed to initialize plugins: %v\n", err)
	}
}
