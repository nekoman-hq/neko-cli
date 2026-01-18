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
	"regexp"

	"github.com/nekoman-hq/neko-cli/internal/log"
)

const configFileName = ".neko.json"

func LoadConfig() (*NekoConfig, error) {

	log.V(log.Config, "Loading config from file...")

	data, err := os.ReadFile(configFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(
				"Configuration not found: No .neko.json configuration found. Run 'neko init' first.",
			)
		} else {
			return nil, fmt.Errorf(
				"Configuration read error: %w", err,
			)
		}
	}

	var config NekoConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf(
			"Configuration parse error: %w", err,
		)
	}

	Validate(&config)

	return &config, nil
}

var semverRegex = regexp.MustCompile(
	`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[\da-zA-Z-]+(?:\.[\da-zA-Z-]+)*)?(?:\+[\da-zA-Z-]+(?:\.[\da-zA-Z-]+)*)?$`,
)

func Validate(cfg *NekoConfig) error {
	log.V(log.Config, "Validating serialised config...")

	if !cfg.ProjectType.IsValid() {
		return errors.New(
			"Invalid configuration: ProjectType is invalid in .neko.json",
		)
	}

	if !cfg.ReleaseSystem.IsValid() {
		return errors.New(
			"Invalid configuration: ReleaseSystem is invalid in .neko.json",
		)
	}

	if cfg.Version == "" {
		return errors.New(
			"Invalid configuration: Version is missing in .neko.json",
		)
	}

	if !semverRegex.MatchString(cfg.Version) {
		return errors.New(
			"Invalid configuration: Version is not a valid semantic version (SemVer)",
		)
	}

	log.Print(log.Config, "\uF00C Config appears valid")

	return nil
}

func SaveConfig(config NekoConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf(
			"Configuration serialization failed: %w", err,
		)
	}

	if err := os.WriteFile(configFileName, data, 0644); err != nil {
		return fmt.Errorf(
			"Configuration write failed: %w", err,
		)
	}
	return nil
}
