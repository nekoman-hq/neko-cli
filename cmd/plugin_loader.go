package cmd

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      03.02.2026
*/

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nekoman-hq/neko-cli/pkg/dispatcher"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func init() {
	rootCmd.PersistentFlags().StringVar(&outputFormat, "output", "table", "Output format (table, json, wide) -- only for plugin responses")
	rootCmd.PersistentFlags().BoolVar(&describe, "describe", false, "Include execution logs and metadata in output -- only for plugin responses")

	// Detect plugin directory
	home, _ := os.UserHomeDir()
	defaultPluginDir := filepath.Join(home, ".neko", "plugins")
	PluginDir = os.Getenv("NEKO_PLUGIN_DIR") // For future use, allows custom plugin dir
	if PluginDir == "" {
		PluginDir = defaultPluginDir
	}
}

// CreatePluginCommand creates a cobra.Command for a plugin based on its manifest.
// This generates the main command (e.g., "release", "deploy") and adds all subcommands
// defined in the plugin's manifest (e.g., "release init", "release create").
//
// Args:
//   - manifest: The plugin manifest containing command definitions
//
// Returns a configured cobra.Command representing the plugin and its subcommands.
func CreatePluginCommand(manifest plugin.Manifest) *cobra.Command {
	// Main command for every plugin e.g., "release", "deploy"
	cmd := &cobra.Command{
		Use:   manifest.Name,
		Short: manifest.Description,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executePlugin(manifest.Name, cmd, args)
		},
	}

	// Subcommands for each plugin command e.g., "release init", "release create"
	for _, pluginCmd := range manifest.Commands {
		subCmd := createSubCommand(manifest.Name, pluginCmd)
		cmd.AddCommand(subCmd)
	}

	return cmd
}

// createSubCommand creates a cobra.Command for a specific plugin subcommand.
// It automatically adds all flags defined in the plugin command's manifest,
// including their types, default values, and required status.
//
// Args:
//   - pluginName: The name of the parent plugin
//   - pluginCmd: The command definition from the plugin manifest
//
// Returns a configured cobra.Command for the subcommand.
func createSubCommand(pluginName string, pluginCmd plugin.Command) *cobra.Command {
	subCmd := &cobra.Command{
		Use:   pluginCmd.Name,
		Short: pluginCmd.Description,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executePlugin(pluginName, cmd, args)
		},
	}

	// Add flags from the plugin manifest
	for _, flag := range pluginCmd.Flags {
		addFlagToCommand(subCmd, flag)
	}

	return subCmd
}

// addFlagToCommand adds a flag to a cobra command based on the plugin's flag definition.
// It handles type conversion and default values for string, bool, and int flag types.
// If the flag is marked as required, it will be enforced by cobra.
//
// Args:
//   - cmd: The cobra.Command to add the flag to
//   - flag: The flag definition from the plugin manifest
//
// Supported flag types:
//   - "string": String flags with optional default value
//   - "bool": Boolean flags with optional default value
//   - "int": Integer flags with optional default value (JSON numbers are float64)
//   - Any other type defaults to string behavior
func addFlagToCommand(cmd *cobra.Command, flag plugin.Flag) {
	switch flag.Type {
	case "string":
		defaultVal := ""
		if flag.Default != nil {
			if s, ok := flag.Default.(string); ok {
				defaultVal = s
			}
		}
		cmd.Flags().String(flag.Name, defaultVal, flag.Description)
	case "bool":
		defaultVal := false
		if flag.Default != nil {
			if b, ok := flag.Default.(bool); ok {
				defaultVal = b
			}
		}
		cmd.Flags().Bool(flag.Name, defaultVal, flag.Description)
	case "int":
		defaultVal := 0
		if flag.Default != nil {
			if i, ok := flag.Default.(float64); ok {
				defaultVal = int(i)
			}
		}
		cmd.Flags().Int(flag.Name, defaultVal, flag.Description)
	default:
		// Default to string
		cmd.Flags().String(flag.Name, "", flag.Description)
	}

	// Mark required flags
	if flag.Required {
		_ = cmd.MarkFlagRequired(flag.Name)
	}
}

