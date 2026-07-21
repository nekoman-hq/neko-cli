package release

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestPipelineRuntimeProjectsAuthoritativeRecoveryAndResumePolicy(t *testing.T) {
	t.Run("prepared is active but not resumable", func(t *testing.T) {
		root := newGitHubActionsDispatchRepository(t)
		journal := newPreparedExecutionJournal(t, newExecutionJournalContext(t, root))
		if _, err := NewReleaseExecutionJournalStore(root).Prepare(journal); err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		response := inspectPipelineRuntimeForTest(t, root)
		assertPipelineRuntimeStatus(t, response, "active")
		recovery := pipelineJSONView(t, response.Data["recovery"])
		if recovery["classification"] != string(ReleaseExecutionRecoveryNotStarted) || recovery["resume_eligible"] != false || recovery["resume_refusal"] != "before_confirmed_commit" {
			t.Fatalf("recovery = %#v", recovery)
		}
	})

	t.Run("confirmed commit is resumable", func(t *testing.T) {
		fixture := newCommittedResumeRelease(t)
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		assertPipelineRuntimeStatus(t, response, "resumable")
		recovery := pipelineJSONView(t, response.Data["recovery"])
		if recovery["classification"] != string(ReleaseExecutionRecoveryInterruptedAfterCommit) || recovery["resume_eligible"] != true || recovery["resume_operation"] != "resume_from_commit_created" {
			t.Fatalf("recovery = %#v", recovery)
		}
	})

	t.Run("confirmed tag is resumable", func(t *testing.T) {
		fixture := newTaggedResumeRelease(t)
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		assertPipelineRuntimeStatus(t, response, "resumable")
		recovery := pipelineJSONView(t, response.Data["recovery"])
		if recovery["resume_operation"] != "resume_from_tag_created" {
			t.Fatalf("recovery = %#v", recovery)
		}
	})

	t.Run("staged commit pending requires manual intervention", func(t *testing.T) {
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
		assertPipelineRuntimeStatus(t, response, "blocked")
		manual := pipelineJSONView(t, response.Data["manual_intervention"])
		if manual["required"] != true {
			t.Fatalf("manual intervention = %#v", manual)
		}
	})

	t.Run("pending commit push is uncertain", func(t *testing.T) {
		fixture := newTaggedResumeRelease(t)
		request := preparePipelineDispatchRequest(t, fixture)
		persistDispatchPreparedExecution(t, fixture, request.Identity)
		store := NewReleaseExecutionJournalStore(fixture.root)
		if _, err := store.BeginPending(loadReleaseExecutionJournalForTest(t, fixture.executionPath).Identity, ReleaseExecutionPendingPushReleaseCommit); err != nil {
			t.Fatalf("pending commit push: %v", err)
		}
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		assertPipelineRuntimeStatus(t, response, "uncertain")
		recovery := pipelineJSONView(t, response.Data["recovery"])
		if recovery["resume_refusal"] != "ambiguous_commit_push" {
			t.Fatalf("recovery = %#v", recovery)
		}
	})

	t.Run("unproven commit push is blocked", func(t *testing.T) {
		fixture := newTaggedResumeRelease(t)
		request := preparePipelineDispatchRequest(t, fixture)
		persistDispatchPreparedExecution(t, fixture, request.Identity)
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		assertPipelineRuntimeStatus(t, response, "blocked")
		recovery := pipelineJSONView(t, response.Data["recovery"])
		if recovery["resume_refusal"] != "unproven_commit_push" || recovery["manual_intervention_required"] != true {
			t.Fatalf("recovery = %#v", recovery)
		}
	})

	t.Run("tag pushed with accepted dispatch is resumable without redispatch", func(t *testing.T) {
		fixture := newTaggedResumeRelease(t)
		identity := prepareAcceptedDispatchForResume(t, fixture)
		persistTagPushedExecution(t, fixture, identity)
		response := inspectPipelineRuntimeForTest(t, fixture.root)
		assertPipelineRuntimeStatus(t, response, "resumable")
		recovery := pipelineJSONView(t, response.Data["recovery"])
		if recovery["resume_operation"] != "resume_from_tag_pushed" || recovery["retry_safety"] != "accepted_request_reused" {
			t.Fatalf("recovery = %#v", recovery)
		}
	})
}

func preparePipelineDispatchRequest(t *testing.T, fixture *resumeReleaseFixture) *ReleaseDispatchRequest {
	t.Helper()
	remote := strings.TrimSpace(gitOutput(t, fixture.root, "remote", "get-url", "origin"))
	result := &GitReleaseResult{
		Unit: fixture.ctx.Unit.ID, Version: fixture.ctx.NextVersion,
		Tag: fixture.ctx.Tag, CommitSHA: fixture.commitSHA,
		RepositoryRemoteName: "origin", RepositoryRemote: remote,
		CommitCreated: true, TagCreated: true,
	}
	request, err := buildReleaseDispatchRequestForTest(fixture.ctx, result)
	if err != nil {
		t.Fatalf("BuildReleaseDispatchRequest: %v", err)
	}
	if _, err := NewDispatchJournalStore(fixture.root).Prepare(request); err != nil {
		t.Fatalf("Prepare dispatch: %v", err)
	}
	return request
}

func assertPipelineRuntimeStatus(t *testing.T, response *plugin.Response, want string) {
	t.Helper()
	if response.ExitCode != 0 || fmt.Sprint(response.Data["status"]) != want {
		t.Fatalf("status = %v exit=%d, want %s/0; data=%#v", response.Data["status"], response.ExitCode, want, response.Data)
	}
}
