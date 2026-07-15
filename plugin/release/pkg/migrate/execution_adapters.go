package migrate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type filesystemMigrationJournalOperations struct{}

func (filesystemMigrationJournalOperations) Start(plan migrationPlan) (journal, error) {
	if plan.kind == recoveryMigrationPlan {
		return plan.journal, nil
	}
	if plan.kind != newMigrationPlan {
		return journal{}, fmt.Errorf("start migration journal: unsupported plan kind %d", plan.kind)
	}
	if err := os.MkdirAll(filepath.Dir(plan.paths.journal), 0755); err != nil {
		return journal{}, fmt.Errorf("create V2 directory %s: %w", filepath.Dir(plan.paths.journal), err)
	}
	prepared := journal{
		SchemaVersion:       1,
		SourcePath:          plan.paths.source,
		SourceContentSHA256: sha256Hex(plan.source.data),
		ConfigContentSHA256: sha256Hex(plan.target.configJSON),
		StateContentSHA256:  sha256Hex(plan.target.stateJSON),
		BackupPath:          plan.paths.backup,
		Stage:               journalStagePrepared,
	}
	if err := persistMigrationJournal(plan.paths.journal, prepared); err != nil {
		return journal{}, err
	}
	return prepared, nil
}

func (filesystemMigrationJournalOperations) ConfirmTargetPersisted(
	plan migrationPlan,
	current journal,
) (journal, error) {
	if current.Stage == journalStageV1Archived {
		return current, nil
	}
	switch current.Stage {
	case journalStagePrepared, journalStageConfigWritten, journalStageStateWritten:
		confirmed := current
		confirmed.Stage = journalStageStateWritten
		if err := persistMigrationJournal(plan.paths.journal, confirmed); err != nil {
			return journal{}, err
		}
		return confirmed, nil
	default:
		return journal{}, fmt.Errorf("confirm persisted migration target from stage %q", current.Stage)
	}
}

func (filesystemMigrationJournalOperations) ConfirmSourceArchived(
	plan migrationPlan,
	current journal,
) (journal, error) {
	if err := validateMigrationJournalStage(current.Stage); err != nil {
		return journal{}, fmt.Errorf("confirm archived migration source: %w", err)
	}
	confirmed := current
	confirmed.Stage = journalStageV1Archived
	if err := persistMigrationJournal(plan.paths.journal, confirmed); err != nil {
		return journal{}, err
	}
	return confirmed, nil
}

func (filesystemMigrationJournalOperations) Remove(plan migrationPlan) error {
	if err := os.Remove(plan.paths.journal); err != nil {
		return fmt.Errorf("remove migration journal %s: %w", plan.paths.journal, err)
	}
	return nil
}

func persistMigrationJournal(path string, value journal) error {
	if err := releaseconfig.AtomicWriteJSON(path, &value, 0644); err != nil {
		return fmt.Errorf("write migration journal %s: %w", path, err)
	}
	return nil
}

type sharedV2MigrationTargetPersister struct{}

func (sharedV2MigrationTargetPersister) Persist(root string, target migrationTarget) error {
	persister := releaseconfig.NewV2ReleasePairPersister(root)
	return persister.Persist(releaseconfig.V2ReleasePair{
		Config: target.config,
		State:  target.state,
	})
}

type filesystemMigrationTargetVerifier struct{}

func (filesystemMigrationTargetVerifier) Verify(plan migrationPlan) error {
	configBytes, err := os.ReadFile(plan.paths.config)
	if err != nil {
		return fmt.Errorf("migration validation failed: read config %s: %w", plan.paths.config, err)
	}
	if !bytes.Equal(configBytes, plan.target.configJSON) {
		return fmt.Errorf("migration validation failed: config content mismatch at %s", plan.paths.config)
	}
	stateBytes, err := os.ReadFile(plan.paths.state)
	if err != nil {
		return fmt.Errorf("migration validation failed: read state %s: %w", plan.paths.state, err)
	}
	if !bytes.Equal(stateBytes, plan.target.stateJSON) {
		return fmt.Errorf("migration validation failed: state content mismatch at %s", plan.paths.state)
	}

	configValue, err := releaseconfig.LoadV2Config(plan.paths.config)
	if err != nil {
		return err
	}
	stateValue, err := releaseconfig.LoadV2State(plan.paths.state)
	if err != nil {
		return err
	}
	if err := releaseconfig.ValidateV2(plan.repositoryRoot, configValue, stateValue); err != nil {
		return err
	}
	return nil
}

type filesystemMigrationSourceArchiver struct{}

func (filesystemMigrationSourceArchiver) Archive(plan migrationPlan) error {
	paths := plan.paths
	sourceHash := sha256Hex(plan.source.data)
	if exists(paths.backup) {
		backupBytes, err := os.ReadFile(paths.backup)
		if err != nil {
			return fmt.Errorf("read backup %s: %w", paths.backup, err)
		}
		if sha256Hex(backupBytes) != sourceHash {
			return fmt.Errorf("migration conflict: existing backup %s differs from source", paths.backup)
		}
		if exists(paths.source) {
			sourceBytes, err := os.ReadFile(paths.source)
			if err != nil {
				return fmt.Errorf("read active V1 config %s: %w", paths.source, err)
			}
			if sha256Hex(sourceBytes) != sourceHash {
				return fmt.Errorf("migration recovery failed: active V1 config changed since journal creation")
			}
			if err := os.Remove(paths.source); err != nil {
				return fmt.Errorf("remove already-archived active V1 config %s: %w", paths.source, err)
			}
		}
		return nil
	}

	if !exists(paths.source) {
		return fmt.Errorf("migration recovery failed: active V1 config missing before backup was created")
	}
	sourceBytes, err := os.ReadFile(paths.source)
	if err != nil {
		return fmt.Errorf("read active V1 config %s: %w", paths.source, err)
	}
	if sha256Hex(sourceBytes) != sourceHash {
		return fmt.Errorf("migration recovery failed: active V1 config changed since journal creation")
	}
	if err := os.Rename(paths.source, paths.backup); err != nil {
		return fmt.Errorf("archive V1 config %s to %s: %w", paths.source, paths.backup, err)
	}
	return nil
}

type filesystemArchivedSourceVerifier struct{}

func (filesystemArchivedSourceVerifier) Verify(plan migrationPlan) error {
	if exists(plan.paths.source) {
		return fmt.Errorf("migration validation failed: active V1 config still exists at %s", plan.paths.source)
	}
	backupBytes, err := os.ReadFile(plan.paths.backup)
	if err != nil {
		return &migrationManualRecoveryError{cause: fmt.Errorf(
			"migration validation failed: read backup %s: %w",
			plan.paths.backup,
			err,
		)}
	}
	if sha256Hex(backupBytes) != sha256Hex(plan.source.data) {
		return &migrationManualRecoveryError{cause: fmt.Errorf(
			"migration validation failed: backup hash mismatch at %s",
			plan.paths.backup,
		)}
	}
	return nil
}
