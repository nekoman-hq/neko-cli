//nolint:staticcheck // Migration evidence intentionally references the deprecated V1 source file.
package evidence

import (
	"errors"
	"os"
	"path/filepath"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

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
