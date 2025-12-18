package cmd

import (
	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/spf13/cobra"
)

var showConfig bool

// checkCmd represents the validate command
var checkCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate or show the Neko configuration",
	Long: `Show or validate the Neko configuration.
You can inspect your current .neko.json or run validations to ensure it is correct.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.LoadConfig()

		if showConfig {
			println("\nCurrent Neko configuration:\n")
			println("  • Project type:   " + cfg.ProjectType)
			println("  • Release system: " + cfg.ReleaseSystem)
			println("  • Version:        " + cfg.Version)
		}

		config.Validate(cfg)
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().BoolVar(&showConfig, "config-show", false, "Display current configuration")
}
