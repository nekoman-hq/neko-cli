package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

const Filename = ".ui.neko.json"

// Config represents the UI plugin configuration
type Config struct {
	ComponentsPath string `json:"componentsPath"` // Relative path where components are stored (e.g., "shared/ui")
}

// Exists checks if the config file exists in the project root
func Exists(projectRoot string) bool {
	configPath := filepath.Join(projectRoot, Filename)
	_, err := os.Stat(configPath)
	return err == nil
}

// Load reads and parses the config file from the project root
func Load(projectRoot string) (*Config, error) {
	configPath := filepath.Join(projectRoot, Filename)

	log.PluginV(log.Config, "Loading config from: %s", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	log.PluginV(log.Config, "Config loaded successfully: componentsPath=%s", cfg.ComponentsPath)

	return &cfg, nil
}

// Create creates a new config file with the specified components path
func Create(projectRoot string, componentsPath string) error {
	configPath := filepath.Join(projectRoot, Filename)

	if Exists(projectRoot) {
		return fmt.Errorf("config file already exists at %s", configPath)
	}

	cfg := &Config{
		ComponentsPath: componentsPath,
	}

	log.PluginV(log.Config, "Creating config at: %s with componentsPath=%s", configPath, componentsPath)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	log.PluginPrint(log.Config, "Config file created: %s", configPath)

	return nil
}

// Update updates an existing config file with new values
func Update(projectRoot string, componentsPath string) error {
	configPath := filepath.Join(projectRoot, Filename)

	if !Exists(projectRoot) {
		return fmt.Errorf("config file does not exist at %s", configPath)
	}

	cfg := &Config{
		ComponentsPath: componentsPath,
	}

	log.PluginV(log.Config, "Updating config at: %s with componentsPath=%s", configPath, componentsPath)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	log.PluginPrint(log.Config, "Config file updated: %s", configPath)

	return nil
}

// GetComponentsPath returns the full path to the components directory
func (c *Config) GetComponentsPath(projectRoot string) string {
	return filepath.Join(projectRoot, c.ComponentsPath)
}

// Validate checks if the config is valid
func (c *Config) Validate() error {
	if c.ComponentsPath == "" {
		return fmt.Errorf("componentsPath cannot be empty")
	}

	// Check for absolute paths (should be relative)
	if filepath.IsAbs(c.ComponentsPath) {
		return fmt.Errorf("componentsPath must be a relative path, got: %s", c.ComponentsPath)
	}

	return nil
}
