package cmd

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
	rootCmd.PersistentFlags().StringVar(&outputFormat, "output", "table", "Output format (table, json, wide)")
	rootCmd.PersistentFlags().BoolVar(&describe, "describe", false, "Include execution logs and metadata in output")

	// Detect plugin directory
	home, _ := os.UserHomeDir()
	defaultPluginDir := filepath.Join(home, ".neko", "plugins")
	pluginDir = os.Getenv("NEKO_PLUGIN_DIR") // For future use, allows custom plugin dir
	if pluginDir == "" {
		pluginDir = defaultPluginDir
	}
}

// CreatePluginCommand creates a cobra.Command for the given plugin manifest
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

// createSubCommand creates a cobra.Command for the given plugin command with flags
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

// addFlagToCommand adds a flag to the command based on the flag definition
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

// executePlugin dispatches the command to the plugin and renders the response
func executePlugin(pluginName string, cmd *cobra.Command, args []string) error {
	d := dispatcher.NewDispatcher(pluginDir)

	// Determine the command name - if we're on the root plugin command and have args,
	// the first arg might be an unknown subcommand
	commandName := cmd.Name()
	actualArgs := args
	if commandName == pluginName && len(args) > 0 {
		// User typed something like "neko release unknownCmd" - treat first arg as command
		commandName = args[0]
		actualArgs = args[1:]
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

// extractFlags extracts the flags from the cobra.Command into a map
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

// mustGetwd returns the current working directory or an empty string on error
func mustGetwd() string {
	wd, _ := os.Getwd()
	return wd
}

// InitializePlugins loads plugins from the plugin directory and adds them to the root command
func InitializePlugins() error {
	// If plugin directory doesn't exist, that's fine - no plugins installed yet
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		return nil
	}

	d := dispatcher.NewDispatcher(pluginDir)

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
