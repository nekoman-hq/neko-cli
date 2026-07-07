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

type fakeTransactionExecutor struct { //nolint:govet // Test double field order follows use in assertions.
	name            string
	validateErr     error
	executeErr      error
	executed        bool
	requirementFile string
}

func (f *fakeTransactionExecutor) Name() string {
	return f.name
}

func (f *fakeTransactionExecutor) ValidateRequirements(_ *ReleaseExecutionContext) error {
	return f.validateErr
}

func (f *fakeTransactionExecutor) ResolveFiles(_ *ReleaseExecutionContext) ([]string, error) {
	if f.requirementFile == "" {
		return nil, nil
	}
	return []string{f.requirementFile}, nil
}

func (f *fakeTransactionExecutor) Execute(_ *ReleaseExecutionContext) error {
	f.executed = true
	return f.executeErr
}

func TestReleaseTransactionPreflightErrorWritesNothing(t *testing.T) {
	root := newCleanV2GitRepository(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	statePath := releaseconfig.V2StatePath(root)
	before := mustReadString(t, statePath)
	executor := &fakeTransactionExecutor{name: "goreleaser", validateErr: fmt.Errorf("preflight failed")}
	tx := mustNewReleaseTransaction(t, ctx, executor)

	_, err := tx.Execute()
	if err == nil {
		t.Fatal("expected preflight error")
	}
	if got := mustReadString(t, statePath); got != before {
		t.Fatalf("state changed after preflight error:\n%s", got)
	}
	if executor.executed {
		t.Fatal("executor started after preflight error")
	}
}

func TestReleaseTransactionRestoresStateBeforeIrreversiblePhase(t *testing.T) {
	root := newCleanV2GitRepository(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	statePath := releaseconfig.V2StatePath(root)
	before := mustReadString(t, statePath)
	executor := &fakeTransactionExecutor{name: "goreleaser"}
	tx := mustNewReleaseTransaction(t, ctx, executor)
	tx.AfterStateWrite = func(_ *ReleaseTransaction) error {
		return fmt.Errorf("executor preparation failed")
	}

	_, err := tx.Execute()
	if err == nil {
		t.Fatal("expected after-state error")
	}
	if !strings.Contains(err.Error(), "restored V2 state") {
		t.Fatalf("expected restore message, got %v", err)
	}
	if got := mustReadString(t, statePath); got != before {
		t.Fatalf("state was not restored:\n%s", got)
	}
	if executor.executed {
		t.Fatal("executor should not execute after preparation error")
	}
}

func TestReleaseTransactionDoesNotRestoreAfterCommitOrTagStarted(t *testing.T) {
	root := newCleanV2GitRepository(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	statePath := releaseconfig.V2StatePath(root)
	before := mustReadString(t, statePath)
	executor := &fakeTransactionExecutor{name: "goreleaser", executeErr: fmt.Errorf("commit failed")}
	tx := mustNewReleaseTransaction(t, ctx, executor)

	_, err := tx.Execute()
	if err == nil {
		t.Fatal("expected executor error")
	}
	if !strings.Contains(err.Error(), "no destructive rollback was attempted") {
		t.Fatalf("expected non-destructive recovery message, got %v", err)
	}
	if got := mustReadString(t, statePath); got == before {
		t.Fatal("state was restored after irreversible phase")
	}
	if tx.Tracker.Phase != ExecutionPhaseFailed || !tx.Tracker.Irreversible {
		t.Fatalf("unexpected tracker state: %#v", tx.Tracker)
	}
}

func TestReleaseTransactionStagesStateBeforeExecutor(t *testing.T) {
	root := newCleanV2GitRepository(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	executor := &fakeTransactionExecutor{name: "goreleaser"}
	tx := mustNewReleaseTransaction(t, ctx, executor)

	if _, err := tx.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !executor.executed {
		t.Fatal("expected executor to run")
	}
	staged := gitOutput(t, root, "diff", "--cached", "--name-only")
	if !strings.Contains(staged, ".neko/release.state.json") {
		t.Fatalf("expected state to be staged, got %q", staged)
	}
}

func TestReleaseTransactionMaterializesJReleaserBeforeStateAndCommitPhase(t *testing.T) {
	root := newCleanV2MaterializationGitRepository(t, "jreleaser")
	ctx := mustBuildTransactionContext(t, root, Minor)
	statePath := releaseconfig.V2StatePath(root)
	jreleaserPath := filepath.Join(root, "jreleaser.yml")
	beforeState := mustReadString(t, statePath)
	beforeJReleaser := mustReadString(t, jreleaserPath)
	executor := &fakeTransactionExecutor{name: "jreleaser"}
	tx := mustNewReleaseTransaction(t, ctx, executor)
	seenMaterializedBeforeState := false
	tx.AfterMaterialization = func(tx *ReleaseTransaction) error {
		if tx.Tracker.Phase != ExecutionPhaseMaterializationApplied {
			t.Fatalf("expected materialization phase, got %s", tx.Tracker.Phase)
		}
		if mustReadString(t, statePath) != beforeState {
			t.Fatal("state changed before materialization hook")
		}
		if !strings.Contains(mustReadString(t, jreleaserPath), "version: 0.3.0") {
			t.Fatal("jreleaser.yml was not materialized before state write")
		}
		seenMaterializedBeforeState = true
		return nil
	}

	if _, err := tx.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !seenMaterializedBeforeState {
		t.Fatal("materialization hook was not called")
	}
	if !executor.executed {
		t.Fatal("expected executor to run")
	}
	staged := gitOutput(t, root, "diff", "--cached", "--name-only")
	if !strings.Contains(staged, ".neko/release.state.json") || !strings.Contains(staged, "jreleaser.yml") {
		t.Fatalf("expected state and jreleaser.yml staged, got %q", staged)
	}
	if beforeJReleaser == mustReadString(t, jreleaserPath) {
		t.Fatal("expected jreleaser.yml to remain materialized after successful transaction")
	}
}

func TestReleaseTransactionRestoresMaterializationStateAndIndexBeforeCommitPhase(t *testing.T) {
	root := newCleanV2MaterializationGitRepository(t, "jreleaser")
	ctx := mustBuildTransactionContext(t, root, Minor)
	statePath := releaseconfig.V2StatePath(root)
	jreleaserPath := filepath.Join(root, "jreleaser.yml")
	beforeState := mustReadString(t, statePath)
	beforeJReleaser := mustReadString(t, jreleaserPath)
	executor := &fakeTransactionExecutor{name: "jreleaser"}
	tx := mustNewReleaseTransaction(t, ctx, executor)
	tx.AfterReleaseFilesStaged = func(_ *ReleaseTransaction) error {
		return fmt.Errorf("stop before commit")
	}

	_, err := tx.Execute()
	if err == nil {
		t.Fatal("expected pre-commit failure")
	}
	if !strings.Contains(err.Error(), "restored V2 state and materialized files") {
		t.Fatalf("expected restore message, got %v", err)
	}
	if got := mustReadString(t, statePath); got != beforeState {
		t.Fatalf("state not restored:\n%s", got)
	}
	if got := mustReadString(t, jreleaserPath); got != beforeJReleaser {
		t.Fatalf("jreleaser.yml not restored:\n%s", got)
	}
	if staged := strings.TrimSpace(gitOutput(t, root, "diff", "--cached", "--name-only")); staged != "" {
		t.Fatalf("expected transaction-staged files to be unstaged, got %q", staged)
	}
	if executor.executed {
		t.Fatal("executor should not start before commit failure")
	}
}

func TestReleaseTransactionBlocksReleaseItV2Local(t *testing.T) {
	root := newCleanV2GitRepository(t, "release-it")
	ctx := mustBuildTransactionContext(t, root, Patch)
	executor := &fakeTransactionExecutor{name: "release-it"}
	tx := mustNewReleaseTransaction(t, ctx, executor)
	before := mustReadString(t, releaseconfig.V2StatePath(root))

	_, err := tx.Execute()
	if err == nil {
		t.Fatal("expected release-it block")
	}
	if !strings.Contains(err.Error(), "V2 local release-it is blocked") {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := mustReadString(t, releaseconfig.V2StatePath(root)); got != before {
		t.Fatal("state changed before release-it block")
	}
	if executor.executed {
		t.Fatal("release-it executor should not start")
	}
}

func TestReleaseTransactionBlocksGitHubActionsBeforeExecutor(t *testing.T) {
	root := newCleanV2GitRepository(t, "goreleaser")
	repository, err := releaseconfig.LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository: %v", err)
	}
	unit := repository.Units[0]
	unit.Delivery = "github-actions"
	ctx, err := BuildReleaseExecutionContext(repository, unit, Patch, false)
	if err != nil {
		t.Fatalf("BuildReleaseExecutionContext: %v", err)
	}
	executor := &fakeTransactionExecutor{name: "goreleaser"}
	tx := mustNewReleaseTransaction(t, ctx, executor)

	_, err = tx.Execute()
	if err == nil {
		t.Fatal("expected github-actions block")
	}
	if !strings.Contains(err.Error(), "github-actions delivery is configured but not implemented yet") {
		t.Fatalf("unexpected error: %v", err)
	}
	if executor.executed {
		t.Fatal("executor should not start for github-actions")
	}
}

func TestReleaseTransactionSourceContainsNoDestructiveRollback(t *testing.T) {
	source := mustReadString(t, filepath.Join("release_transaction.go"))
	if strings.Contains(source, "reset --hard") || strings.Contains(source, "clean -fd") {
		t.Fatal("V2 transaction source must not contain destructive rollback commands")
	}
}

func newCleanV2GitRepository(t *testing.T, executor string) string {
	t.Helper()
	root := newV2StateTestRepository(t)
	cfg := fmt.Sprintf(`{"schemaVersion":2,"units":[{"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"%s","delivery":"local"}},{"id":"web","paths":["web/**"],"workingDirectory":".","tagPrefix":"web/v","executor":{"type":"jreleaser","delivery":"local"}}]}`, executor)
	if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(cfg), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	gitCmd(t, root, "init")
	gitCmd(t, root, "config", "user.email", "test@example.com")
	gitCmd(t, root, "config", "user.name", "Test User")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "initial")
	return root
}

func newCleanV2MaterializationGitRepository(t *testing.T, executor string) string {
	t.Helper()
	root := newV2MaterializationRepository(t, executor)
	gitCmd(t, root, "init")
	gitCmd(t, root, "config", "user.email", "test@example.com")
	gitCmd(t, root, "config", "user.name", "Test User")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "initial")
	return root
}

func mustBuildTransactionContext(t *testing.T, root string, releaseType Type) *ReleaseExecutionContext {
	t.Helper()
	repository, err := releaseconfig.LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository: %v", err)
	}
	ctx, err := BuildReleaseExecutionContext(repository, repository.Units[0], releaseType, false)
	if err != nil {
		t.Fatalf("BuildReleaseExecutionContext: %v", err)
	}
	return ctx
}

func mustNewReleaseTransaction(t *testing.T, ctx *ReleaseExecutionContext, executor transactionExecutor) *ReleaseTransaction {
	t.Helper()
	tx, err := NewReleaseTransaction(ctx, executor)
	if err != nil {
		t.Fatalf("NewReleaseTransaction: %v", err)
	}
	return tx
}

func gitCmd(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, string(output), err)
	}
}

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, string(output), err)
	}
	return string(output)
}
