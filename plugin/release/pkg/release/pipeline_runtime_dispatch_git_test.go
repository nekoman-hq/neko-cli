package release

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPipelineRuntimeCorrelatesDispatchByExactIdentity(t *testing.T) {
	t.Run("accepted dispatch completes local handoff", func(t *testing.T) {
		fixture := newTaggedResumeRelease(t)
		identity := prepareAcceptedDispatchForResume(t, fixture)
		persistTagPushedExecution(t, fixture, identity)
		journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath)
		if err := journal.ConfirmPhase(ReleaseExecutionHandoffReady, ReleaseExecutionJournalUpdate{}, time.Now()); err != nil {
			t.Fatalf("confirm handoff: %v", err)
		}
		writeExecutionJournalFixture(t, fixture.executionPath, journal)

		response := inspectPipelineRuntimeForTest(t, fixture.root)
		if response.ExitCode != 0 || fmt.Sprint(response.Data["status"]) != "completed" {
			t.Fatalf("response = %#v", response)
		}
		dispatch := pipelineJSONView(t, response.Data["dispatch"])
		if dispatch["identity"] != identity.SHA256 || dispatch["correlation"] != "exact" || dispatch["state"] != string(DispatchJournalAccepted) {
			t.Fatalf("dispatch = %#v", dispatch)
		}
		assertPipelineStageRuntime(t, response, "workflow-request-submission", "confirmed")
		assertPipelineStageRuntime(t, response, "handoff-confirmation", "confirmed")
	})

	t.Run("rejected dispatch remains a successful inspection", func(t *testing.T) {
		fixture := newTaggedResumeRelease(t)
		identity := prepareRejectedDispatchForResume(t, fixture)
		persistTagPushedExecution(t, fixture, identity)
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		if response.ExitCode != 0 || fmt.Sprint(response.Data["status"]) != "rejected" {
			t.Fatalf("response = %#v", response)
		}
		assertPipelineStageRuntime(t, response, "workflow-request-submission", "rejected")
	})

	t.Run("unknown dispatch is uncertain and is not retried", func(t *testing.T) {
		fixture := newTaggedResumeRelease(t)
		identity := prepareUnknownDispatchForResume(t, fixture)
		persistTagPushedExecution(t, fixture, identity)
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		if response.ExitCode != 0 || fmt.Sprint(response.Data["status"]) != "uncertain" {
			t.Fatalf("response = %#v", response)
		}
		assertPipelineStageRuntime(t, response, "workflow-request-submission", "unknown")
	})

	t.Run("dispatch linked to no exact execution remains unlinked", func(t *testing.T) {
		fixture := newTaggedResumeRelease(t)
		identity := prepareAcceptedDispatchForResume(t, fixture)
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		if response.ExitCode != 1 || fmt.Sprint(response.Data["status"]) != "invalid" {
			t.Fatalf("response = %#v", response)
		}
		dispatch := pipelineJSONView(t, response.Data["dispatch"])
		if dispatch["identity"] != "" || dispatch["unlinked_count"] != float64(1) {
			t.Fatalf("dispatch = %#v, unrelated identity %s was attached", dispatch, identity.SHA256)
		}
	})

	t.Run("malformed exactly linked dispatch is invalid", func(t *testing.T) {
		fixture := newTaggedResumeRelease(t)
		request, dispatchPath := prepareStartedDispatchForResume(t, fixture)
		persistTagPushedExecution(t, fixture, request.Identity)
		if err := os.WriteFile(dispatchPath, []byte("{not-json"), 0o600); err != nil {
			t.Fatalf("corrupt dispatch: %v", err)
		}
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		if response.ExitCode != 1 || fmt.Sprint(response.Data["status"]) != "invalid" {
			t.Fatalf("response = %#v", response)
		}
	})

	t.Run("dispatch identity mismatch is invalid", func(t *testing.T) {
		fixture := newTaggedResumeRelease(t)
		request := preparePipelineDispatchRequest(t, fixture)
		persistDispatchPreparedExecution(t, fixture, request.Identity)
		path, err := NewDispatchJournalStore(fixture.root).JournalPath(request.Identity)
		if err != nil {
			t.Fatalf("JournalPath: %v", err)
		}
		journal := loadDispatchJournalForTest(t, path)
		journal.Identity.SHA256 = strings.Repeat("c", 64)
		writeDispatchJournalForTest(t, path, journal)
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		if response.ExitCode != 1 || fmt.Sprint(response.Data["status"]) != "invalid" {
			t.Fatalf("response = %#v", response)
		}
	})
}

