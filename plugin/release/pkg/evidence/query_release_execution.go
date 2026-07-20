package evidence

import (
	"path/filepath"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

func inspectReleaseExecutionJournals(directory string) ([]EvidenceRecord, []EvidenceDiagnostic, error) {
	paths, err := sortedJSONFiles(directory)
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
