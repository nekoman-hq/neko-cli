package evidence

import (
	"path/filepath"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

func inspectDispatchJournals(directory string) ([]EvidenceRecord, []EvidenceDiagnostic, error) {
	paths, err := sortedJSONFiles(directory)
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
