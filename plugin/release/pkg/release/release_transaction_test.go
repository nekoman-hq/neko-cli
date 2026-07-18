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

func TestReleaseTransactionBlocksV2NonDryRunBeforeAnyMutation(t *testing.T) {
	root := newCleanV2GitRepository(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	statePath := releaseconfig.V2StatePath(root)
	before := mustReadString(t, statePath)
	executor := &fakeTransactionExecutor{name: "goreleaser"}
	tx := mustNewReleaseTransaction(t, ctx, executor)

	_, err := tx.Execute()
	if err == nil {
		t.Fatal("expected V2 local delivery block")
	}
	if !strings.Contains(err.Error(), "V2 local delivery is not supported") {
		t.Fatalf("expected local delivery rejection, got %v", err)
	}
	if got := mustReadString(t, statePath); got != before {
		t.Fatalf("state changed despite local delivery rejection:\n%s", got)
	}
	if executor.executed {
		t.Fatal("executor started despite local delivery rejection")
	}
	if status := strings.TrimSpace(gitOutput(t, root, "status", "--porcelain")); status != "" {
		t.Fatalf("expected clean repository after block, got %q", status)
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
		t.Fatal("expected V2 block")
	}
	if !strings.Contains(err.Error(), "V2 local delivery is not supported") {
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
		t.Fatal("expected V2 block")
	}
	if !strings.Contains(err.Error(), "V2 local delivery is not supported") {
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
	for _, removedScaffold := range []string{
		"prepareReleaseFilesForCoordinator",
		"AfterMaterialization",
		"AfterStateWrite",
		"ensureGitClean",
		"unstageKnownFiles",
		"restore --staged",
	} {
		if strings.Contains(source, removedScaffold) {
			t.Fatalf("release transaction retained inactive local scaffold %q", removedScaffold)
		}
	}
}

func newCleanV2GitRepository(t *testing.T, executor string) string {
	t.Helper()
	root := newV2StateTestRepository(t)
	mustWriteReleaseTestFile(t, root, ".github/workflows/release-api.yml", "name: release api\n")
	mustWriteReleaseTestFile(t, root, ".github/workflows/release-web.yml", "name: release web\n")
	cfg := fmt.Sprintf(`{"schemaVersion":2,"units":[{"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"%s","delivery":"github-actions","workflow":".github/workflows/release-api.yml"}},{"id":"web","paths":["web/**"],"workingDirectory":".","tagPrefix":"web/v","executor":{"type":"jreleaser","delivery":"github-actions","workflow":".github/workflows/release-web.yml"}}]}`, executor)
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

func mustWriteReleaseTestFile(t *testing.T, root, relativePath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
