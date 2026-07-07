package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterializationTransactionRestoresExistingFileBytesAndMode(t *testing.T) {
	root := newV2MaterializationRepository(t, "jreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	path := filepath.Join(root, "jreleaser.yml")
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	before := mustReadString(t, path)
	change, err := newMaterializedFileChange(ctx, path, []byte(before), []byte("project:\n  version: 0.2.1\n"), 0600, true, "test restore", true)
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	plan := newMaterializationPlan(ctx)
	plan.Changes = []MaterializedFileChange{change}
	tx := NewMaterializationTransaction(&plan)
	if captureErr := tx.CaptureSnapshots(); captureErr != nil {
		t.Fatalf("CaptureSnapshots: %v", captureErr)
	}
	if _, applyErr := tx.Apply(); applyErr != nil {
		t.Fatalf("Apply: %v", applyErr)
	}
	if got := mustReadString(t, path); got == before {
		t.Fatal("expected materialized file to change")
	}
	if restoreErr := tx.Restore(); restoreErr != nil {
		t.Fatalf("Restore: %v", restoreErr)
	}
	if got := mustReadString(t, path); got != before {
		t.Fatalf("expected exact restore:\n%s", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected mode 0600, got %v", info.Mode().Perm())
	}
}

func TestMaterializationTransactionRemovesNewFileOnRestore(t *testing.T) {
	root := newV2MaterializationRepository(t, "goreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	path := filepath.Join(root, "generated.version")
	change, err := newMaterializedFileChange(ctx, path, nil, []byte("0.2.1\n"), 0644, false, "test generated file restore", true)
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	plan := newMaterializationPlan(ctx)
	plan.Changes = []MaterializedFileChange{change}
	tx := NewMaterializationTransaction(&plan)
	if err := tx.CaptureSnapshots(); err != nil {
		t.Fatalf("CaptureSnapshots: %v", err)
	}
	if _, err := tx.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected generated file: %v", err)
	}
	if err := tx.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected generated file removed, got %v", err)
	}
}

func TestMaterializationTransactionFailureRestoresAlreadyAppliedFiles(t *testing.T) {
	root := newV2MaterializationRepository(t, "jreleaser")
	ctx := mustBuildTransactionContext(t, root, Patch)
	firstPath := filepath.Join(root, "jreleaser.yml")
	before := mustReadString(t, firstPath)
	badPath := filepath.Join(root, "missing-dir", "file.txt")
	first, err := newMaterializedFileChange(ctx, firstPath, []byte(before), []byte("project:\n  version: 0.2.1\n"), 0644, true, "first change", true)
	if err != nil {
		t.Fatalf("first change: %v", err)
	}
	second, err := newMaterializedFileChange(ctx, badPath, nil, []byte("bad"), 0644, true, "write should fail", true)
	if err != nil {
		t.Fatalf("second change: %v", err)
	}
	plan := newMaterializationPlan(ctx)
	plan.Changes = []MaterializedFileChange{first, second}
	tx := NewMaterializationTransaction(&plan)
	if err := tx.CaptureSnapshots(); err != nil {
		t.Fatalf("CaptureSnapshots: %v", err)
	}
	if _, err := tx.Apply(); err == nil {
		t.Fatal("expected write failure")
	}
	if got := mustReadString(t, firstPath); got != before {
		t.Fatalf("first file was not restored after write failure:\n%s", got)
	}
}

func TestMaterializationTransactionSourceContainsNoDestructiveCleanup(t *testing.T) {
	source := mustReadString(t, filepath.Join("materialization_transaction.go"))
	if strings.Contains(source, "reset --hard") || strings.Contains(source, "clean -fd") {
		t.Fatal("materialization transaction must not contain destructive cleanup")
	}
}
