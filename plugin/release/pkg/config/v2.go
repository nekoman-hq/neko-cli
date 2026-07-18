package config

//lint:file-ignore fieldalignment Canonical release models keep logical field order for readability and JSON-domain documentation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
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

// UnitKind optionally classifies a V2 release unit.
type UnitKind string

const (
	UnitKindPlugin UnitKind = "plugin"
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
	Kind             UnitKind   `json:"kind,omitempty"`
	Plugin           *V2Plugin  `json:"plugin,omitempty"`
	Executor         V2Executor `json:"executor"`
}

// V2Plugin configures plugin-registry metadata for a V2 plugin unit.
type V2Plugin struct {
	Name        string `json:"name"`
	Manifest    string `json:"manifest"`
	AssetPrefix string `json:"assetPrefix"`
	BinaryName  string `json:"binaryName"`
}

// V2Executor configures the release executor and delivery channel for a unit.
type V2Executor struct {
	Type     ExecutorType `json:"type"`
	Delivery DeliveryType `json:"delivery,omitempty"`
	Workflow string       `json:"workflow,omitempty"`
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

var (
	unitIDPattern             = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	pluginPublicNamePattern   = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	pluginArtifactNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

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
		units = append(units, ReleaseUnit{
			ID:               unit.ID,
			DisplayName:      unit.DisplayName,
			Paths:            append([]string(nil), unit.Paths...),
			WorkingDirectory: workingDirectory,
			TagPrefix:        unit.TagPrefix,
			Kind:             string(unit.Kind),
			ExecutorType:     string(unit.Executor.Type),
			Delivery:         string(delivery),
			Workflow:         unit.Executor.Workflow,
			Version:          state.Units[unit.ID].Version,
		})
		if unit.Plugin != nil {
			last := &units[len(units)-1]
			last.IsPlugin = true
			last.PluginName = unit.Plugin.Name
			last.PluginManifestPath = unit.Plugin.Manifest
			last.PluginAssetPrefix = unit.Plugin.AssetPrefix
			last.PluginBinaryName = unit.Plugin.BinaryName
		}
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
	}

	return nil
}

// CanonicalV2Config returns the normalized JSON bytes future writers should use.
func CanonicalV2Config(cfg V2ReleaseConfig) ([]byte, error) {
	for i := range cfg.Units {
		if cfg.Units[i].WorkingDirectory == "" {
			cfg.Units[i].WorkingDirectory = "."
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
	if err := validateV2UnitPluginMetadata(repositoryRoot, unit); err != nil {
		return err
	}
	return validateV2Executor(repositoryRoot, unit.ID, unit.Executor)
}

func validateV2UnitPluginMetadata(repositoryRoot string, unit V2Unit) error {
	if unit.Kind != "" && unit.Kind != UnitKindPlugin {
		return fmt.Errorf("v2 config unit %q has unknown kind %q", unit.ID, unit.Kind)
	}
	if unit.Kind == "" {
		if unit.Plugin != nil {
			return fmt.Errorf("v2 config unit %q plugin metadata requires kind %q", unit.ID, UnitKindPlugin)
		}
		return nil
	}
	if unit.Plugin == nil {
		return fmt.Errorf("v2 config unit %q kind %q requires plugin metadata", unit.ID, UnitKindPlugin)
	}
	if !strings.HasPrefix(unit.ID, "plugin-") {
		return fmt.Errorf("v2 config plugin unit %q id must start with plugin-", unit.ID)
	}
	expectedTagPrefix := unit.ID + "/v"
	if unit.TagPrefix != expectedTagPrefix {
		return fmt.Errorf("v2 config plugin unit %q tagPrefix must be %q", unit.ID, expectedTagPrefix)
	}
	if err := validatePluginName(unit.ID, unit.Plugin.Name); err != nil {
		return err
	}
	if err := validatePluginManifest(repositoryRoot, unit.ID, unit.Plugin.Manifest); err != nil {
		return err
	}
	if err := validatePluginArtifactName(unit.ID, "assetPrefix", unit.Plugin.AssetPrefix); err != nil {
		return err
	}
	if unit.Plugin.AssetPrefix != unit.ID {
		return fmt.Errorf("v2 config plugin unit %q assetPrefix must equal unit id %q", unit.ID, unit.ID)
	}
	if err := validatePluginArtifactName(unit.ID, "binaryName", unit.Plugin.BinaryName); err != nil {
		return err
	}
	return nil
}

func validatePluginName(unitID, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("v2 config plugin unit %q plugin.name is required", unitID)
	}
	if name != strings.TrimSpace(name) || !pluginPublicNamePattern.MatchString(name) {
		return fmt.Errorf("v2 config plugin unit %q plugin.name %q must match [a-z][a-z0-9-]*", unitID, name)
	}
	if strings.HasPrefix(name, "plugin-") {
		return fmt.Errorf("v2 config plugin unit %q plugin.name %q must not start with plugin-", unitID, name)
	}
	return nil
}

func validatePluginManifest(repositoryRoot, unitID, manifest string) error {
	if strings.TrimSpace(manifest) == "" {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest is required", unitID)
	}
	if manifest != strings.TrimSpace(manifest) {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest must not have leading or trailing whitespace", unitID)
	}
	if strings.Contains(manifest, `\`) {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q must use forward slashes", unitID, manifest)
	}
	if strings.HasPrefix(manifest, "/") || filepath.IsAbs(manifest) {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q must be repository-root-relative", unitID, manifest)
	}
	if strings.Contains(manifest, "://") || strings.ContainsAny(manifest, "?#@$`~{}[]!*") {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q must not contain URL, query, fragment, ref, or shell syntax", unitID, manifest)
	}
	clean := path.Clean(manifest)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != manifest {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q must be a clean repository-root-relative path", unitID, manifest)
	}
	if !strings.HasSuffix(manifest, "/manifest.json") && manifest != "manifest.json" {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q must end with manifest.json", unitID, manifest)
	}
	if repositoryRoot == "" {
		return nil
	}
	return validatePluginManifestAtRepositoryRoot(repositoryRoot, unitID, manifest)
}

