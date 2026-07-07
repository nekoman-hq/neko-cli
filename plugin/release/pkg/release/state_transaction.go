package release

import (
	"fmt"
	"os"
	"path/filepath"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// StateSnapshot is an exact local snapshot of the V2 state file before a
// transaction writes a new unit version.
type StateSnapshot struct {
	Path    string
	Bytes   []byte
	Mode    os.FileMode
	Existed bool
}

// StateTransaction writes and restores the canonical V2 release state file.
type StateTransaction struct {
	RepositoryRoot string
	StatePath      string
	Snapshot       StateSnapshot
}

func NewStateTransaction(repositoryRoot string) *StateTransaction {
	return &StateTransaction{
		RepositoryRoot: repositoryRoot,
		StatePath:      releaseconfig.V2StatePath(repositoryRoot),
	}
}

func (st *StateTransaction) CaptureSnapshot() error {
	info, err := os.Stat(st.StatePath)
	if err != nil {
		if os.IsNotExist(err) {
			st.Snapshot = StateSnapshot{Path: st.StatePath, Existed: false, Mode: 0644}
			return nil
		}
		return fmt.Errorf("capture state snapshot %s: %w", st.StatePath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("capture state snapshot %s: path is a directory", st.StatePath)
	}
	data, err := os.ReadFile(st.StatePath)
	if err != nil {
		return fmt.Errorf("capture state snapshot %s: %w", st.StatePath, err)
	}
	st.Snapshot = StateSnapshot{
		Path:    st.StatePath,
		Bytes:   append([]byte(nil), data...),
		Mode:    info.Mode().Perm(),
		Existed: true,
	}
	return nil
}

func (st *StateTransaction) RestoreSnapshot() error {
	if st.Snapshot.Path == "" {
		return fmt.Errorf("state snapshot is missing")
	}
	if !st.Snapshot.Existed {
		if err := os.Remove(st.Snapshot.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("restore state snapshot %s: %w", st.Snapshot.Path, err)
		}
		return nil
	}
	return releaseconfig.AtomicWriteFile(st.Snapshot.Path, st.Snapshot.Bytes, st.Snapshot.Mode)
}

func (st *StateTransaction) WriteUnitVersion(unitID, nextVersion string) error {
	cfg, err := releaseconfig.LoadV2Config(releaseconfig.V2ConfigPath(st.RepositoryRoot))
	if err != nil {
		return err
	}
	currentState, err := releaseconfig.LoadV2State(st.StatePath)
	if err != nil {
		return err
	}
	nextState := cloneV2State(currentState)
	unitState, ok := nextState.Units[unitID]
	if !ok {
		return fmt.Errorf("v2 state is missing unit %q", unitID)
	}
	unitState.Version = nextVersion
	nextState.Units[unitID] = unitState
	if err := releaseconfig.ValidateV2(st.RepositoryRoot, cfg, nextState); err != nil {
		return fmt.Errorf("target v2 state is invalid: %w", err)
	}

	mode := os.FileMode(0644)
	if st.Snapshot.Mode != 0 {
		mode = st.Snapshot.Mode
	}
	if err := releaseconfig.AtomicWriteJSON(st.StatePath, nextState, mode); err != nil {
		return err
	}
	if _, err := releaseconfig.LoadV2Repository(st.RepositoryRoot); err != nil {
		return fmt.Errorf("v2 state validation after write failed: %w", err)
	}
	return nil
}

func (st *StateTransaction) RelativeStatePath() (string, error) {
	rel, err := filepath.Rel(st.RepositoryRoot, st.StatePath)
	if err != nil {
		return "", fmt.Errorf("state path cannot be related to repository root: %w", err)
	}
	return rel, nil
}

func cloneV2State(state *releaseconfig.V2ReleaseState) *releaseconfig.V2ReleaseState {
	clone := &releaseconfig.V2ReleaseState{
		SchemaVersion: state.SchemaVersion,
		Units:         make(map[string]releaseconfig.V2UnitState, len(state.Units)),
	}
	for id, unitState := range state.Units {
		clone.Units[id] = unitState
	}
	return clone
}
