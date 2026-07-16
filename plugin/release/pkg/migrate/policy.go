package migrate

import "fmt"

type migrationRecoveryClassification uint8

const (
	migrationReady migrationRecoveryClassification = iota + 1
	migrationPartiallyApplied
	migrationAlreadyComplete
	migrationIncompleteTarget
	migrationSourceTargetConflict
	migrationSourceMissing
	migrationPairRecoveryWithoutJournal
)

type migrationRepositoryEvidence struct {
	journalExists      bool
	pairRecoveryExists bool
	sourceExists       bool
	configExists       bool
	stateExists        bool
}

func classifyMigrationEvidence(evidence migrationRepositoryEvidence) migrationRecoveryClassification {
	switch {
	case evidence.journalExists:
		return migrationPartiallyApplied
	case evidence.pairRecoveryExists:
		return migrationPairRecoveryWithoutJournal
	case evidence.configExists && evidence.stateExists && !evidence.sourceExists:
		return migrationAlreadyComplete
	case evidence.configExists != evidence.stateExists:
		return migrationIncompleteTarget
	case evidence.sourceExists && (evidence.configExists || evidence.stateExists):
		return migrationSourceTargetConflict
	case evidence.sourceExists:
		return migrationReady
	default:
		return migrationSourceMissing
	}
}

type migrationPlanningOperation uint8

const (
	planNewMigration migrationPlanningOperation = iota + 1
	planInterruptedMigration
	returnCompletedMigration
	inspectUnsupportedMigrationSource
	refuseIncompleteMigrationTarget
	refuseMigrationSourceTargetConflict
	refusePairRecoveryWithoutMigrationJournal
)

func selectMigrationPlanningOperation(classification migrationRecoveryClassification) (migrationPlanningOperation, error) {
	switch classification {
	case migrationReady:
		return planNewMigration, nil
	case migrationPartiallyApplied:
		return planInterruptedMigration, nil
	case migrationAlreadyComplete:
		return returnCompletedMigration, nil
	case migrationSourceMissing:
		return inspectUnsupportedMigrationSource, nil
	case migrationIncompleteTarget:
		return refuseIncompleteMigrationTarget, nil
	case migrationSourceTargetConflict:
		return refuseMigrationSourceTargetConflict, nil
	case migrationPairRecoveryWithoutJournal:
		return refusePairRecoveryWithoutMigrationJournal, nil
	default:
		return 0, fmt.Errorf("migration recovery failed: unknown migration classification %d", classification)
	}
}

func selectRecoveryTargetOperation(configExists, stateExists bool) migrationTargetOperation {
	if configExists && stateExists {
		return retainMigrationTarget
	}
	return persistMigrationTarget
}

func selectRecoverySourceOperation(activeSourceExists bool) migrationSourceOperation {
	if activeSourceExists {
		return archiveMigrationSource
	}
	return retainArchivedMigrationSource
}
