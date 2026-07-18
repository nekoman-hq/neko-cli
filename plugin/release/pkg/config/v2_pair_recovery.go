package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	v2PairRecoveryFileName      = "release.pair-recovery.json"
	v2PairRecoverySchemaVersion = 1
)

type v2PairReplacementEvidence string

const (
	v2PairReplacementNotStarted v2PairReplacementEvidence = "not-started"
	v2PairReplacementPending    v2PairReplacementEvidence = "pending"
	v2PairReplacementConfirmed  v2PairReplacementEvidence = "confirmed"
)

type v2PairRestorationEvidence string

const (
	v2PairRestorationNotStarted v2PairRestorationEvidence = "not-started"
	v2PairRestorationPending    v2PairRestorationEvidence = "pending"
	v2PairRestorationConfirmed  v2PairRestorationEvidence = "confirmed"
	v2PairRestorationFailed     v2PairRestorationEvidence = "failed"
)

// V2PairRecoveryPath returns the repository-local V2 pair recovery evidence path.
func V2PairRecoveryPath(root string) string {
	return filepath.Join(root, V2Directory, v2PairRecoveryFileName)
}

type v2PairRecoveryEvidence struct {
	ConfigPath        string                    `json:"configPath"`
	StatePath         string                    `json:"statePath"`
	ConfigReplacement v2PairReplacementEvidence `json:"configReplacement"`
	StateReplacement  v2PairReplacementEvidence `json:"stateReplacement"`
	Restoration       v2PairRestorationEvidence `json:"restoration"`
	PriorConfig       v2PairRecoveryFile        `json:"priorConfig"`
	PriorState        v2PairRecoveryFile        `json:"priorState"`
	IntendedConfig    v2PairRecoveryFile        `json:"intendedConfig"`
	IntendedState     v2PairRecoveryFile        `json:"intendedState"`
	SchemaVersion     int                       `json:"schemaVersion"`
	Completed         bool                      `json:"completed"`
}

type v2PairRecoveryFile struct {
	SHA256 string      `json:"sha256,omitempty"`
	Data   []byte      `json:"data,omitempty"`
	Mode   os.FileMode `json:"mode,omitempty"`
	Exists bool        `json:"exists"`
}

func newV2PairRecoveryEvidence(root string, configSnapshot, stateSnapshot v2FileSnapshot, configData, stateData []byte) v2PairRecoveryEvidence {
	return v2PairRecoveryEvidence{
		SchemaVersion:     v2PairRecoverySchemaVersion,
		ConfigPath:        V2ConfigPath(root),
		StatePath:         V2StatePath(root),
		PriorConfig:       v2PairRecoveryFileFromSnapshot(configSnapshot),
		PriorState:        v2PairRecoveryFileFromSnapshot(stateSnapshot),
		IntendedConfig:    newV2PairRecoveryFile(configData, v2ReleaseFileMode),
		IntendedState:     newV2PairRecoveryFile(stateData, v2ReleaseFileMode),
		ConfigReplacement: v2PairReplacementNotStarted,
		StateReplacement:  v2PairReplacementNotStarted,
		Restoration:       v2PairRestorationNotStarted,
	}
}

func v2PairRecoveryFileFromSnapshot(snapshot v2FileSnapshot) v2PairRecoveryFile {
	if !snapshot.exists {
		return v2PairRecoveryFile{}
	}
	return newV2PairRecoveryFile(snapshot.data, snapshot.mode)
}

func newV2PairRecoveryFile(data []byte, mode os.FileMode) v2PairRecoveryFile {
	return v2PairRecoveryFile{
		Exists: true,
		Mode:   mode.Perm(),
		SHA256: sha256HexBytes(data),
		Data:   append([]byte(nil), data...),
	}
}

func (file v2PairRecoveryFile) snapshot() v2FileSnapshot {
	if !file.Exists {
		return v2FileSnapshot{}
	}
	return v2FileSnapshot{
		data:   append([]byte(nil), file.Data...),
		mode:   file.Mode.Perm(),
		exists: true,
	}
}

