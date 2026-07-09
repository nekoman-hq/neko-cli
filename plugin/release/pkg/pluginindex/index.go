package pluginindex

//lint:file-ignore fieldalignment JSON schema order mirrors the generated public index.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const (
	SchemaVersion     = 1
	DefaultRepository = "nekoman-hq/neko-cli"
)

// GenerateOptions configures plugin-index generation.
type GenerateOptions struct {
	Root       string
	Repository string
}

// WriteOptions configures index serialization.
type WriteOptions struct {
	Pretty bool
}

// Index is the future public plugin registry index.
//
//nolint:govet // JSON schema order is part of the generated artifact contract.
type Index struct {
	SchemaVersion int           `json:"schemaVersion"`
	Repository    string        `json:"repository"`
	Plugins       []PluginEntry `json:"plugins"`
}

// PluginEntry describes one installable plugin release unit.
type PluginEntry struct {
	Name        string `json:"name"`
	Unit        string `json:"unit"`
	Version     string `json:"version"`
	Tag         string `json:"tag"`
	TagPrefix   string `json:"tagPrefix"`
	Manifest    string `json:"manifest"`
	AssetPrefix string `json:"assetPrefix"`
	BinaryName  string `json:"binaryName"`
	Description string `json:"description"`
}

type pluginManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// Generate builds a deterministic plugin index from V2 plugin units and manifests.
func Generate(ctx context.Context, options GenerateOptions) (*Index, error) {
	root := strings.TrimSpace(options.Root)
	if root == "" {
		root = "."
	}
	repository := strings.TrimSpace(options.Repository)
	if repository == "" {
		repository = DefaultRepository
	}

	cfg, err := loadConfig(root)
	if err != nil {
		return nil, err
	}
	state, err := loadState(root)
	if err != nil {
		return nil, err
	}
	if state.Units == nil {
		return nil, fmt.Errorf("plugin index state units are missing")
	}

	if err := validateDuplicateUnitIDs(cfg.Units); err != nil {
		return nil, err
	}

	pluginNames := map[string]string{}
	tags := map[string]string{}
	entries := make([]PluginEntry, 0)
	for _, unit := range cfg.Units {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if unit.Kind != releaseconfig.UnitKindPlugin {
			continue
		}
		entry, err := buildPluginEntry(root, unit, state)
		if err != nil {
			return nil, err
		}
		if previousUnit, exists := pluginNames[entry.Name]; exists {
			return nil, fmt.Errorf("plugin index duplicate plugin name %q in units %q and %q", entry.Name, previousUnit, entry.Unit)
		}
		if previousUnit, exists := tags[entry.Tag]; exists {
			return nil, fmt.Errorf("plugin index duplicate tag %q in units %q and %q", entry.Tag, previousUnit, entry.Unit)
		}
		pluginNames[entry.Name] = entry.Unit
		tags[entry.Tag] = entry.Unit
		entries = append(entries, entry)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	return &Index{
		SchemaVersion: SchemaVersion,
		Repository:    repository,
		Plugins:       entries,
	}, nil
}

// Write serializes an index as stable, pretty JSON.
func Write(index *Index, writer io.Writer) error {
	return WriteWithOptions(index, writer, WriteOptions{Pretty: true})
}

// WriteWithOptions serializes an index as JSON.
func WriteWithOptions(index *Index, writer io.Writer, options WriteOptions) error {
	encoder := json.NewEncoder(writer)
	if options.Pretty {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(index)
}

func loadConfig(root string) (*releaseconfig.V2ReleaseConfig, error) {
	path := releaseconfig.V2ConfigPath(root)
	cfg, err := releaseconfig.LoadV2Config(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("plugin index config file is missing: %s", path)
		}
		return nil, fmt.Errorf("plugin index config invalid: %w", err)
	}
	return cfg, nil
}

func loadState(root string) (*releaseconfig.V2ReleaseState, error) {
	path := releaseconfig.V2StatePath(root)
	state, err := releaseconfig.LoadV2State(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("plugin index state file is missing: %s", path)
		}
		return nil, fmt.Errorf("plugin index state invalid: %w", err)
	}
	return state, nil
}

func validateDuplicateUnitIDs(units []releaseconfig.V2Unit) error {
	seen := map[string]struct{}{}
	for _, unit := range units {
		if _, exists := seen[unit.ID]; exists {
			return fmt.Errorf("plugin index duplicate unit id %q", unit.ID)
		}
		seen[unit.ID] = struct{}{}
	}
	return nil
}

func buildPluginEntry(root string, unit releaseconfig.V2Unit, state *releaseconfig.V2ReleaseState) (PluginEntry, error) {
	if unit.Plugin == nil {
		return PluginEntry{}, fmt.Errorf("plugin index unit %q has kind %q but no plugin metadata", unit.ID, unit.Kind)
	}
	unitState, ok := state.Units[unit.ID]
	if !ok {
		return PluginEntry{}, fmt.Errorf("plugin index plugin unit %q has no matching state entry", unit.ID)
	}
	version := strings.TrimSpace(unitState.Version)
	if _, err := semver.StrictNewVersion(version); err != nil {
		return PluginEntry{}, fmt.Errorf("plugin index plugin unit %q state version %q is not valid semver", unit.ID, unitState.Version)
	}

	manifest, err := readPluginManifest(root, unit.ID, unit.Plugin.Manifest)
	if err != nil {
		return PluginEntry{}, err
	}
	if manifest.Name != unit.Plugin.Name {
		return PluginEntry{}, fmt.Errorf("plugin index plugin unit %q manifest name %q does not match plugin.name %q", unit.ID, manifest.Name, unit.Plugin.Name)
	}
	if manifest.Version != version {
		return PluginEntry{}, fmt.Errorf("plugin index plugin unit %q manifest version %q does not match state version %q", unit.ID, manifest.Version, version)
	}

	return PluginEntry{
		Name:        unit.Plugin.Name,
		Unit:        unit.ID,
		Version:     version,
		Tag:         unit.TagPrefix + version,
		TagPrefix:   unit.TagPrefix,
		Manifest:    unit.Plugin.Manifest,
		AssetPrefix: unit.Plugin.AssetPrefix,
		BinaryName:  unit.Plugin.BinaryName,
		Description: manifest.Description,
	}, nil
}

func readPluginManifest(root, unitID, manifestPath string) (pluginManifest, error) {
	if strings.TrimSpace(manifestPath) == "" {
		return pluginManifest{}, fmt.Errorf("plugin index plugin unit %q plugin manifest path is required", unitID)
	}
	if filepath.IsAbs(manifestPath) || strings.HasPrefix(manifestPath, "/") || strings.Contains(manifestPath, `\`) {
		return pluginManifest{}, fmt.Errorf("plugin index plugin unit %q manifest %q must be repository-root-relative and use forward slashes", unitID, manifestPath)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(manifestPath)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != manifestPath {
		return pluginManifest{}, fmt.Errorf("plugin index plugin unit %q manifest %q must be a clean repository-root-relative path", unitID, manifestPath)
	}

	path := filepath.Join(root, filepath.FromSlash(manifestPath))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return pluginManifest{}, fmt.Errorf("plugin index plugin unit %q manifest file is missing: %s", unitID, manifestPath)
		}
		return pluginManifest{}, fmt.Errorf("plugin index plugin unit %q manifest %s cannot be read: %w", unitID, manifestPath, err)
	}
	var manifest pluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return pluginManifest{}, fmt.Errorf("plugin index plugin unit %q manifest %s is malformed: %w", unitID, manifestPath, err)
	}
	return manifest, nil
}
