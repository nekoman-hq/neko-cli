package pluginindex

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestPluginIndexQueryUseCaseBuildsDeterministicTypedIndexWithoutMutation(t *testing.T) {
	cfg := &releaseconfig.V2ReleaseConfig{
		SchemaVersion: 2,
		Units: []releaseconfig.V2Unit{
			pluginIndexQueryUnit("plugin-ui", "plugin-ui/v", "ui", "plugin/ui/manifest.json"),
			{ID: "cli"},
			pluginIndexQueryUnit("plugin-release", "plugin-release/v", "release", "plugin/release/manifest.json"),
		},
	}
	state := &releaseconfig.V2ReleaseState{
		SchemaVersion: 2,
		Units: map[string]releaseconfig.V2UnitState{
			"plugin-ui":      {Version: "1.0.0"},
			"cli":            {Version: "3.0.2"},
			"plugin-release": {Version: "4.0.2"},
		},
	}
	sources := &fakePluginIndexSourceReader{
		cfg:   cfg,
		state: state,
		manifests: map[string]pluginManifest{
			"plugin-ui":      {Name: "ui", Version: "1.0.0", Description: "UI plugin"},
			"plugin-release": {Name: "release", Version: "4.0.2", Description: "Release plugin"},
		},
	}
	configBefore := clonePluginIndexConfig(t, cfg)
	stateBefore := clonePluginIndexState(t, state)
	useCase := pluginIndexQueryUseCase{sources: sources}

	index, err := useCase.Query(context.Background(), GenerateOptions{Root: " repo ", Repository: " example/project "})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if index.Repository != "example/project" || len(index.Plugins) != 2 {
		t.Fatalf("index = %#v", index)
	}
	if index.Plugins[0].Name != "release" || index.Plugins[1].Name != "ui" {
		t.Fatalf("plugins not sorted deterministically: %#v", index.Plugins)
	}
	if sources.configCalls != 1 || sources.stateCalls != 1 || sources.configRoot != "repo" || sources.stateRoot != "repo" {
		t.Fatalf("source calls/roots = %#v", sources)
	}
	if !reflect.DeepEqual(sources.manifestUnits, []string{"plugin-ui", "plugin-release"}) {
		t.Fatalf("manifest reads = %#v", sources.manifestUnits)
	}
	if !reflect.DeepEqual(cfg, configBefore) || !reflect.DeepEqual(state, stateBefore) {
		t.Fatalf("query mutated source data\nconfig=%#v\nstate=%#v", cfg, state)
	}
}

func TestPluginIndexQueryUseCaseStopsAtFocusedReadAndValidationFailures(t *testing.T) {
	t.Run("config", func(t *testing.T) {
		sources := &fakePluginIndexSourceReader{configErr: errors.New("config failed")}
		_, err := (pluginIndexQueryUseCase{sources: sources}).Query(context.Background(), GenerateOptions{})
		if err == nil || err.Error() != "config failed" {
			t.Fatalf("Query error = %v", err)
		}
		if sources.configCalls != 1 || sources.stateCalls != 0 || len(sources.manifestUnits) != 0 {
			t.Fatalf("later reads occurred: %#v", sources)
		}
	})

	t.Run("state", func(t *testing.T) {
		sources := &fakePluginIndexSourceReader{cfg: &releaseconfig.V2ReleaseConfig{}, stateErr: errors.New("state failed")}
		_, err := (pluginIndexQueryUseCase{sources: sources}).Query(context.Background(), GenerateOptions{})
		if err == nil || err.Error() != "state failed" {
			t.Fatalf("Query error = %v", err)
		}
		if sources.stateCalls != 1 || len(sources.manifestUnits) != 0 {
			t.Fatalf("manifest read after state failure: %#v", sources)
		}
	})

	t.Run("duplicate unit", func(t *testing.T) {
		unit := pluginIndexQueryUnit("plugin-release", "plugin-release/v", "release", "plugin/release/manifest.json")
		sources := &fakePluginIndexSourceReader{
			cfg:   &releaseconfig.V2ReleaseConfig{Units: []releaseconfig.V2Unit{unit, unit}},
			state: &releaseconfig.V2ReleaseState{Units: map[string]releaseconfig.V2UnitState{}},
		}
		_, err := (pluginIndexQueryUseCase{sources: sources}).Query(context.Background(), GenerateOptions{})
		if err == nil || !strings.Contains(err.Error(), `duplicate unit id "plugin-release"`) {
			t.Fatalf("Query error = %v", err)
		}
		if len(sources.manifestUnits) != 0 {
			t.Fatalf("manifest read after duplicate unit failure: %#v", sources.manifestUnits)
		}
	})

	t.Run("candidate", func(t *testing.T) {
		unit := pluginIndexQueryUnit("plugin-release", "plugin-release/v", "release", "plugin/release/manifest.json")
		sources := &fakePluginIndexSourceReader{
			cfg:   &releaseconfig.V2ReleaseConfig{Units: []releaseconfig.V2Unit{unit}},
			state: &releaseconfig.V2ReleaseState{Units: map[string]releaseconfig.V2UnitState{}},
		}
		_, err := (pluginIndexQueryUseCase{sources: sources}).Query(context.Background(), GenerateOptions{})
		if err == nil || !strings.Contains(err.Error(), "has no matching state entry") {
			t.Fatalf("Query error = %v", err)
		}
		if len(sources.manifestUnits) != 0 {
			t.Fatalf("manifest read after candidate failure: %#v", sources.manifestUnits)
		}
	})

	t.Run("manifest", func(t *testing.T) {
		sources := pluginIndexQuerySources()
		sources.manifestErr = map[string]error{"plugin-release": errors.New("manifest failed")}
		_, err := (pluginIndexQueryUseCase{sources: sources}).Query(context.Background(), GenerateOptions{})
		if err == nil || err.Error() != "manifest failed" {
			t.Fatalf("Query error = %v", err)
		}
		if !reflect.DeepEqual(sources.manifestUnits, []string{"plugin-release"}) {
			t.Fatalf("later manifest read after failure: %#v", sources.manifestUnits)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		sources := pluginIndexQuerySources()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := (pluginIndexQueryUseCase{sources: sources}).Query(ctx, GenerateOptions{})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Query error = %v, want context.Canceled", err)
		}
		if len(sources.manifestUnits) != 0 {
			t.Fatalf("manifest read after cancellation: %#v", sources.manifestUnits)
		}
	})
}

