package update

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicReplacementPreservesModeAndHasNoBackup(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "neko")
	oldContent := []byte("old executable")
	if err := os.WriteFile(target, oldContent, 0o700); err != nil {
		t.Fatal(err)
	}
	installation := installation{canonicalTarget: target, targetParent: root, targetMode: 0o700}
	reservation, err := newOSReplacementCapability().Reserve(installation)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	reservedPath := reservation.Path()
	if filepath.Dir(reservedPath) != root || reservedPath == target {
		t.Fatalf("reserved path = %q", reservedPath)
	}
	if err := reservation.Commit([]byte("new executable"), installation.targetMode); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := reservation.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new executable" {
		t.Fatalf("target = %q", content)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %#o", info.Mode().Perm())
	}
	if _, err := os.Stat(target + ".backup"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("fixed backup exists or stat failed: %v", err)
	}
	if _, err := os.Stat(reservedPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("renamed reservation still exists: %v", err)
	}
}

func TestAtomicReplacementRenameFailureLeavesTargetByteIdentical(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "neko")
	oldContent := []byte("old executable")
	if err := os.WriteFile(target, oldContent, 0o755); err != nil {
		t.Fatal(err)
	}
	ops := &faultReplacementOps{replacementOps: osReplacementOps{}, renameErr: errors.New("frozen rename failure")}
	capability := &osReplacementCapability{ops: ops}
	reservation, err := capability.Reserve(installation{canonicalTarget: target, targetParent: root, targetMode: 0o755})
	if err != nil {
		t.Fatal(err)
	}
	if err := reservation.Commit([]byte("new executable"), 0o755); err == nil || !strings.Contains(err.Error(), "frozen rename failure") {
		t.Fatalf("Commit error = %v", err)
	}
	if err := reservation.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, oldContent) {
		t.Fatalf("target changed to %q", content)
	}
}

func TestAtomicReplacementReportsCommittedDirectorySyncFailure(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "neko")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	ops := &faultReplacementOps{replacementOps: osReplacementOps{}, directorySyncErr: errors.New("frozen directory sync failure")}
	reservation, err := (&osReplacementCapability{ops: ops}).Reserve(installation{canonicalTarget: target, targetParent: root, targetMode: 0o755})
	if err != nil {
		t.Fatal(err)
	}
	err = reservation.Commit([]byte("new"), 0o755)
	var updateErr *updateError
	if !errors.As(err, &updateErr) || updateErr.kind != errorCommittedSync || !updateErr.changed {
		t.Fatalf("Commit error = %#v", err)
	}
	content, readErr := os.ReadFile(target)
	if readErr != nil || string(content) != "new" {
		t.Fatalf("committed target = %q err=%v", content, readErr)
	}
	if cleanupErr := reservation.Cleanup(); cleanupErr != nil {
		t.Fatalf("Cleanup after commit: %v", cleanupErr)
	}
}

func TestReplacementPrecommitFailureCleanupIsAccurate(t *testing.T) {
	file := &faultReplacementFile{name: "/fixture/.neko-update-1", writeErr: errors.New("frozen write failure")}
	ops := &memoryReplacementOps{file: file}
	reservation, err := (&osReplacementCapability{ops: ops}).Reserve(installation{canonicalTarget: "/fixture/neko", targetParent: "/fixture", targetMode: 0o755})
	if err != nil {
		t.Fatal(err)
	}
	if err := reservation.Commit([]byte("new"), 0o755); err == nil || !strings.Contains(err.Error(), "frozen write failure") {
		t.Fatalf("Commit error = %v", err)
	}
	if err := reservation.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if ops.removeCalls != 1 {
		t.Fatalf("remove calls = %d", ops.removeCalls)
	}

	file = &faultReplacementFile{name: "/fixture/.neko-update-2", writeErr: errors.New("frozen write failure")}
	ops = &memoryReplacementOps{file: file, removeErr: errors.New("frozen cleanup failure")}
	reservation, err = (&osReplacementCapability{ops: ops}).Reserve(installation{canonicalTarget: "/fixture/neko", targetParent: "/fixture", targetMode: 0o755})
	if err != nil {
		t.Fatal(err)
	}
	_ = reservation.Commit([]byte("new"), 0o755)
	if err := reservation.Cleanup(); err == nil || !strings.Contains(err.Error(), "frozen cleanup failure") {
		t.Fatalf("Cleanup error = %v", err)
	}
}

type faultReplacementOps struct {
	replacementOps
	renameErr        error
	directorySyncErr error
}

func (ops *faultReplacementOps) Rename(oldPath, newPath string) error {
	if ops.renameErr != nil {
		return ops.renameErr
	}
	return ops.replacementOps.Rename(oldPath, newPath)
}

func (ops *faultReplacementOps) OpenDirectory(path string) (syncFile, error) {
	if ops.directorySyncErr != nil {
		return &faultSyncFile{syncErr: ops.directorySyncErr}, nil
	}
	return ops.replacementOps.OpenDirectory(path)
}

type faultSyncFile struct {
	syncErr error
}

func (file *faultSyncFile) Sync() error  { return file.syncErr }
func (file *faultSyncFile) Close() error { return nil }

type faultReplacementFile struct {
	bytes.Buffer
	name     string
	writeErr error
	closed   bool
}

func (file *faultReplacementFile) Write(body []byte) (int, error) {
	if file.writeErr != nil {
		return 0, file.writeErr
	}
	return file.Buffer.Write(body)
}

func (file *faultReplacementFile) Name() string { return file.name }
func (file *faultReplacementFile) Sync() error  { return nil }
func (file *faultReplacementFile) Close() error {
	file.closed = true
	return nil
}

type memoryReplacementOps struct {
	file        replacementFile
	removeErr   error
	removeCalls int
}

func (ops *memoryReplacementOps) CreateTemp(string, string) (replacementFile, error) {
	return ops.file, nil
}
func (ops *memoryReplacementOps) Chmod(string, fs.FileMode) error { return nil }
func (ops *memoryReplacementOps) Open(string) (syncFile, error) {
	return &faultSyncFile{}, nil
}
func (ops *memoryReplacementOps) OpenDirectory(string) (syncFile, error) {
	return &faultSyncFile{}, nil
}
func (ops *memoryReplacementOps) Rename(string, string) error { return nil }
func (ops *memoryReplacementOps) Remove(string) error {
	ops.removeCalls++
	return ops.removeErr
}
