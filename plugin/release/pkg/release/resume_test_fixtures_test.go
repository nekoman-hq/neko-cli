package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type resumeReleaseFixture struct {
	root          string
	bare          string
	ctx           *ReleaseExecutionContext
	executionPath string
	commitSHA     string
	dispatchPath  string
}

func newCommittedResumeRelease(t *testing.T) *resumeReleaseFixture {
	t.Helper()
	root, bare, ctx := newActiveGitHubActionsReleaseRepository(t)
	journal := newPreparedExecutionJournal(t, ctx)
	resolution, err := NewReleaseExecutionJournalStore(root).Prepare(journal)
	if err != nil {
		t.Fatalf("Prepare execution journal: %v", err)
	}
	state := NewStateTransaction(root)
	if err := state.CaptureSnapshot(); err != nil {
		t.Fatalf("CaptureSnapshot: %v", err)
	}
	if err := state.WriteUnitVersion(ctx.Unit.ID, ctx.NextVersion); err != nil {
		t.Fatalf("WriteUnitVersion: %v", err)
	}
	gitCmd(t, root, "add", ".neko/release.state.json")
	gitCmd(t, root, "commit", "-m", ReleaseCommitMessage(ctx))
	commitSHA := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	advanceExecutionJournalToCommitCreated(t, resolution.Journal, commitSHA, time.Now())
	writeExecutionJournalFixture(t, resolution.Path, resolution.Journal)
	return &resumeReleaseFixture{
		root:          root,
		bare:          bare,
		ctx:           ctx,
		executionPath: resolution.Path,
		commitSHA:     commitSHA,
	}
}

func newTaggedResumeRelease(t *testing.T) *resumeReleaseFixture {
	t.Helper()
	fixture := newCommittedResumeRelease(t)
	gitCmd(t, fixture.root, "tag", fixture.ctx.Tag, fixture.commitSHA)
	journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath)
	if err := journal.BeginPending(ReleaseExecutionPendingCreateUnitTag, time.Now()); err != nil {
		t.Fatalf("begin tag creation: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionTagCreated, ReleaseExecutionJournalUpdate{TagTargetSHA: fixture.commitSHA}, time.Now()); err != nil {
		t.Fatalf("confirm tag creation: %v", err)
	}
	writeExecutionJournalFixture(t, fixture.executionPath, journal)
	return fixture
}

func prepareAcceptedDispatchForResume(t *testing.T, fixture *resumeReleaseFixture) ReleaseDispatchIdentity {
	t.Helper()
	request, path := prepareStartedDispatchForResume(t, fixture)
	resolution, err := NewDispatchJournalStore(fixture.root).Transition(request, DispatchJournalAccepted, DispatchJournalMetadata{ResponseStatus: "204"}, "")
	if err != nil {
		t.Fatalf("persist accepted dispatch journal: %v", err)
	}
	fixture.dispatchPath = path
	if resolution.Path != path {
		t.Fatalf("accepted dispatch journal path changed: %s != %s", resolution.Path, path)
	}
	return request.Identity
}

func prepareRejectedDispatchForResume(t *testing.T, fixture *resumeReleaseFixture) ReleaseDispatchIdentity {
	t.Helper()
	request, path := prepareStartedDispatchForResume(t, fixture)
	if _, err := NewDispatchJournalStore(fixture.root).Transition(request, DispatchJournalRejected, DispatchJournalMetadata{ResponseStatus: "422"}, "request rejected"); err != nil {
		t.Fatalf("persist rejected dispatch journal: %v", err)
	}
	fixture.dispatchPath = path
	return request.Identity
}

func prepareUnknownDispatchForResume(t *testing.T, fixture *resumeReleaseFixture) ReleaseDispatchIdentity {
	t.Helper()
	request, path := prepareStartedDispatchForResume(t, fixture)
	if _, err := NewDispatchJournalStore(fixture.root).Transition(request, DispatchJournalUnknown, DispatchJournalMetadata{}, "outcome uncertain"); err != nil {
		t.Fatalf("persist uncertain dispatch journal: %v", err)
	}
	fixture.dispatchPath = path
	return request.Identity
}

