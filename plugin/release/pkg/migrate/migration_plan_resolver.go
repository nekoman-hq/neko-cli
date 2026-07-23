//nolint:staticcheck // Migration planning intentionally bridges the deprecated V1 source to V2.
package migrate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type filesystemMigrationPlanResolver struct{}

func (filesystemMigrationPlanResolver) Resolve(root string) (migrationPlan, error) {
	paths := migrationPaths(root)
	log.PluginV(log.Config, "Inspecting migration repository state")
	log.PluginV(log.Config, "Locating V1 release configuration")
	evidence := migrationRepositoryEvidence{
		journalExists:      exists(paths.journal),
		pairRecoveryExists: exists(paths.pairRecovery),
		sourceExists:       exists(paths.source),
		configExists:       exists(paths.config),
		stateExists:        exists(paths.state),
	}
	operation, err := selectMigrationPlanningOperation(classifyMigrationEvidence(evidence))
	if err != nil {
		return migrationPlan{}, err
	}

	switch operation {
	case planInterruptedMigration:
		return resolveRecoveryPlan(root, paths)
	case returnCompletedMigration:
		if _, err := releaseconfig.LoadV2Repository(root); err != nil {
			return migrationPlan{}, err
		}
		return completedMigrationPlan(root, paths), nil
	case refuseIncompleteMigrationTarget:
		return migrationPlan{}, fmt.Errorf("incomplete V2 configuration: both %s and %s are required", paths.config, paths.state)
	case refuseMigrationSourceTargetConflict:
		return migrationPlan{}, fmt.Errorf("migration conflict: active V1 config and V2 files exist without migration journal")
	case refusePairRecoveryWithoutMigrationJournal:
		return migrationPlan{}, fmt.Errorf("migration recovery failed: V2 pair recovery evidence exists without migration journal: %s", paths.pairRecovery)
	case planNewMigration:
		return resolveNewMigrationPlan(root, paths)
	case inspectUnsupportedMigrationSource:
		if nested, ok, err := findNestedV1(root, paths.source); err != nil {
			return migrationPlan{}, err
		} else if ok {
			return migrationPlan{}, fmt.Errorf("nested V1 release configuration cannot be migrated as a single-unit repository; create a V2 multi-unit configuration explicitly instead: %s", nested)
		}
		return migrationPlan{}, fmt.Errorf("no release configuration found to migrate in %s", root)
	default:
		return migrationPlan{}, fmt.Errorf("migration recovery failed: unsupported planning operation %d", operation)
	}
}

func resolveRecoveryPlan(root string, paths migrationPathSet) (migrationPlan, error) {
	journal, err := loadJournal(paths.journal)
	if err != nil {
		return migrationPlan{}, err
	}
	if journal.SchemaVersion != 1 || journal.SourcePath != paths.source || journal.BackupPath != paths.backup {
		return migrationPlan{}, fmt.Errorf("migration recovery failed: journal %s does not match repository paths", paths.journal)
	}

	plan, err := resolvePlanFromJournal(root, paths, journal)
	if err != nil {
		return migrationPlan{}, err
	}
	plan.kind = recoveryMigrationPlan
	plan.actions = recoveryActions(paths, journal)
	return plan, nil
}

func resolveNewMigrationPlan(root string, paths migrationPathSet) (migrationPlan, error) {
	source, err := captureMigrationFile(paths.source)
	if err != nil {
		return migrationPlan{}, fmt.Errorf("read V1 config %s: %w", paths.source, err)
	}
	backup, err := captureMigrationFile(paths.backup)
	if err != nil {
		return migrationPlan{}, fmt.Errorf("read V1 backup %s: %w", paths.backup, err)
	}
	return constructMigrationPlan(root, paths, source, backup, newMigrationPlan)
}

func resolvePlanFromJournal(root string, paths migrationPathSet, journal *journal) (migrationPlan, error) {
	activeSourceExists := exists(paths.source)
	var source migrationFileSnapshot
	if activeSourceExists {
		activeSource, err := captureMigrationFile(paths.source)
		if err != nil {
			return migrationPlan{}, fmt.Errorf("read V1 config %s: %w", paths.source, err)
		}
		if sha256Hex(activeSource.data) != journal.SourceContentSHA256 {
			return migrationPlan{}, fmt.Errorf("migration recovery failed: active V1 config %s does not match journal hash", paths.source)
		}
		source = activeSource
	} else if exists(paths.backup) {
		archivedSource, err := captureMigrationFile(paths.backup)
		if err != nil {
			return migrationPlan{}, fmt.Errorf("read V1 backup %s: %w", paths.backup, err)
		}
		if sha256Hex(archivedSource.data) != journal.SourceContentSHA256 {
			return migrationPlan{}, fmt.Errorf("migration recovery failed: V1 backup %s does not match journal hash", paths.backup)
		}
		source = archivedSource
	} else {
		return migrationPlan{}, fmt.Errorf("migration recovery failed: neither active V1 config nor backup exists")
	}

	backup, err := captureMigrationFile(paths.backup)
	if err != nil {
		return migrationPlan{}, fmt.Errorf("read V1 backup %s: %w", paths.backup, err)
	}
	plan, err := constructMigrationPlan(root, paths, source, backup, recoveryMigrationPlan)
	if err != nil {
		return migrationPlan{}, err
	}
	if sha256Hex(plan.target.configJSON) != journal.ConfigContentSHA256 || sha256Hex(plan.target.stateJSON) != journal.StateContentSHA256 {
		return migrationPlan{}, fmt.Errorf("migration recovery failed: planned V2 content does not match journal hashes")
	}
	if err := verifyExistingIfPresent(paths.config, plan.target.configJSON, "config"); err != nil {
		return migrationPlan{}, err
	}
	if err := verifyExistingIfPresent(paths.state, plan.target.stateJSON, "state"); err != nil {
		return migrationPlan{}, err
	}
	plan.journal = *journal
	plan.targetOperation = selectRecoveryTargetOperation(exists(paths.config), exists(paths.state))
	plan.sourceOperation = selectRecoverySourceOperation(activeSourceExists)
	return plan, nil
}

func verifyExistingIfPresent(path string, expected []byte, label string) error {
	if !exists(path) {
		return nil
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read existing %s %s: %w", label, path, err)
	}
	if !bytes.Equal(current, expected) {
		return fmt.Errorf("migration conflict: existing %s %s differs from planned content", label, path)
	}
	return nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
