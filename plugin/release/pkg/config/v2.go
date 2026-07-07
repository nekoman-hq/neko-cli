package config

//lint:file-ignore fieldalignment Canonical release models keep logical field order for readability and JSON-domain documentation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

const (
	V2Directory      = ".neko"
	V2ConfigFileName = "release.config.json"
	V2StateFileName  = "release.state.json"
)

// ExecutorType is a supported release executor identifier in V2 config.
type ExecutorType string

const (
	ExecutorJReleaser  ExecutorType = "jreleaser"
	ExecutorReleaseIt  ExecutorType = "release-it"
	ExecutorGoReleaser ExecutorType = "goreleaser"
)

// DeliveryType is the configured release delivery channel.
type DeliveryType string

const (
	DeliveryLocal         DeliveryType = "local"
	DeliveryGitHubActions DeliveryType = "github-actions"
)

// V2ReleaseConfig is the committed repository architecture file.
//
//nolint:govet // JSON-domain order mirrors the canonical config document.
type V2ReleaseConfig struct {
	SchemaVersion int      `json:"schemaVersion"`
	Units         []V2Unit `json:"units"`
}

// V2Unit configures one releaseable unit in a repository.
//
//nolint:govet // JSON-domain order mirrors the canonical config document.
type V2Unit struct {
	ID               string     `json:"id"`
	DisplayName      string     `json:"displayName,omitempty"`
	Paths            []string   `json:"paths"`
	WorkingDirectory string     `json:"workingDirectory,omitempty"`
	TagPrefix        string     `json:"tagPrefix"`
	Executor         V2Executor `json:"executor"`
}

// V2Executor configures the release executor and delivery channel for a unit.
type V2Executor struct {
	Type     ExecutorType `json:"type"`
	Delivery DeliveryType `json:"delivery,omitempty"`
}

// V2ReleaseState is the version source of truth for all configured V2 units.
//
//nolint:govet // JSON-domain order mirrors the canonical state document.
type V2ReleaseState struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Units         map[string]V2UnitState `json:"units"`
}

// V2UnitState stores mutable release state for one V2 unit.
type V2UnitState struct {
	Version string `json:"version"`
}

var unitIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// V2ConfigPath returns the canonical V2 config path for repositoryRoot.
func V2ConfigPath(repositoryRoot string) string {
	return filepath.Join(repositoryRoot, V2Directory, V2ConfigFileName)
}

// V2StatePath returns the canonical V2 state path for repositoryRoot.
func V2StatePath(repositoryRoot string) string {
	return filepath.Join(repositoryRoot, V2Directory, V2StateFileName)
}

// V2ConfigExists checks whether a repository root contains the V2 config.
func V2ConfigExists(repositoryRoot string) bool {
	_, err := os.Stat(V2ConfigPath(repositoryRoot))
	return err == nil
}

// V2StateExists checks whether a repository root contains the V2 state.
func V2StateExists(repositoryRoot string) bool {
	_, err := os.Stat(V2StatePath(repositoryRoot))
	return err == nil
}

// LoadV2Repository strictly loads, validates, and normalizes a V2 repository.
func LoadV2Repository(repositoryRoot string) (*ReleaseRepository, error) {
	cfg, err := LoadV2Config(V2ConfigPath(repositoryRoot))
	if err != nil {
		return nil, err
	}
	state, err := LoadV2State(V2StatePath(repositoryRoot))
	if err != nil {
		return nil, err
	}
	if err := ValidateV2(repositoryRoot, cfg, state); err != nil {
		return nil, err
	}
	return NormalizeV2Repository(repositoryRoot, cfg, state), nil
}

// LoadV2Config strictly decodes a V2 release config file.
func LoadV2Config(path string) (*V2ReleaseConfig, error) {
	var cfg V2ReleaseConfig
	if err := readStrictJSON(path, &cfg); err != nil {
		return nil, fmt.Errorf("v2 config %s: %w", path, err)
	}
	return &cfg, nil
}

// LoadV2State strictly decodes a V2 release state file.
func LoadV2State(path string) (*V2ReleaseState, error) {
	var state V2ReleaseState
	if err := readStrictJSON(path, &state); err != nil {
		return nil, fmt.Errorf("v2 state %s: %w", path, err)
	}
	return &state, nil
}