func validatePluginManifestAtRepositoryRoot(repositoryRoot, unitID, manifest string) error {
	absoluteRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return fmt.Errorf("v2 config plugin unit %q repository root %q cannot be resolved: %w", unitID, repositoryRoot, err)
	}
	manifestPath := filepath.Join(absoluteRoot, filepath.FromSlash(manifest))
	info, err := os.Lstat(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q does not exist", unitID, manifest)
		}
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q cannot be inspected: %w", unitID, manifest, err)
	}
	if info.IsDir() {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q is a directory, expected a file", unitID, manifest)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return fmt.Errorf("v2 config plugin unit %q repository root %q cannot be resolved physically: %w", unitID, absoluteRoot, err)
	}
	resolvedManifest, err := filepath.EvalSymlinks(manifestPath)
	if err != nil {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q cannot be resolved physically: %w", unitID, manifest, err)
	}
	if !pathInside(resolvedRoot, resolvedManifest) {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q resolves outside repository root", unitID, manifest)
	}
	resolvedInfo, err := os.Stat(resolvedManifest)
	if err != nil {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q cannot be inspected after symlink resolution: %w", unitID, manifest, err)
	}
	if !resolvedInfo.Mode().IsRegular() {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q is not a regular file", unitID, manifest)
	}
	if filepath.Base(resolvedManifest) != "manifest.json" {
		return fmt.Errorf("v2 config plugin unit %q plugin.manifest %q must resolve to manifest.json", unitID, manifest)
	}
	return nil
}

func validatePluginArtifactName(unitID, field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("v2 config plugin unit %q plugin.%s is required", unitID, field)
	}
	if value != strings.TrimSpace(value) || !pluginArtifactNamePattern.MatchString(value) {
		return fmt.Errorf("v2 config plugin unit %q plugin.%s %q must match [a-z0-9][a-z0-9-]*", unitID, field, value)
	}
	if strings.Contains(value, ".") || strings.ContainsAny(value, `/\ `+"\t\r\n$`;&|<>") {
		return fmt.Errorf("v2 config plugin unit %q plugin.%s %q must be a conservative filename without slashes, extension, or shell syntax", unitID, field, value)
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

// IsValid reports whether the delivery type is a recognized release delivery
// value. V2 executable configs currently support github-actions only; local is
// retained as a known value so legacy V1 data and invalid V2 configs can be
// reported clearly.
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
