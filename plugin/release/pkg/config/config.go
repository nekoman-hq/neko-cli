package config

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since     17.12.2025
*/

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

const FileName = ".release.neko.json"

// Exists checks if the configuration file already exists
func Exists() bool {
	return LegacyConfigExistsAt(".")
}

// LegacyConfigExistsAt checks for a V1 release config in a specific root.
func LegacyConfigExistsAt(root string) bool {
	_, err := os.Stat(filepath.Join(root, FileName))
	return err == nil
}

func LoadConfig() (*NekoConfig, error) {
	return LoadConfigAt(FileName)
}

// LoadConfigAt reads a legacy .release.neko.json file without changing the
// public V1 schema fields used by existing init and release flows.
func LoadConfigAt(path string) (*NekoConfig, error) {
	log.PluginV(log.Config, "Loading config from file...")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(
				"configuration not found: No .release.neko.json configuration found. Run 'neko release init' first",
			)
		} else {
			return nil, fmt.Errorf(
				"configuration read error: %w", err,
			)
		}
	}

	var config NekoConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf(
			"configuration parse error: %w", err,
		)
	}

	if err := Validate(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

var semverRegex = regexp.MustCompile(
	`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[\da-zA-Z-]+(?:\.[\da-zA-Z-]+)*)?(?:\+[\da-zA-Z-]+(?:\.[\da-zA-Z-]+)*)?$`,
)

func Validate(cfg *NekoConfig) error {
	log.PluginV(log.Config, "Validating serialised config...")

	if !cfg.ProjectType.IsValid() {
		return errors.New(
			"invalid configuration: ProjectType is invalid in ..release.neko.json",
		)
	}

	if !cfg.ReleaseSystem.IsValid() {
		return errors.New(
			"invalid configuration: ReleaseSystem is invalid in ..release.neko.json",
		)
	}

	if cfg.Version == "" {
		return errors.New(
			"invalid configuration: Version is missing in .release.neko.json",
		)
	}

	if !semverRegex.MatchString(cfg.Version) {
		return errors.New(
			"invalid configuration: Version is not a valid semantic version (SemVer)",
		)
	}

	log.PluginPrint(log.Config, "\uF00C Config appears valid")

	return nil
}

func SaveConfig(config NekoConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("configuration serialization failed: %w", err)
	}
	if err = os.WriteFile(FileName, data, 0644); err != nil {
		return fmt.Errorf("configuration write failed: %w", err)
	}
	return nil
}
