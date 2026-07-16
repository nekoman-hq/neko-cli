package evidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

type evidenceQueryUseCase struct{}

func newEvidenceQueryUseCase() evidenceQueryUseCase {
	return evidenceQueryUseCase{}
}

func (useCase evidenceQueryUseCase) Query(_ context.Context, request evidenceQueryRequest) (evidenceQueryResult, error) {
	var result evidenceQueryResult
	if includeEvidenceFamily(request.Family, FamilyReleaseExecution) {
		records, diagnostics, err := inspectReleaseExecutionJournals(request.RepositoryRoot)
		if err != nil {
			return evidenceQueryResult{}, err
		}
		result.Records = appendFilteredEvidenceRecords(result.Records, records, request.Unit)
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
	}
	if includeEvidenceFamily(request.Family, FamilyDispatch) {
		records, diagnostics, err := inspectDispatchJournals(request.RepositoryRoot)
		if err != nil {
			return evidenceQueryResult{}, err
		}
		result.Records = appendFilteredEvidenceRecords(result.Records, records, request.Unit)
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
	}
	if includeEvidenceFamily(request.Family, FamilyMigration) {
		record, diagnostics := inspectMigrationJournal(request.RepositoryRoot)
		result.Records = appendFilteredEvidenceRecords(result.Records, record, request.Unit)
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
	}
	if includeEvidenceFamily(request.Family, FamilyV1Compensation) {
		record, diagnostics, err := inspectV1CompensationEvidence(request.RepositoryRoot)
		if err != nil {
			return evidenceQueryResult{}, err
		}
		result.Records = appendFilteredEvidenceRecords(result.Records, record, request.Unit)
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
	}
	if includeEvidenceFamily(request.Family, FamilyV2PairRecovery) {
		record, diagnostics := inspectV2PairRecoveryEvidence(request.RepositoryRoot)
		result.Records = appendFilteredEvidenceRecords(result.Records, record, request.Unit)
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
	}
	sortEvidenceResult(&result)
	return result, nil
}

func includeEvidenceFamily(selected, family string) bool {
	return selected == "" || selected == family
}

func appendFilteredEvidenceRecords(target, records []EvidenceRecord, unit string) []EvidenceRecord {
	if strings.TrimSpace(unit) == "" {
		return append(target, records...)
	}
	for _, record := range records {
		if record.Unit == unit {
			target = append(target, record)
		}
	}
	return target
}

func inspectReleaseExecutionJournals(root string) ([]EvidenceRecord, []EvidenceDiagnostic, error) {
	dir, err := release.NewReleaseExecutionJournalStore(root).JournalDirectory()
	if err != nil {
		return nil, nil, err
	}
	paths, err := sortedJSONFiles(dir)
	if err != nil {
		return nil, nil, err
	}
	var records []EvidenceRecord
	var diagnostics []EvidenceDiagnostic
	for _, path := range paths {
		data, diagnostic, ok := readEvidenceBytes(FamilyReleaseExecution, path)
		if !ok {
			diagnostics = append(diagnostics, diagnostic)
			continue
		}
		var journal release.ReleaseExecutionJournal
		if !decodeEvidenceJSON(FamilyReleaseExecution, path, data, &journal, &diagnostics) {
			continue
		}
		if journal.SchemaVersion != 1 {
			diagnostics = append(diagnostics, unsupportedEvidenceDiagnostic(FamilyReleaseExecution, path))
			continue
		}
		if !journal.State.Valid() || !journal.PendingAction.Valid() || !safeEvidenceHash(journal.Identity.SHA256) {
			diagnostics = append(diagnostics, invalidEvidenceDiagnostic(FamilyReleaseExecution, path))
			continue
		}
		if filepath.Base(path) != journal.Identity.SHA256+".json" {
			diagnostics = append(diagnostics, conflictingEvidenceDiagnostic(FamilyReleaseExecution, path))
			continue
		}
		records = append(records, releaseExecutionRecord(path, data, journal))
	}
	return records, diagnostics, nil
}

