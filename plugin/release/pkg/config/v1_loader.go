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

// V1FileName is the legacy release config file name.
//
// Deprecated: V1 is supported only as the legacy compatibility format.
const V1FileName = ".release.neko.json"

// V1Exists checks if the legacy configuration file exists.
//
// Deprecated: use V1ConfigExistsAt with an explicit repository root instead.
func V1Exists() bool {
	return V1ConfigExistsAt(".")
}

// V1ConfigExistsAt checks for a V1 release config in a specific root.
func V1ConfigExistsAt(root string) bool {
	_, err := os.Stat(filepath.Join(root, V1FileName))
	return err == nil
}

// V1LoadConfig loads the legacy config from the current working directory.
//
// Deprecated: use V1LoadConfigAt with an explicit path instead.
func V1LoadConfig() (*V1ReleaseConfig, error) {
	return V1LoadConfigAt(V1FileName)
}

// V1LoadConfigAt reads a legacy .release.neko.json file without changing the
// public V1 schema fields used by existing init and release flows.
func V1LoadConfigAt(path string) (*V1ReleaseConfig, error) {
	return v1LoadConfigAt(path, true)
}

func v1LoadConfigAt(path string, reportProgress bool) (*V1ReleaseConfig, error) {
	if reportProgress {
		log.PluginV(log.Config, "Loading config from file...")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(
				"configuration not found: no release configuration found. Run 'neko release init' to create V2 config/state, or 'neko release migrate' to convert an existing V1 config",
			)
		} else {
			return nil, fmt.Errorf(
				"configuration read error: %w", err,
			)
		}
	}

	var config V1ReleaseConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf(
			"configuration parse error: %w", err,
		)
	}

	if err := v1Validate(&config, reportProgress); err != nil {
		return nil, err
	}

	return &config, nil
}

var semverRegex = regexp.MustCompile(
	`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[\da-zA-Z-]+(?:\.[\da-zA-Z-]+)*)?(?:\+[\da-zA-Z-]+(?:\.[\da-zA-Z-]+)*)?$`,
)

// V1Validate validates the legacy .release.neko.json schema.
//
// Deprecated: V1 is supported only as the legacy compatibility format.
func V1Validate(cfg *V1ReleaseConfig) error {
	return v1Validate(cfg, true)
}

// V1ValidateForInspection applies the same compatibility validation without
// emitting lifecycle progress for a deterministic read-only query.
func V1ValidateForInspection(cfg *V1ReleaseConfig) error {
	return v1Validate(cfg, false)
}

func v1Validate(cfg *V1ReleaseConfig, reportProgress bool) error {
	if reportProgress {
		log.PluginV(log.Config, "Validating serialised config...")
	}
	if !cfg.ProjectType.IsValid() {
		return errors.New(
			"invalid configuration: V1ProjectType is invalid in ..release.neko.json",
		)
	}

	if !cfg.ReleaseSystem.IsValid() {
		return errors.New(
			"invalid configuration: V1ReleaseSystem is invalid in ..release.neko.json",
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

	if reportProgress {
		log.PluginPrint(log.Config, "\uF00C Config appears valid")
	}

	return nil
}

// V1SaveConfig writes the legacy .release.neko.json file.
//
// Deprecated: use V1SaveConfigAt with an explicit repository root instead.
func V1SaveConfig(config V1ReleaseConfig) error {
	return V1SaveConfigAt(".", config)
}

// V1SaveConfigAt writes the canonical legacy file within repositoryRoot.
func V1SaveConfigAt(repositoryRoot string, config V1ReleaseConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("configuration serialization failed: %w", err)
	}
	if err = os.WriteFile(filepath.Join(repositoryRoot, V1FileName), data, 0644); err != nil {
		return fmt.Errorf("configuration write failed: %w", err)
	}
	return nil
}
