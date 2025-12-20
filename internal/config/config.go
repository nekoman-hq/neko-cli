package config

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since     17.12.2025
*/

import (
	"encoding/json"
	"os"
	"regexp"

	"github.com/nekoman-hq/neko-cli/internal/errors"
)

const configFileName = ".neko.json"

func LoadConfig() *NekoConfig {
	data, err := os.ReadFile(configFileName)
	if err != nil {
		if os.IsNotExist(err) {
			errors.Fatal(
				"Configuration not found",
				"No .neko.json configuration found. Run 'neko init' first.",
				errors.ErrConfigNotExists,
			)
		} else {
			errors.Fatal(
				"Configuration read error",
				err.Error(),
				errors.ErrConfigRead,
			)
		}
	}

	var config NekoConfig
	if err := json.Unmarshal(data, &config); err != nil {
		errors.Fatal(
			"Configuration parse error",
			"Failed to parse .neko.json: "+err.Error(),
			errors.ErrConfigMarshal,
		)
	}

	Validate(&config)

	return &config
}

var semverRegex = regexp.MustCompile(
	`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[\da-zA-Z-]+(?:\.[\da-zA-Z-]+)*)?(?:\+[\da-zA-Z-]+(?:\.[\da-zA-Z-]+)*)?$`,
)

func Validate(cfg *NekoConfig) {
	if !cfg.ProjectType.IsValid() {
		errors.Error(
			"Invalid configuration",
			"ProjectType is invalid in .neko.json",
			errors.ErrConfigMarshal,
		)
		return
	}

	if !cfg.ReleaseSystem.IsValid() {
		errors.Error(
			"Invalid configuration",
			"ReleaseSystem is invalid in .neko.json",
			errors.ErrConfigMarshal,
		)
		return
	}

	if cfg.Version == "" {
		errors.Error(
			"Invalid configuration",
			"Version is missing in .neko.json",
			errors.ErrConfigMarshal,
		)
		return
	}

	if !semverRegex.MatchString(cfg.Version) {
		errors.Error(
			"Invalid configuration",
			"Version is not a valid semantic version (SemVer)",
			errors.ErrVersionViolation,
		)
		return
	}

	println("\n✓ Configuration appears valid")
}

func SaveConfig(config NekoConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		errors.Fatal(
			"Configuration serialization failed",
			"Could not marshal .neko.json: "+err.Error(),
			errors.ErrConfigMarshal,
		)
		return err
	}

	if err := os.WriteFile(configFileName, data, 0644); err != nil {
		errors.Fatal(
			"Configuration write failed",
			"Could not write .neko.json: "+err.Error(),
			errors.ErrConfigWrite,
		)
		return err
	}
	return nil
}
