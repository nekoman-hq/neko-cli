package release

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/pipelineinspection"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestPipelineRuntimeExecutionJournalScenarios(t *testing.T) {
	t.Run("untouched", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		response := inspectPipelineRuntimeForTest(t, root)
		if response.ExitCode != 0 || fmt.Sprint(response.Data["status"]) != "ready" {
			t.Fatalf("response = %#v", response)
		}
		execution := pipelineJSONView(t, response.Data["execution"])
		if execution["present"] != false || execution["journal_count"] != float64(0) {
			t.Fatalf("execution = %#v", execution)
		}
	})

	t.Run("prepared", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		journal := newPreparedExecutionJournal(t, newExecutionJournalContext(t, root))
		if _, err := NewReleaseExecutionJournalStore(root).Prepare(journal); err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		response := inspectPipelineRuntimeForTest(t, root)
		if response.ExitCode != 0 || fmt.Sprint(response.Data["status"]) != "active" {
			t.Fatalf("response = %#v", response)
		}
		execution := pipelineJSONView(t, response.Data["execution"])
		if execution["identity"] != journal.Identity.SHA256 || execution["state"] != string(ReleaseExecutionPrepared) {
			t.Fatalf("execution = %#v", execution)
		}
		assertPipelineStageRuntime(t, response, "execution-journal-preparation", "confirmed")
		assertPipelineStageRuntime(t, response, "release-file-materialization", "not_started")
	})

	t.Run("materialization and state are confirmed monotonically", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		journal := newPreparedExecutionJournal(t, newExecutionJournalContext(t, root))
		store := NewReleaseExecutionJournalStore(root)
		if _, err := store.Prepare(journal); err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		if _, err := store.ConfirmPhase(journal.Identity, ReleaseExecutionPreflightValidated, ReleaseExecutionJournalUpdate{}); err != nil {
			t.Fatalf("confirm preflight: %v", err)
		}
		if _, err := store.BeginPending(journal.Identity, ReleaseExecutionPendingApplyMaterialization); err != nil {
			t.Fatalf("pending materialization: %v", err)
		}
		if _, err := store.ConfirmPhase(journal.Identity, ReleaseExecutionMaterializationApplied, ReleaseExecutionJournalUpdate{}); err != nil {
			t.Fatalf("confirm materialization: %v", err)
		}
		if _, err := store.BeginPending(journal.Identity, ReleaseExecutionPendingWriteState); err != nil {
			t.Fatalf("pending state: %v", err)
		}
		if _, err := store.ConfirmPhase(journal.Identity, ReleaseExecutionStateWritten, ReleaseExecutionJournalUpdate{}); err != nil {
			t.Fatalf("confirm state: %v", err)
		}
		response := inspectPipelineRuntimeForTest(t, root)
		assertPipelineStageRuntime(t, response, "release-file-materialization", "confirmed")
		assertPipelineStageRuntime(t, response, "selected-unit-state-write", "confirmed")
		assertPipelineStageRuntime(t, response, "known-release-file-staging", "not_started")
	})

	t.Run("commit pending", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		ctx := newExecutionJournalContext(t, root)
		journal := newPreparedExecutionJournal(t, ctx)
		writePlannedStateForRecoveryTest(t, ctx)
		store := NewReleaseExecutionJournalStore(root)
		if _, err := store.Prepare(journal); err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		advanceExecutionStoreToStagedForPipelineTest(t, store, journal.Identity)
		if _, err := store.BeginPending(journal.Identity, ReleaseExecutionPendingCreateReleaseCommit); err != nil {
			t.Fatalf("pending commit: %v", err)
		}
		response := inspectPipelineRuntimeForTest(t, root)
		assertPipelineStageRuntime(t, response, "known-release-file-staging", "confirmed")
		assertPipelineStageRuntime(t, response, "release-commit-creation", "pending")
		assertPipelineStageRuntime(t, response, "unit-tag-creation", "not_started")
	})

	t.Run("multiple unresolved journals are invalid without recency selection", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		ctx := newExecutionJournalContext(t, root)
		first := newPreparedExecutionJournal(t, ctx)
		store := NewReleaseExecutionJournalStore(root)
		if _, err := store.Prepare(first); err != nil {
			t.Fatalf("Prepare first: %v", err)
		}
		other := *ctx
		other.NextVersion = "0.2.2"
		other.Tag = "api/v0.2.2"
		baseSHA := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
		remote := strings.TrimSpace(gitOutput(t, root, "remote", "get-url", "origin"))
		second := mustBuildExecutionJournal(t, &other, newExecutionJournalKnownFiles(t, &other), baseSHA, remote)
		second.CreatedAt = first.CreatedAt.Add(-24 * time.Hour)
		second.UpdatedAt = second.CreatedAt
		if _, err := store.Prepare(second); err != nil {
			t.Fatalf("Prepare second: %v", err)
		}
		response := inspectPipelineRuntimeForTest(t, root)
		if response.ExitCode != 1 || fmt.Sprint(response.Data["status"]) != "invalid" {
			t.Fatalf("response = %#v", response)
		}
		execution := pipelineJSONView(t, response.Data["execution"])
		if execution["unresolved_count"] != float64(2) || execution["identity"] != "" {
			t.Fatalf("execution = %#v", execution)
		}
	})

	t.Run("malformed execution journal is invalid", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		directory, err := NewReleaseExecutionJournalStore(root).JournalDirectory()
		if err != nil {
			t.Fatalf("JournalDirectory: %v", err)
		}
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(directory, strings.Repeat("a", 64)+".json"), []byte("{not-json"), 0o600); err != nil {
			t.Fatalf("write malformed journal: %v", err)
		}
		response := inspectPipelineRuntimeForTest(t, root)
		if response.ExitCode != 1 || fmt.Sprint(response.Data["status"]) != "invalid" {
			t.Fatalf("response = %#v", response)
		}
	})
}

