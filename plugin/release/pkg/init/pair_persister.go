package init

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const v2ReleaseFileMode os.FileMode = 0644

type v2TemporaryFile interface {
	Write(data []byte, mode os.FileMode) error
	Replace() error
	Discard()
}

type v2FileSnapshot struct {
	data   []byte
	mode   os.FileMode
	exists bool
}

type v2PairPersistenceDisk interface {
	CreateDirectory() error
	CaptureConfig() (v2FileSnapshot, error)
	CaptureState() (v2FileSnapshot, error)
	CreateConfigTemp() (v2TemporaryFile, error)
	CreateStateTemp() (v2TemporaryFile, error)
	RestoreConfig(snapshot v2FileSnapshot) error
	RestoreState(snapshot v2FileSnapshot) error
}

type v2ReleasePairPersister struct {
	disk v2PairPersistenceDisk
}

func newV2ReleasePairPersister(root string) v2ReleasePairPersister {
	return v2ReleasePairPersister{disk: &osV2PairPersistenceDisk{root: root}}
}

// Persist prepares both files before replacing either one. If either replace
// fails it attempts to restore both prior snapshots. This is rollback-backed
// paired persistence, not cross-file atomicity: a process or machine crash
// between the two renames can still leave a mixed pair.
func (persister v2ReleasePairPersister) Persist(pair v2ReleasePair) error {
	configForSerialization := pair.Config
	configForSerialization.Units = make([]config.V2Unit, len(pair.Config.Units))
	for index := range pair.Config.Units {
		configForSerialization.Units[index] = cloneV2Unit(pair.Config.Units[index])
	}
	configData, err := config.CanonicalV2Config(configForSerialization)
	if err != nil {
		return fmt.Errorf("prepare config data: %w", err)
	}
	stateData, err := config.CanonicalV2State(pair.State)
	if err != nil {
		return fmt.Errorf("prepare state data: %w", err)
	}
	if createErr := persister.disk.CreateDirectory(); createErr != nil {
		return fmt.Errorf("create %s directory: %w", config.V2Directory, createErr)
	}

	configSnapshot, err := persister.disk.CaptureConfig()
	if err != nil {
		return fmt.Errorf("capture existing V2 config: %w", err)
	}
	stateSnapshot, err := persister.disk.CaptureState()
	if err != nil {
		return fmt.Errorf("capture existing V2 state: %w", err)
	}

	preparedConfig, err := persister.disk.CreateConfigTemp()
	if err != nil {
		return fmt.Errorf("create V2 config temp file: %w", err)
	}
	defer preparedConfig.Discard()
	if writeConfigErr := preparedConfig.Write(configData, v2ReleaseFileMode); writeConfigErr != nil {
		return fmt.Errorf("write V2 config temp file: %w", writeConfigErr)
	}
	preparedState, err := persister.disk.CreateStateTemp()
	if err != nil {
		return fmt.Errorf("create V2 state temp file: %w", err)
	}
	defer preparedState.Discard()
	if err := preparedState.Write(stateData, v2ReleaseFileMode); err != nil {
		return fmt.Errorf("write V2 state temp file: %w", err)
	}

	if err := preparedConfig.Replace(); err != nil {
		return persister.rollback("replace V2 config", err, configSnapshot, stateSnapshot)
	}
	if err := preparedState.Replace(); err != nil {
		return persister.rollback("replace V2 state", err, configSnapshot, stateSnapshot)
	}
	return nil
}

func (persister v2ReleasePairPersister) rollback(
	operation string,
	cause error,
	configSnapshot v2FileSnapshot,
	stateSnapshot v2FileSnapshot,
) error {
	restorationFailures := make([]string, 0, 2)
	if err := persister.disk.RestoreConfig(configSnapshot); err != nil {
		restorationFailures = append(restorationFailures, fmt.Sprintf("restore V2 config: %v", err))
	}
	if err := persister.disk.RestoreState(stateSnapshot); err != nil {
		restorationFailures = append(restorationFailures, fmt.Sprintf("restore V2 state: %v", err))
	}
	if len(restorationFailures) > 0 {
		return &v2PairPersistenceError{
			operation:   operation,
			cause:       cause,
			restoration: strings.Join(restorationFailures, "; "),
		}
	}
	return fmt.Errorf("%s: %w; previous config/state pair restored", operation, cause)
}

type v2PairPersistenceError struct {
	operation   string
	cause       error
	restoration string
}

func (persistenceError *v2PairPersistenceError) Error() string {
	return fmt.Sprintf(
		"%s: %v; rollback failed (%s); manual recovery required",
		persistenceError.operation,
		persistenceError.cause,
		persistenceError.restoration,
	)
}

func (persistenceError *v2PairPersistenceError) Unwrap() error {
	return persistenceError.cause
}

type osV2PairPersistenceDisk struct {
	root string
}

func (disk *osV2PairPersistenceDisk) CreateDirectory() error {
	return os.MkdirAll(filepath.Join(disk.root, config.V2Directory), 0755)
}

func (disk *osV2PairPersistenceDisk) CaptureConfig() (v2FileSnapshot, error) {
	return captureV2File(config.V2ConfigPath(disk.root))
}

func (disk *osV2PairPersistenceDisk) CaptureState() (v2FileSnapshot, error) {
	return captureV2File(config.V2StatePath(disk.root))
}

func (disk *osV2PairPersistenceDisk) CreateConfigTemp() (v2TemporaryFile, error) {
	return config.CreateAtomicFileReplacement(config.V2ConfigPath(disk.root))
}

func (disk *osV2PairPersistenceDisk) CreateStateTemp() (v2TemporaryFile, error) {
	return config.CreateAtomicFileReplacement(config.V2StatePath(disk.root))
}

func (disk *osV2PairPersistenceDisk) RestoreConfig(snapshot v2FileSnapshot) error {
	return restoreV2File(config.V2ConfigPath(disk.root), snapshot)
}

func (disk *osV2PairPersistenceDisk) RestoreState(snapshot v2FileSnapshot) error {
	return restoreV2File(config.V2StatePath(disk.root), snapshot)
}

func captureV2File(path string) (v2FileSnapshot, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return v2FileSnapshot{}, nil
	}
	if err != nil {
		return v2FileSnapshot{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return v2FileSnapshot{}, err
	}
	return v2FileSnapshot{
		data:   append([]byte(nil), data...),
		mode:   info.Mode().Perm(),
		exists: true,
	}, nil
}

func restoreV2File(path string, snapshot v2FileSnapshot) error {
	if !snapshot.exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return config.AtomicWriteFile(path, snapshot.data, snapshot.mode)
}