// NormalizeV2Repository applies internal defaults without mutating files.
func NormalizeV2Repository(repositoryRoot string, cfg *V2ReleaseConfig, state *V2ReleaseState) *ReleaseRepository {
	units := make([]ReleaseUnit, 0, len(cfg.Units))
	for _, unit := range cfg.Units {
		workingDirectory := unit.WorkingDirectory
		if workingDirectory == "" {
			workingDirectory = "."
		}
		delivery := unit.Executor.Delivery
		if delivery == "" {
			delivery = DeliveryLocal
		}
		units = append(units, ReleaseUnit{
			ID:               unit.ID,
			DisplayName:      unit.DisplayName,
			Paths:            append([]string(nil), unit.Paths...),
			WorkingDirectory: workingDirectory,
			TagPrefix:        unit.TagPrefix,
			ExecutorType:     string(unit.Executor.Type),
			Delivery:         string(delivery),
			Version:          state.Units[unit.ID].Version,
		})
	}

	return &ReleaseRepository{
		RepositoryRoot: repositoryRoot,
		SchemaVersion:  2,
		SourceFormat:   SourceFormatV2,
		Units:          units,
	}
}

// ValidateV2 checks config and state together. It validates paths against
// repositoryRoot when supplied, including existing working directories.
func ValidateV2(repositoryRoot string, cfg *V2ReleaseConfig, state *V2ReleaseState) error {
	if cfg.SchemaVersion != 2 {
		return fmt.Errorf("v2 config schemaVersion must be exactly 2, got %d", cfg.SchemaVersion)
	}
	if state.SchemaVersion != 2 {
		return fmt.Errorf("v2 state schemaVersion must be exactly 2, got %d", state.SchemaVersion)
	}
	if len(cfg.Units) == 0 {
		return fmt.Errorf("v2 config must define at least one unit")
	}
	if state.Units == nil {
		return fmt.Errorf("v2 state units must be present")
	}

	ids := make(map[string]struct{}, len(cfg.Units))
	tagPrefixes := make(map[string]string, len(cfg.Units))
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
	}

	for id := range ids {
		unitState, ok := state.Units[id]
		if !ok {
			return fmt.Errorf("v2 state is missing unit %q", id)
		}
		if _, err := semver.NewVersion(unitState.Version); err != nil {
			return fmt.Errorf("v2 state unit %q version %q is not valid SemVer", id, unitState.Version)
		}
	}
	for id := range state.Units {
		if _, ok := ids[id]; !ok {
			return fmt.Errorf("v2 state contains unknown unit %q", id)
		}
	}

	return nil
}

// CanonicalV2Config returns the normalized JSON bytes future writers should use.
func CanonicalV2Config(cfg V2ReleaseConfig) ([]byte, error) {
	for i := range cfg.Units {
		if cfg.Units[i].WorkingDirectory == "" {
			cfg.Units[i].WorkingDirectory = "."
		}
		if cfg.Units[i].Executor.Delivery == "" {
			cfg.Units[i].Executor.Delivery = DeliveryLocal
		}
	}
	return marshalCanonicalJSON(cfg)
}

// CanonicalV2State returns the normalized JSON bytes future writers should use.
func CanonicalV2State(state V2ReleaseState) ([]byte, error) {
	return marshalCanonicalJSON(state)
}

func validateV2Unit(repositoryRoot string, unit V2Unit) error {
	if !unitIDPattern.MatchString(unit.ID) {
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
	if err := validateTagPrefix(unit.ID, unit.TagPrefix); err != nil {
		return err
	}
	if !unit.Executor.Type.IsValid() {
		return fmt.Errorf("v2 config unit %q has unknown executor %q", unit.ID, unit.Executor.Type)
	}
	delivery := unit.Executor.Delivery
	if delivery == "" {
		delivery = DeliveryLocal
	}
	if !delivery.IsValid() {
		return fmt.Errorf("v2 config unit %q has unknown delivery %q", unit.ID, unit.Executor.Delivery)
	}
	return nil
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
		info, err := os.Stat(filepath.Join(repositoryRoot, clean))
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("v2 config unit %q workingDirectory %q does not exist", unitID, workingDirectory)
			}
			return fmt.Errorf("v2 config unit %q workingDirectory %q cannot be inspected: %w", unitID, workingDirectory, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("v2 config unit %q workingDirectory %q is not a directory", unitID, workingDirectory)
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

func validateTagPrefix(unitID, tagPrefix string) error {
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

// IsValid reports whether the executor is supported by V2.
func (e ExecutorType) IsValid() bool {
	switch e {
	case ExecutorJReleaser, ExecutorReleaseIt, ExecutorGoReleaser:
		return true
	default:
		return false
	}
}

// IsValid reports whether the delivery type is supported by V2.
func (d DeliveryType) IsValid() bool {
	switch d {
	case DeliveryLocal, DeliveryGitHubActions:
		return true
	default:
		return false
	}
}

func readStrictJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON content")
	}
	return nil
}

func marshalCanonicalJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
