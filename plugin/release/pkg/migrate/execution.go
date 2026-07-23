package migrate

import (
	"fmt"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

type migrationJournalOperations interface {
	Start(plan migrationPlan) (journal, error)
	ConfirmTargetPersisted(plan migrationPlan, current journal) (journal, error)
	ConfirmSourceArchived(plan migrationPlan, current journal) (journal, error)
	Remove(plan migrationPlan) error
}

type migrationTargetPairPersister interface {
	Persist(root string, target migrationTarget) error
}

type migrationTargetVerifier interface {
	Verify(plan migrationPlan) error
}

type migrationSourceArchiver interface {
	Archive(plan migrationPlan) error
}

type migrationArchivedSourceVerifier interface {
	Verify(plan migrationPlan) error
}

type migrationPlanExecution struct {
	journal        migrationJournalOperations
	targets        migrationTargetPairPersister
	targetVerifier migrationTargetVerifier
	sourceArchiver migrationSourceArchiver
	sourceVerifier migrationArchivedSourceVerifier
}

func newMigrationPlanExecution() migrationPlanExecution {
	return migrationPlanExecution{
		journal:        filesystemMigrationJournalOperations{},
		targets:        sharedV2MigrationTargetPersister{},
		targetVerifier: filesystemMigrationTargetVerifier{},
		sourceArchiver: filesystemMigrationSourceArchiver{},
		sourceVerifier: filesystemArchivedSourceVerifier{},
	}
}

func (execution migrationPlanExecution) Execute(plan migrationPlan) error {
	log.PluginV(log.Exec, "Preparing migration recovery journal")
	currentJournal, startErr := execution.journal.Start(plan)
	if startErr != nil {
		return newMigrationExecutionFailure(migrationJournalFailure, startErr)
	}

	switch plan.targetOperation {
	case persistMigrationTarget:
		log.PluginV(log.Exec, "Writing V2 configuration and state")
		if persistErr := execution.targets.Persist(plan.repositoryRoot, plan.target); persistErr != nil {
			return newMigrationExecutionFailure(migrationTargetPersistenceFailure, persistErr)
		}
		confirmedJournal, confirmErr := execution.journal.ConfirmTargetPersisted(plan, currentJournal)
		if confirmErr != nil {
			return newMigrationExecutionFailure(migrationJournalFailure, confirmErr)
		}
		currentJournal = confirmedJournal
		log.PluginV(log.Exec, "V2 configuration and state written")
	case retainMigrationTarget:
		// Recovery planning already proved that both target files match the journal.
		log.PluginV(log.Exec, "Existing V2 configuration and state retained for recovery")
	default:
		return newMigrationExecutionFailure(
			migrationTargetPersistenceFailure,
			fmt.Errorf("unsupported migration target operation %d", plan.targetOperation),
		)
	}

	log.PluginV(log.Exec, "Validating persisted V2 migration artifacts")
	if verifyTargetErr := execution.targetVerifier.Verify(plan); verifyTargetErr != nil {
		return newMigrationExecutionFailure(migrationTargetVerificationFailure, verifyTargetErr)
	}

	switch plan.sourceOperation {
	case archiveMigrationSource:
		log.PluginV(log.Exec, "Archiving legacy V1 configuration")
		if archiveErr := execution.sourceArchiver.Archive(plan); archiveErr != nil {
			return newMigrationExecutionFailure(migrationSourceCleanupFailure, archiveErr)
		}
		if _, confirmErr := execution.journal.ConfirmSourceArchived(plan, currentJournal); confirmErr != nil {
			return newMigrationExecutionFailure(migrationJournalFailure, confirmErr)
		}
		log.PluginV(log.Exec, "Legacy V1 configuration archived")
	case retainArchivedMigrationSource:
		// Recovery planning already selected the hash-matched backup as the source.
		log.PluginV(log.Exec, "Existing legacy V1 archive retained for recovery")
	default:
		return newMigrationExecutionFailure(
			migrationSourceCleanupFailure,
			fmt.Errorf("unsupported migration source operation %d", plan.sourceOperation),
		)
	}

	if err := execution.sourceVerifier.Verify(plan); err != nil {
		return newMigrationExecutionFailure(migrationSourceVerificationFailure, err)
	}
	log.PluginV(log.Exec, "Completing migration journal")
	if err := execution.journal.Remove(plan); err != nil {
		return newMigrationExecutionFailure(migrationJournalFailure, err)
	}
	log.PluginV(log.Exec, "Migration execution completed")
	return nil
}
