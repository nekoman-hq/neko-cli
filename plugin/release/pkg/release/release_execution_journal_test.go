package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestBuildReleaseExecutionJournalIdentityIsDeterministicAndSensitive(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx := newExecutionJournalContext(t, root)
	files := newExecutionJournalKnownFiles(t, ctx)
	baseSHA := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	remote := strings.TrimSpace(gitOutput(t, root, "remote", "get-url", "origin"))

	journal := mustBuildExecutionJournal(t, ctx, files, baseSHA, remote)
	again := mustBuildExecutionJournal(t, ctx, files, baseSHA, remote)
	if journal.Identity.SHA256 != again.Identity.SHA256 {
		t.Fatalf("equivalent intent must produce same identity: %s != %s", journal.Identity.SHA256, again.Identity.SHA256)
	}
	if !isSafeDispatchIdentityHash(journal.Identity.SHA256) {
		t.Fatalf("identity hash is not filename-safe: %s", journal.Identity.SHA256)
	}

	tests := []struct { //nolint:govet // Mutation order mirrors identity fields.
		name   string
		mutate func(*ReleaseExecutionContext, *string, *string)
	}{
		{name: "base", mutate: func(_ *ReleaseExecutionContext, base, _ *string) { *base = strings.Repeat("a", 40) }},
		{name: "unit", mutate: func(ctx *ReleaseExecutionContext, _, _ *string) { ctx.Unit.ID = "web" }},
		{name: "version-tag", mutate: func(ctx *ReleaseExecutionContext, _, _ *string) {
			ctx.NextVersion = "0.2.2"
			ctx.Tag = "api/v0.2.2"
		}},
		{name: "executor", mutate: func(ctx *ReleaseExecutionContext, _, _ *string) { ctx.Executor = "jreleaser" }},
		{name: "delivery", mutate: func(ctx *ReleaseExecutionContext, _, _ *string) { ctx.Delivery = "local"; ctx.Workflow = "" }},
		{name: "workflow", mutate: func(ctx *ReleaseExecutionContext, _, _ *string) { ctx.Workflow = ".github/workflows/other.yml" }},
		{name: "remote", mutate: func(_ *ReleaseExecutionContext, _, remote *string) { *remote = "https://github.com/nekoman/other.git" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCtx := *ctx
			nextBase := baseSHA
			nextRemote := remote
			tt.mutate(&nextCtx, &nextBase, &nextRemote)
			nextJournal := mustBuildExecutionJournal(t, &nextCtx, files, nextBase, nextRemote)
			if nextJournal.Identity.SHA256 == journal.Identity.SHA256 {
				t.Fatalf("identity did not change for %s", tt.name)
			}
		})
	}
}

func TestReleaseExecutionJournalStoreCreatesAndReusesUnderGitCommonDir(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx := newExecutionJournalContext(t, root)
	journal := newPreparedExecutionJournal(t, ctx)
	store := NewReleaseExecutionJournalStore(root)

	resolution, err := store.Prepare(journal)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !resolution.Created || resolution.Journal.State != ReleaseExecutionPrepared {
		t.Fatalf("unexpected resolution: %#v", resolution)
	}
	commonDir := strings.TrimSpace(gitOutput(t, root, "rev-parse", "--git-common-dir"))
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(root, commonDir)
	}
	if !strings.HasPrefix(resolution.Path, filepath.Join(commonDir, "neko", "release", "executions")) {
		t.Fatalf("journal path %s not under git common dir %s", resolution.Path, commonDir)
	}
	if strings.Contains(resolution.Path, filepath.Join(root, ".neko")) {
		t.Fatalf("execution journal must not be in worktree .neko: %s", resolution.Path)
	}
	data := mustReadString(t, resolution.Path)
	for _, forbidden := range []string{"GITHUB_TOKEN", "Authorization", "Bearer", "secret-token"} {
		if strings.Contains(data, forbidden) {
			t.Fatalf("journal contains forbidden token-like value %q:\n%s", forbidden, data)
		}
	}
	if strings.Contains(data, journal.KnownReleaseFiles[0].AbsolutePath) {
		t.Fatalf("absolute path leaked into persisted journal:\n%s", data)
	}

	again, err := store.Prepare(journal)
	if err != nil {
		t.Fatalf("Prepare again: %v", err)
	}
	if !again.Reused || again.Created {
		t.Fatalf("expected reuse, got %#v", again)
	}
}

