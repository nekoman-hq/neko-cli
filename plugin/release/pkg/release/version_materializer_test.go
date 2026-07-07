package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestJReleaserMaterializerPlansOnlyJReleaserYML(t *testing.T) {
	root := newV2MaterializationRepository(t, "jreleaser")
	ctx := mustBuildTransactionContext(t, root, Minor)
	plan, err := JReleaserMaterializer{}.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := (JReleaserMaterializer{}).Validate(plan); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("expected one change, got %#v", plan.Changes)
	}
	change := plan.Changes[0]
	if change.RepositoryRelativePath != "jreleaser.yml" {
		t.Fatalf("expected jreleaser.yml, got %#v", change)
	}
	if !change.RequiredForReleaseCommit {
		t.Fatalf("expected jreleaser.yml to be required for release commit")
	}
	if !strings.Contains(string(change.AfterContent), "version: 0.3.0") {
		t.Fatalf("expected next version in materialized content:\n%s", string(change.AfterContent))
	}
	if mustReadString(t, filepath.Join(root, "jreleaser.yml")) != string(change.BeforeContent) {
		t.Fatal("Plan must not write jreleaser.yml")
	}
}

func TestGoReleaserMaterializerIsNoop(t *testing.T) {
	root := newV2MaterializationRepository(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	plan, err := GoReleaserMaterializer{}.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if err := (GoReleaserMaterializer{}).Validate(plan); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(plan.Changes) != 0 {
		t.Fatalf("expected no goreleaser materialization changes, got %#v", plan.Changes)
	}
}

func TestReleaseItMaterializerAdvertisesBlockOnly(t *testing.T) {
	root := newV2MaterializationRepository(t, "release-it")
	ctx := mustBuildTransactionContext(t, root, Patch)
	plan, err := ReleaseItMaterializer{}.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Changes) != 0 {
		t.Fatalf("release-it must not materialize files for V2 real execution, got %#v", plan.Changes)
	}
	if plan.BlockedReason == "" {
		t.Fatal("expected release-it blocked reason")
	}
}

func TestMaterializationPlanRejectsOutsideRepository(t *testing.T) {
	root := newV2MaterializationRepository(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	plan := newMaterializationPlan(ctx)
	plan.Changes = []MaterializedFileChange{{
		AbsolutePath:           outside,
		RepositoryRelativePath: "../outside.txt",
		AfterContent:           []byte("x"),
		Reason:                 "test outside repository rejection",
	}}
	if err := ValidateMaterializationPlan(&plan); err == nil {
		t.Fatal("expected outside repository error")
	}
}

func TestMaterializationPlanRejectsDuplicateTargets(t *testing.T) {
	root := newV2MaterializationRepository(t, "jreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	path := filepath.Join(root, "jreleaser.yml")
	change, err := newMaterializedFileChange(ctx, path, []byte("a"), []byte("b"), 0644, true, "test duplicate", true)
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	plan := newMaterializationPlan(ctx)
	plan.Changes = []MaterializedFileChange{change, change}
	if err := ValidateMaterializationPlan(&plan); err == nil {
		t.Fatal("expected duplicate target error")
	}
}

func newV2MaterializationRepository(t *testing.T, executor string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".neko"), 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".goreleaser.yml"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write goreleaser config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".release-it.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write release-it config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "jreleaser.yml"), []byte("project:\n  name: api\n  version: 0.2.0\nrelease:\n  github:\n    owner: nekoman-hq\n"), 0644); err != nil {
		t.Fatalf("write jreleaser config: %v", err)
	}
	cfg := `{"schemaVersion":2,"units":[{"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"` + executor + `","delivery":"local"}}]}`
	state := `{"schemaVersion":2,"units":{"api":{"version":"0.2.0"}}}`
	if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(cfg), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(releaseconfig.V2StatePath(root), []byte(state), 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	return root
}
