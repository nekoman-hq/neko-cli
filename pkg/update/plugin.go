package update

import (
	"fmt"
	_ "strings"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"golang.org/x/mod/semver"
)

// PluginOptions holds options for plugin updates
type PluginOptions struct {
	PluginDir string
	Force     bool
	DryRun    bool
	All       bool
}

// Result tracks the result of a plugin update
type Result struct {
	Error          error
	PluginName     string
	CurrentVersion string
	LatestVersion  string
	SkipReason     string
	Success        bool
	Skipped        bool
}

// Plugin handles updating plugins using the existing plugin.Manager
func Plugin(args []string, opts PluginOptions) error {
	manager := plugin.NewManager(opts.PluginDir)

	installed, err := manager.ListInstalled()
	if err != nil {
		return fmt.Errorf("failed to list installed plugins: %w", err)
	}

	if len(installed) == 0 {
		log.Print(log.Exec, "No plugins are currently installed")
		return nil
	}

	pluginsToUpdate, err := selectPluginsToUpdate(args, installed, opts.All)
	if err != nil {
		return err
	}

	if opts.All {
		log.Print(log.Exec, fmt.Sprintf("Updating %d plugin(s)...", len(pluginsToUpdate)))
	}

	results := updatePlugins(manager, pluginsToUpdate, installed, opts)
	printUpdateSummary(results, opts.DryRun)

	return nil
}

// selectPluginsToUpdate determines which plugins should be updated
func selectPluginsToUpdate(args []string, installed map[string]string, updateAll bool) ([]string, error) {
	if updateAll {
		plugins := make([]string, 0, len(installed))
		for name := range installed {
			plugins = append(plugins, name)
		}
		return plugins, nil
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("please specify a plugin name or use --all to update all plugins")
	}

	pluginName := args[0]
	if _, exists := installed[pluginName]; !exists {
		return nil, fmt.Errorf("plugin '%s' is not installed", pluginName)
	}

	return []string{pluginName}, nil
}

// updatePlugins performs the actual plugin updates
func updatePlugins(manager *plugin.Manager, pluginNames []string, installed map[string]string, opts PluginOptions) []Result {
	var results []Result
	registry := plugin.NewRegistry()

	for _, pluginName := range pluginNames {
		result := updateSinglePlugin(manager, registry, pluginName, installed[pluginName], opts)
		results = append(results, result)
	}

	return results
}

// updateSinglePlugin updates a single plugin
func updateSinglePlugin(manager *plugin.Manager, registry *plugin.Registry, pluginName, currentVersion string, opts PluginOptions) Result {
	result := Result{
		PluginName:     pluginName,
		CurrentVersion: currentVersion,
	}

	log.Print(log.Exec, fmt.Sprintf("Checking updates for %s (current: %s)...", pluginName, currentVersion))

	// Get the plugin's version for comparison
	latestPluginVersion, err := registry.GetPluginVersion(pluginName)
	if err != nil {
		result.Error = fmt.Errorf("failed to check version: %w", err)
		log.Print(log.Exec, fmt.Sprintf("Failed to check %s: %v", pluginName, err))
		return result
	}

	result.LatestVersion = latestPluginVersion

	currentVer := normalizeVersion(currentVersion)
	latestVer := normalizeVersion(latestPluginVersion)

	needsUpdate := semver.Compare(currentVer, latestVer) < 0

	if !needsUpdate && !opts.Force {
		result.Skipped = true
		result.SkipReason = "already on latest version"
		log.Print(log.Exec, fmt.Sprintf("%s is already on the latest version (%s)", pluginName, currentVersion))
		return result
	}

	if needsUpdate {
		log.Print(log.Exec, fmt.Sprintf("%s: %s → %s", pluginName, currentVersion, latestPluginVersion))
	} else {
		log.Print(log.Exec, fmt.Sprintf("Forcing update of %s to %s", pluginName, latestPluginVersion))
	}

	if opts.DryRun {
		result.Skipped = true
		result.SkipReason = "dry-run mode"
		return result
	}

	// Install using "latest" which resolves through the plugin registry index.
	if err := manager.Install(pluginName, "latest"); err != nil {
		result.Error = fmt.Errorf("failed to update: %w", err)
		log.Print(log.Exec, fmt.Sprintf("Failed to update %s: %v", pluginName, err))
		return result
	}

	result.Success = true
	log.Print(log.Exec, fmt.Sprintf("%s updated to version %s", pluginName, latestPluginVersion))

	return result
}

// printUpdateSummary prints a summary of the update results
func printUpdateSummary(results []Result, dryRun bool) {
	if dryRun {
		log.Print(log.Exec, "Update check complete (dry-run mode, not installing)")
		return
	}

	var successCount, failureCount, skipCount int
	var errors []string

	for _, result := range results {
		if result.Success {
			successCount++
		} else if result.Skipped {
			skipCount++
		} else if result.Error != nil {
			failureCount++
			errors = append(errors, fmt.Sprintf("%s: %v", result.PluginName, result.Error))
		}
	}

	if failureCount > 0 {
		log.Print(log.Exec, fmt.Sprintf("Updated %d plugin(s), %d failed, %d skipped",
			successCount, failureCount, skipCount))
		for _, errMsg := range errors {
			log.Print(log.Exec, errMsg)
		}
		return
	}

	if successCount > 0 {
		log.Print(log.Exec, fmt.Sprintf("Successfully updated %d plugin(s)", successCount))
	} else if skipCount > 0 {
		log.Print(log.Exec, "All plugins are up to date")
	}
}