func TestReleaseExecutionJournalStoreSupportsLinkedWorktreeCommonDir(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	worktreeRoot := filepath.Join(t.TempDir(), "linked")
	gitCmd(t, root, "worktree", "add", worktreeRoot)
	ctx := newExecutionJournalContext(t, worktreeRoot)
	journal := newPreparedExecutionJournal(t, ctx)
	store := NewReleaseExecutionJournalStore(worktreeRoot)
	path, err := store.JournalPath(journal.Identity)
	if err != nil {
		t.Fatalf("JournalPath: %v", err)
	}
	if strings.Contains(path, filepath.Join(worktreeRoot, ".neko")) {
		t.Fatalf("linked worktree journal must not be in worktree .neko: %s", path)
	}
	commonDir := strings.TrimSpace(gitOutput(t, worktreeRoot, "rev-parse", "--git-common-dir"))
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(worktreeRoot, commonDir)
	}
	if !strings.HasPrefix(path, filepath.Join(commonDir, "neko", "release", "executions")) {
		t.Fatalf("journal path %s not under linked worktree common dir %s", path, commonDir)
	}
}

func TestReleaseExecutionJournalStoreRejectsCorruptAndImmutableMismatch(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx := newExecutionJournalContext(t, root)
	journal := newPreparedExecutionJournal(t, ctx)
	store := NewReleaseExecutionJournalStore(root)
	resolution, err := store.Prepare(journal)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	mismatch := *journal
	mismatch.NextVersion = "9.9.9"
	if _, err := store.Prepare(&mismatch); err == nil {
		t.Fatal("expected immutable mismatch")
	}
	if err := os.WriteFile(resolution.Path, []byte("{not-json"), 0600); err != nil {
		t.Fatalf("write corrupt journal: %v", err)
	}
	if _, err := store.Load(journal.Identity); err == nil || !strings.Contains(err.Error(), "parse release execution journal") {
		t.Fatalf("expected corrupt journal failure, got %v", err)
	}
}

func TestReleaseExecutionJournalStorePersistsPendingAndConfirmedPhase(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx := newExecutionJournalContext(t, root)
	journal := newPreparedExecutionJournal(t, ctx)
	store := NewReleaseExecutionJournalStore(root)
	resolution, err := store.Prepare(journal)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if _, err := store.ConfirmPhase(journal.Identity, ReleaseExecutionPreflightValidated, ReleaseExecutionJournalUpdate{}); err != nil {
		t.Fatalf("ConfirmPhase preflight: %v", err)
	}
	if _, err := store.BeginPending(journal.Identity, ReleaseExecutionPendingApplyMaterialization); err != nil {
		t.Fatalf("BeginPending: %v", err)
	}
	pending := loadReleaseExecutionJournalForTest(t, resolution.Path)
	if pending.PendingAction != ReleaseExecutionPendingApplyMaterialization {
		t.Fatalf("pending action not persisted: %#v", pending)
	}
	if _, err := store.ConfirmPhase(journal.Identity, ReleaseExecutionMaterializationApplied, ReleaseExecutionJournalUpdate{}); err != nil {
		t.Fatalf("ConfirmPhase: %v", err)
	}
	confirmed := loadReleaseExecutionJournalForTest(t, resolution.Path)
	if confirmed.State != ReleaseExecutionMaterializationApplied || confirmed.PendingAction != ReleaseExecutionPendingNone {
		t.Fatalf("confirmed phase not persisted: %#v", confirmed)
	}
}

