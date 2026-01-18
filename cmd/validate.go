package cmd

import (
	"fmt"

	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/log"
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
		cfg, err := config.LoadConfig()
		if err != nil {
			errors.Fatal(
				"Loading Configuration failed",
				err.Error(),
				errors.ErrConfig,
			)
		}

		if showConfig {
			println(fmt.Sprintf("\n%s %s\n",
				log.ColorText(log.ColorCyan, "\uF013"),
				log.ColorText(log.ColorBold, "Current Neko configuration:")))

			println(fmt.Sprintf("  %s Project type:   %s",
				log.ColorText(log.ColorCyan, "\uF0C0"),
				log.ColorText(log.ColorYellow, string(cfg.ProjectType))))

			println(fmt.Sprintf("  %s Release system: %s",
				log.ColorText(log.ColorCyan, "\uF1B3"),
				log.ColorText(log.ColorYellow, string(cfg.ReleaseSystem))))

			println(fmt.Sprintf("  %s Version:        %s\n",
				log.ColorText(log.ColorCyan, "\uF02B"),
				log.ColorText(log.ColorGreen, cfg.Version)))
		}
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.Flags().BoolVar(&showConfig, "config-show", false, "Display current configuration")
}
