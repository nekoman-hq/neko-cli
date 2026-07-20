package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var unitIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type v2ConfigStateAlignmentError struct {
	message string
}

func (alignmentError *v2ConfigStateAlignmentError) Error() string {
	return alignmentError.message
}

// IsV2ConfigStateAlignmentError reports whether validation failed because
// config and state do not describe the same release-unit identities.
func IsV2ConfigStateAlignmentError(err error) bool {
	var alignmentError *v2ConfigStateAlignmentError
	return errors.As(err, &alignmentError)
}

// ValidateReleaseUnitID applies the canonical V2 unit identifier policy.
func ValidateReleaseUnitID(unitID string) error {
	if !unitIDPattern.MatchString(unitID) {
		return fmt.Errorf("release unit id %q is invalid; use [a-z][a-z0-9-]*", unitID)
	}
	return nil
}

// ValidateV2 checks config and state together. It validates paths against
// repositoryRoot when supplied, including existing working directories.
func ValidateV2(repositoryRoot string, cfg *V2ReleaseConfig, state *V2ReleaseState) error {
	return validateV2ConfigAndState(repositoryRoot, cfg, state, true)
}

func validateV2ConfigAndState(repositoryRoot string, cfg *V2ReleaseConfig, state *V2ReleaseState, requireState bool) error {
	if cfg == nil {
		return fmt.Errorf("v2 config is missing")
	}
	if cfg.SchemaVersion != 2 {
		return fmt.Errorf("v2 config schemaVersion must be exactly 2, got %d", cfg.SchemaVersion)
	}
	if !requireState && state == nil {
		state = &V2ReleaseState{SchemaVersion: 2}
	}
	if state == nil {
		return fmt.Errorf("v2 state is missing")
	}
	if state.SchemaVersion != 2 {
		return fmt.Errorf("v2 state schemaVersion must be exactly 2, got %d", state.SchemaVersion)
	}
	if len(cfg.Units) == 0 {
		return fmt.Errorf("v2 config must define at least one unit")
	}
	if requireState && state.Units == nil {
		return fmt.Errorf("v2 state units must be present")
	}

	ids := make(map[string]struct{}, len(cfg.Units))
	tagPrefixes := make(map[string]string, len(cfg.Units))
	pluginNames := make(map[string]string, len(cfg.Units))
	for _, unit := range cfg.Units {
		if err := validateV2Unit(repositoryRoot, unit); err != nil {
			return err
		}
		if _, ok := ids[unit.ID]; ok {
			return fmt.Errorf("v2 config unit %q is defined more than once", unit.ID)
		}
		ids[unit.ID] = struct{}{}

		for seenPrefix, seenUnitID := range tagPrefixes {
			if tagPrefixesOverlap(seenPrefix, unit.TagPrefix) {
				return fmt.Errorf("v2 config unit %q tagPrefix %q overlaps with unit %q tagPrefix %q", unit.ID, unit.TagPrefix, seenUnitID, seenPrefix)
			}
		}
		tagPrefixes[unit.TagPrefix] = unit.ID
		if unit.Kind == UnitKindPlugin && unit.Plugin != nil {
			if previousUnitID, ok := pluginNames[unit.Plugin.Name]; ok {
				return fmt.Errorf("v2 config plugin name %q is used by both unit %q and unit %q", unit.Plugin.Name, previousUnitID, unit.ID)
			}
			pluginNames[unit.Plugin.Name] = unit.ID
		}
	}

	if requireState {
		for id := range ids {
			unitState, ok := state.Units[id]
			if !ok {
				return &v2ConfigStateAlignmentError{message: fmt.Sprintf("v2 state is missing unit %q", id)}
			}
			if _, err := CanonicalReleaseVersion(unitState.Version); err != nil {
				return fmt.Errorf("v2 state unit %q version %q is not valid SemVer", id, unitState.Version)
			}
		}
		for id := range state.Units {
			if _, ok := ids[id]; !ok {
				return &v2ConfigStateAlignmentError{message: fmt.Sprintf("v2 state contains unknown unit %q", id)}
			}
		}
	}

	return nil
}

