package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestHandleResumeReportsCorruptJournalScan(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	resolution := prepareUnresolvedExecutionJournal(t, root)
	if err := os.WriteFile(resolution.Path, []byte("{not-json"), 0600); err != nil {
		t.Fatalf("corrupt execution journal: %v", err)
	}
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(resumeRequest("api", true))

	assertResumeCommandError(t, resp, err, "JOURNAL_SCAN_FAILED", "parse release execution journal")
}

func TestHandleResumeReportsUnknownJournalStateDuringScan(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	resolution := prepareUnresolvedExecutionJournal(t, root)
	resolution.Journal.State = ReleaseExecutionJournalState("future-phase")
	writeExecutionJournalFixture(t, resolution.Path, resolution.Journal)
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(resumeRequest("api", true))

	assertResumeCommandError(t, resp, err, "JOURNAL_SCAN_FAILED", "invalid state")
}

func TestHandleResumeRejectsV1SourceFormat(t *testing.T) {
	root := t.TempDir()
	config := `{
  "project-name": "neko-cli",
  "project-owner": "nekoman-hq",
  "project-type": "backend",
  "release-system": "goreleaser",
  "version": "1.2.3"
}`
	if err := os.WriteFile(filepath.Join(root, releaseconfig.V1FileName), []byte(config), 0644); err != nil { //nolint:staticcheck // V1 compatibility contract.
		t.Fatalf("write V1 config: %v", err)
	}
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(resumeRequest("", false))

	assertResumeCommandError(t, resp, err, "RESUME_UNSUPPORTED", "supports V2 github-actions releases only")
}

func TestHandleResumeIgnoresJournalForDifferentRemote(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx := newExecutionJournalContext(t, root)
	files := newExecutionJournalKnownFiles(t, ctx)
	baseSHA := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	journal := mustBuildExecutionJournal(t, ctx, files, baseSHA, "https://github.com/nekoman/other.git")
	if _, err := NewReleaseExecutionJournalStore(root).Prepare(journal); err != nil {
		t.Fatalf("Prepare different-remote journal: %v", err)
	}
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(resumeRequest("api", true))

	assertResumeCommandError(t, resp, err, "NO_RESUMABLE_JOURNAL", "unit api")
}

func TestHandleResumeIgnoresJournalForDifferentUnit(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	journal := newPreparedExecutionJournal(t, newExecutionJournalContext(t, root))
	identity, err := newReleaseExecutionIdentity(
		journal.RepositoryRemote,
		journal.BaseCommitSHA,
		"web",
		journal.CurrentVersion,
		journal.NextVersion,
		journal.Tag,
		journal.Executor,
		journal.Delivery,
		journal.WorkflowPath,
	)
	if err != nil {
		t.Fatalf("newReleaseExecutionIdentity: %v", err)
	}
	journal.UnitID = "web"
	journal.Identity = identity
	if _, prepareErr := NewReleaseExecutionJournalStore(root).Prepare(journal); prepareErr != nil {
		t.Fatalf("Prepare different-unit journal: %v", prepareErr)
	}
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(resumeRequest("api", true))

	assertResumeCommandError(t, resp, err, "NO_RESUMABLE_JOURNAL", "unit api")
}

func TestHandleResumeRejectsUnsupportedJournalDelivery(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	config := `{"schemaVersion":2,"units":[{"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"goreleaser","delivery":"local"}}]}`
	if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(config), 0644); err != nil {
		t.Fatalf("write local-delivery config: %v", err)
	}
	repository, err := releaseconfig.LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository: %v", err)
	}
	ctx, err := BuildReleaseExecutionContext(repository, repository.Units[0], Patch, false)
	if err != nil {
		t.Fatalf("BuildReleaseExecutionContext: %v", err)
	}
	if _, prepareErr := NewReleaseExecutionJournalStore(root).Prepare(newPreparedExecutionJournal(t, ctx)); prepareErr != nil {
		t.Fatalf("Prepare local-delivery journal: %v", prepareErr)
	}
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertResumeCommandError(t, resp, err, "RESUME_UNSUPPORTED", "github-actions releases only")
}

