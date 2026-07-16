package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// V2ReleasePair contains one complete config/state pair ready for validation or persistence.
type V2ReleasePair struct { //nolint:govet // Config precedes its matching mutable state.
	Config V2ReleaseConfig
	State  V2ReleaseState
}

// V2ReleasePairPersister persists one complete, already validated V2 pair.
type V2ReleasePairPersister struct {
	disk     v2PairPersistenceDisk
	recovery *v2PairRecoveryStore
	root     string
}

// NewV2ReleasePairPersister creates the canonical config/state pair writer for a repository.
func NewV2ReleasePairPersister(root string) V2ReleasePairPersister {
	return V2ReleasePairPersister{
		disk:     &osV2PairPersistenceDisk{root: root},
		recovery: newV2PairRecoveryStore(root),
		root:     root,
	}
}

// Persist records pair-recovery evidence before unsafe replacement. This is
// crash-recoverable paired persistence, not cross-file atomicity: a process or
// machine crash between the two renames can still expose a mixed pair.
func (persister V2ReleasePairPersister) Persist(pair V2ReleasePair) error {
	configForSerialization := pair.Config
	configForSerialization.Units = make([]V2Unit, len(pair.Config.Units))
	for index := range pair.Config.Units {
		configForSerialization.Units[index] = clonePairV2Unit(pair.Config.Units[index])
	}
	configData, err := CanonicalV2Config(configForSerialization)
	if err != nil {
		return fmt.Errorf("prepare config data: %w", err)
	}
	stateData, err := CanonicalV2State(pair.State)
	if err != nil {
		return fmt.Errorf("prepare state data: %w", err)
	}
	if createErr := persister.disk.CreateDirectory(); createErr != nil {
		return fmt.Errorf("create %s directory: %w", V2Directory, createErr)
	}
	if recoverErr := persister.recoverUnresolvedPair(); recoverErr != nil {
		return recoverErr
	}

	configSnapshot, err := persister.disk.CaptureConfig()
	if err != nil {
		return fmt.Errorf("capture existing V2 config: %w", err)
	}
	stateSnapshot, err := persister.disk.CaptureState()
	if err != nil {
		return fmt.Errorf("capture existing V2 state: %w", err)
	}
	recoveryEvidence := newV2PairRecoveryEvidence(persister.root, configSnapshot, stateSnapshot, configData, stateData)
	if persister.recovery != nil {
		if evidenceErr := persister.recovery.CreatePairRecoveryEvidence(recoveryEvidence); evidenceErr != nil {
			return evidenceErr
		}
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
	if writeStateErr := preparedState.Write(stateData, v2ReleaseFileMode); writeStateErr != nil {
		return fmt.Errorf("write V2 state temp file: %w", writeStateErr)
	}

	if persister.recovery != nil {
		if err := persister.recovery.RecordConfigReplacementPending(&recoveryEvidence); err != nil {
			return err
		}
	}
	if err := preparedConfig.Replace(); err != nil {
		return persister.rollback("replace V2 config", err, configSnapshot, stateSnapshot, &recoveryEvidence)
	}
	if persister.root != "" {
		if err := verifyV2PairFileBytes(V2ConfigPath(persister.root), configData); err != nil {
			return err
		}
	}
	if persister.recovery != nil {
		if err := persister.recovery.ConfirmConfigReplacement(&recoveryEvidence); err != nil {
			return err
		}
		if err := persister.recovery.RecordStateReplacementPending(&recoveryEvidence); err != nil {
			return err
		}
	}
	if err := preparedState.Replace(); err != nil {
		return persister.rollback("replace V2 state", err, configSnapshot, stateSnapshot, &recoveryEvidence)
	}
	if persister.root != "" {
		if err := verifyV2PairFileBytes(V2StatePath(persister.root), stateData); err != nil {
			return err
		}
	}
	if persister.recovery != nil {
		if err := persister.recovery.ConfirmStateReplacement(&recoveryEvidence); err != nil {
			return err
		}
	}
	if persister.root != "" {
		if err := validatePersistedV2Pair(persister.root, configData, stateData); err != nil {
			return err
		}
	}
	if persister.recovery != nil {
		if err := persister.recovery.MarkPairRecoveryCompleted(&recoveryEvidence); err != nil {
			return err
		}
		if err := persister.recovery.ClosePairRecoveryEvidence(); err != nil {
			return err
		}
	}
	return nil
}

func (persister V2ReleasePairPersister) rollback(
	operation string,
	cause error,
	configSnapshot v2FileSnapshot,
	stateSnapshot v2FileSnapshot,
	recoveryEvidence *v2PairRecoveryEvidence,
) error {
	if persister.recovery != nil && recoveryEvidence != nil {
		_ = persister.recovery.RecordOriginalPairRestorationPending(recoveryEvidence)
	}
	restorationFailures := make([]string, 0, 2)
	if err := persister.disk.RestoreConfig(configSnapshot); err != nil {
		restorationFailures = append(restorationFailures, fmt.Sprintf("restore V2 config: %v", err))
	}
	if err := persister.disk.RestoreState(stateSnapshot); err != nil {
		restorationFailures = append(restorationFailures, fmt.Sprintf("restore V2 state: %v", err))
	}
	if len(restorationFailures) > 0 {
		if persister.recovery != nil && recoveryEvidence != nil {
			_ = persister.recovery.RecordOriginalPairRestorationFailed(recoveryEvidence)
		}
		return &V2PairPersistenceError{
			operation:   operation,
			cause:       cause,
			restoration: strings.Join(restorationFailures, "; "),
		}
	}
	if persister.recovery != nil && recoveryEvidence != nil {
		_ = persister.recovery.ConfirmOriginalPairRestoration(recoveryEvidence)
		_ = persister.recovery.ClosePairRecoveryEvidence()
	}
	return fmt.Errorf("%s: %w; previous config/state pair restored", operation, cause)
}

func (persister V2ReleasePairPersister) recoverUnresolvedPair() error {
	if persister.recovery == nil {
		return nil
	}
	evidence, err := persister.recovery.LoadUnresolved()
	if err != nil {
		return err
	}
	if evidence == nil {
		return nil
	}
	observation, err := observeV2PairRecovery(persister.root, *evidence)
	if err != nil {
		return &V2PairRecoveryError{manual: true, cause: err}
	}
	decision := selectV2PairRecoveryOperation(*evidence, observation)
	switch decision.kind {
	case v2PairRecoveryAlreadyComplete:
		if err := persister.recovery.MarkPairRecoveryCompleted(evidence); err != nil {
			return err
		}
		if err := persister.recovery.ClosePairRecoveryEvidence(); err != nil {
			return err
		}
		return discardV2PairPreparedTemps(persister.root)
	case v2PairRecoveryRestoreOriginal:
		if err := persister.restoreOriginalPairFromEvidence(evidence); err != nil {
			return err
		}
		return discardV2PairPreparedTemps(persister.root)
	case v2PairRecoveryManual:
		return &V2PairRecoveryError{manual: true, cause: fmt.Errorf("%s", decision.reason)}
	default:
		return &V2PairRecoveryError{
			manual: true,
			cause:  fmt.Errorf("unknown V2 pair recovery decision %d", decision.kind),
		}
	}
}

func (persister V2ReleasePairPersister) restoreOriginalPairFromEvidence(evidence *v2PairRecoveryEvidence) error {
	if err := persister.recovery.RecordOriginalPairRestorationPending(evidence); err != nil {
		return err
	}
	restorationFailures := make([]string, 0, 2)
	if err := restoreV2File(evidence.ConfigPath, evidence.PriorConfig.snapshot()); err != nil {
		restorationFailures = append(restorationFailures, fmt.Sprintf("restore V2 config: %v", err))
	}
	if err := restoreV2File(evidence.StatePath, evidence.PriorState.snapshot()); err != nil {
		restorationFailures = append(restorationFailures, fmt.Sprintf("restore V2 state: %v", err))
	}
	if len(restorationFailures) > 0 {
		_ = persister.recovery.RecordOriginalPairRestorationFailed(evidence)
		return &V2PairRecoveryError{
			manual: true,
			cause:  fmt.Errorf("V2 pair recovery restoration failed (%s)", strings.Join(restorationFailures, "; ")),
		}
	}
	if err := verifyV2PairRecoveredSnapshot(evidence.ConfigPath, evidence.PriorConfig); err != nil {
		_ = persister.recovery.RecordOriginalPairRestorationFailed(evidence)
		return &V2PairRecoveryError{manual: true, cause: err}
	}
	if err := verifyV2PairRecoveredSnapshot(evidence.StatePath, evidence.PriorState); err != nil {
		_ = persister.recovery.RecordOriginalPairRestorationFailed(evidence)
		return &V2PairRecoveryError{manual: true, cause: err}
	}
	if err := persister.recovery.ConfirmOriginalPairRestoration(evidence); err != nil {
		return err
	}
	return persister.recovery.ClosePairRecoveryEvidence()
}

func observeV2PairRecovery(root string, evidence v2PairRecoveryEvidence) (v2PairRecoveryObservation, error) {
	intendedPairValid := false
	if err := validatePersistedV2PairBytes(root, evidence.IntendedConfig.Data, evidence.IntendedState.Data); err == nil {
		intendedPairValid = true
	}
	config, err := observeV2PairFile(evidence.ConfigPath)
	if err != nil {
		return v2PairRecoveryObservation{}, err
	}
	state, err := observeV2PairFile(evidence.StatePath)
	if err != nil {
		return v2PairRecoveryObservation{}, err
	}
	return v2PairRecoveryObservation{
		config:            config,
		state:             state,
		intendedPairValid: intendedPairValid,
	}, nil
}

func observeV2PairFile(path string) (v2PairObservedFile, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return v2PairObservedFile{}, nil
	}
	if err != nil {
		return v2PairObservedFile{}, fmt.Errorf("stat %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return v2PairObservedFile{}, fmt.Errorf("read %s: %w", path, err)
	}
	return v2PairObservedFile{
		exists: true,
		mode:   info.Mode().Perm(),
		sha:    sha256HexBytes(data),
	}, nil
}

func verifyV2PairFileBytes(path string, want []byte) error {
	if path == "" {
		return nil
	}
	got, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("verify V2 pair file %s: %w", path, err)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("verify V2 pair file %s: content mismatch", path)
	}
	return nil
}

