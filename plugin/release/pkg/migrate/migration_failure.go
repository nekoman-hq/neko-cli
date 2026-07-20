package migrate

import "errors"

type migrationFailure struct {
	cause                  error
	kind                   migrationFailureKind
	manualRecoveryRequired bool
}

func (failure *migrationFailure) Error() string {
	return failure.cause.Error()
}

func (failure *migrationFailure) Unwrap() error {
	return failure.cause
}

type migrationFailureKind uint8

const (
	migrationPlanningFailure migrationFailureKind = iota + 1
	migrationJournalFailure
	migrationTargetPersistenceFailure
	migrationTargetVerificationFailure
	migrationSourceCleanupFailure
	migrationSourceVerificationFailure
	migrationRestorationFailure
)

type migrationExecutionFailure struct {
	cause                  error
	kind                   migrationFailureKind
	manualRecoveryRequired bool
}

func newMigrationExecutionFailure(kind migrationFailureKind, cause error) *migrationExecutionFailure {
	manualRecoveryRequired := false
	var restorationFailure interface {
		RestorationFailed() bool
	}
	if errors.As(cause, &restorationFailure) && restorationFailure.RestorationFailed() {
		kind = migrationRestorationFailure
	}
	var manualFailure interface {
		ManualRecoveryRequired() bool
	}
	if errors.As(cause, &manualFailure) && manualFailure.ManualRecoveryRequired() {
		manualRecoveryRequired = true
	}
	return &migrationExecutionFailure{
		kind:                   kind,
		cause:                  cause,
		manualRecoveryRequired: manualRecoveryRequired,
	}
}

func (failure *migrationExecutionFailure) Error() string {
	return failure.cause.Error()
}

func (failure *migrationExecutionFailure) Unwrap() error {
	return failure.cause
}

func migrationFailureFromExecution(err error) *migrationFailure {
	var executionFailure *migrationExecutionFailure
	if errors.As(err, &executionFailure) {
		return &migrationFailure{
			kind:                   executionFailure.kind,
			cause:                  executionFailure,
			manualRecoveryRequired: executionFailure.manualRecoveryRequired,
		}
	}
	return &migrationFailure{kind: migrationTargetPersistenceFailure, cause: err}
}

type migrationManualRecoveryError struct {
	cause error
}

func (failure *migrationManualRecoveryError) Error() string {
	return failure.cause.Error() + "; manual recovery required"
}

func (failure *migrationManualRecoveryError) Unwrap() error {
	return failure.cause
}

func (failure *migrationManualRecoveryError) ManualRecoveryRequired() bool {
	return true
}
