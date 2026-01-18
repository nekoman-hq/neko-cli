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
				"configuration not found: No .neko.json configuration found. Run 'neko init' first",
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
	log.V(log.Config, "Validating serialised config...")

	if !cfg.ProjectType.IsValid() {
		return errors.New(
			"invalid configuration: ProjectType is invalid in .neko.json",
		)
	}

	if !cfg.ReleaseSystem.IsValid() {
		return errors.New(
			"invalid configuration: ReleaseSystem is invalid in .neko.json",
		)
	}

	if cfg.Version == "" {
		return errors.New(
			"invalid configuration: Version is missing in .neko.json",
		)
	}

	if !semverRegex.MatchString(cfg.Version) {
		return errors.New(
			"invalid configuration: Version is not a valid semantic version (SemVer)",
		)
	}

	log.Print(log.Config, "\uF00C Config appears valid")

	return nil
}

func SaveConfig(config NekoConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("configuration serialization failed: %w", err)
	}
	if err = os.WriteFile(configFileName, data, 0644); err != nil {
		return fmt.Errorf("configuration write failed: %w", err)
	}
	return nil
}