func (evidence v2PairRecoveryEvidence) validate(root string) error {
	if evidence.SchemaVersion != v2PairRecoverySchemaVersion {
		return fmt.Errorf("unsupported V2 pair recovery schema version %d", evidence.SchemaVersion)
	}
	if evidence.ConfigPath != V2ConfigPath(root) || evidence.StatePath != V2StatePath(root) {
		return fmt.Errorf("V2 pair recovery evidence does not match repository paths")
	}
	if err := validateV2PairRecoveryFile("prior config", evidence.PriorConfig, true); err != nil {
		return err
	}
	if err := validateV2PairRecoveryFile("prior state", evidence.PriorState, true); err != nil {
		return err
	}
	if err := validateV2PairRecoveryFile("intended config", evidence.IntendedConfig, false); err != nil {
		return err
	}
	if err := validateV2PairRecoveryFile("intended state", evidence.IntendedState, false); err != nil {
		return err
	}
	if err := validateV2PairReplacementEvidence("config replacement", evidence.ConfigReplacement); err != nil {
		return err
	}
	if err := validateV2PairReplacementEvidence("state replacement", evidence.StateReplacement); err != nil {
		return err
	}
	if err := validateV2PairRestorationEvidence(evidence.Restoration); err != nil {
		return err
	}
	if evidence.StateReplacement != v2PairReplacementNotStarted &&
		evidence.ConfigReplacement == v2PairReplacementNotStarted {
		return fmt.Errorf("state replacement evidence exists before config replacement evidence")
	}
	if evidence.Completed &&
		(evidence.ConfigReplacement != v2PairReplacementConfirmed ||
			evidence.StateReplacement != v2PairReplacementConfirmed) {
		return fmt.Errorf("completed V2 pair recovery evidence lacks confirmed replacements")
	}
	return nil
}

func validateV2PairRecoveryFile(label string, file v2PairRecoveryFile, allowMissing bool) error {
	if !file.Exists {
		if !allowMissing {
			return fmt.Errorf("%s recovery file must exist", label)
		}
		if file.Mode != 0 || file.SHA256 != "" || len(file.Data) > 0 {
			return fmt.Errorf("%s recovery file has data for a missing file", label)
		}
		return nil
	}
	if file.Mode == 0 || file.Mode.Perm() != file.Mode || file.Mode.Perm() > 0777 {
		return fmt.Errorf("%s recovery file has invalid mode %04o", label, file.Mode)
	}
	if file.SHA256 == "" {
		return fmt.Errorf("%s recovery file is missing sha256", label)
	}
	if got := sha256HexBytes(file.Data); got != file.SHA256 {
		return fmt.Errorf("%s recovery file sha256 mismatch", label)
	}
	return nil
}

func validateV2PairReplacementEvidence(label string, value v2PairReplacementEvidence) error {
	switch value {
	case v2PairReplacementNotStarted, v2PairReplacementPending, v2PairReplacementConfirmed:
		return nil
	default:
		return fmt.Errorf("unknown %s value %q", label, value)
	}
}

func validateV2PairRestorationEvidence(value v2PairRestorationEvidence) error {
	switch value {
	case v2PairRestorationNotStarted, v2PairRestorationPending, v2PairRestorationConfirmed, v2PairRestorationFailed:
		return nil
	default:
		return fmt.Errorf("unknown restoration value %q", value)
	}
}

type v2PairRecoveryStore struct {
	path string
	root string
}

func newV2PairRecoveryStore(root string) *v2PairRecoveryStore {
	if root == "" {
		return nil
	}
	return &v2PairRecoveryStore{root: root, path: V2PairRecoveryPath(root)}
}

func (store *v2PairRecoveryStore) LoadUnresolved() (*v2PairRecoveryEvidence, error) {
	if store == nil {
		return nil, nil
	}
	data, err := os.ReadFile(store.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read V2 pair recovery evidence %s: %w", store.path, err)
	}
	var evidence v2PairRecoveryEvidence
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&evidence); err != nil {
		return nil, &V2PairRecoveryError{manual: true, cause: fmt.Errorf("parse V2 pair recovery evidence %s: %w", store.path, err)}
	}
	if err := evidence.validate(store.root); err != nil {
		return nil, &V2PairRecoveryError{manual: true, cause: fmt.Errorf("validate V2 pair recovery evidence %s: %w", store.path, err)}
	}
	return &evidence, nil
}

// ValidateV2PairRecoveryReadiness performs a read-only check for unresolved
// pair-recovery evidence. It never closes, restores, or rewrites evidence.
func ValidateV2PairRecoveryReadiness(root string) error {
	store := newV2PairRecoveryStore(root)
	evidence, err := store.LoadUnresolved()
	if err != nil {
		return err
	}
	if evidence != nil {
		return fmt.Errorf("unresolved V2 pair recovery evidence is present")
	}
	return nil
}

func (store *v2PairRecoveryStore) CreatePairRecoveryEvidence(evidence v2PairRecoveryEvidence) error {
	if store == nil {
		return nil
	}
	return store.persist(evidence)
}

func (store *v2PairRecoveryStore) RecordConfigReplacementPending(evidence *v2PairRecoveryEvidence) error {
	if store == nil {
		return nil
	}
	evidence.ConfigReplacement = v2PairReplacementPending
	return store.persist(*evidence)
}

func (store *v2PairRecoveryStore) ConfirmConfigReplacement(evidence *v2PairRecoveryEvidence) error {
	if store == nil {
		return nil
	}
	evidence.ConfigReplacement = v2PairReplacementConfirmed
	return store.persist(*evidence)
}

func (store *v2PairRecoveryStore) RecordStateReplacementPending(evidence *v2PairRecoveryEvidence) error {
	if store == nil {
		return nil
	}
	evidence.StateReplacement = v2PairReplacementPending
	return store.persist(*evidence)
}

