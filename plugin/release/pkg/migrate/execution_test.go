package migrate

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMigrationPlanExecutionUsesVisibleSafeOrder(t *testing.T) {
	recorder := &migrationExecutionRecorder{}
	execution := fakeMigrationPlanExecution(recorder)

	if err := execution.Execute(migrationExecutionTestPlan()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := []string{
		"start-journal",
		"persist-target",
		"confirm-target",
		"verify-target",
		"archive-source",
		"confirm-source",
		"verify-source",
		"remove-journal",
	}
	if !reflect.DeepEqual(recorder.calls, want) {
		t.Fatalf("execution order = %v, want %v", recorder.calls, want)
	}
}

func TestMigrationPlanExecutionStopsAtEachFailureBoundary(t *testing.T) {
	sentinel := errors.New("sentinel")

	t.Run("journal start", func(t *testing.T) {
		recorder := &migrationExecutionRecorder{}
		execution := fakeMigrationPlanExecution(recorder)
		journalOperations, ok := execution.journal.(*fakeMigrationJournalOperations)
		if !ok {
			t.Fatalf("journal operations type = %T", execution.journal)
		}
		journalOperations.startErr = sentinel
		assertMigrationExecutionFailure(t, execution.Execute(migrationExecutionTestPlan()), migrationJournalFailure)
		assertMigrationExecutionCalls(t, recorder, "start-journal")
	})

	t.Run("target persistence", func(t *testing.T) {
		recorder := &migrationExecutionRecorder{}
		execution := fakeMigrationPlanExecution(recorder)
		targets, ok := execution.targets.(*fakeMigrationTargetPersister)
		if !ok {
			t.Fatalf("target persister type = %T", execution.targets)
		}
		targets.err = sentinel
		assertMigrationExecutionFailure(t, execution.Execute(migrationExecutionTestPlan()), migrationTargetPersistenceFailure)
		assertMigrationExecutionCalls(t, recorder, "start-journal", "persist-target")
	})

	t.Run("target journal confirmation", func(t *testing.T) {
		recorder := &migrationExecutionRecorder{}
		execution := fakeMigrationPlanExecution(recorder)
		journalOperations, ok := execution.journal.(*fakeMigrationJournalOperations)
		if !ok {
			t.Fatalf("journal operations type = %T", execution.journal)
		}
		journalOperations.confirmTargetErr = sentinel
		assertMigrationExecutionFailure(t, execution.Execute(migrationExecutionTestPlan()), migrationJournalFailure)
		assertMigrationExecutionCalls(t, recorder, "start-journal", "persist-target", "confirm-target")
	})

	t.Run("target verification", func(t *testing.T) {
		recorder := &migrationExecutionRecorder{}
		execution := fakeMigrationPlanExecution(recorder)
		targetVerifier, ok := execution.targetVerifier.(*fakeMigrationTargetVerifier)
		if !ok {
			t.Fatalf("target verifier type = %T", execution.targetVerifier)
		}
		targetVerifier.err = sentinel
		assertMigrationExecutionFailure(t, execution.Execute(migrationExecutionTestPlan()), migrationTargetVerificationFailure)
		assertMigrationExecutionCalls(t, recorder, "start-journal", "persist-target", "confirm-target", "verify-target")
	})

	t.Run("source archive", func(t *testing.T) {
		recorder := &migrationExecutionRecorder{}
		execution := fakeMigrationPlanExecution(recorder)
		sourceArchiver, ok := execution.sourceArchiver.(*fakeMigrationSourceArchiver)
		if !ok {
			t.Fatalf("source archiver type = %T", execution.sourceArchiver)
		}
		sourceArchiver.err = sentinel
		assertMigrationExecutionFailure(t, execution.Execute(migrationExecutionTestPlan()), migrationSourceCleanupFailure)
		assertMigrationExecutionCalls(t, recorder, "start-journal", "persist-target", "confirm-target", "verify-target", "archive-source")
	})

	t.Run("source journal confirmation", func(t *testing.T) {
		recorder := &migrationExecutionRecorder{}
		execution := fakeMigrationPlanExecution(recorder)
		journalOperations, ok := execution.journal.(*fakeMigrationJournalOperations)
		if !ok {
			t.Fatalf("journal operations type = %T", execution.journal)
		}
		journalOperations.confirmSourceErr = sentinel
		assertMigrationExecutionFailure(t, execution.Execute(migrationExecutionTestPlan()), migrationJournalFailure)
		assertMigrationExecutionCalls(t, recorder, "start-journal", "persist-target", "confirm-target", "verify-target", "archive-source", "confirm-source")
	})

	t.Run("source verification", func(t *testing.T) {
		recorder := &migrationExecutionRecorder{}
		execution := fakeMigrationPlanExecution(recorder)
		sourceVerifier, ok := execution.sourceVerifier.(*fakeArchivedSourceVerifier)
		if !ok {
			t.Fatalf("source verifier type = %T", execution.sourceVerifier)
		}
		sourceVerifier.err = sentinel
		assertMigrationExecutionFailure(t, execution.Execute(migrationExecutionTestPlan()), migrationSourceVerificationFailure)
		assertMigrationExecutionCalls(t, recorder, "start-journal", "persist-target", "confirm-target", "verify-target", "archive-source", "confirm-source", "verify-source")
	})

	t.Run("journal removal", func(t *testing.T) {
		recorder := &migrationExecutionRecorder{}
		execution := fakeMigrationPlanExecution(recorder)
		journalOperations, ok := execution.journal.(*fakeMigrationJournalOperations)
		if !ok {
			t.Fatalf("journal operations type = %T", execution.journal)
		}
		journalOperations.removeErr = sentinel
		assertMigrationExecutionFailure(t, execution.Execute(migrationExecutionTestPlan()), migrationJournalFailure)
		assertMigrationExecutionCalls(t, recorder, "start-journal", "persist-target", "confirm-target", "verify-target", "archive-source", "confirm-source", "verify-source", "remove-journal")
	})
}

func TestMigrationPlanExecutionInvokesOnlySelectedRecoveryOperations(t *testing.T) {
	t.Run("complete target and active source", func(t *testing.T) {
		recorder := &migrationExecutionRecorder{}
		execution := fakeMigrationPlanExecution(recorder)
		plan := migrationExecutionTestPlan()
		plan.kind = recoveryMigrationPlan
		plan.targetOperation = retainMigrationTarget
		if err := execution.Execute(plan); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		assertMigrationExecutionCalls(t, recorder, "start-journal", "verify-target", "archive-source", "confirm-source", "verify-source", "remove-journal")
	})

	t.Run("complete target and archived source", func(t *testing.T) {
		recorder := &migrationExecutionRecorder{}
		execution := fakeMigrationPlanExecution(recorder)
		plan := migrationExecutionTestPlan()
		plan.kind = recoveryMigrationPlan
		plan.targetOperation = retainMigrationTarget
		plan.sourceOperation = retainArchivedMigrationSource
		if err := execution.Execute(plan); err != nil {
			t.Fatalf("Execute: %v", err)
		}
		assertMigrationExecutionCalls(t, recorder, "start-journal", "verify-target", "verify-source", "remove-journal")
	})
}

func TestMigrationTargetVerificationFailureRetainsActiveSource(t *testing.T) {
	root := t.TempDir()
	paths := migrationPaths(root)
	if err := os.WriteFile(paths.source, []byte(v1Fixture), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	source := newMigrationFileSnapshot(paths.source, []byte(v1Fixture), 0600)
	plan, err := constructMigrationPlan(root, paths, source, migrationFileSnapshot{path: paths.backup}, newMigrationPlan)
	if err != nil {
		t.Fatalf("construct plan: %v", err)
	}
	execution := newMigrationPlanExecution()
	execution.targetVerifier = &fakeMigrationTargetVerifier{err: errors.New("verification failed")}

	failure := execution.Execute(plan)
	assertMigrationExecutionFailure(t, failure, migrationTargetVerificationFailure)
	assertFileBytesAndMode(t, paths.source, []byte(v1Fixture), 0600)
	if exists(paths.backup) {
		t.Fatal("source was archived after target verification failure")
	}
	if !exists(paths.config) || !exists(paths.state) || !exists(paths.journal) {
		t.Fatal("recoverable target/journal evidence was not preserved")
	}
	assertNoMigrationTemporaryFiles(t, filepath.Dir(paths.config))
}

func TestMigrationTargetPersistenceFailureRetainsActiveSource(t *testing.T) {
	root := t.TempDir()
	paths := migrationPaths(root)
	if err := os.WriteFile(paths.source, []byte(v1Fixture), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	plan, err := constructMigrationPlan(
		root,
		paths,
		newMigrationFileSnapshot(paths.source, []byte(v1Fixture), 0600),
		migrationFileSnapshot{path: paths.backup},
		newMigrationPlan,
	)
	if err != nil {
		t.Fatalf("construct plan: %v", err)
	}
	execution := newMigrationPlanExecution()
	execution.targets = &fakeMigrationTargetPersister{err: errors.New("persistence failed")}

	failure := execution.Execute(plan)
	assertMigrationExecutionFailure(t, failure, migrationTargetPersistenceFailure)
	assertFileBytesAndMode(t, paths.source, []byte(v1Fixture), 0600)
	if exists(paths.backup) || exists(paths.config) || exists(paths.state) {
		t.Fatal("target persistence failure changed source or target files")
	}
	if !exists(paths.journal) {
		t.Fatal("prepared recovery journal was not retained")
	}
}

func TestMigrationSourceCleanupFailurePreservesValidSourceAndTarget(t *testing.T) {
	root := t.TempDir()
	paths := migrationPaths(root)
	if err := os.WriteFile(paths.source, []byte(v1Fixture), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	plan, err := constructMigrationPlan(
		root,
		paths,
		newMigrationFileSnapshot(paths.source, []byte(v1Fixture), 0600),
		migrationFileSnapshot{path: paths.backup},
		newMigrationPlan,
	)
	if err != nil {
		t.Fatalf("construct plan: %v", err)
	}
	changedSource := []byte("changed after planning")
	if err := os.WriteFile(paths.source, changedSource, 0600); err != nil {
		t.Fatalf("change source: %v", err)
	}

	failure := newMigrationPlanExecution().Execute(plan)
	assertMigrationExecutionFailure(t, failure, migrationSourceCleanupFailure)
	assertFileBytesAndMode(t, paths.source, changedSource, 0600)
	if exists(paths.backup) {
		t.Fatal("changed source was archived")
	}
	assertFileBytesAndMode(t, paths.config, plan.target.configJSON, 0644)
	assertFileBytesAndMode(t, paths.state, plan.target.stateJSON, 0644)
	if !exists(paths.journal) {
		t.Fatal("recovery journal was removed after cleanup failure")
	}
}

func TestMigrationJournalRemovalFailurePreservesCompletedEvidence(t *testing.T) {
	root := t.TempDir()
	paths := migrationPaths(root)
	if err := os.WriteFile(paths.source, []byte(v1Fixture), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	plan, err := constructMigrationPlan(
		root,
		paths,
		newMigrationFileSnapshot(paths.source, []byte(v1Fixture), 0600),
		migrationFileSnapshot{path: paths.backup},
		newMigrationPlan,
	)
	if err != nil {
		t.Fatalf("construct plan: %v", err)
	}
	execution := newMigrationPlanExecution()
	execution.journal = failMigrationJournalRemoval{delegate: filesystemMigrationJournalOperations{}}

	failure := execution.Execute(plan)
	assertMigrationExecutionFailure(t, failure, migrationJournalFailure)
	if exists(paths.source) || !exists(paths.backup) || !exists(paths.config) || !exists(paths.state) || !exists(paths.journal) {
		t.Fatal("journal removal failure did not retain complete recoverable evidence")
	}
	assertFileBytesAndMode(t, paths.backup, []byte(v1Fixture), 0600)
}

func TestMigrationPairRestorationFailureIsTypedForManualRecovery(t *testing.T) {
	recorder := &migrationExecutionRecorder{}
	execution := fakeMigrationPlanExecution(recorder)
	targets, ok := execution.targets.(*fakeMigrationTargetPersister)
	if !ok {
		t.Fatalf("target persister type = %T", execution.targets)
	}
	targets.err = manualMigrationTestError{}

	failure := execution.Execute(migrationExecutionTestPlan())
	var executionFailure *migrationExecutionFailure
	if !errors.As(failure, &executionFailure) || executionFailure.kind != migrationRestorationFailure || !executionFailure.manualRecoveryRequired {
		t.Fatalf("restoration failure classification = %#v", executionFailure)
	}
	commandFailure := migrationFailureFromExecution(failure)
	if commandFailure.kind != migrationRestorationFailure || !commandFailure.manualRecoveryRequired {
		t.Fatalf("command failure classification = %#v", commandFailure)
	}
}

func TestMigrationInvalidOnlyBackupRequiresManualRecovery(t *testing.T) {
	root := t.TempDir()
	paths := migrationPaths(root)
	if err := os.WriteFile(paths.backup, []byte("corrupt backup"), 0600); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	plan := migrationPlan{
		paths:  paths,
		source: newMigrationFileSnapshot(paths.backup, []byte(v1Fixture), 0600),
	}

	err := (filesystemArchivedSourceVerifier{}).Verify(plan)
	failure := newMigrationExecutionFailure(migrationSourceVerificationFailure, err)
	if failure.kind != migrationSourceVerificationFailure || !failure.manualRecoveryRequired {
		t.Fatalf("backup failure classification = %#v", failure)
	}
}

type migrationExecutionRecorder struct {
	calls []string
}

func (recorder *migrationExecutionRecorder) record(call string) {
	recorder.calls = append(recorder.calls, call)
}

type fakeMigrationJournalOperations struct { //nolint:govet // Failures follow the journal operation order.
	recorder         *migrationExecutionRecorder
	startErr         error
	confirmTargetErr error
	confirmSourceErr error
	removeErr        error
}

func (operations *fakeMigrationJournalOperations) Start(_ migrationPlan) (journal, error) {
	operations.recorder.record("start-journal")
	return journal{Stage: journalStagePrepared}, operations.startErr
}

func (operations *fakeMigrationJournalOperations) ConfirmTargetPersisted(_ migrationPlan, current journal) (journal, error) {
	operations.recorder.record("confirm-target")
	current.Stage = journalStageStateWritten
	return current, operations.confirmTargetErr
}

func (operations *fakeMigrationJournalOperations) ConfirmSourceArchived(_ migrationPlan, current journal) (journal, error) {
	operations.recorder.record("confirm-source")
	current.Stage = journalStageV1Archived
	return current, operations.confirmSourceErr
}

func (operations *fakeMigrationJournalOperations) Remove(_ migrationPlan) error {
	operations.recorder.record("remove-journal")
	return operations.removeErr
}

type fakeMigrationTargetPersister struct {
	recorder *migrationExecutionRecorder
	err      error
}

func (persister *fakeMigrationTargetPersister) Persist(_ string, _ migrationTarget) error {
	if persister.recorder != nil {
		persister.recorder.record("persist-target")
	}
	return persister.err
}

type fakeMigrationTargetVerifier struct {
	recorder *migrationExecutionRecorder
	err      error
}

func (verifier *fakeMigrationTargetVerifier) Verify(_ migrationPlan) error {
	if verifier.recorder != nil {
		verifier.recorder.record("verify-target")
	}
	return verifier.err
}

type fakeMigrationSourceArchiver struct {
	recorder *migrationExecutionRecorder
	err      error
}

func (archiver *fakeMigrationSourceArchiver) Archive(_ migrationPlan) error {
	archiver.recorder.record("archive-source")
	return archiver.err
}

type fakeArchivedSourceVerifier struct {
	recorder *migrationExecutionRecorder
	err      error
}

func (verifier *fakeArchivedSourceVerifier) Verify(_ migrationPlan) error {
	verifier.recorder.record("verify-source")
	return verifier.err
}

type manualMigrationTestError struct{}

func (manualMigrationTestError) Error() string {
	return "manual recovery required"
}

func (manualMigrationTestError) ManualRecoveryRequired() bool {
	return true
}

func (manualMigrationTestError) RestorationFailed() bool {
	return true
}

type failMigrationJournalRemoval struct {
	delegate filesystemMigrationJournalOperations
}

func (operations failMigrationJournalRemoval) Start(plan migrationPlan) (journal, error) {
	return operations.delegate.Start(plan)
}

func (operations failMigrationJournalRemoval) ConfirmTargetPersisted(plan migrationPlan, current journal) (journal, error) {
	return operations.delegate.ConfirmTargetPersisted(plan, current)
}

func (operations failMigrationJournalRemoval) ConfirmSourceArchived(plan migrationPlan, current journal) (journal, error) {
	return operations.delegate.ConfirmSourceArchived(plan, current)
}

func (failMigrationJournalRemoval) Remove(migrationPlan) error {
	return errors.New("remove failed")
}

func migrationExecutionTestPlan() migrationPlan {
	root := "/repo"
	return migrationPlan{
		repositoryRoot:  root,
		paths:           migrationPaths(root),
		kind:            newMigrationPlan,
		targetOperation: persistMigrationTarget,
		sourceOperation: archiveMigrationSource,
	}
}

func fakeMigrationPlanExecution(recorder *migrationExecutionRecorder) migrationPlanExecution {
	return migrationPlanExecution{
		journal:        &fakeMigrationJournalOperations{recorder: recorder},
		targets:        &fakeMigrationTargetPersister{recorder: recorder},
		targetVerifier: &fakeMigrationTargetVerifier{recorder: recorder},
		sourceArchiver: &fakeMigrationSourceArchiver{recorder: recorder},
		sourceVerifier: &fakeArchivedSourceVerifier{recorder: recorder},
	}
}

func assertMigrationExecutionFailure(t *testing.T, err error, wantKind migrationFailureKind) {
	t.Helper()
	var failure *migrationExecutionFailure
	if !errors.As(err, &failure) || failure.kind != wantKind {
		t.Fatalf("failure = %#v, want kind %d", failure, wantKind)
	}
}

func assertMigrationExecutionCalls(t *testing.T, recorder *migrationExecutionRecorder, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(recorder.calls, want) {
		t.Fatalf("calls = %v, want %v", recorder.calls, want)
	}
}

func assertNoMigrationTemporaryFiles(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read directory: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("unexpected temporary file: %s", entry.Name())
		}
	}
}