func releaseExecutionRecord(path string, data []byte, journal release.ReleaseExecutionJournal) EvidenceRecord {
	classification, safeToResume, automatic, manual, guidance := classifyReleaseExecution(journal)
	return EvidenceRecord{
		Family:                FamilyReleaseExecution,
		Identity:              journal.Identity.SHA256,
		Owner:                 "release execution journal",
		Unit:                  journal.UnitID,
		Version:               journal.NextVersion,
		Tag:                   journal.Tag,
		State:                 string(journal.State),
		PendingAction:         string(journal.PendingAction),
		Classification:        classification,
		SafeToResume:          safeToResume,
		AutomaticContinuation: automatic,
		ManualRecovery:        manual,
		LifecycleAllowed:      classification == ClassificationCompleted,
		LifecycleOperation:    lifecycleOperation(classification == ClassificationCompleted),
		Guidance:              guidance,
		Path:                  path,
		DigestSHA256:          sha256Hex(data),
		CreatedAt:             formatEvidenceTime(journal.CreatedAt.String()),
		UpdatedAt:             formatEvidenceTime(journal.UpdatedAt.String()),
	}
}

func classifyReleaseExecution(journal release.ReleaseExecutionJournal) (string, bool, bool, bool, string) {
	if journal.State == release.ReleaseExecutionHandoffReady {
		return ClassificationCompleted, false, false, false, "Release execution was handed off. Keep dispatch evidence for audit or inspect before archival."
	}
	if journal.PendingAction == release.ReleaseExecutionPendingPushReleaseCommit ||
		journal.PendingAction == release.ReleaseExecutionPendingPushUnitTag {
		return ClassificationUncertain, false, false, true, "A push was marked pending. Do not infer remote state or retry blindly."
	}
	if journal.PendingAction != release.ReleaseExecutionPendingNone &&
		journal.PendingAction != release.ReleaseExecutionPendingApplyMaterialization {
		return ClassificationActive, false, false, true, "A local mutation was marked pending. Inspect before continuing."
	}
	if journal.ReleaseCommitSHA != "" {
		switch journal.State {
		case release.ReleaseExecutionCommitCreated, release.ReleaseExecutionTagCreated, release.ReleaseExecutionTagPushed:
			return ClassificationResumable, true, true, false, "Existing resume policy can continue after local checks."
		case release.ReleaseExecutionCommitPushed, release.ReleaseExecutionDispatchJournalPrepared:
			return ClassificationManualRecoveryRequired, false, false, true, "Push evidence is incomplete. Manual inspection is required before continuation."
		}
	}
	return ClassificationActive, false, false, false, "No terminal handoff is recorded."
}

func inspectDispatchJournals(root string) ([]EvidenceRecord, []EvidenceDiagnostic, error) {
	dir, err := release.NewDispatchJournalStore(root).JournalDirectory()
	if err != nil {
		return nil, nil, err
	}
	paths, err := sortedJSONFiles(dir)
	if err != nil {
		return nil, nil, err
	}
	var records []EvidenceRecord
	var diagnostics []EvidenceDiagnostic
	for _, path := range paths {
		data, diagnostic, ok := readEvidenceBytes(FamilyDispatch, path)
		if !ok {
			diagnostics = append(diagnostics, diagnostic)
			continue
		}
		var journal release.DispatchJournal
		if !decodeEvidenceJSON(FamilyDispatch, path, data, &journal, &diagnostics) {
			continue
		}
		if journal.SchemaVersion != 1 {
			diagnostics = append(diagnostics, unsupportedEvidenceDiagnostic(FamilyDispatch, path))
			continue
		}
		if !journal.State.Valid() || !safeEvidenceHash(journal.Identity.SHA256) {
			diagnostics = append(diagnostics, invalidEvidenceDiagnostic(FamilyDispatch, path))
			continue
		}
		if filepath.Base(path) != journal.Identity.SHA256+".json" {
			diagnostics = append(diagnostics, conflictingEvidenceDiagnostic(FamilyDispatch, path))
			continue
		}
		records = append(records, dispatchRecord(path, data, journal))
	}
	return records, diagnostics, nil
}

