package release

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestHandleResumeContinuesCommitCreatedThroughAcceptedHandoff(t *testing.T) {
	fixture := newCommittedResumeRelease(t)
	prepareAcceptedDispatchForResume(t, fixture)
	dispatchBefore := mustReadString(t, fixture.dispatchPath)
	t.Setenv("GITHUB_TOKEN", releaseSecretSentinel)
	withWorkingDirectoryRoot(t, fixture.root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertSuccessfulResumeResponse(t, resp, err)
	journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath)
	if journal.State != ReleaseExecutionHandoffReady || journal.ReleaseCommitSHA != journal.TagTargetSHA {
		t.Fatalf("commit-created resume did not reach a consistent handoff: %#v", journal)
	}
	if tagTarget := strings.TrimSpace(gitOutput(t, fixture.root, "rev-parse", "refs/tags/"+fixture.ctx.Tag+"^{}")); tagTarget != fixture.commitSHA {
		t.Fatalf("commit-created resume tagged %s, expected %s", tagTarget, fixture.commitSHA)
	}
	assertReleaseRefsPushed(t, fixture)
	if after := mustReadString(t, fixture.dispatchPath); after != dispatchBefore {
		t.Fatalf("accepted dispatch journal was rewritten during resume:\n%s", after)
	}
	assertSecretAbsentFromResponse(t, resp)
	assertSecretAbsentFromFiles(t, fixture.executionPath, fixture.dispatchPath)
}