func inspectPipelineRuntimeForTest(t *testing.T, rootPath string) *plugin.Response {
	t.Helper()
	root, err := workspace.ValidateRepositoryRoot(rootPath)
	if err != nil {
		t.Fatalf("ValidateRepositoryRoot: %v", err)
	}
	response, err := HandlePipelineAt(root, plugin.Request{Command: "pipeline", Flags: map[string]any{"unit": "api"}})
	if err != nil {
		t.Fatalf("HandlePipelineAt: %v", err)
	}
	return response
}

func assertPipelineStageRuntime(t *testing.T, response *plugin.Response, stageID, want string) {
	t.Helper()
	stages, ok := response.Data["stages"].([]pipelineinspection.LifecycleStage)
	if !ok {
		t.Fatalf("stages type = %T", response.Data["stages"])
	}
	for _, stage := range stages {
		if stage.ID == stageID {
			if string(stage.RuntimeStatus) != want {
				t.Fatalf("stage %s runtime = %s, want %s", stageID, stage.RuntimeStatus, want)
			}
			return
		}
	}
	t.Fatalf("stage %s not found", stageID)
}

func advanceExecutionStoreToStagedForPipelineTest(t *testing.T, store *ReleaseExecutionJournalStore, identity ReleaseExecutionIdentity) {
	t.Helper()
	if _, err := store.ConfirmPhase(identity, ReleaseExecutionPreflightValidated, ReleaseExecutionJournalUpdate{}); err != nil {
		t.Fatalf("preflight: %v", err)
	}
	for _, step := range []struct {
		pending ReleaseExecutionPendingAction
		state   ReleaseExecutionJournalState
	}{
		{ReleaseExecutionPendingApplyMaterialization, ReleaseExecutionMaterializationApplied},
		{ReleaseExecutionPendingWriteState, ReleaseExecutionStateWritten},
		{ReleaseExecutionPendingStageReleaseFiles, ReleaseExecutionReleaseFilesStaged},
	} {
		if _, err := store.BeginPending(identity, step.pending); err != nil {
			t.Fatalf("pending %s: %v", step.pending, err)
		}
		if _, err := store.ConfirmPhase(identity, step.state, ReleaseExecutionJournalUpdate{}); err != nil {
			t.Fatalf("confirm %s: %v", step.state, err)
		}
	}
}