func dispatchRecord(path string, data []byte, journal release.DispatchJournal) EvidenceRecord {
	classification, manual, guidance := classifyDispatch(journal.State)
	return EvidenceRecord{
		Family:                FamilyDispatch,
		Identity:              journal.Identity.SHA256,
		Owner:                 "dispatch journal",
		Unit:                  journal.UnitID,
		Version:               journal.Version,
		Tag:                   journal.Tag,
		State:                 string(journal.State),
		Classification:        classification,
		SafeToResume:          journal.State == release.DispatchJournalPrepared || journal.State == release.DispatchJournalAccepted,
		AutomaticContinuation: journal.State == release.DispatchJournalPrepared || journal.State == release.DispatchJournalAccepted,
		ManualRecovery:        manual,
		Guidance:              guidance,
		Path:                  path,
		DigestSHA256:          sha256Hex(data),
		CreatedAt:             formatEvidenceTime(journal.CreatedAt.String()),
		UpdatedAt:             formatEvidenceTime(journal.UpdatedAt.String()),
	}
}

func classifyDispatch(state release.DispatchJournalState) (string, bool, string) {
	switch state {
	case release.DispatchJournalPrepared:
		return ClassificationActive, false, "Dispatch request is prepared locally and has not been sent."
	case release.DispatchJournalRequestStarted:
		return ClassificationUncertain, true, "Dispatch request started. Do not retry without manual inspection."
	case release.DispatchJournalAccepted:
		return ClassificationCompleted, false, "GitHub accepted the workflow dispatch request."
	case release.DispatchJournalRejected:
		return ClassificationTerminal, true, "GitHub rejected the workflow dispatch request. Manual recovery is required."
	case release.DispatchJournalUnknown:
		return ClassificationUncertain, true, "Dispatch outcome is unknown. Do not retry blindly."
	default:
		return ClassificationCorrupt, true, "Dispatch state is invalid."
	}
}

func inspectV1CompensationEvidence(root string) ([]EvidenceRecord, []EvidenceDiagnostic, error) {
	path, err := v1CompensationEvidencePath(root)
	if err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, []EvidenceDiagnostic{unreadableEvidenceDiagnostic(FamilyV1Compensation, path)}, nil
	}
	var diagnostics []EvidenceDiagnostic
	var evidence release.V1CompensationEvidence
	if !decodeEvidenceJSON(FamilyV1Compensation, path, data, &evidence, &diagnostics) {
		return nil, diagnostics, nil
	}
	if err := evidence.Validate(); err != nil {
		return nil, []EvidenceDiagnostic{invalidEvidenceDiagnostic(FamilyV1Compensation, path)}, nil
	}
	decision := release.SelectV1CompensationOperation(&evidence)
	record := v1CompensationRecord(path, data, evidence, decision)
	return []EvidenceRecord{record}, nil, nil
}

func v1CompensationRecord(path string, data []byte, evidence release.V1CompensationEvidence, decision release.V1CompensationDecision) EvidenceRecord {
	classification, safeToResume, automatic, manual, guidance := classifyV1Compensation(decision)
	return EvidenceRecord{
		Family:                FamilyV1Compensation,
		Identity:              evidence.Identity.SHA256,
		Owner:                 "v1 compensation evidence",
		Version:               evidence.Identity.IntendedVersion,
		Tag:                   evidence.Identity.Tag,
		State:                 string(evidence.Compensation.Status),
		PendingAction:         string(evidence.Compensation.PendingAction),
		Classification:        classification,
		SafeToResume:          safeToResume,
		AutomaticContinuation: automatic,
		ManualRecovery:        manual,
		LifecycleAllowed:      classification == ClassificationCompleted,
		LifecycleOperation:    lifecycleOperation(classification == ClassificationCompleted),
		Guidance:              guidance,
		Path:                  path,
		DigestSHA256:          sha256Hex(data),
		CreatedAt:             formatEvidenceTime(evidence.CreatedAt.String()),
		UpdatedAt:             formatEvidenceTime(evidence.UpdatedAt.String()),
	}
}