func validatePersistedV2Pair(root string, configData, stateData []byte) error {
	if err := verifyV2PairFileBytes(V2ConfigPath(root), configData); err != nil {
		return err
	}
	if err := verifyV2PairFileBytes(V2StatePath(root), stateData); err != nil {
		return err
	}
	return validatePersistedV2PairBytes(root, configData, stateData)
}

func validatePersistedV2PairBytes(root string, configData, stateData []byte) error {
	var cfg V2ReleaseConfig
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return fmt.Errorf("parse intended V2 config: %w", err)
	}
	var state V2ReleaseState
	if err := json.Unmarshal(stateData, &state); err != nil {
		return fmt.Errorf("parse intended V2 state: %w", err)
	}
	if err := ValidateV2(root, &cfg, &state); err != nil {
		return err
	}
	return nil
}

func verifyV2PairRecoveredSnapshot(path string, want v2PairRecoveryFile) error {
	observed, err := observeV2PairFile(path)
	if err != nil {
		return err
	}
	if !observed.matches(want) {
		return fmt.Errorf("restored V2 pair file %s does not match recovery evidence", path)
	}
	if want.Exists && observed.mode != want.Mode.Perm() {
		return fmt.Errorf("restored V2 pair file %s mode = %04o, want %04o", path, observed.mode, want.Mode.Perm())
	}
	return nil
}

