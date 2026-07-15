package migrate

import "fmt"

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
	currentJournal, startErr := execution.journal.Start(plan)
	if startErr != nil {
		return newMigrationExecutionFailure(migrationJournalFailure, startErr)
	}

	switch plan.targetOperation {
	case persistMigrationTarget:
		if persistErr := execution.targets.Persist(plan.repositoryRoot, plan.target); persistErr != nil {
			return newMigrationExecutionFailure(migrationTargetPersistenceFailure, persistErr)
		}
		confirmedJournal, confirmErr := execution.journal.ConfirmTargetPersisted(plan, currentJournal)
		if confirmErr != nil {
			return newMigrationExecutionFailure(migrationJournalFailure, confirmErr)
		}
		currentJournal = confirmedJournal
	case retainMigrationTarget:
		// Recovery planning already proved that both target files match the journal.
	default:
		return newMigrationExecutionFailure(
			migrationTargetPersistenceFailure,
			fmt.Errorf("unsupported migration target operation %d", plan.targetOperation),
		)
	}

	if verifyTargetErr := execution.targetVerifier.Verify(plan); verifyTargetErr != nil {
		return newMigrationExecutionFailure(migrationTargetVerificationFailure, verifyTargetErr)
	}

	switch plan.sourceOperation {
	case archiveMigrationSource:
		if archiveErr := execution.sourceArchiver.Archive(plan); archiveErr != nil {
			return newMigrationExecutionFailure(migrationSourceCleanupFailure, archiveErr)
		}
		if _, confirmErr := execution.journal.ConfirmSourceArchived(plan, currentJournal); confirmErr != nil {
			return newMigrationExecutionFailure(migrationJournalFailure, confirmErr)
		}
	case retainArchivedMigrationSource:
		// Recovery planning already selected the hash-matched backup as the source.
	default:
		return newMigrationExecutionFailure(
			migrationSourceCleanupFailure,
			fmt.Errorf("unsupported migration source operation %d", plan.sourceOperation),
		)
	}

	if err := execution.sourceVerifier.Verify(plan); err != nil {
		return newMigrationExecutionFailure(migrationSourceVerificationFailure, err)
	}
	if err := execution.journal.Remove(plan); err != nil {
		return newMigrationExecutionFailure(migrationJournalFailure, err)
	}
	return nil
}
