package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReleaseExecutionRecoveryAssessmentStatuses(t *testing.T) {
	t.Run("fresh prepared is not started", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		journal := newPreparedExecutionJournal(t, newExecutionJournalContext(t, root))
		assessment, err := assessReleaseExecutionRecoveryForTest(root, journal)
		if err != nil {
			t.Fatalf("AssessReleaseExecutionRecovery: %v", err)
		}
		if assessment.Status != ReleaseExecutionRecoveryNotStarted || !assessment.SafeToContinue {
			t.Fatalf("unexpected assessment: %#v", assessment)
		}
	})

	t.Run("pending materialization with postimages is interrupted before commit", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		journal := newPreparedExecutionJournal(t, newExecutionJournalContext(t, root))
		statePath := filepath.Join(root, filepath.FromSlash(journal.KnownReleaseFiles[0].RepositoryRelativePath))
		if err := os.WriteFile(statePath, []byte(`{"schemaVersion":2,"units":{"api":{"version":"0.2.1"}}}`), 0644); err != nil {
			t.Fatalf("write state postimage: %v", err)
		}
		hash, _, err := hashFileIfExists(statePath)
		if err != nil {
			t.Fatalf("hash postimage: %v", err)
		}
		journal.KnownReleaseFiles[0].PostimageSHA256 = hash
		if phaseErr := journal.ConfirmPhase(ReleaseExecutionPreflightValidated, ReleaseExecutionJournalUpdate{}, time.Now()); phaseErr != nil {
			t.Fatalf("preflight: %v", phaseErr)
		}
		if pendingErr := journal.BeginPending(ReleaseExecutionPendingApplyMaterialization, time.Now()); pendingErr != nil {
			t.Fatalf("pending materialization: %v", pendingErr)
		}
		assessment, err := assessReleaseExecutionRecoveryForTest(root, journal)
		if err != nil {
			t.Fatalf("AssessReleaseExecutionRecovery: %v", err)
		}
		if assessment.Status != ReleaseExecutionRecoveryInterruptedBeforeCommit {
			t.Fatalf("unexpected assessment: %#v", assessment)
		}
	})

	t.Run("commit without tag is interrupted after commit", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		ctx := newExecutionJournalContext(t, root)
		journal := newPreparedExecutionJournal(t, ctx)
		writePlannedStateForRecoveryTest(t, ctx)
		commitSHA := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
		advanceExecutionJournalToCommitCreated(t, journal, commitSHA, time.Now())
		assessment, err := assessReleaseExecutionRecoveryForTest(root, journal)
		if err != nil {
			t.Fatalf("AssessReleaseExecutionRecovery: %v", err)
		}
		if assessment.Status != ReleaseExecutionRecoveryInterruptedAfterCommit {
			t.Fatalf("unexpected assessment: %#v", assessment)
		}
	})

	t.Run("expected tag target is recognized", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		ctx := newExecutionJournalContext(t, root)
		journal := newPreparedExecutionJournal(t, ctx)
		writePlannedStateForRecoveryTest(t, ctx)
		commitSHA := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
		advanceExecutionJournalToCommitCreated(t, journal, commitSHA, time.Now())
		gitCmd(t, root, "tag", journal.Tag, commitSHA)
		if err := journal.BeginPending(ReleaseExecutionPendingCreateUnitTag, time.Now()); err != nil {
			t.Fatalf("pending tag: %v", err)
		}
		if err := journal.ConfirmPhase(ReleaseExecutionTagCreated, ReleaseExecutionJournalUpdate{TagTargetSHA: commitSHA}, time.Now()); err != nil {
			t.Fatalf("tag: %v", err)
		}
		assessment, err := assessReleaseExecutionRecoveryForTest(root, journal)
		if err != nil {
			t.Fatalf("AssessReleaseExecutionRecovery: %v", err)
		}
		if assessment.Status != ReleaseExecutionRecoveryInterruptedAfterTag {
			t.Fatalf("unexpected assessment: %#v", assessment)
		}
	})
}