// executePlugin dispatches a command to the appropriate plugin and renders the response.
// It constructs a plugin.Request from the cobra command context, executes it via the
// dispatcher, and renders the result using the configured output format.
//
// This function handles the special case where a user invokes a plugin's root command
// with an unknown subcommand (e.g., "neko release unknownCmd"), treating the first
// argument as the command name.
//
// Args:
//   - pluginName: The name of the plugin to execute
//   - cmd: The cobra.Command being executed
//   - args: The command-line arguments
//
// Returns an error if:
//   - Required flags are missing
//   - Plugin execution fails
//   - Response rendering fails
func executePlugin(pluginName string, cmd *cobra.Command, args []string) error {
	d := dispatcher.NewDispatcher(PluginDir)

	// Determine the command name - if we're on the root plugin command and have args,
	// the first arg might be an unknown subcommand
	commandName := cmd.Name()
	actualArgs := args
	if commandName == pluginName && len(args) > 0 {
		// User typed something like "neko release unknownCmd" - treat first arg as command
		commandName = args[0]
		actualArgs = args[1:]
	}

	// Load plugin manifest to validate required flags
	manifest, err := GetInstalledPluginManifest(pluginName)
	if err != nil {
		return fmt.Errorf("failed to load plugin manifest: %w", err)
	}

	// Find the command definition in manifest
	var cmdDef *plugin.Command
	for _, c := range manifest.Commands {
		if c.Name == commandName {
			cmdDef = &c
			break
		}
	}

	// Validate required flags if we found the command definition
	if cmdDef != nil {
		if validationErr := validateRequiredFlagsFromManifest(cmd, cmdDef.Flags); validationErr != nil {
			return validationErr
		}
	}

	req := plugin.Request{
		Command: commandName,
		Args:    actualArgs,
		Flags:   extractFlags(cmd),
		Context: plugin.Context{
			WorkingDir: mustGetwd(),
			User:       os.Getenv("USER"),
			Verbose:    verbose,
		},
	}

	ctx := context.Background()
	resp, err := d.Dispatch(ctx, pluginName, req)
	if err != nil {
		return fmt.Errorf("failed to execute plugin: %w", err)
	}

	opts := renderer.RenderOptions{
		Format:   renderer.OutputFormat(outputFormat),
		Describe: describe,
	}
	return renderer.RenderWithOptions(resp, opts)
}

// validateRequiredFlagsFromManifest checks that all required flags from the manifest
// have been set by the user. This validation happens before dispatching to the plugin.
//
// Args:
//   - cmd: The cobra.Command to check flags on
//   - flagDefs: The flag definitions from the plugin manifest
//
// Returns an error if any required flag is missing.
func validateRequiredFlagsFromManifest(cmd *cobra.Command, flagDefs []plugin.Flag) error {
	var missingFlags []string

	for _, flagDef := range flagDefs {
		if flagDef.Required {
			flag := cmd.Flags().Lookup(flagDef.Name)
			if flag == nil {
				missingFlags = append(missingFlags, flagDef.Name)
			} else if !flag.Changed {
				missingFlags = append(missingFlags, flagDef.Name)
			}
		}
	}

	if len(missingFlags) > 0 {
		return fmt.Errorf("required flag(s) not set: %v", missingFlags)
	}

	return nil
}

// extractFlags extracts all flags from a cobra.Command into a map.
// Only flags that have been explicitly set (changed from their default) are included.
// The function preserves type information for bool and int flags, converting them
// to their appropriate Go types. Other flag types are stored as strings.
//
// Args:
//   - cmd: The cobra.Command to extract flags from
//
// Returns a map of flag names to their values with appropriate types.
func extractFlags(cmd *cobra.Command) map[string]any {
	flags := make(map[string]any)

	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Changed {
			// Try to get typed value
			switch flag.Value.Type() {
			case "bool":
				if b, err := cmd.Flags().GetBool(flag.Name); err == nil {
					flags[flag.Name] = b
				}
			case "int":
				if i, err := cmd.Flags().GetInt(flag.Name); err == nil {
					flags[flag.Name] = i
				}
			default:
				flags[flag.Name] = flag.Value.String()
			}
		}
	})

	return flags
}

// mustGetwd returns the current working directory or an empty string if it cannot
// be determined. This is a helper function that silently handles errors, suitable
// for non-critical path resolution.
//
// Returns the current working directory path, or empty string on error.
func mustGetwd() string {
	wd, _ := os.Getwd()
	return wd
}

// InitializePlugins loads all installed plugins from the plugin directory and
// registers them as commands in the root cobra command. This function is called
// during CLI initialization to make plugins available to users.
//
// If the plugin directory does not exist, the function returns nil (no error),
// as this is a valid state when no plugins are installed yet.
//
// Returns an error if:
//   - The plugin directory exists but cannot be read
//   - Plugin manifests cannot be loaded
//
// Note: Individual plugin loading errors are not fatal and won't prevent
// other plugins from being loaded.
func InitializePlugins() error {
	// If plugin directory doesn't exist, that's fine - no plugins installed yet
	if _, err := os.Stat(PluginDir); os.IsNotExist(err) {
		return nil
	}

	d := dispatcher.NewDispatcher(PluginDir)

	manifests, err := d.ListPlugins()
	if err != nil {
		return fmt.Errorf("failed to list plugins: %w", err)
	}

	for _, manifest := range manifests {
		cmd := CreatePluginCommand(manifest)
		rootCmd.AddCommand(cmd)
	}

	return nil
}