func TestReleaseExecutionJournalStateMachineAndPendingActions(t *testing.T) {
	journal := newPreparedExecutionJournal(t, newExecutionJournalContext(t, newGitHubActionsDispatchRepository(t)))
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	if err := journal.ConfirmPhase(ReleaseExecutionMaterializationApplied, ReleaseExecutionJournalUpdate{}, now); err == nil {
		t.Fatal("confirming materialization without pending action must fail")
	}
	if err := journal.ConfirmPhase(ReleaseExecutionPreflightValidated, ReleaseExecutionJournalUpdate{}, now); err != nil {
		t.Fatalf("preflight transition: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionPrepared, ReleaseExecutionJournalUpdate{}, now); err == nil {
		t.Fatal("backward transition must fail")
	}
	if err := journal.BeginPending(ReleaseExecutionPendingApplyMaterialization, now); err != nil {
		t.Fatalf("BeginPending: %v", err)
	}
	if err := journal.BeginPending(ReleaseExecutionPendingWriteState, now); err == nil {
		t.Fatal("pending action must not be replaceable")
	}
	if err := journal.ConfirmPhase(ReleaseExecutionMaterializationApplied, ReleaseExecutionJournalUpdate{}, now); err != nil {
		t.Fatalf("confirm materialization: %v", err)
	}
	if journal.PendingAction != ReleaseExecutionPendingNone {
		t.Fatalf("pending action was not cleared: %s", journal.PendingAction)
	}
	if err := journal.BeginPending(ReleaseExecutionPendingWriteState, now); err != nil {
		t.Fatalf("BeginPending write state: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionStateWritten, ReleaseExecutionJournalUpdate{}, now); err != nil {
		t.Fatalf("confirm state: %v", err)
	}
	if err := journal.BeginPending(ReleaseExecutionPendingStageReleaseFiles, now); err != nil {
		t.Fatalf("BeginPending stage: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionReleaseFilesStaged, ReleaseExecutionJournalUpdate{}, now); err != nil {
		t.Fatalf("confirm stage: %v", err)
	}
	if err := journal.BeginPending(ReleaseExecutionPendingCreateReleaseCommit, now); err != nil {
		t.Fatalf("BeginPending commit: %v", err)
	}
	commitSHA := strings.Repeat("b", 40)
	if err := journal.ConfirmPhase(ReleaseExecutionCommitCreated, ReleaseExecutionJournalUpdate{ReleaseCommitSHA: commitSHA}, now); err != nil {
		t.Fatalf("confirm commit: %v", err)
	}
	if err := journal.BeginPending(ReleaseExecutionPendingCreateUnitTag, now); err != nil {
		t.Fatalf("BeginPending tag: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionTagCreated, ReleaseExecutionJournalUpdate{TagTargetSHA: commitSHA}, now); err != nil {
		t.Fatalf("confirm tag: %v", err)
	}
	if err := journal.BeginPending(ReleaseExecutionPendingCreateDispatchJournal, now); err != nil {
		t.Fatalf("BeginPending dispatch journal: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionDispatchJournalPrepared, ReleaseExecutionJournalUpdate{DispatchJournalIdentity: strings.Repeat("c", 64)}, now); err != nil {
		t.Fatalf("confirm dispatch journal: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionCommitPushed, ReleaseExecutionJournalUpdate{CommitPushStatus: "pushed"}, now); err == nil {
		t.Fatal("commit push requires pending action")
	}
}

func TestReleaseExecutionJournalOnceOnlyMetadataCannotChange(t *testing.T) {
	journal := newPreparedExecutionJournal(t, newExecutionJournalContext(t, newGitHubActionsDispatchRepository(t)))
	now := time.Now()
	advanceExecutionJournalToCommitCreated(t, journal, strings.Repeat("a", 40), now)
	if err := journal.applyOnceOnlyUpdate(ReleaseExecutionCommitCreated, ReleaseExecutionJournalUpdate{ReleaseCommitSHA: strings.Repeat("b", 40)}); err == nil {
		t.Fatal("release commit SHA must not change after set")
	}
}

func mustBuildExecutionJournal(t *testing.T, ctx *ReleaseExecutionContext, files KnownReleaseFiles, baseSHA, remote string) *ReleaseExecutionJournal {
	t.Helper()
	journal, err := BuildReleaseExecutionJournal(ctx, BuildReleasePlan(ctx), files, baseSHA, remote)
	if err != nil {
		t.Fatalf("BuildReleaseExecutionJournal: %v", err)
	}
	return journal
}

func newPreparedExecutionJournal(t *testing.T, ctx *ReleaseExecutionContext) *ReleaseExecutionJournal {
	t.Helper()
	files := newExecutionJournalKnownFiles(t, ctx)
	baseSHA := strings.TrimSpace(gitOutput(t, ctx.RepositoryRoot, "rev-parse", "HEAD"))
	remote := strings.TrimSpace(gitOutput(t, ctx.RepositoryRoot, "remote", "get-url", "origin"))
	return mustBuildExecutionJournal(t, ctx, files, baseSHA, remote)
}

func newExecutionJournalContext(t *testing.T, root string) *ReleaseExecutionContext {
	t.Helper()
	repository, err := releaseconfig.LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository: %v", err)
	}
	ctx, err := BuildReleaseExecutionContext(repository, repository.Units[0], Patch, false)
	if err != nil {
		t.Fatalf("BuildReleaseExecutionContext: %v", err)
	}
	return ctx
}

func newExecutionJournalKnownFiles(t *testing.T, ctx *ReleaseExecutionContext) KnownReleaseFiles {
	t.Helper()
	files, err := NewKnownReleaseFiles(ctx, nil)
	if err != nil {
		t.Fatalf("NewKnownReleaseFiles: %v", err)
	}
	return files
}

func loadReleaseExecutionJournalForTest(t *testing.T, path string) *ReleaseExecutionJournal {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	var journal ReleaseExecutionJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		t.Fatalf("decode journal: %v", err)
	}
	return &journal
}

func advanceExecutionJournalToCommitCreated(t *testing.T, journal *ReleaseExecutionJournal, commitSHA string, now time.Time) {
	t.Helper()
	if err := journal.ConfirmPhase(ReleaseExecutionPreflightValidated, ReleaseExecutionJournalUpdate{}, now); err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if err := journal.BeginPending(ReleaseExecutionPendingApplyMaterialization, now); err != nil {
		t.Fatalf("pending materialization: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionMaterializationApplied, ReleaseExecutionJournalUpdate{}, now); err != nil {
		t.Fatalf("materialization: %v", err)
	}
	if err := journal.BeginPending(ReleaseExecutionPendingWriteState, now); err != nil {
		t.Fatalf("pending state: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionStateWritten, ReleaseExecutionJournalUpdate{}, now); err != nil {
		t.Fatalf("state: %v", err)
	}
	if err := journal.BeginPending(ReleaseExecutionPendingStageReleaseFiles, now); err != nil {
		t.Fatalf("pending stage: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionReleaseFilesStaged, ReleaseExecutionJournalUpdate{}, now); err != nil {
		t.Fatalf("stage: %v", err)
	}
	if err := journal.BeginPending(ReleaseExecutionPendingCreateReleaseCommit, now); err != nil {
		t.Fatalf("pending commit: %v", err)
	}
	if err := journal.ConfirmPhase(ReleaseExecutionCommitCreated, ReleaseExecutionJournalUpdate{ReleaseCommitSHA: commitSHA}, now); err != nil {
		t.Fatalf("commit: %v", err)
	}
}
