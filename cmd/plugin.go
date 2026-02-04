package cmd

import (
	"fmt"
	"os"

	"github.com/nekoman-hq/neko-cli/pkg/dispatcher"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage neko plugins",
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	RunE:  runPluginList,
}

var pluginAvailableCmd = &cobra.Command{
	Use:   "available",
	Short: "List available plugins from the registry",
	RunE:  runPluginAvailable,
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install [plugin-name]",
	Short: "Install a plugin from the registry",
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginInstall,
}

var pluginUninstallCmd = &cobra.Command{
	Use:   "uninstall [plugin-name]",
	Short: "Uninstall a plugin",
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginUninstall,
}

var (
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

func runPluginUninstall(_ *cobra.Command, args []string) error {
	pluginName := args[0]

	manager := plugin.NewManager(pluginDir)
	if err := manager.Uninstall(pluginName); err != nil {
		return err
	}

	fmt.Printf("%s%s✓%s Plugin '%s' uninstalled successfully!\n", log.ColorGreen, log.ColorBold, log.ColorReset, pluginName)
	return nil
}

// GetInstalledPluginManifest returns the manifest for an installed plugin
func GetInstalledPluginManifest(pluginName string) (*plugin.Manifest, error) {
	manager := plugin.NewManager(pluginDir)
	return manager.GetManifest(pluginName)
}

// Helper functions for styled output

func printEmptyPluginList() {
	fmt.Printf("%sNo plugins installed.%s\n", log.ColorBrightYellow, log.ColorReset)
	fmt.Printf("%sUse 'neko plugin available' to see available plugins.%s\n", log.ColorBrightBlack, log.ColorReset)
}

func printTableHeader(name, version, desc, author string) {
	fmt.Println()
	fmt.Printf("  %s%s%-15s %-10s %-40s %s%s\n",
		log.ColorCyan, log.ColorBold, name, version, desc, author, log.ColorReset)
}

func printAvailableHeader(name, version, status string) {
	fmt.Println()
	fmt.Printf("  %s%s%-15s %-15s %s%s\n",
		log.ColorCyan, log.ColorBold, name, version, status, log.ColorReset)
}

func printTableSeparator() {
	log.PrintTableSeparator()
}

func printPluginRow(name, version, desc, author string) {
	fmt.Printf("  %s%s%-15s%s %s%-10s%s %-40s %s%s%s\n",
		log.ColorBrightWhite, log.ColorBold, name, log.ColorReset,
		log.ColorBrightYellow, version, log.ColorReset,
		desc,
		log.ColorPurple, author, log.ColorReset)
}

func printAvailableRow(name, version, status, statusColor string) {
	fmt.Printf("  %s%s%-15s%s %s%-15s%s %s%s%s\n",
		log.ColorBrightWhite, log.ColorBold, name, log.ColorReset,
		log.ColorBrightYellow, version, log.ColorReset,
		statusColor, status, log.ColorReset)
}