func (store *v2PairRecoveryStore) ConfirmStateReplacement(evidence *v2PairRecoveryEvidence) error {
	if store == nil {
		return nil
	}
	evidence.StateReplacement = v2PairReplacementConfirmed
	return store.persist(*evidence)
}

func (store *v2PairRecoveryStore) RecordOriginalPairRestorationPending(evidence *v2PairRecoveryEvidence) error {
	if store == nil {
		return nil
	}
	evidence.Restoration = v2PairRestorationPending
	return store.persist(*evidence)
}

func (store *v2PairRecoveryStore) ConfirmOriginalPairRestoration(evidence *v2PairRecoveryEvidence) error {
	if store == nil {
		return nil
	}
	evidence.Restoration = v2PairRestorationConfirmed
	return store.persist(*evidence)
}

func (store *v2PairRecoveryStore) RecordOriginalPairRestorationFailed(evidence *v2PairRecoveryEvidence) error {
	if store == nil {
		return nil
	}
	evidence.Restoration = v2PairRestorationFailed
	return store.persist(*evidence)
}

func (store *v2PairRecoveryStore) MarkPairRecoveryCompleted(evidence *v2PairRecoveryEvidence) error {
	if store == nil {
		return nil
	}
	evidence.Completed = true
	evidence.ConfigReplacement = v2PairReplacementConfirmed
	evidence.StateReplacement = v2PairReplacementConfirmed
	return store.persist(*evidence)
}

func (store *v2PairRecoveryStore) ClosePairRecoveryEvidence() error {
	if store == nil {
		return nil
	}
	if err := os.Remove(store.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove V2 pair recovery evidence %s: %w", store.path, err)
	}
	return nil
}

func (store *v2PairRecoveryStore) persist(evidence v2PairRecoveryEvidence) error {
	if err := evidence.validate(store.root); err != nil {
		return err
	}
	if err := AtomicWriteJSON(store.path, &evidence, 0644); err != nil {
		return fmt.Errorf("write V2 pair recovery evidence %s: %w", store.path, err)
	}
	return nil
}

type v2PairObservedFile struct {
	sha    string
	mode   os.FileMode
	exists bool
}

type v2PairRecoveryObservation struct {
	config            v2PairObservedFile
	state             v2PairObservedFile
	intendedPairValid bool
}

type v2PairRecoveryDecisionKind uint8

const (
	v2PairRecoveryAlreadyComplete v2PairRecoveryDecisionKind = iota + 1
	v2PairRecoveryRestoreOriginal
	v2PairRecoveryManual
)

type v2PairRecoveryDecision struct {
	reason string
	kind   v2PairRecoveryDecisionKind
}

func selectV2PairRecoveryOperation(evidence v2PairRecoveryEvidence, observation v2PairRecoveryObservation) v2PairRecoveryDecision {
	if observation.config.matches(evidence.IntendedConfig) &&
		observation.state.matches(evidence.IntendedState) &&
		observation.intendedPairValid {
		return v2PairRecoveryDecision{kind: v2PairRecoveryAlreadyComplete}
	}
	if observation.config.matches(evidence.PriorConfig) &&
		observation.state.matches(evidence.PriorState) {
		return v2PairRecoveryDecision{kind: v2PairRecoveryRestoreOriginal}
	}
	if observation.canRestore(evidence) {
		return v2PairRecoveryDecision{kind: v2PairRecoveryRestoreOriginal}
	}
	return v2PairRecoveryDecision{
		kind:   v2PairRecoveryManual,
		reason: "V2 pair recovery evidence conflicts with current config/state files",
	}
}

func (observation v2PairRecoveryObservation) canRestore(evidence v2PairRecoveryEvidence) bool {
	configRecoverable := observation.config.matches(evidence.PriorConfig) || observation.config.matches(evidence.IntendedConfig)
	stateRecoverable := observation.state.matches(evidence.PriorState) || observation.state.matches(evidence.IntendedState)
	return configRecoverable && stateRecoverable
}

func (observed v2PairObservedFile) matches(file v2PairRecoveryFile) bool {
	if observed.exists != file.Exists {
		return false
	}
	if !observed.exists {
		return true
	}
	return observed.sha == file.SHA256
}

// V2PairRecoveryError reports unresolved pair recovery evidence that cannot be resolved automatically.
type V2PairRecoveryError struct {
	cause  error
	manual bool
}

func (recoveryError *V2PairRecoveryError) Error() string {
	if recoveryError.manual {
		return recoveryError.cause.Error() + "; manual recovery required"
	}
	return recoveryError.cause.Error()
}

func (recoveryError *V2PairRecoveryError) Unwrap() error {
	return recoveryError.cause
}

func (recoveryError *V2PairRecoveryError) ManualRecoveryRequired() bool {
	return recoveryError.manual
}

func sha256HexBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
