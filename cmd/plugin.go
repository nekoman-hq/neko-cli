package cmd

/*
This is the plugin command implementation for the neko-cli.

@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      03.02.2026
*/

import (
	"fmt"
	"os"

	"github.com/nekoman-hq/neko-cli/pkg/dispatcher"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/spf13/cobra"
)

// pluginCmd is the root command for plugin management operations.
// It provides subcommands for listing, installing, and uninstalling plugins.
var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage neko plugins",
}

// pluginListCmd lists all installed plugins with their metadata.
var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	RunE:  runPluginList,
}

// pluginAvailableCmd displays all plugins available in the registry,
// showing their installation status and available updates.
var pluginAvailableCmd = &cobra.Command{
	Use:   "available",
	Short: "List available plugins from the registry",
	RunE:  runPluginAvailable,
}

// pluginInstallCmd installs a plugin from the registry.
// Accepts an optional --version flag to install a specific version.
var pluginInstallCmd = &cobra.Command{
	Use:   "install [plugin-name]",
	Short: "Install a plugin from the registry",
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginInstall,
}

// pluginUninstallCmd removes an installed plugin from the system.
var pluginUninstallCmd = &cobra.Command{
	Use:   "uninstall [plugin-name]",
	Short: "Uninstall a plugin",
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginUninstall,
}

var (
	// installVersion specifies which version to install.
	// Defaults to "latest" if not specified.
	installVersion string
)

func init() {
	rootCmd.AddCommand(pluginCmd)
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginAvailableCmd)
	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginUninstallCmd)

	pluginInstallCmd.Flags().StringVar(&installVersion, "version", "latest", "Version to install")
}

// runPluginList executes the 'plugin list' command.
// It displays all installed plugins in a formatted table with name, version,
// description, and author information.
//
// Returns an error if the plugin directory cannot be read, or nil on success.
func runPluginList(*cobra.Command, []string) error {
	d := dispatcher.NewDispatcher(pluginDir)

	manifests, err := d.ListPlugins()
	if err != nil {
		if os.IsNotExist(err) {
			printEmptyPluginList()
			return nil
		}
		return fmt.Errorf("failed to list plugins: %w", err)
	}

	if len(manifests) == 0 {
		printEmptyPluginList()
		return nil
	}

	// Print header
	printTableHeader("NAME", "VERSION", "DESCRIPTION", "AUTHOR")
	printTableSeparator()

	// Print rows
	for _, m := range manifests {
		printPluginRow(m.Name, m.Version, plugin.Truncate(m.Description, 40), m.Author)
	}

	fmt.Println()
	fmt.Printf("  %s%d plugin(s) installed%s\n", log.ColorBrightBlack, len(manifests), log.ColorReset)

	return nil
}

// runPluginAvailable executes the 'plugin available' command.
// It fetches and displays all plugins from the registry, comparing them
// with locally installed versions to show installation status and available updates.
//
// Returns an error if the registry cannot be reached, or nil on success.
func runPluginAvailable(*cobra.Command, []string) error {
	manager := plugin.NewManager(pluginDir)

	plugins, err := manager.GetAvailablePlugins()
	if err != nil {
		return fmt.Errorf("failed to fetch available plugins: %w", err)
	}

	if len(plugins) == 0 {
		fmt.Printf("%sNo plugins available.%s\n", log.ColorBrightYellow, log.ColorReset)
		return nil
	}

	// Print header
	printAvailableHeader("NAME", "LATEST VERSION", "STATUS")
	printTableSeparator()

	// Get installed plugins for comparison
	installedMap, _ := manager.ListInstalled()

	for _, p := range plugins {
		status := "not installed"
		statusColor := log.ColorBrightBlack
		if v, ok := installedMap[p.Name]; ok {
			if v == p.Version {
				status = "✓ installed"
				statusColor = log.ColorGreen
			} else {
				status = fmt.Sprintf("↑ update available (%s)", v)
				statusColor = log.ColorBrightYellow
			}
		}
		printAvailableRow(p.Name, p.Version, status, statusColor)
	}

	fmt.Println()
	fmt.Printf("  %s%d plugin(s) available%s\n", log.ColorBrightBlack, len(plugins), log.ColorReset)

	return nil
}

