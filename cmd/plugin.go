package cmd

import (
	"fmt"
	"os"

	"github.com/nekoman-hq/neko-cli/pkg/dispatcher"
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
			fmt.Println("No plugins installed.")
			fmt.Println("Use 'neko plugin available' to see available plugins.")
			return nil
		}
		return fmt.Errorf("failed to list plugins: %w", err)
	}

	if len(manifests) == 0 {
		fmt.Println("No plugins installed.")
		fmt.Println("Use 'neko plugin available' to see available plugins.")
		return nil
	}

	fmt.Printf("%-15s %-10s %-40s %s\n", "NAME", "VERSION", "DESCRIPTION", "AUTHOR")
	for _, m := range manifests {
		fmt.Printf("%-15s %-10s %-40s %s\n", m.Name, m.Version, plugin.Truncate(m.Description, 40), m.Author)
	}

	return nil
}

func runPluginAvailable(*cobra.Command, []string) error {
	manager := plugin.NewManager(pluginDir)

	plugins, err := manager.GetAvailablePlugins()
	if err != nil {
		return fmt.Errorf("failed to fetch available plugins: %w", err)
	}

	if len(plugins) == 0 {
		fmt.Println("No plugins available.")
		return nil
	}

	fmt.Printf("%-15s %-15s %s\n", "NAME", "LATEST VERSION", "STATUS")

	// Get installed plugins for comparison
	installedMap, _ := manager.ListInstalled()

	for _, p := range plugins {
		status := "not installed"
		if v, ok := installedMap[p.Name]; ok {
			if v == p.Version {
				status = "installed"
			} else {
				status = fmt.Sprintf("installed (%s)", v)
			}
		}
		fmt.Printf("%-15s %-15s %s\n", p.Name, p.Version, status)
	}

	return nil
}

func runPluginInstall(_ *cobra.Command, args []string) error {
	pluginName := args[0]

	fmt.Printf("Installing plugin '%s'...\n", pluginName)

	manager := plugin.NewManager(pluginDir)
	if err := manager.Install(pluginName, installVersion); err != nil {
		return err
	}

	fmt.Printf("Plugin '%s' installed successfully!\n", pluginName)
	return nil
}

func runPluginUninstall(_ *cobra.Command, args []string) error {
	pluginName := args[0]

	manager := plugin.NewManager(pluginDir)
	if err := manager.Uninstall(pluginName); err != nil {
		return err
	}

	fmt.Printf("Plugin '%s' uninstalled successfully!\n", pluginName)
	return nil
}

// GetInstalledPluginManifest returns the manifest for an installed plugin
func GetInstalledPluginManifest(pluginName string) (*plugin.Manifest, error) {
	manager := plugin.NewManager(pluginDir)
	return manager.GetManifest(pluginName)
}