func validateV2Unit(repositoryRoot string, unit V2Unit) error {
	if err := ValidateReleaseUnitID(unit.ID); err != nil {
		return fmt.Errorf("v2 config unit id %q is invalid; use [a-z][a-z0-9-]*", unit.ID)
	}
	if len(unit.Paths) == 0 {
		return fmt.Errorf("v2 config unit %q must define at least one path pattern", unit.ID)
	}
	if err := validateWorkingDirectory(repositoryRoot, unit.ID, unit.WorkingDirectory); err != nil {
		return err
	}
	for _, pattern := range unit.Paths {
		if err := validatePathPattern(unit.ID, pattern); err != nil {
			return err
		}
	}
	if err := ValidateV2TagPrefix(unit.ID, unit.TagPrefix); err != nil {
		return err
	}
	if err := validateV2UnitPluginMetadata(repositoryRoot, unit); err != nil {
		return err
	}
	return validateV2Executor(repositoryRoot, unit.ID, unit.Executor)
}

func validateWorkingDirectory(repositoryRoot, unitID, workingDirectory string) error {
	if workingDirectory == "" {
		workingDirectory = "."
	}
	if strings.TrimSpace(workingDirectory) == "" {
		return fmt.Errorf("v2 config unit %q workingDirectory must not be empty", unitID)
	}
	if filepath.IsAbs(workingDirectory) {
		return fmt.Errorf("v2 config unit %q workingDirectory %q must be relative", unitID, workingDirectory)
	}
	clean := filepath.Clean(workingDirectory)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("v2 config unit %q workingDirectory %q leaves the repository", unitID, workingDirectory)
	}
	if repositoryRoot != "" {
		absoluteRoot, err := filepath.Abs(repositoryRoot)
		if err != nil {
			return fmt.Errorf("v2 config unit %q repository root %q cannot be resolved: %w", unitID, repositoryRoot, err)
		}
		workingDirectoryPath := filepath.Join(absoluteRoot, clean)
		info, err := os.Stat(workingDirectoryPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("v2 config unit %q workingDirectory %q does not exist", unitID, workingDirectory)
			}
			return fmt.Errorf("v2 config unit %q workingDirectory %q cannot be inspected: %w", unitID, workingDirectory, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("v2 config unit %q workingDirectory %q is not a directory", unitID, workingDirectory)
		}
		resolvedRoot, err := filepath.EvalSymlinks(absoluteRoot)
		if err != nil {
			return fmt.Errorf("v2 config unit %q repository root %q cannot be resolved physically: %w", unitID, absoluteRoot, err)
		}
		resolvedWorkingDirectory, err := filepath.EvalSymlinks(workingDirectoryPath)
		if err != nil {
			return fmt.Errorf("v2 config unit %q workingDirectory %q cannot be resolved physically: %w", unitID, workingDirectory, err)
		}
		if !pathInside(resolvedRoot, resolvedWorkingDirectory) {
			return fmt.Errorf("v2 config unit %q workingDirectory %q resolves outside repository root", unitID, workingDirectory)
		}
	}
	return nil
}

func validatePathPattern(unitID, pattern string) error {
	if strings.TrimSpace(pattern) == "" {
		return fmt.Errorf("v2 config unit %q path pattern must not be empty", unitID)
	}
	if filepath.IsAbs(pattern) {
		return fmt.Errorf("v2 config unit %q path pattern %q must be relative", unitID, pattern)
	}
	clean := filepath.Clean(pattern)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("v2 config unit %q path pattern %q leaves the repository", unitID, pattern)
	}
	return nil
}

// ValidateV2TagPrefix applies the canonical Release V2 tag-prefix policy for
// one unit without requiring a synthetic full unit configuration.
func ValidateV2TagPrefix(unitID, tagPrefix string) error {
	if strings.TrimSpace(tagPrefix) == "" {
		return fmt.Errorf("v2 config unit %q tagPrefix must not be empty", unitID)
	}
	if filepath.IsAbs(tagPrefix) || strings.HasPrefix(tagPrefix, "/") {
		return fmt.Errorf("v2 config unit %q tagPrefix %q must be relative", unitID, tagPrefix)
	}
	if strings.Contains(tagPrefix, "..") || strings.ContainsAny(tagPrefix, " \t\r\n") || strings.Contains(tagPrefix, "//") {
		return fmt.Errorf("v2 config unit %q tagPrefix %q is unsafe", unitID, tagPrefix)
	}
	if _, err := NewTagSpec(tagPrefix); err != nil {
		return fmt.Errorf("v2 config unit %q tagPrefix %q is invalid: %w", unitID, tagPrefix, err)
	}
	return nil
}

func tagPrefixesOverlap(a, b string) bool {
	return a == b || strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}