func TestHandleResumeTreatsCompletedJournalAsNotResumable(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	resolution := prepareUnresolvedExecutionJournal(t, root)
	resolution.Journal.State = ReleaseExecutionHandoffReady
	writeExecutionJournalFixture(t, resolution.Path, resolution.Journal)
	beforeHEAD := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertResumeCommandError(t, resp, err, "NO_RESUMABLE_JOURNAL", "unit api")
	if afterHEAD := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD")); afterHEAD != beforeHEAD {
		t.Fatalf("completed journal handling changed HEAD: before=%s after=%s", beforeHEAD, afterHEAD)
	}
}

func TestHandleResumeBlocksRecoveryConflict(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	prepareUnresolvedExecutionJournal(t, root)
	state := `{"schemaVersion":2,"units":{"api":{"version":"0.2.2"}}}`
	if err := os.WriteFile(releaseconfig.V2StatePath(root), []byte(state), 0644); err != nil {
		t.Fatalf("write conflicting state: %v", err)
	}
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertResumeCommandError(t, resp, err, "RESUME_BLOCKED", "Known release files do not match")
}

func TestHandleResumeBlocksRecoveryCorruption(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	resolution := prepareUnresolvedExecutionJournal(t, root)
	resolution.Journal.Identity.SHA256 = ""
	writeExecutionJournalFixture(t, resolution.Path, resolution.Journal)
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertResumeCommandError(t, resp, err, "RESUME_BLOCKED", "structurally invalid")
}

func TestHandleResumeRejectsConfigJournalConflictWithoutLeakingToken(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	prepareUnresolvedExecutionJournal(t, root)
	workflow := filepath.Join(root, ".github", "workflows", "release-api-v2.yml")
	if err := os.WriteFile(workflow, []byte("name: replacement release\n"), 0644); err != nil {
		t.Fatalf("write replacement workflow: %v", err)
	}
	config := `{"schemaVersion":2,"units":[{"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-api-v2.yml"}}]}`
	if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(config), 0644); err != nil {
		t.Fatalf("write conflicting config: %v", err)
	}
	t.Setenv("GITHUB_TOKEN", releaseSecretSentinel)
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertResumeCommandError(t, resp, err, "JOURNAL_CONFLICT", "no longer matches")
	assertSecretAbsentFromResponse(t, resp)
}

func TestHandleResumeTreatsPreflightValidatedAsUnsafePreCommit(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	resolution := prepareUnresolvedExecutionJournal(t, root)
	store := NewReleaseExecutionJournalStore(root)
	if _, err := store.ConfirmPhase(resolution.Journal.Identity, ReleaseExecutionPreflightValidated, ReleaseExecutionJournalUpdate{}); err != nil {
		t.Fatalf("confirm preflight phase: %v", err)
	}
	t.Setenv("GITHUB_TOKEN", "test-token")
	withWorkingDirectoryRoot(t, root)

	resp, err := HandleResume(resumeRequest("api", false))

	assertResumeCommandError(t, resp, err, "RESUME_FAILED", "resume before release commit is not yet safe")
}

func prepareUnresolvedExecutionJournal(t *testing.T, root string) ReleaseExecutionJournalResolution {
	t.Helper()
	journal := newPreparedExecutionJournal(t, newExecutionJournalContext(t, root))
	resolution, err := NewReleaseExecutionJournalStore(root).Prepare(journal)
	if err != nil {
		t.Fatalf("Prepare execution journal: %v", err)
	}
	return *resolution
}

func resumeRequest(unit string, dryRun bool) plugin.Request {
	return plugin.Request{
		Command: "resume",
		Flags: map[string]any{
			"unit":    unit,
			"dry-run": dryRun,
		},
	}
}

func assertResumeCommandError(t *testing.T, resp *plugin.Response, err error, code, messagePart string) {
	t.Helper()
	if err != nil {
		t.Fatalf("resume command returned a Go error: %v", err)
	}
	if resp == nil || resp.Status != "error" || resp.Error == nil {
		t.Fatalf("expected resume plugin error response, got %#v", resp)
	}
	if resp.Error.Code != code || !strings.Contains(resp.Error.Message, messagePart) {
		t.Fatalf("unexpected resume error contract: code=%q message=%q", resp.Error.Code, resp.Error.Message)
	}
	if resp.Metadata.Command != "resume" || resp.Metadata.Plugin == "" || resp.Metadata.Version == "" || resp.Metadata.Timestamp.IsZero() {
		t.Fatalf("resume error metadata is incomplete: %#v", resp.Metadata)
	}
}