func prepareStartedDispatchForResume(t *testing.T, fixture *resumeReleaseFixture) (*ReleaseDispatchRequest, string) {
	t.Helper()
	remoteURL := strings.TrimSpace(gitOutput(t, fixture.root, "remote", "get-url", "origin"))
	identity, err := newReleaseDispatchIdentity(
		"origin",
		remoteURL,
		fixture.ctx.Unit.ID,
		fixture.ctx.NextVersion,
		fixture.ctx.Tag,
		fixture.commitSHA,
		fixture.ctx.Workflow,
		fixture.ctx.Executor,
		fixture.ctx.Delivery,
	)
	if err != nil {
		t.Fatalf("newReleaseDispatchIdentity: %v", err)
	}
	request := &ReleaseDispatchRequest{
		RepositoryRemoteName: "origin",
		UnitID:               fixture.ctx.Unit.ID,
		Version:              fixture.ctx.NextVersion,
		Tag:                  fixture.ctx.Tag,
		ReleaseCommitSHA:     fixture.commitSHA,
		WorkflowPath:         fixture.ctx.Workflow,
		WorkflowFileName:     filepath.Base(fixture.ctx.Workflow),
		Delivery:             fixture.ctx.Delivery,
		Executor:             fixture.ctx.Executor,
		Inputs: map[string]string{
			"unit":        fixture.ctx.Unit.ID,
			"version":     fixture.ctx.NextVersion,
			"tag":         fixture.ctx.Tag,
			"release_sha": fixture.commitSHA,
		},
		Identity: identity,
	}
	store := NewDispatchJournalStore(fixture.root)
	resolution, err := store.Prepare(request)
	if err != nil {
		t.Fatalf("Prepare dispatch journal: %v", err)
	}
	if _, err := store.Transition(request, DispatchJournalRequestStarted, DispatchJournalMetadata{RequestStartedAt: time.Now()}, ""); err != nil {
		t.Fatalf("persist request-started dispatch journal: %v", err)
	}
	return request, resolution.Path
}

func persistDispatchPreparedExecution(t *testing.T, fixture *resumeReleaseFixture, identity ReleaseDispatchIdentity) {
	t.Helper()
	journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath)
	if err := journal.BeginPending(ReleaseExecutionPendingCreateDispatchJournal, time.Now()); err != nil {
		t.Fatalf("begin dispatch journal preparation: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionDispatchJournalPrepared, ReleaseExecutionJournalUpdate{DispatchJournalIdentity: identity.SHA256}, time.Now()); err != nil {
		t.Fatalf("confirm dispatch journal preparation: %v", err)
	}
	writeExecutionJournalFixture(t, fixture.executionPath, journal)
}

func persistTagPushedExecution(t *testing.T, fixture *resumeReleaseFixture, identity ReleaseDispatchIdentity) {
	t.Helper()
	persistDispatchPreparedExecution(t, fixture, identity)
	journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath)
	if err := journal.BeginPending(ReleaseExecutionPendingPushReleaseCommit, time.Now()); err != nil {
		t.Fatalf("begin commit push: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionCommitPushed, ReleaseExecutionJournalUpdate{CommitPushStatus: "pushed"}, time.Now()); err != nil {
		t.Fatalf("confirm commit push: %v", err)
	}
	if err := journal.BeginPending(ReleaseExecutionPendingPushUnitTag, time.Now()); err != nil {
		t.Fatalf("begin tag push: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionTagPushed, ReleaseExecutionJournalUpdate{TagPushStatus: "pushed"}, time.Now()); err != nil {
		t.Fatalf("confirm tag push: %v", err)
	}
	writeExecutionJournalFixture(t, fixture.executionPath, journal)
	branch := strings.TrimSpace(gitOutput(t, fixture.root, "symbolic-ref", "--short", "HEAD"))
	gitCmd(t, fixture.root, "push", "origin", "HEAD:refs/heads/"+branch)
	gitCmd(t, fixture.root, "push", "origin", "refs/tags/"+fixture.ctx.Tag+":refs/tags/"+fixture.ctx.Tag)
}

func writeExecutionJournalFixture(t *testing.T, path string, journal *ReleaseExecutionJournal) {
	t.Helper()
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		t.Fatalf("marshal execution journal fixture: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write execution journal fixture: %v", err)
	}
}
