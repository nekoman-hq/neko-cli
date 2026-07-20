package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

type migrationJournalStage string

const (
	journalStagePrepared      migrationJournalStage = "prepared"
	journalStageConfigWritten migrationJournalStage = "config-written"
	journalStageStateWritten  migrationJournalStage = "state-written"
	journalStageV1Archived    migrationJournalStage = "v1-archived"
)

//nolint:govet // Journal field order mirrors the documented recovery file.
type journal struct {
	SchemaVersion       int                   `json:"schemaVersion"`
	SourcePath          string                `json:"sourcePath"`
	SourceContentSHA256 string                `json:"sourceContentSHA256"`
	ConfigContentSHA256 string                `json:"configContentSHA256"`
	StateContentSHA256  string                `json:"stateContentSHA256"`
	BackupPath          string                `json:"backupPath"`
	Stage               migrationJournalStage `json:"stage"`
}

func loadJournal(path string) (*journal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read migration journal %s: %w", path, err)
	}
	var storedJournal journal
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&storedJournal); err != nil {
		return nil, fmt.Errorf("parse migration journal %s: %w", path, err)
	}
	if err := validateMigrationJournalStage(storedJournal.Stage); err != nil {
		return nil, fmt.Errorf("parse migration journal %s: %w", path, err)
	}
	return &storedJournal, nil
}

func validateMigrationJournalStage(stage migrationJournalStage) error {
	switch stage {
	case journalStagePrepared, journalStageConfigWritten, journalStageStateWritten, journalStageV1Archived:
		return nil
	default:
		return fmt.Errorf("unknown migration journal stage %q", stage)
	}
}

func recoveryActions(paths migrationPathSet, journal *journal) []string {
	actions := []string{"validate migration journal"}
	if exists(paths.pairRecovery) {
		actions = append(actions, "recover interrupted V2 config/state pair")
	}
	if !exists(paths.config) {
		actions = append(actions, "write missing .neko/release.config.json")
	}
	if !exists(paths.state) {
		actions = append(actions, "write missing .neko/release.state.json")
	}
	if exists(paths.source) {
		actions = append(actions, "archive active .release.neko.json")
	}
	actions = append(actions, "validate migrated V2 configuration", "remove migration journal")
	if journal.Stage != "" {
		actions = append([]string{fmt.Sprintf("resume from journal stage %s", journal.Stage)}, actions...)
	}
	return actions
}