func TestHandleResumeDoesNotAdvanceCommitCreatedWhenExpectedTagAlreadyExists(t *testing.T) {
	fixture := newCommittedResumeRelease(t)
	gitCmd(t, fixture.root, "tag", fixture.ctx.Tag, fixture.commitSHA)
	journalBefore := mustReadString(t, fixture.executionPath)
	t.Setenv("GITHUB_TOKEN", "test-token")
	withWorkingDirectoryRoot(t, fixture.root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertResumeCommandError(t, resp, err, "RESUME_FAILED", "resume from state commit-created requires manual inspection")
	if journalAfter := mustReadString(t, fixture.executionPath); journalAfter != journalBefore {
		t.Fatalf("existing expected tag changed the commit-created journal:\n%s", journalAfter)
	}
	if entries, readErr := os.ReadDir(filepath.Join(fixture.bare, "refs", "heads")); readErr != nil || len(entries) != 0 {
		t.Fatalf("existing expected tag advanced the remote branch: entries=%d err=%v", len(entries), readErr)
	}
}

func TestHandleResumeContinuesTagCreatedWithoutRepeatingCommitOrTag(t *testing.T) {
	fixture := newTaggedResumeRelease(t)
	prepareAcceptedDispatchForResume(t, fixture)
	commitCountBefore := strings.TrimSpace(gitOutput(t, fixture.root, "rev-list", "--count", "HEAD"))
	tagPath := filepath.Join(fixture.root, ".git", "refs", "tags", filepath.FromSlash(fixture.ctx.Tag))
	tagBefore, err := os.ReadFile(tagPath)
	if err != nil {
		t.Fatalf("read tag before resume: %v", err)
	}
	tagInfoBefore, err := os.Stat(tagPath)
	if err != nil {
		t.Fatalf("stat tag before resume: %v", err)
	}
	t.Setenv("GITHUB_TOKEN", "test-token")
	withWorkingDirectoryRoot(t, fixture.root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertSuccessfulResumeResponse(t, resp, err)
	if commitCountAfter := strings.TrimSpace(gitOutput(t, fixture.root, "rev-list", "--count", "HEAD")); commitCountAfter != commitCountBefore {
		t.Fatalf("tag-created resume repeated the release commit: before=%s after=%s", commitCountBefore, commitCountAfter)
	}
	tagAfter, err := os.ReadFile(tagPath)
	if err != nil {
		t.Fatalf("read tag after resume: %v", err)
	}
	tagInfoAfter, err := os.Stat(tagPath)
	if err != nil {
		t.Fatalf("stat tag after resume: %v", err)
	}
	if string(tagAfter) != string(tagBefore) || !tagInfoAfter.ModTime().Equal(tagInfoBefore.ModTime()) {
		t.Fatal("tag-created resume rewrote the already confirmed unit tag")
	}
	assertReleaseRefsPushed(t, fixture)
	if journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath); journal.State != ReleaseExecutionHandoffReady {
		t.Fatalf("tag-created resume did not reach handoff: %#v", journal)
	}
}

func TestHandleResumeContinuesTagPushedWithoutRepushingOrRedispatching(t *testing.T) {
	fixture := newTaggedResumeRelease(t)
	identity := prepareAcceptedDispatchForResume(t, fixture)
	persistTagPushedExecution(t, fixture, identity)
	dispatchBefore := mustReadString(t, fixture.dispatchPath)
	gitCmd(t, fixture.root, "remote", "set-url", "--push", "origin", filepath.Join(t.TempDir(), "missing.git"))
	t.Setenv("GITHUB_TOKEN", releaseSecretSentinel)
	withWorkingDirectoryRoot(t, fixture.root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertSuccessfulResumeResponse(t, resp, err)
	if after := mustReadString(t, fixture.dispatchPath); after != dispatchBefore {
		t.Fatalf("accepted dispatch journal was rewritten during tag-pushed resume:\n%s", after)
	}
	if journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath); journal.State != ReleaseExecutionHandoffReady {
		t.Fatalf("tag-pushed resume did not confirm handoff: %#v", journal)
	}
	assertSecretAbsentFromResponse(t, resp)
	assertSecretAbsentFromFiles(t, fixture.executionPath, fixture.dispatchPath)
}

func TestHandleResumeRejectsAmbiguousPendingCommitPush(t *testing.T) {
	fixture := newTaggedResumeRelease(t)
	identity := prepareAcceptedDispatchForResume(t, fixture)
	persistDispatchPreparedExecution(t, fixture, identity)
	journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath)
	if err := journal.BeginPending(ReleaseExecutionPendingPushReleaseCommit, time.Now()); err != nil {
		t.Fatalf("begin pending commit push: %v", err)
	}
	writeExecutionJournalFixture(t, fixture.executionPath, journal)
	t.Setenv("GITHUB_TOKEN", "")
	withWorkingDirectoryRoot(t, fixture.root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertResumeCommandError(t, resp, err, "RESUME_BLOCKED", "pending push action is ambiguous")
	if after := loadReleaseExecutionJournalForTest(t, fixture.executionPath); after.State != ReleaseExecutionDispatchJournalPrepared || after.PendingAction != ReleaseExecutionPendingPushReleaseCommit {
		t.Fatalf("ambiguous commit push state changed during blocked resume: %#v", after)
	}
}

func TestHandleResumeRejectsAmbiguousPendingTagPush(t *testing.T) {
	fixture := newTaggedResumeRelease(t)
	identity := prepareAcceptedDispatchForResume(t, fixture)
	persistDispatchPreparedExecution(t, fixture, identity)
	journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath)
	if err := journal.BeginPending(ReleaseExecutionPendingPushReleaseCommit, time.Now()); err != nil {
		t.Fatalf("begin commit push: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionCommitPushed, ReleaseExecutionJournalUpdate{CommitPushStatus: "pushed"}, time.Now()); err != nil {
		t.Fatalf("confirm commit push: %v", err)
	}
	if err := journal.BeginPending(ReleaseExecutionPendingPushUnitTag, time.Now()); err != nil {
		t.Fatalf("begin pending tag push: %v", err)
	}
	writeExecutionJournalFixture(t, fixture.executionPath, journal)
	t.Setenv("GITHUB_TOKEN", "")
	withWorkingDirectoryRoot(t, fixture.root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertResumeCommandError(t, resp, err, "RESUME_BLOCKED", "pending push action is ambiguous")
	if after := loadReleaseExecutionJournalForTest(t, fixture.executionPath); after.State != ReleaseExecutionCommitPushed || after.PendingAction != ReleaseExecutionPendingPushUnitTag {
		t.Fatalf("ambiguous tag push state changed during blocked resume: %#v", after)
	}
}

func TestHandleResumeDoesNotInferCommitPushFromDispatchJournalPreparation(t *testing.T) {
	fixture := newTaggedResumeRelease(t)
	identity := prepareAcceptedDispatchForResume(t, fixture)
	persistDispatchPreparedExecution(t, fixture, identity)
	t.Setenv("GITHUB_TOKEN", "test-token")
	withWorkingDirectoryRoot(t, fixture.root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertResumeCommandError(t, resp, err, "RESUME_FAILED", "cannot prove push completion for state dispatch-journal-prepared")
	if journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath); journal.State != ReleaseExecutionDispatchJournalPrepared {
		t.Fatalf("blocked resume changed dispatch-prepared state: %#v", journal)
	}
}

func TestHandleResumeDoesNotInferTagPushFromCommitPushConfirmation(t *testing.T) {
	fixture := newTaggedResumeRelease(t)
	identity := prepareAcceptedDispatchForResume(t, fixture)
	persistDispatchPreparedExecution(t, fixture, identity)
	journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath)
	if err := journal.BeginPending(ReleaseExecutionPendingPushReleaseCommit, time.Now()); err != nil {
		t.Fatalf("begin commit push: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionCommitPushed, ReleaseExecutionJournalUpdate{CommitPushStatus: "pushed"}, time.Now()); err != nil {
		t.Fatalf("confirm commit push: %v", err)
	}
	writeExecutionJournalFixture(t, fixture.executionPath, journal)
	t.Setenv("GITHUB_TOKEN", "test-token")
	withWorkingDirectoryRoot(t, fixture.root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertResumeCommandError(t, resp, err, "RESUME_FAILED", "cannot prove push completion for state commit-pushed")
	if after := loadReleaseExecutionJournalForTest(t, fixture.executionPath); after.State != ReleaseExecutionCommitPushed || after.TagPushStatus != "" {
		t.Fatalf("blocked resume inferred tag push completion: %#v", after)
	}
}

func TestHandleResumeDoesNotRetryRejectedDispatch(t *testing.T) {
	fixture := newTaggedResumeRelease(t)
	identity := prepareRejectedDispatchForResume(t, fixture)
	persistTagPushedExecution(t, fixture, identity)
	dispatchBefore := mustReadString(t, fixture.dispatchPath)
	t.Setenv("GITHUB_TOKEN", releaseSecretSentinel)
	withWorkingDirectoryRoot(t, fixture.root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertResumeCommandError(t, resp, err, "RESUME_FAILED", "dispatch journal is rejected")
	if after := mustReadString(t, fixture.dispatchPath); after != dispatchBefore {
		t.Fatalf("rejected dispatch journal changed during blocked retry:\n%s", after)
	}
	if journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath); journal.State != ReleaseExecutionTagPushed {
		t.Fatalf("rejected dispatch falsely completed handoff: %#v", journal)
	}
	assertSecretAbsentFromResponse(t, resp)
}

func TestHandleResumeDoesNotRetryUncertainDispatch(t *testing.T) {
	fixture := newTaggedResumeRelease(t)
	identity := prepareUnknownDispatchForResume(t, fixture)
	persistTagPushedExecution(t, fixture, identity)
	dispatchBefore := mustReadString(t, fixture.dispatchPath)
	t.Setenv("GITHUB_TOKEN", releaseSecretSentinel)
	withWorkingDirectoryRoot(t, fixture.root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertResumeCommandError(t, resp, err, "RESUME_FAILED", "dispatch journal is unknown")
	if after := mustReadString(t, fixture.dispatchPath); after != dispatchBefore {
		t.Fatalf("uncertain dispatch journal changed during blocked retry:\n%s", after)
	}
	if journal := loadReleaseExecutionJournalForTest(t, fixture.executionPath); journal.State != ReleaseExecutionTagPushed {
		t.Fatalf("uncertain dispatch falsely completed handoff: %#v", journal)
	}
	assertSecretAbsentFromResponse(t, resp)
	assertSecretAbsentFromFiles(t, fixture.executionPath, fixture.dispatchPath)
}

func assertSuccessfulResumeResponse(t *testing.T, resp *plugin.Response, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("resume returned a Go error: %v", err)
	}
	if resp == nil || resp.Status != "success" || resp.Error != nil {
		t.Fatalf("expected successful resume response, got %#v", resp)
	}
	wantProperties := []string{
		"Unit", "Version", "Tag", "Release Commit", "Workflow", "Execution Journal", "Dispatch Journal",
		"Execution State", "Dispatch State", "Dispatch Run", "Status",
	}
	if got := responseProperties(t, resp.Data["items"]); !slices.Equal(got, wantProperties) {
		t.Fatalf("unexpected successful resume response order: got %#v want %#v", got, wantProperties)
	}
	if resp.Metadata.Command != "resume" || resp.RendererHint != "table" {
		t.Fatalf("unexpected successful resume metadata or renderer: %#v", resp)
	}
}

func assertReleaseRefsPushed(t *testing.T, fixture *resumeReleaseFixture) {
	t.Helper()
	branch := strings.TrimSpace(gitOutput(t, fixture.root, "symbolic-ref", "--short", "HEAD"))
	if got := strings.TrimSpace(gitDirOutput(t, fixture.bare, "rev-parse", "refs/heads/"+branch)); got != fixture.commitSHA {
		t.Fatalf("remote branch target = %s, expected %s", got, fixture.commitSHA)
	}
	if got := strings.TrimSpace(gitDirOutput(t, fixture.bare, "rev-parse", "refs/tags/"+fixture.ctx.Tag+"^{}")); got != fixture.commitSHA {
		t.Fatalf("remote tag target = %s, expected %s", got, fixture.commitSHA)
	}
}

func assertSecretAbsentFromFiles(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if strings.Contains(mustReadString(t, path), releaseSecretSentinel) {
			t.Fatal("secret sentinel appeared in a persisted release file")
		}
	}
}