func classifyV1Compensation(decision release.V1CompensationDecision) (string, bool, bool, bool, string) {
	switch decision.Kind {
	case release.V1CompensationAlreadyComplete:
		return ClassificationCompleted, false, false, false, "V1 compensation evidence is already completed."
	case release.V1CompensationNoRecoveryNeeded:
		return ClassificationTerminal, false, false, false, "No unsafe V1 release effect is recorded."
	case release.V1CompensationPerformOperation, release.V1CompensationMarkComplete:
		return ClassificationResumable, true, true, false, "V1 compensation has one typed automatic continuation."
	case release.V1CompensationRequireManual:
		return ClassificationManualRecoveryRequired, false, false, true, "V1 compensation requires manual recovery."
	default:
		return ClassificationUncertain, false, false, true, "V1 compensation decision is unknown."
	}
}

func v1CompensationEvidencePath(root string) (string, error) {
	executionDir, err := release.NewReleaseExecutionJournalStore(root).JournalDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(executionDir), "v1-compensation", "current.json"), nil
}

type migrationJournalEvidence struct {
	SourcePath          string `json:"sourcePath"`
	SourceContentSHA256 string `json:"sourceContentSHA256"`
	ConfigContentSHA256 string `json:"configContentSHA256"`
	StateContentSHA256  string `json:"stateContentSHA256"`
	BackupPath          string `json:"backupPath"`
	Stage               string `json:"stage"`
	SchemaVersion       int    `json:"schemaVersion"`
}

func inspectMigrationJournal(root string) ([]EvidenceRecord, []EvidenceDiagnostic) {
	path := filepath.Join(root, releaseconfig.V2Directory, "release.migration.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []EvidenceDiagnostic{unreadableEvidenceDiagnostic(FamilyMigration, path)}
	}
	var diagnostics []EvidenceDiagnostic
	var journal migrationJournalEvidence
	if !decodeEvidenceJSON(FamilyMigration, path, data, &journal, &diagnostics) {
		return nil, diagnostics
	}
	if journal.SchemaVersion != 1 {
		return nil, []EvidenceDiagnostic{unsupportedEvidenceDiagnostic(FamilyMigration, path)}
	}
	if !validMigrationStage(journal.Stage) {
		return nil, []EvidenceDiagnostic{invalidEvidenceDiagnostic(FamilyMigration, path)}
	}
	expectedSource := filepath.Join(root, releaseconfig.V1FileName)
	expectedBackup := filepath.Join(root, ".release.neko.json.v1.bak")
	if journal.SourcePath != expectedSource || journal.BackupPath != expectedBackup {
		return nil, []EvidenceDiagnostic{conflictingEvidenceDiagnostic(FamilyMigration, path)}
	}
	return []EvidenceRecord{{
		Family:                FamilyMigration,
		Identity:              sha256Hex(data),
		Owner:                 "migration journal",
		State:                 journal.Stage,
		PendingAction:         migrationPendingAction(journal.Stage),
		Classification:        ClassificationResumable,
		SafeToResume:          true,
		AutomaticContinuation: true,
		ManualRecovery:        false,
		Guidance:              migrationGuidance(journal.Stage),
		Path:                  path,
		DigestSHA256:          sha256Hex(data),
	}}, nil
}

func validMigrationStage(stage string) bool {
	switch stage {
	case "prepared", "config-written", "state-written", "v1-archived":
		return true
	default:
		return false
	}
}

func migrationPendingAction(stage string) string {
	switch stage {
	case "prepared", "config-written":
		return "persist-target"
	case "state-written":
		return "archive-source"
	case "v1-archived":
		return "remove-journal"
	default:
		return "manual-inspection"
	}
}

