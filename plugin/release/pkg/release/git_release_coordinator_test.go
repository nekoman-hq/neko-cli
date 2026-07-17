package release

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestGitReleasePreflightAcceptsCleanRepository(t *testing.T) {
	root := newCleanV2GitRepositoryWithRemote(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	files := mustKnownReleaseFiles(t, ctx, nil)

	preflight, err := NewGitReleaseCoordinator().Preflight(ctx, files)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if preflight.Remote != "origin" || preflight.Branch == "" || preflight.UpstreamBranch == "" {
		t.Fatalf("unexpected preflight result: %#v", preflight)
	}
}

func TestGitReleasePreflightBlocksUnstagedForeignChange(t *testing.T) {
	root := newCleanV2GitRepositoryWithRemote(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	files := mustKnownReleaseFiles(t, ctx, nil)
	if err := os.WriteFile(filepath.Join(root, "foreign.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write foreign file: %v", err)
	}

	_, err := NewGitReleaseCoordinator().Preflight(ctx, files)
	if err == nil || !strings.Contains(err.Error(), v2CleanlinessMessage) {
		t.Fatalf("expected clean worktree error, got %v", err)
	}
}

func TestGitReleasePreflightBlocksStagedForeignChange(t *testing.T) {
	root := newCleanV2GitRepositoryWithRemote(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	files := mustKnownReleaseFiles(t, ctx, nil)
	if err := os.WriteFile(filepath.Join(root, "foreign.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write foreign file: %v", err)
	}
	gitCmd(t, root, "add", "foreign.txt")

	_, err := NewGitReleaseCoordinator().Preflight(ctx, files)
	if err == nil || !strings.Contains(err.Error(), v2CleanlinessMessage) {
		t.Fatalf("expected clean index error, got %v", err)
	}
}

func TestGitReleasePreflightRequiresRemoteAndUpstream(t *testing.T) {
	root := newCleanV2GitRepository(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	files := mustKnownReleaseFiles(t, ctx, nil)

	_, err := NewGitReleaseCoordinator().Preflight(ctx, files)
	if err == nil || !strings.Contains(err.Error(), "upstream remote") {
		t.Fatalf("expected upstream remote error, got %v", err)
	}
}

func TestGitReleasePreflightRejectsKnownFileOutsideRepository(t *testing.T) {
	root := newCleanV2GitRepositoryWithRemote(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	files := KnownReleaseFiles{RepositoryRoot: root}
	files.Files = append(files.Files, KnownReleaseFile{AbsolutePath: filepath.Join(filepath.Dir(root), "outside-state.json")})

	_, err := NewGitReleaseCoordinator().Preflight(ctx, files)
	if err == nil || !strings.Contains(err.Error(), "outside repository root") {
		t.Fatalf("expected outside path error, got %v", err)
	}
}

func TestGitReleaseStageDryRunDoesNotMutateGit(t *testing.T) {
	root := newCleanV2GitRepositoryWithRemote(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	ctx.DryRun = true
	files := mustKnownReleaseFiles(t, ctx, nil)
	beforeHead := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))

	err := NewGitReleaseCoordinator().Stage(ctx, files)
	if err == nil || !strings.Contains(err.Error(), "dry run does not stage") {
		t.Fatalf("expected dry-run stage error, got %v", err)
	}
	if head := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD")); head != beforeHead {
		t.Fatalf("dry run changed HEAD: %s != %s", head, beforeHead)
	}
	if tag := strings.TrimSpace(gitOutput(t, root, "tag", "--list", ctx.Tag)); tag != "" {
		t.Fatalf("dry run created tag %q", tag)
	}
}

func TestGitReleaseStageOnlyKnownReleaseFiles(t *testing.T) {
	root := newCleanV2MaterializationGitRepositoryWithRemote(t, "jreleaser")
	ctx := mustBuildTransactionContext(t, root, Minor)
	_, files := prepareReleaseFilesForGitCoordinator(t, root, ctx)

	if err := NewGitReleaseCoordinator().Stage(ctx, files); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	staged := sortedNonEmptyLines(gitOutput(t, root, "diff", "--cached", "--name-only"))
	if !sameStringSet(staged, []string{".neko/release.state.json", "jreleaser.yml"}) {
		t.Fatalf("unexpected staged files: %#v", staged)
	}
}

func TestGitReleaseStageBlocksExtraStagedFileWithoutUnstagingIt(t *testing.T) {
	root := newCleanV2GitRepositoryWithRemote(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	_, files := prepareReleaseFilesForGitCoordinator(t, root, ctx)
	if err := os.WriteFile(filepath.Join(root, "foreign.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write foreign file: %v", err)
	}
	gitCmd(t, root, "add", "foreign.txt")

	err := NewGitReleaseCoordinator().Stage(ctx, files)
	if err == nil || !strings.Contains(err.Error(), "Foreign change") {
		t.Fatalf("expected foreign staged error, got %v", err)
	}
	staged := gitOutput(t, root, "diff", "--cached", "--name-only")
	if !strings.Contains(staged, "foreign.txt") {
		t.Fatalf("foreign staged file was altered: %q", staged)
	}
}

func TestGitReleaseVerifyStagedFilesDetectsMissingExpectedFile(t *testing.T) {
	root := newCleanV2MaterializationGitRepositoryWithRemote(t, "jreleaser")
	ctx := mustBuildTransactionContext(t, root, Minor)
	_, files := prepareReleaseFilesForGitCoordinator(t, root, ctx)
	gitCmd(t, root, "add", ".neko/release.state.json")

	err := NewGitReleaseCoordinator().VerifyStagedFiles(ctx, files)
	if err == nil || !strings.Contains(err.Error(), "differ from expected") {
		t.Fatalf("expected staged set error, got %v", err)
	}
}

func TestGitReleaseStageFailureUnstagesOnlyKnownFiles(t *testing.T) {
	root := newCleanV2MaterializationGitRepositoryWithRemote(t, "jreleaser")
	ctx := mustBuildTransactionContext(t, root, Minor)
	_, files := prepareReleaseFilesForGitCoordinator(t, root, ctx)
	gitCmd(t, root, "add", ".neko/release.state.json")
	if err := os.WriteFile(filepath.Join(root, "foreign.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write foreign file: %v", err)
	}
	gitCmd(t, root, "add", "foreign.txt")

	err := NewGitReleaseCoordinator().VerifyStagedFiles(ctx, files)
	if err == nil {
		t.Fatal("expected staged-set mismatch")
	}
	if unstageErr := NewGitReleaseCoordinator().UnstageKnown(files); unstageErr != nil {
		t.Fatalf("UnstageKnown: %v", unstageErr)
	}
	staged := gitOutput(t, root, "diff", "--cached", "--name-only")
	if strings.Contains(staged, ".neko/release.state.json") || strings.Contains(staged, "jreleaser.yml") {
		t.Fatalf("known release files remain staged: %q", staged)
	}
	if !strings.Contains(staged, "foreign.txt") {
		t.Fatalf("foreign staged file was removed: %q", staged)
	}
}

func TestGitReleaseCommitContainsExactFilesMessageAndVersion(t *testing.T) {
	root := newCleanV2MaterializationGitRepositoryWithRemote(t, "jreleaser")
	ctx := mustBuildTransactionContext(t, root, Minor)
	_, files := prepareReleaseFilesForGitCoordinator(t, root, ctx)
	coordinator := NewGitReleaseCoordinator()
	if err := coordinator.Stage(ctx, files); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	commitSHA, err := coordinator.Commit(ctx, files)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if subject := strings.TrimSpace(gitOutput(t, root, "log", "-1", "--format=%s")); subject != "chore(release): api api/v0.3.0" {
		t.Fatalf("unexpected commit subject %q", subject)
	}
	changed := sortedNonEmptyLines(gitOutput(t, root, "diff-tree", "--no-commit-id", "--name-only", "-r", commitSHA))
	if !sameStringSet(changed, []string{".neko/release.state.json", "jreleaser.yml"}) {
		t.Fatalf("unexpected commit files: %#v", changed)
	}
	state := gitOutput(t, root, "show", commitSHA+":.neko/release.state.json")
	if !strings.Contains(state, `"version": "0.3.0"`) {
		t.Fatalf("expected new version in committed state:\n%s", state)
	}
}

func TestGitReleaseCommitDryRunCreatesNoCommit(t *testing.T) {
	root := newCleanV2GitRepositoryWithRemote(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	ctx.DryRun = true
	files := mustKnownReleaseFiles(t, ctx, nil)
	beforeHead := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))

	if _, err := NewGitReleaseCoordinator().Commit(ctx, files); err == nil {
		t.Fatal("expected dry-run commit error")
	}
	if head := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD")); head != beforeHead {
		t.Fatalf("dry-run commit changed HEAD: %s != %s", head, beforeHead)
	}
}

func TestGitReleaseCreateUnitTagsAndIdempotency(t *testing.T) {
	root := newCleanV2GitRepositoryWithRemote(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	_, files := prepareReleaseFilesForGitCoordinator(t, root, ctx)
	coordinator := NewGitReleaseCoordinator()
	if err := coordinator.Stage(ctx, files); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	commitSHA, err := coordinator.Commit(ctx, files)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	created, err := coordinator.CreateTag(ctx, commitSHA)
	if err != nil {
		t.Fatalf("CreateTag api: %v", err)
	}
	if !created {
		t.Fatal("expected api tag to be created")
	}
	createdAgain, err := coordinator.CreateTag(ctx, commitSHA)
	if err != nil {
		t.Fatalf("CreateTag idempotent: %v", err)
	}
	if createdAgain {
		t.Fatal("expected existing tag on same commit to be idempotent")
	}

	webCtx := *ctx
	webCtx.Unit.ID = "web"
	webCtx.TagSpec, err = releaseconfig.NewTagSpec("web/v")
	if err != nil {
		t.Fatalf("NewTagSpec: %v", err)
	}
	webCtx.Tag = webCtx.TagSpec.Format(webCtx.NextVersion)
	if _, err := coordinator.CreateTag(&webCtx, commitSHA); err != nil {
		t.Fatalf("CreateTag web: %v", err)
	}
	tags := gitOutput(t, root, "tag", "--list")
	if !strings.Contains(tags, "api/v0.2.1") || !strings.Contains(tags, "web/v0.2.1") {
		t.Fatalf("expected separate unit tags, got %q", tags)
	}
}

func TestGitReleaseCreateTagRejectsExistingTagOnDifferentCommit(t *testing.T) {
	root := newCleanV2GitRepositoryWithRemote(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	initialHead := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	gitCmd(t, root, "tag", ctx.Tag, initialHead)
	_, files := prepareReleaseFilesForGitCoordinator(t, root, ctx)
	coordinator := NewGitReleaseCoordinator()
	if err := coordinator.Stage(ctx, files); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	commitSHA, err := coordinator.Commit(ctx, files)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if _, err := coordinator.CreateTag(ctx, commitSHA); err == nil || !strings.Contains(err.Error(), "already points") {
		t.Fatalf("expected tag conflict, got %v", err)
	}
}

func TestGitReleaseCreateTagDryRunCreatesNoTag(t *testing.T) {
	root := newCleanV2GitRepositoryWithRemote(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	ctx.DryRun = true
	if _, err := NewGitReleaseCoordinator().CreateTag(ctx, "abc123"); err == nil {
		t.Fatal("expected dry-run tag error")
	}
	if tag := strings.TrimSpace(gitOutput(t, root, "tag", "--list", ctx.Tag)); tag != "" {
		t.Fatalf("dry-run tag created %q", tag)
	}
}

func TestGitReleaseFocusedMethodsPushCommitThenTagToBareRemote(t *testing.T) {
	root := newCleanV2MaterializationGitRepositoryWithRemote(t, "jreleaser")
	ctx := mustBuildTransactionContext(t, root, Minor)
	coordinator := NewGitReleaseCoordinator()
	files := mustKnownReleaseFiles(t, ctx, mustPlanMaterialization(t, ctx))
	if _, err := coordinator.Preflight(ctx, files); err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	_, files = prepareReleaseFilesForGitCoordinator(t, root, ctx)
	gitCmd(t, root, "tag", "foreign/v9.9.9")

	result := runFocusedGitRelease(t, coordinator, ctx, files)
	if !result.CommitCreated || !result.TagCreated || !result.CommitPushed || !result.TagPushed {
		t.Fatalf("unexpected result: %#v", result)
	}
	remote := strings.TrimSpace(gitOutput(t, root, "remote", "get-url", "origin"))
	remoteTag := gitDirOutput(t, remote, "show-ref", "--tags", ctx.Tag)
	if !strings.Contains(remoteTag, "refs/tags/"+ctx.Tag) {
		t.Fatalf("expected remote unit tag, got %q", remoteTag)
	}
	if foreign := strings.TrimSpace(gitDirOutput(t, remote, "tag", "--list", "foreign/v9.9.9")); foreign != "" {
		t.Fatalf("foreign tag was pushed: %q", foreign)
	}
}

func TestGitReleaseCommitPushFailureSkipsTagPush(t *testing.T) {
	root := newCleanV2GitRepositoryWithRemote(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	_, files := prepareReleaseFilesForGitCoordinator(t, root, ctx)
	runner := &recordingGitRunner{failCommitPush: true}
	coordinator := newGitReleaseCoordinatorWithRunner(runner)

	result, err := runFocusedGitReleaseUntilPush(t, coordinator, ctx, files)
	if err == nil || !strings.Contains(err.Error(), "push V2 release commit") {
		t.Fatalf("expected commit push failure, got %v", err)
	}
	if result.CommitPushed || result.TagPushed {
		t.Fatalf("unexpected push result: %#v", result)
	}
	if runner.tagPushes != 0 {
		t.Fatalf("tag push should not start after commit push failure, got %d", runner.tagPushes)
	}
	if result.CommitSHA == "" || !strings.Contains(result.RecoveryGuidance, result.CommitSHA) {
		t.Fatalf("missing commit recovery guidance: %#v", result)
	}
}

func TestGitReleaseTagPushFailureDoesNotRollbackCommit(t *testing.T) {
	root := newCleanV2GitRepositoryWithRemote(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	_, files := prepareReleaseFilesForGitCoordinator(t, root, ctx)
	runner := &recordingGitRunner{failTagPush: true}
	coordinator := newGitReleaseCoordinatorWithRunner(runner)

	result, err := runFocusedGitReleaseUntilPush(t, coordinator, ctx, files)
	if err == nil || !strings.Contains(err.Error(), "push V2 unit tag") {
		t.Fatalf("expected tag push failure, got %v", err)
	}
	if !result.CommitPushed || result.TagPushed {
		t.Fatalf("unexpected push result: %#v", result)
	}
	if !strings.Contains(result.RecoveryGuidance, "was pushed, but tag") {
		t.Fatalf("unexpected recovery guidance: %q", result.RecoveryGuidance)
	}
	if head := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD")); head != result.CommitSHA {
		t.Fatalf("local commit was rolled back: %s != %s", head, result.CommitSHA)
	}
	if tag := strings.TrimSpace(gitOutput(t, root, "tag", "--points-at", result.CommitSHA)); !strings.Contains(tag, ctx.Tag) {
		t.Fatalf("local tag missing after failed tag push: %q", tag)
	}
}

func TestGitReleaseCoordinatorSourceContainsNoDestructiveRollbackOrRemoteDelete(t *testing.T) {
	for _, file := range []string{"git_release_coordinator.go", "git_release_preflight.go"} {
		source := mustReadString(t, filepath.Join(file))
		for _, forbidden := range []string{"reset --hard", "clean -fd", "--delete", "push origin --delete"} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s must not contain destructive command %q", file, forbidden)
			}
		}
	}
}

func runFocusedGitRelease(t *testing.T, coordinator *GitReleaseCoordinator, ctx *ReleaseExecutionContext, files KnownReleaseFiles) *GitReleaseResult {
	t.Helper()
	result, err := runFocusedGitReleaseUntilPush(t, coordinator, ctx, files)
	if err != nil {
		t.Fatalf("focused Git release: %v", err)
	}
	return result
}

func runFocusedGitReleaseUntilPush(t *testing.T, coordinator *GitReleaseCoordinator, ctx *ReleaseExecutionContext, files KnownReleaseFiles) (*GitReleaseResult, error) {
	t.Helper()
	if err := coordinator.Stage(ctx, files); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	commitSHA, err := coordinator.Commit(ctx, files)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	tagCreated, err := coordinator.CreateTag(ctx, commitSHA)
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	result := newGitReleaseResult(ctx, files)
	result.CommitSHA = commitSHA
	result.CommitCreated = true
	result.TagCreated = tagCreated
	return result, coordinator.Push(ctx, commitSHA, result)
}

type recordingGitRunner struct {
	failCommitPush bool
	failTagPush    bool
	tagPushes      int
}

func (runner *recordingGitRunner) Run(repositoryRoot string, args ...string) (string, error) {
	if len(args) >= 3 && args[0] == "push" {
		if strings.HasPrefix(args[2], "refs/tags/") {
			runner.tagPushes++
			if runner.failTagPush {
				return "", fmt.Errorf("simulated tag push failure")
			}
		}
		if strings.HasPrefix(args[2], "HEAD:") && runner.failCommitPush {
			return "", fmt.Errorf("simulated commit push failure")
		}
	}
	return execGitRunner{}.Run(repositoryRoot, args...)
}

func newCleanV2GitRepositoryWithRemote(t *testing.T, executor string) string {
	t.Helper()
	root := newCleanV2GitRepository(t, executor)
	addBareRemote(t, root)
	return root
}

func newCleanV2MaterializationGitRepositoryWithRemote(t *testing.T, executor string) string {
	t.Helper()
	root := newCleanV2MaterializationGitRepository(t, executor)
	addBareRemote(t, root)
	return root
}

func addBareRemote(t *testing.T, root string) {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "origin.git")
	gitCmd(t, root, "init", "--bare", remote)
	gitCmd(t, root, "remote", "add", "origin", remote)
	gitCmd(t, root, "push", "-u", "origin", "HEAD")
}

func mustPlanMaterialization(t *testing.T, ctx *ReleaseExecutionContext) *MaterializationPlan {
	t.Helper()
	materializer, err := ResolveVersionMaterializer(ctx.Executor)
	if err != nil {
		t.Fatalf("ResolveVersionMaterializer: %v", err)
	}
	plan, err := materializer.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := materializer.Validate(plan); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return plan
}

func mustKnownReleaseFiles(t *testing.T, ctx *ReleaseExecutionContext, plan *MaterializationPlan) KnownReleaseFiles {
	t.Helper()
	files, err := NewKnownReleaseFiles(ctx, plan)
	if err != nil {
		t.Fatalf("NewKnownReleaseFiles: %v", err)
	}
	return files
}

func prepareReleaseFilesForGitCoordinator(t *testing.T, root string, ctx *ReleaseExecutionContext) (*MaterializationPlan, KnownReleaseFiles) {
	t.Helper()
	plan := mustPlanMaterialization(t, ctx)
	materialization := NewMaterializationTransaction(plan)
	if err := materialization.CaptureSnapshots(); err != nil {
		t.Fatalf("CaptureSnapshots: %v", err)
	}
	if _, err := materialization.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	state := NewStateTransaction(root)
	if err := state.CaptureSnapshot(); err != nil {
		t.Fatalf("CaptureSnapshot: %v", err)
	}
	if err := state.WriteUnitVersion(ctx.Unit.ID, ctx.NextVersion); err != nil {
		t.Fatalf("WriteUnitVersion: %v", err)
	}
	return plan, mustKnownReleaseFiles(t, ctx, plan)
}

func gitDirOutput(t *testing.T, gitDir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"--git-dir", gitDir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git --git-dir %s %v: %s: %v", gitDir, args, string(output), err)
	}
	return string(output)
}