// runPluginInstall executes the 'plugin install' command.
// It downloads and installs the specified plugin from the registry.
// The version can be specified using the --version flag; defaults to "latest".
//
// Args:
//   - args[0]: The name of the plugin to install
//
// Returns an error if installation fails, or nil on success.
func runPluginInstall(_ *cobra.Command, args []string) error {
	pluginName := args[0]

	fmt.Printf("%s%sInstalling plugin '%s'%s", log.ColorCyan, log.ColorBold, pluginName, log.ColorReset)
	if installVersion != "latest" {
		fmt.Printf(" metadata %s", installVersion)
	}
	fmt.Println("...")

	manager := plugin.NewManager(pluginDir)
	if err := manager.Install(pluginName, installVersion); err != nil {
		return err
	}

	fmt.Printf("%s%s✓%s Plugin '%s' installed successfully!\n", log.ColorGreen, log.ColorBold, log.ColorReset, pluginName)
	return nil
}

// runPluginUninstall executes the 'plugin uninstall' command.
// It removes the specified plugin and all its associated files from the system.
//
// Args:
//   - args[0]: The name of the plugin to uninstall
//
// Returns an error if uninstallation fails, or nil on success.
func runPluginUninstall(_ *cobra.Command, args []string) error {
	pluginName := args[0]

	manager := plugin.NewManager(pluginDir)
	if err := manager.Uninstall(pluginName); err != nil {
		return err
	}

	fmt.Printf("%s%s✓%s Plugin '%s' uninstalled successfully!\n", log.ColorGreen, log.ColorBold, log.ColorReset, pluginName)
	return nil
}

// GetInstalledPluginManifest retrieves the manifest file for a specific installed plugin.
// This function is exported and can be used by other packages to access plugin metadata.
//
// Args:
//   - pluginName: The name of the plugin to retrieve the manifest for
//
// Returns:
//   - The plugin manifest containing metadata like name, version, description, etc.
//   - An error if the plugin is not installed or the manifest cannot be read
func GetInstalledPluginManifest(pluginName string) (*plugin.Manifest, error) {
	manager := plugin.NewManager(pluginDir)
	return manager.GetManifest(pluginName)
}

// Helper functions for styled output

// printEmptyPluginList displays a message when no plugins are installed,
// with a hint to check available plugins.
func printEmptyPluginList() {
	fmt.Printf("%sNo plugins installed.%s\n", log.ColorBrightYellow, log.ColorReset)
	fmt.Printf("%sUse 'neko plugin available' to see available plugins.%s\n", log.ColorBrightBlack, log.ColorReset)
}

// printTableHeader prints the column headers for the plugin list table.
func printTableHeader(name, version, desc, author string) {
	fmt.Println()
	fmt.Printf("  %s%s%-15s %-10s %-40s %s%s\n",
		log.ColorCyan, log.ColorBold, name, version, desc, author, log.ColorReset)
}

// printAvailableHeader prints the column headers for the available plugins table.
func printAvailableHeader(name, version, status string) {
	fmt.Println()
	fmt.Printf("  %s%s%-15s %-15s %s%s\n",
		log.ColorCyan, log.ColorBold, name, version, status, log.ColorReset)
}

// printTableSeparator prints a separator line for table formatting.
func printTableSeparator() {
	log.PrintTableSeparator()
}

// printPluginRow prints a formatted row of plugin information in the list table.
func printPluginRow(name, version, desc, author string) {
	fmt.Printf("  %s%s%-15s%s %s%-10s%s %-40s %s%s%s\n",
		log.ColorBrightWhite, log.ColorBold, name, log.ColorReset,
		log.ColorBrightYellow, version, log.ColorReset,
		desc,
		log.ColorPurple, author, log.ColorReset)
}

// printAvailableRow prints a formatted row showing an available plugin's information
// with its installation status.
func printAvailableRow(name, version, status, statusColor string) {
	fmt.Printf("  %s%s%-15s%s %s%-15s%s %s%s%s\n",
		log.ColorBrightWhite, log.ColorBold, name, log.ColorReset,
		log.ColorBrightYellow, version, log.ColorReset,
		statusColor, status, log.ColorReset)
}
