package cmd

import (
	"github.com/nekoman-hq/neko-cli/pkg/update"
	"github.com/spf13/cobra"
)

// Update flags - declared ONCE in cmd package
var (
	updateForce  bool
	updateDryRun bool
	updateAll    bool
)

// updateCmd represents the main update command for core tool
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update neko-cli core tool",
	Long: `Update the neko-cli core tool to the latest version.
	
Example:
  neko update              # Update to latest version
  neko update --dry-run    # Check for updates without installing
  neko update --force      # Force update even if already on latest`,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := update.CoreOptions{
			Force:  updateForce,
			DryRun: updateDryRun,
		}
		cmd.SilenceUsage = true // Don't show usage on error
		return update.Core(opts)
	},
}

// pluginUpdateCmd represents the plugin update command
var pluginUpdateCmd = &cobra.Command{
	Use:   "update [plugin-name]",
	Short: "Update a specific plugin or all plugins",
	Long: `Update one or more plugins to their latest versions.
	
Examples:
  neko plugin update release       # Update specific plugin
  neko plugin update --all         # Update all installed plugins
  neko plugin update --dry-run     # Check for plugin updates without installing
  neko plugin update --force       # Force update even if on latest`,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := update.PluginOptions{
			PluginDir: PluginDir,
			Force:     updateForce,
			DryRun:    updateDryRun,
			All:       updateAll,
		}
		cmd.SilenceUsage = true // Don't show usage on error
		return update.Plugin(args, opts)
	},
}

func init() {
	// Add update command to root
	rootCmd.AddCommand(updateCmd)

	// Add update flags for core
	updateCmd.Flags().BoolVar(&updateForce, "force", false, "Force update even if already on latest version")
	updateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "Check for updates without installing")

	// Add plugin update command to plugin command
	pluginCmd.AddCommand(pluginUpdateCmd)

	// Add plugin update flags
	pluginUpdateCmd.Flags().BoolVar(&updateAll, "all", false, "Update all installed plugins")
	pluginUpdateCmd.Flags().BoolVar(&updateForce, "force", false, "Force update even if already on latest version")
	pluginUpdateCmd.Flags().BoolVar(&updateDryRun, "dry-run", false, "Check for updates without installing")
}
