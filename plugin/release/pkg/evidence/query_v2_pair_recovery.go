package evidence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type v2PairRecoveryEvidence struct {
	ConfigPath        string             `json:"configPath"`
	StatePath         string             `json:"statePath"`
	ConfigReplacement string             `json:"configReplacement"`
	StateReplacement  string             `json:"stateReplacement"`
	Restoration       string             `json:"restoration"`
	PriorConfig       v2PairRecoveryFile `json:"priorConfig"`
	PriorState        v2PairRecoveryFile `json:"priorState"`
	IntendedConfig    v2PairRecoveryFile `json:"intendedConfig"`
	IntendedState     v2PairRecoveryFile `json:"intendedState"`
	SchemaVersion     int                `json:"schemaVersion"`
	Completed         bool               `json:"completed"`
}

type v2PairRecoveryFile struct {
	SHA256 string      `json:"sha256,omitempty"`
	Data   []byte      `json:"data,omitempty"`
	Mode   os.FileMode `json:"mode,omitempty"`
	Exists bool        `json:"exists"`
}

func inspectV2PairRecoveryEvidence(root string) ([]EvidenceRecord, []EvidenceDiagnostic) {
	path := releaseconfig.V2PairRecoveryPath(root)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []EvidenceDiagnostic{unreadableEvidenceDiagnostic(FamilyV2PairRecovery, path)}
	}
	var diagnostics []EvidenceDiagnostic
	var evidence v2PairRecoveryEvidence
	if !decodeEvidenceJSON(FamilyV2PairRecovery, path, data, &evidence, &diagnostics) {
		return nil, diagnostics
	}
	if evidence.SchemaVersion != 1 {
		return nil, []EvidenceDiagnostic{unsupportedEvidenceDiagnostic(FamilyV2PairRecovery, path)}
	}
	if err := validateV2PairRecoveryEvidence(root, evidence); err != nil {
		return nil, []EvidenceDiagnostic{invalidEvidenceDiagnostic(FamilyV2PairRecovery, path)}
	}
	classification, automatic, manual, guidance := classifyV2PairRecovery(root, evidence)
	return []EvidenceRecord{{
		Family:                FamilyV2PairRecovery,
		Identity:              sha256Hex(data),
		Owner:                 "v2 pair recovery evidence",
		State:                 v2PairRecoveryState(evidence),
		PendingAction:         v2PairPendingAction(evidence),
		Classification:        classification,
		SafeToResume:          automatic,
		AutomaticContinuation: automatic,
		ManualRecovery:        manual,
		LifecycleAllowed:      evidence.Completed && classification == ClassificationCompleted,
		LifecycleOperation:    lifecycleOperation(evidence.Completed && classification == ClassificationCompleted),
		Guidance:              guidance,
		Path:                  path,
		DigestSHA256:          sha256Hex(data),
	}}, nil
}

func validateV2PairRecoveryEvidence(root string, evidence v2PairRecoveryEvidence) error {
	if evidence.ConfigPath != releaseconfig.V2ConfigPath(root) || evidence.StatePath != releaseconfig.V2StatePath(root) {
		return fmt.Errorf("path mismatch")
	}
	for label, value := range map[string]string{
		"config replacement": evidence.ConfigReplacement,
		"state replacement":  evidence.StateReplacement,
	} {
		if value != "not-started" && value != "pending" && value != "confirmed" {
			return fmt.Errorf("%s invalid", label)
		}
	}
	if evidence.Restoration != "not-started" && evidence.Restoration != "pending" && evidence.Restoration != "confirmed" && evidence.Restoration != "failed" {
		return fmt.Errorf("restoration invalid")
	}
	for _, file := range []v2PairRecoveryFile{evidence.PriorConfig, evidence.PriorState, evidence.IntendedConfig, evidence.IntendedState} {
		if file.Exists && file.SHA256 != sha256Hex(file.Data) {
			return fmt.Errorf("hash mismatch")
		}
	}
	if evidence.Completed && (evidence.ConfigReplacement != "confirmed" || evidence.StateReplacement != "confirmed") {
		return fmt.Errorf("completed without confirmed replacements")
	}
	return nil
}

func classifyV2PairRecovery(root string, evidence v2PairRecoveryEvidence) (string, bool, bool, string) {
	if evidence.Restoration == "failed" {
		return ClassificationManualRecoveryRequired, false, true, "V2 pair restoration failed. Manual recovery is required."
	}
	intendedConfig := observedFileMatches(releaseconfig.V2ConfigPath(root), evidence.IntendedConfig)
	intendedState := observedFileMatches(releaseconfig.V2StatePath(root), evidence.IntendedState)
	if evidence.Completed || (intendedConfig && intendedState && intendedPairBytesValid(root, evidence)) {
		return ClassificationCompleted, true, false, "Current config/state match the intended pair; the pair owner can close evidence."
	}
	if observedFileRecoverable(releaseconfig.V2ConfigPath(root), evidence.PriorConfig, evidence.IntendedConfig) &&
		observedFileRecoverable(releaseconfig.V2StatePath(root), evidence.PriorState, evidence.IntendedState) {
		return ClassificationResumable, true, false, "Pair evidence can be resolved by the V2 pair owner."
	}
	return ClassificationConflicting, false, true, "Current config/state conflict with pair evidence. Manual recovery is required."
}

func v2PairRecoveryState(evidence v2PairRecoveryEvidence) string {
	if evidence.Completed {
		return "completed"
	}
	return strings.Join([]string{
		"config=" + evidence.ConfigReplacement,
		"state=" + evidence.StateReplacement,
		"restoration=" + evidence.Restoration,
	}, " ")
}

func v2PairPendingAction(evidence v2PairRecoveryEvidence) string {
	switch {
	case evidence.Restoration == "pending":
		return "restore-original-pair"
	case evidence.ConfigReplacement == "pending":
		return "replace-config"
	case evidence.StateReplacement == "pending":
		return "replace-state"
	case evidence.Completed:
		return "close-evidence"
	default:
		return "resolve-pair"
	}
}

func intendedPairBytesValid(root string, evidence v2PairRecoveryEvidence) bool {
	var cfg releaseconfig.V2ReleaseConfig
	var state releaseconfig.V2ReleaseState
	if json.Unmarshal(evidence.IntendedConfig.Data, &cfg) != nil || json.Unmarshal(evidence.IntendedState.Data, &state) != nil {
		return false
	}
	return releaseconfig.ValidateV2(root, &cfg, &state) == nil
}

func observedFileRecoverable(path string, prior, intended v2PairRecoveryFile) bool {
	return observedFileMatches(path, prior) || observedFileMatches(path, intended)
}

func observedFileMatches(path string, want v2PairRecoveryFile) bool {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return !want.Exists
	}
	if err != nil || !want.Exists {
		return false
	}
	return sha256Hex(data) == want.SHA256
}
