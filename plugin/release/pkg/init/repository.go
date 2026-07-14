package init

import (
	"os"
	"path/filepath"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type v2ReleasePair struct { //nolint:govet // The pair keeps config before its matching state.
	Config config.V2ReleaseConfig
	State  config.V2ReleaseState
}

func newV2ReleasePair(unit constructedV2Unit) v2ReleasePair {
	return v2ReleasePair{
		Config: config.V2ReleaseConfig{
			SchemaVersion: 2,
			Units:         []config.V2Unit{cloneV2Unit(unit.Unit)},
		},
		State: config.V2ReleaseState{
			SchemaVersion: 2,
			Units: map[string]config.V2UnitState{
				unit.Unit.ID: unit.State,
			},
		},
	}
}

func appendV2ReleaseUnit(current v2ReleasePair, unit constructedV2Unit) v2ReleasePair {
	units := make([]config.V2Unit, len(current.Config.Units), len(current.Config.Units)+1)
	for index := range current.Config.Units {
		units[index] = cloneV2Unit(current.Config.Units[index])
	}
	units = append(units, cloneV2Unit(unit.Unit))

	stateUnits := make(map[string]config.V2UnitState, len(current.State.Units)+1)
	for id, state := range current.State.Units {
		stateUnits[id] = state
	}
	stateUnits[unit.Unit.ID] = unit.State

	return v2ReleasePair{
		Config: config.V2ReleaseConfig{
			SchemaVersion: current.Config.SchemaVersion,
			Units:         units,
		},
		State: config.V2ReleaseState{
			SchemaVersion: current.State.SchemaVersion,
			Units:         stateUnits,
		},
	}
}

func cloneV2Unit(unit config.V2Unit) config.V2Unit {
	unit.Paths = append([]string(nil), unit.Paths...)
	if unit.Plugin != nil {
		pluginConfig := *unit.Plugin
		unit.Plugin = &pluginConfig
	}
	return unit
}

type v2PairLoadError struct {
	err  error
	part string
}

func (loadError *v2PairLoadError) Error() string {
	return loadError.err.Error()
}

type v2Repository struct {
	persister v2ReleasePairPersister
	root      string
}

func newV2Repository(root string) *v2Repository {
	return &v2Repository{
		root:      root,
		persister: newV2ReleasePairPersister(root),
	}
}

func (repository *v2Repository) Presence() v2RepositoryPresence {
	return v2RepositoryPresence{
		LegacyConfig: fileExists(filepath.Join(repository.root, legacyV1ConfigFileName)),
		Config:       config.V2ConfigExists(repository.root),
		State:        config.V2StateExists(repository.root),
	}
}

func (repository *v2Repository) LoadPair() (v2ReleasePair, error) {
	cfg, err := config.LoadV2Config(config.V2ConfigPath(repository.root))
	if err != nil {
		return v2ReleasePair{}, &v2PairLoadError{err: err, part: "config"}
	}
	state, err := config.LoadV2State(config.V2StatePath(repository.root))
	if err != nil {
		return v2ReleasePair{}, &v2PairLoadError{err: err, part: "state"}
	}
	return v2ReleasePair{Config: *cfg, State: *state}, nil
}

func (repository *v2Repository) ValidatePair(pair v2ReleasePair) error {
	return config.ValidateV2(repository.root, &pair.Config, &pair.State)
}

func (repository *v2Repository) PersistPair(pair v2ReleasePair) error {
	return repository.persister.Persist(pair)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