func TestReleaseExecutionRecoveryAssessmentConflicts(t *testing.T) {
	t.Run("divergent file hash", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		journal := newPreparedExecutionJournal(t, newExecutionJournalContext(t, root))
		statePath := filepath.Join(root, filepath.FromSlash(journal.KnownReleaseFiles[0].RepositoryRelativePath))
		if err := os.WriteFile(statePath, []byte("changed"), 0644); err != nil {
			t.Fatalf("write divergent state: %v", err)
		}
		assessment, err := assessReleaseExecutionRecoveryForTest(root, journal)
		if err != nil {
			t.Fatalf("AssessReleaseExecutionRecovery: %v", err)
		}
		if assessment.Status != ReleaseExecutionRecoveryConflicted {
			t.Fatalf("expected conflict, got %#v", assessment)
		}
	})

	t.Run("mismatched tag target", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		ctx := newExecutionJournalContext(t, root)
		journal := newPreparedExecutionJournal(t, ctx)
		writePlannedStateForRecoveryTest(t, ctx)
		commitSHA := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
		advanceExecutionJournalToCommitCreated(t, journal, commitSHA, time.Now())
		gitCmd(t, root, "commit", "--allow-empty", "-m", "other")
		otherSHA := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
		gitCmd(t, root, "tag", journal.Tag, otherSHA)
		if err := journal.BeginPending(ReleaseExecutionPendingCreateUnitTag, time.Now()); err != nil {
			t.Fatalf("pending tag: %v", err)
		}
		if err := journal.ConfirmPhase(ReleaseExecutionTagCreated, ReleaseExecutionJournalUpdate{TagTargetSHA: commitSHA}, time.Now()); err != nil {
			t.Fatalf("tag: %v", err)
		}
		assessment, err := assessReleaseExecutionRecoveryForTest(root, journal)
		if err != nil {
			t.Fatalf("AssessReleaseExecutionRecovery: %v", err)
		}
		if assessment.Status != ReleaseExecutionRecoveryConflicted {
			t.Fatalf("expected conflict, got %#v", assessment)
		}
	})

	t.Run("malformed journal object is corrupted", func(t *testing.T) {
		assessment, err := assessReleaseExecutionRecoveryForTest(t.TempDir(), &ReleaseExecutionJournal{SchemaVersion: releaseExecutionJournalSchemaVersion, State: "bad"})
		if err != nil {
			t.Fatalf("AssessReleaseExecutionRecovery: %v", err)
		}
		if assessment.Status != ReleaseExecutionRecoveryCorrupted {
			t.Fatalf("expected corrupted, got %#v", assessment)
		}
	})
}

func writePlannedStateForRecoveryTest(t *testing.T, ctx *ReleaseExecutionContext) {
	t.Helper()
	state := NewStateTransaction(ctx.RepositoryRoot)
	if err := state.CaptureSnapshot(); err != nil {
		t.Fatalf("CaptureSnapshot: %v", err)
	}
	if err := state.WriteUnitVersion(ctx.Unit.ID, ctx.NextVersion); err != nil {
		t.Fatalf("WriteUnitVersion: %v", err)
	}
}

func TestReleaseExecutionRecoveryDoesNotMutateRepository(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	journal := newPreparedExecutionJournal(t, newExecutionJournalContext(t, root))
	before := strings.TrimSpace(gitOutput(t, root, "status", "--porcelain", "--untracked-files=all"))
	if _, err := assessReleaseExecutionRecoveryForTest(root, journal); err != nil {
		t.Fatalf("AssessReleaseExecutionRecovery: %v", err)
	}
	after := strings.TrimSpace(gitOutput(t, root, "status", "--porcelain", "--untracked-files=all"))
	if before != after {
		t.Fatalf("assessment mutated repository: before=%q after=%q", before, after)
	}
}

func assessReleaseExecutionRecoveryForTest(repositoryRoot string, journal *ReleaseExecutionJournal) (*ReleaseExecutionRecoveryAssessment, error) {
	return AssessReleaseExecutionRecovery(repositoryRoot, journal, resumeGitAdapter{coordinator: NewGitReleaseCoordinator()})
}