func discardV2PairPreparedTemps(root string) error {
	for _, pattern := range []string{
		filepath.Join(root, V2Directory, "."+V2ConfigFileName+".tmp-*"),
		filepath.Join(root, V2Directory, "."+V2StateFileName+".tmp-*"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("find V2 pair temp files %s: %w", pattern, err)
		}
		for _, match := range matches {
			if err := os.Remove(match); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove V2 pair temp file %s: %w", match, err)
			}
		}
	}
	return nil
}

// V2PairPersistenceError reports an incomplete rollback that requires manual recovery.
type V2PairPersistenceError struct {
	operation   string
	cause       error
	restoration string
}

func (persistenceError *V2PairPersistenceError) Error() string {
	return fmt.Sprintf(
		"%s: %v; rollback failed (%s); manual recovery required",
		persistenceError.operation,
		persistenceError.cause,
		persistenceError.restoration,
	)
}

func (persistenceError *V2PairPersistenceError) Unwrap() error {
	return persistenceError.cause
}

// ManualRecoveryRequired reports that at least one original pair file could not be restored.
func (persistenceError *V2PairPersistenceError) ManualRecoveryRequired() bool {
	return true
}

// RestorationFailed identifies this as an incomplete config/state restoration.
func (persistenceError *V2PairPersistenceError) RestorationFailed() bool {
	return true
}

type osV2PairPersistenceDisk struct {
	root string
}

func (disk *osV2PairPersistenceDisk) CreateDirectory() error {
	return os.MkdirAll(filepath.Join(disk.root, V2Directory), 0755)
}

func (disk *osV2PairPersistenceDisk) CaptureConfig() (v2FileSnapshot, error) {
	return captureV2File(V2ConfigPath(disk.root))
}

func (disk *osV2PairPersistenceDisk) CaptureState() (v2FileSnapshot, error) {
	return captureV2File(V2StatePath(disk.root))
}

func (disk *osV2PairPersistenceDisk) CreateConfigTemp() (v2TemporaryFile, error) {
	return CreateAtomicFileReplacement(V2ConfigPath(disk.root))
}

func (disk *osV2PairPersistenceDisk) CreateStateTemp() (v2TemporaryFile, error) {
	return CreateAtomicFileReplacement(V2StatePath(disk.root))
}

func (disk *osV2PairPersistenceDisk) RestoreConfig(snapshot v2FileSnapshot) error {
	return restoreV2File(V2ConfigPath(disk.root), snapshot)
}

func (disk *osV2PairPersistenceDisk) RestoreState(snapshot v2FileSnapshot) error {
	return restoreV2File(V2StatePath(disk.root), snapshot)
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
	return AtomicWriteFile(path, snapshot.data, snapshot.mode)
}

func clonePairV2Unit(unit V2Unit) V2Unit {
	unit.Paths = append([]string(nil), unit.Paths...)
	if unit.Plugin != nil {
		pluginConfig := *unit.Plugin
		unit.Plugin = &pluginConfig
	}
	return unit
}