func migrationGuidance(stage string) string {
	switch stage {
	case "prepared", "config-written":
		return "Migration journal can resume target verification and persistence through the migration owner."
	case "state-written":
		return "Migration target is recorded; source archival must still be verified by migration."
	case "v1-archived":
		return "Migration source archival is recorded; final journal removal must still complete."
	default:
		return "Migration journal state is unknown. Manual recovery is required."
	}
}

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

func sortedJSONFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read evidence directory %s: %w", dir, err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(paths)
	return paths, nil
}

func readEvidenceBytes(family, path string) ([]byte, EvidenceDiagnostic, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, unreadableEvidenceDiagnostic(family, path), false
	}
	return data, EvidenceDiagnostic{}, true
}

func decodeEvidenceJSON(family, path string, data []byte, target any, diagnostics *[]EvidenceDiagnostic) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		*diagnostics = append(*diagnostics, corruptEvidenceDiagnostic(family, path))
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		*diagnostics = append(*diagnostics, corruptEvidenceDiagnostic(family, path))
		return false
	}
	return true
}

func unreadableEvidenceDiagnostic(family, path string) EvidenceDiagnostic {
	return EvidenceDiagnostic{
		Family:         family,
		Path:           path,
		Classification: ClassificationUncertain,
		Code:           "unreadable",
		Guidance:       "Evidence could not be read. Preserve the file and inspect permissions manually.",
	}
}

func corruptEvidenceDiagnostic(family, path string) EvidenceDiagnostic {
	return EvidenceDiagnostic{
		Family:         family,
		Path:           path,
		Classification: ClassificationCorrupt,
		Code:           "corrupt-json",
		Guidance:       "Evidence could not be decoded safely. Preserve the file and inspect manually.",
	}
}

func unsupportedEvidenceDiagnostic(family, path string) EvidenceDiagnostic {
	return EvidenceDiagnostic{
		Family:         family,
		Path:           path,
		Classification: ClassificationUnsupported,
		Code:           "unsupported-schema",
		Guidance:       "Evidence uses an unsupported schema. Preserve the file and inspect with a compatible release plugin.",
	}
}

func invalidEvidenceDiagnostic(family, path string) EvidenceDiagnostic {
	return EvidenceDiagnostic{
		Family:         family,
		Path:           path,
		Classification: ClassificationCorrupt,
		Code:           "invalid-state",
		Guidance:       "Evidence has invalid typed fields. Preserve the file and recover manually.",
	}
}

func conflictingEvidenceDiagnostic(family, path string) EvidenceDiagnostic {
	return EvidenceDiagnostic{
		Family:         family,
		Path:           path,
		Classification: ClassificationConflicting,
		Code:           "conflicting-identity",
		Guidance:       "Evidence identity or owner paths conflict with its location. Preserve the file and recover manually.",
	}
}

func sortEvidenceResult(result *evidenceQueryResult) {
	sort.SliceStable(result.Records, func(i, j int) bool {
		return evidenceRecordSortKey(result.Records[i]) < evidenceRecordSortKey(result.Records[j])
	})
	sort.SliceStable(result.Diagnostics, func(i, j int) bool {
		return result.Diagnostics[i].Family+"\x00"+result.Diagnostics[i].Path < result.Diagnostics[j].Family+"\x00"+result.Diagnostics[j].Path
	})
}

func evidenceRecordSortKey(record EvidenceRecord) string {
	return record.Family + "\x00" + record.Path + "\x00" + record.Identity
}

func safeEvidenceHash(hash string) bool {
	if len(hash) != 64 {
		return false
	}
	for _, r := range hash {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func lifecycleOperation(allowed bool) string {
	if allowed {
		return "archive-completed"
	}
	return ""
}

func formatEvidenceTime(value string) string {
	if strings.HasPrefix(value, "0001-01-01") {
		return ""
	}
	return value
}