func TestPluginIndexEntryBuildersArePureAndValidateTypedInput(t *testing.T) {
	unit := pluginIndexQueryUnit("plugin-release", "plugin-release/v", "release", "plugin/release/manifest.json")
	state := &releaseconfig.V2ReleaseState{Units: map[string]releaseconfig.V2UnitState{
		"plugin-release": {Version: "4.0.2"},
	}}

	candidate, err := buildPluginEntryCandidate(unit, state)
	if err != nil {
		t.Fatalf("buildPluginEntryCandidate: %v", err)
	}
	entry, err := completePluginEntry(candidate, pluginManifest{Name: "release", Version: "4.0.2", Description: "Release plugin"})
	if err != nil {
		t.Fatalf("completePluginEntry: %v", err)
	}
	if entry.Tag != "plugin-release/v4.0.2" || entry.Description != "Release plugin" {
		t.Fatalf("entry = %#v", entry)
	}

	unit.Plugin = nil
	if _, err := buildPluginEntryCandidate(unit, state); err == nil || !strings.Contains(err.Error(), "no plugin metadata") {
		t.Fatalf("missing metadata error = %v", err)
	}
	if _, err := completePluginEntry(candidate, pluginManifest{Name: "other", Version: "4.0.2"}); err == nil || !strings.Contains(err.Error(), "does not match plugin.name") {
		t.Fatalf("manifest mismatch error = %v", err)
	}
}

//nolint:govet // Test fake groups configured values before captured calls.
type fakePluginIndexSourceReader struct {
	manifestErr   map[string]error
	manifests     map[string]pluginManifest
	cfg           *releaseconfig.V2ReleaseConfig
	state         *releaseconfig.V2ReleaseState
	configErr     error
	stateErr      error
	configRoot    string
	stateRoot     string
	manifestUnits []string
	configCalls   int
	stateCalls    int
}

func (reader *fakePluginIndexSourceReader) LoadConfig(root string) (*releaseconfig.V2ReleaseConfig, error) {
	reader.configCalls++
	reader.configRoot = root
	return reader.cfg, reader.configErr
}

func (reader *fakePluginIndexSourceReader) LoadState(root string) (*releaseconfig.V2ReleaseState, error) {
	reader.stateCalls++
	reader.stateRoot = root
	return reader.state, reader.stateErr
}

func (reader *fakePluginIndexSourceReader) ReadManifest(_ string, unitID string, _ string) (pluginManifest, error) {
	reader.manifestUnits = append(reader.manifestUnits, unitID)
	if err := reader.manifestErr[unitID]; err != nil {
		return pluginManifest{}, err
	}
	return reader.manifests[unitID], nil
}

func pluginIndexQuerySources() *fakePluginIndexSourceReader {
	return &fakePluginIndexSourceReader{
		cfg: &releaseconfig.V2ReleaseConfig{Units: []releaseconfig.V2Unit{
			pluginIndexQueryUnit("plugin-release", "plugin-release/v", "release", "plugin/release/manifest.json"),
			pluginIndexQueryUnit("plugin-ui", "plugin-ui/v", "ui", "plugin/ui/manifest.json"),
		}},
		state: &releaseconfig.V2ReleaseState{Units: map[string]releaseconfig.V2UnitState{
			"plugin-release": {Version: "4.0.2"},
			"plugin-ui":      {Version: "1.0.0"},
		}},
		manifests: map[string]pluginManifest{
			"plugin-release": {Name: "release", Version: "4.0.2"},
			"plugin-ui":      {Name: "ui", Version: "1.0.0"},
		},
	}
}

func pluginIndexQueryUnit(id, tagPrefix, name, manifest string) releaseconfig.V2Unit {
	return releaseconfig.V2Unit{
		ID:        id,
		Kind:      releaseconfig.UnitKindPlugin,
		TagPrefix: tagPrefix,
		Plugin: &releaseconfig.V2Plugin{
			Name:        name,
			Manifest:    manifest,
			AssetPrefix: id,
			BinaryName:  id,
		},
	}
}

func clonePluginIndexConfig(t *testing.T, cfg *releaseconfig.V2ReleaseConfig) *releaseconfig.V2ReleaseConfig {
	t.Helper()
	cloned := *cfg
	cloned.Units = append([]releaseconfig.V2Unit(nil), cfg.Units...)
	for i, unit := range cfg.Units {
		if unit.Plugin != nil {
			plugin := *unit.Plugin
			cloned.Units[i].Plugin = &plugin
		}
	}
	return &cloned
}

func clonePluginIndexState(t *testing.T, state *releaseconfig.V2ReleaseState) *releaseconfig.V2ReleaseState {
	t.Helper()
	cloned := *state
	cloned.Units = make(map[string]releaseconfig.V2UnitState, len(state.Units))
	for id, unit := range state.Units {
		cloned.Units[id] = unit
	}
	return &cloned
}