func TestPipelineRuntimeInspectsOnlyLocalGitEvidence(t *testing.T) {
	t.Run("confirmed commit and tag are locally consistent", func(t *testing.T) {
		fixture := newTaggedResumeRelease(t)
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		localGit := pipelineJSONView(t, response.Data["local_git"])
		for key, want := range map[string]any{
			"scope":                         "local_only",
			"remote_freshness":              "remote_not_inspected",
			"expected_commit":               fixture.commitSHA,
			"commit_exists":                 true,
			"commit_content_verified":       true,
			"expected_tag":                  fixture.ctx.Tag,
			"tag_exists":                    true,
			"tag_matches_expected_commit":   true,
			"head_contains_expected_commit": true,
		} {
			if localGit[key] != want {
				t.Errorf("local_git[%s] = %#v, want %#v; full=%#v", key, localGit[key], want, localGit)
			}
		}
	})

	t.Run("commit content mismatch is invalid", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		ctx := newExecutionJournalContext(t, root)
		journal := newPreparedExecutionJournal(t, ctx)
		resolution, err := NewReleaseExecutionJournalStore(root).Prepare(journal)
		if err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		writePlannedStateForRecoveryTest(t, ctx)
		if err := os.WriteFile(filepath.Join(root, "unexpected.txt"), []byte("unexpected\n"), 0o644); err != nil {
			t.Fatalf("write unexpected file: %v", err)
		}
		gitCmd(t, root, "add", ".neko/release.state.json", "unexpected.txt")
		gitCmd(t, root, "commit", "-m", ReleaseCommitMessage(ctx))
		commitSHA := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
		advanceExecutionJournalToCommitCreated(t, resolution.Journal, commitSHA, time.Now())
		writeExecutionJournalFixture(t, resolution.Path, resolution.Journal)
		response := inspectPipelineRuntimeForTest(t, root)
		if response.ExitCode != 1 || fmt.Sprint(response.Data["status"]) != "invalid" {
			t.Fatalf("response = %#v", response)
		}
		localGit := pipelineJSONView(t, response.Data["local_git"])
		if localGit["commit_exists"] != true || localGit["commit_content_verified"] != false {
			t.Fatalf("local_git = %#v", localGit)
		}
	})

	t.Run("missing expected commit is invalid", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		ctx := newExecutionJournalContext(t, root)
		journal := newPreparedExecutionJournal(t, ctx)
		writePlannedStateForRecoveryTest(t, ctx)
		advanceExecutionJournalToCommitCreated(t, journal, strings.Repeat("1", 40), time.Now())
		store := NewReleaseExecutionJournalStore(root)
		path, err := store.JournalPath(journal.Identity)
		if err != nil {
			t.Fatalf("JournalPath: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		writeExecutionJournalFixture(t, path, journal)
		response := inspectPipelineRuntimeForTest(t, root)
		if response.ExitCode != 1 || fmt.Sprint(response.Data["status"]) != "invalid" {
			t.Fatalf("response = %#v", response)
		}
		localGit := pipelineJSONView(t, response.Data["local_git"])
		if localGit["commit_exists"] != false {
			t.Fatalf("local_git = %#v", localGit)
		}
	})

	t.Run("missing expected tag is invalid", func(t *testing.T) {
		fixture := newCommittedResumeRelease(t)
		journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath)
		if err := journal.BeginPending(ReleaseExecutionPendingCreateUnitTag, time.Now()); err != nil {
			t.Fatalf("pending tag: %v", err)
		}
		if err := journal.ConfirmPhase(ReleaseExecutionTagCreated, ReleaseExecutionJournalUpdate{TagTargetSHA: fixture.commitSHA}, time.Now()); err != nil {
			t.Fatalf("confirm tag: %v", err)
		}
		writeExecutionJournalFixture(t, fixture.executionPath, journal)
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		if response.ExitCode != 1 || fmt.Sprint(response.Data["status"]) != "invalid" {
			t.Fatalf("response = %#v", response)
		}
		localGit := pipelineJSONView(t, response.Data["local_git"])
		if localGit["tag_exists"] != false {
			t.Fatalf("local_git = %#v", localGit)
		}
	})

	t.Run("tag target mismatch is invalid", func(t *testing.T) {
		fixture := newTaggedResumeRelease(t)
		gitCmd(t, fixture.root, "tag", "-f", fixture.ctx.Tag, "HEAD~1")
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		if response.ExitCode != 1 || fmt.Sprint(response.Data["status"]) != "invalid" {
			t.Fatalf("response = %#v", response)
		}
		localGit := pipelineJSONView(t, response.Data["local_git"])
		if localGit["tag_exists"] != true || localGit["tag_matches_expected_commit"] != false {
			t.Fatalf("local_git = %#v", localGit)
		}
	})

	t.Run("index and worktree preserve recovery evidence", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		ctx := newExecutionJournalContext(t, root)
		journal := newPreparedExecutionJournal(t, ctx)
		store := NewReleaseExecutionJournalStore(root)
		if _, err := store.Prepare(journal); err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		writePlannedStateForRecoveryTest(t, ctx)
		if _, err := store.ConfirmPhase(journal.Identity, ReleaseExecutionPreflightValidated, ReleaseExecutionJournalUpdate{}); err != nil {
			t.Fatalf("preflight: %v", err)
		}
		if _, err := store.BeginPending(journal.Identity, ReleaseExecutionPendingApplyMaterialization); err != nil {
			t.Fatalf("pending materialization: %v", err)
		}
		if _, err := store.ConfirmPhase(journal.Identity, ReleaseExecutionMaterializationApplied, ReleaseExecutionJournalUpdate{}); err != nil {
			t.Fatalf("materialization: %v", err)
		}
		if _, err := store.BeginPending(journal.Identity, ReleaseExecutionPendingWriteState); err != nil {
			t.Fatalf("pending state: %v", err)
		}
		if _, err := store.ConfirmPhase(journal.Identity, ReleaseExecutionStateWritten, ReleaseExecutionJournalUpdate{}); err != nil {
			t.Fatalf("state: %v", err)
		}
		statePath := filepath.Join(root, ".neko", "release.state.json")
		gitCmd(t, root, "add", ".neko/release.state.json")
		content, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatalf("read state: %v", err)
		}
		if err := os.WriteFile(statePath, append(content, '\n'), 0o644); err != nil {
			t.Fatalf("change worktree state: %v", err)
		}
		response := inspectPipelineRuntimeForTest(t, root)
		localGit := pipelineJSONView(t, response.Data["local_git"])
		if localGit["index_contains_recovery_evidence"] != true || localGit["worktree_contains_recovery_evidence"] != true {
			t.Fatalf("local_git = %#v", localGit)
		}
	})
}

