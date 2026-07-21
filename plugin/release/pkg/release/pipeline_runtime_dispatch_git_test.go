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
}
