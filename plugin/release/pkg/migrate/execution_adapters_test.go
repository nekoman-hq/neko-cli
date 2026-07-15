package migrate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFilesystemMigrationJournalOperationsPersistCompatibleTypedStages(t *testing.T) {
	root := t.TempDir()
	paths := migrationPaths(root)
	plan := migrationPlan{
		repositoryRoot: root,
		paths:          paths,
		kind:           newMigrationPlan,
		source:         newMigrationFileSnapshot(paths.source, []byte("source"), 0600),
		target: migrationTarget{
			configJSON: []byte("config"),
			stateJSON:  []byte("state"),
		},
	}
	operations := filesystemMigrationJournalOperations{}

	prepared, err := operations.Start(plan)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	assertPersistedMigrationJournal(t, paths.journal, prepared, journalStagePrepared)
	stateWritten, err := operations.ConfirmTargetPersisted(plan, prepared)
	if err != nil {
		t.Fatalf("ConfirmTargetPersisted: %v", err)
	}
	assertPersistedMigrationJournal(t, paths.journal, stateWritten, journalStageStateWritten)
	archived, err := operations.ConfirmSourceArchived(plan, stateWritten)
	if err != nil {
		t.Fatalf("ConfirmSourceArchived: %v", err)
	}
	assertPersistedMigrationJournal(t, paths.journal, archived, journalStageV1Archived)
	if err := operations.Remove(plan); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if exists(paths.journal) {
		t.Fatal("journal survived successful removal")
	}
}

func TestFilesystemMigrationJournalRecoveryStartIsReadOnly(t *testing.T) {
	root := t.TempDir()
	paths := migrationPaths(root)
	want := journal{SchemaVersion: 1, Stage: journalStageConfigWritten}
	plan := migrationPlan{
		repositoryRoot: root,
		paths:          paths,
		kind:           recoveryMigrationPlan,
		journal:        want,
	}

	got, err := (filesystemMigrationJournalOperations{}).Start(plan)
	if err != nil {
		t.Fatalf("Start recovery: %v", err)
	}
	if !reflect.DeepEqual(got, want) || exists(paths.journal) || exists(filepath.Dir(paths.journal)) {
		t.Fatalf("recovery start changed disk or journal: got=%#v want=%#v", got, want)
	}
}

func TestFilesystemMigrationJournalRejectsInvalidTargetConfirmationStage(t *testing.T) {
	plan := migrationPlan{paths: migrationPaths(t.TempDir())}
	_, err := (filesystemMigrationJournalOperations{}).ConfirmTargetPersisted(
		plan,
		journal{Stage: migrationJournalStage("invalid")},
	)
	if err == nil {
		t.Fatal("invalid target confirmation stage was accepted")
	}
}

func assertPersistedMigrationJournal(t *testing.T, path string, want journal, wantStage migrationJournalStage) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	var got journal
	if decodeErr := json.Unmarshal(data, &got); decodeErr != nil {
		t.Fatalf("parse journal: %v", decodeErr)
	}
	if !reflect.DeepEqual(got, want) || got.Stage != wantStage {
		t.Fatalf("journal = %#v, want %#v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat journal: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Fatalf("journal mode = %04o, want 0644", info.Mode().Perm())
	}
}