func TestPipelineRuntimePushAndIdentityBoundaries(t *testing.T) {
	t.Run("commit push confirmation remains local and blocked before tag push proof", func(t *testing.T) {
		fixture := newTaggedResumeRelease(t)
		request := preparePipelineDispatchRequest(t, fixture)
		persistDispatchPreparedExecution(t, fixture, request.Identity)
		journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath)
		if err := journal.BeginPending(ReleaseExecutionPendingPushReleaseCommit, time.Now()); err != nil {
			t.Fatalf("pending commit push: %v", err)
		}
		if err := journal.ConfirmPhase(ReleaseExecutionCommitPushed, ReleaseExecutionJournalUpdate{CommitPushStatus: "pushed"}, time.Now()); err != nil {
			t.Fatalf("confirm commit push: %v", err)
		}
		writeExecutionJournalFixture(t, fixture.executionPath, journal)
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		assertPipelineRuntimeStatus(t, response, "blocked")
		assertPipelineStageRuntime(t, response, "release-commit-push", "confirmed")
		assertPipelineStageRuntime(t, response, "unit-tag-push", "blocked")
		localGit := pipelineJSONView(t, response.Data["local_git"])
		if localGit["remote_freshness"] != "remote_not_inspected" {
			t.Fatalf("local_git = %#v", localGit)
		}
	})

	t.Run("tag push confirmation is resumable with prepared dispatch", func(t *testing.T) {
		fixture := newTaggedResumeRelease(t)
		request := preparePipelineDispatchRequest(t, fixture)
		persistTagPushedExecution(t, fixture, request.Identity)
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		assertPipelineRuntimeStatus(t, response, "resumable")
		assertPipelineStageRuntime(t, response, "release-commit-push", "confirmed")
		assertPipelineStageRuntime(t, response, "unit-tag-push", "confirmed")
		assertPipelineStageRuntime(t, response, "workflow-request-submission", "not_started")
	})

	t.Run("execution identity mismatch is invalid", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		journal := newPreparedExecutionJournal(t, newExecutionJournalContext(t, root))
		resolution, err := NewReleaseExecutionJournalStore(root).Prepare(journal)
		if err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		resolution.Journal.Identity.SHA256 = strings.Repeat("b", 64)
		writeExecutionJournalFixture(t, resolution.Path, resolution.Journal)
		response := inspectPipelineRuntimeForTest(t, root)
		if response.ExitCode != 1 || fmt.Sprint(response.Data["status"]) != "invalid" {
			t.Fatalf("response = %#v", response)
		}
	})
}
