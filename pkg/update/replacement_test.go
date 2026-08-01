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
	inspected := installation{canonicalTarget: target, targetParent: root, targetMode: 0o700}
	reservation, err := newOSReplacementCapability().Reserve(inspected)
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	reservedPath := reservation.Path()
	if filepath.Dir(reservedPath) != root || reservedPath == target {
		t.Fatalf("reserved path = %q", reservedPath)
	}
	if commitErr := reservation.Commit([]byte("new executable"), inspected.targetMode); commitErr != nil {
		t.Fatalf("Commit: %v", commitErr)
	}
	if cleanupErr := reservation.Cleanup(); cleanupErr != nil {
		t.Fatalf("Cleanup: %v", cleanupErr)
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
	if commitErr := reservation.Commit([]byte("new executable"), 0o755); commitErr == nil || !strings.Contains(commitErr.Error(), "frozen rename failure") {
		t.Fatalf("Commit error = %v", commitErr)
	}
	if cleanupErr := reservation.Cleanup(); cleanupErr != nil {
		t.Fatalf("Cleanup: %v", cleanupErr)
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
	if commitErr := reservation.Commit([]byte("new"), 0o755); commitErr == nil || !strings.Contains(commitErr.Error(), "frozen write failure") {
		t.Fatalf("Commit error = %v", commitErr)
	}
	if cleanupErr := reservation.Cleanup(); cleanupErr != nil {
		t.Fatalf("Cleanup: %v", cleanupErr)
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
	if cleanupErr := reservation.Cleanup(); cleanupErr == nil || !strings.Contains(cleanupErr.Error(), "frozen cleanup failure") {
		t.Fatalf("Cleanup error = %v", cleanupErr)
	}
}

func TestReplacementFailureStagesRemainPrecommit(t *testing.T) {
	tests := []struct { //nolint:govet // Field order keeps fault stages readable.
		name      string
		kind      errorKind
		file      *faultReplacementFile
		configure func(*memoryReplacementOps)
	}{
		{name: "write", kind: errorSiblingWrite, file: &faultReplacementFile{name: "/fixture/.neko-update-write", writeErr: errors.New("write")}},
		{name: "initial fsync", kind: errorFileSync, file: &faultReplacementFile{name: "/fixture/.neko-update-sync", syncErr: errors.New("sync")}},
		{name: "close", kind: errorSiblingWrite, file: &faultReplacementFile{name: "/fixture/.neko-update-close", closeErr: errors.New("close")}},
		{name: "chmod", kind: errorMode, file: &faultReplacementFile{name: "/fixture/.neko-update-mode"}, configure: func(ops *memoryReplacementOps) { ops.chmodErr = errors.New("chmod") }},
		{name: "reopen", kind: errorFileSync, file: &faultReplacementFile{name: "/fixture/.neko-update-open"}, configure: func(ops *memoryReplacementOps) { ops.openErr = errors.New("open") }},
		{name: "second fsync", kind: errorFileSync, file: &faultReplacementFile{name: "/fixture/.neko-update-second-sync"}, configure: func(ops *memoryReplacementOps) { ops.openFile = &faultSyncFile{syncErr: errors.New("second sync")} }},
		{name: "rename", kind: errorRename, file: &faultReplacementFile{name: "/fixture/.neko-update-rename"}, configure: func(ops *memoryReplacementOps) { ops.renameErr = errors.New("rename") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ops := &memoryReplacementOps{file: test.file}
			if test.configure != nil {
				test.configure(ops)
			}
			reservation, err := (&osReplacementCapability{ops: ops}).Reserve(installation{canonicalTarget: "/fixture/neko", targetParent: "/fixture", targetMode: 0o700})
			if err != nil {
				t.Fatalf("Reserve: %v", err)
			}
			err = reservation.Commit([]byte("new"), 0o700)
			var updateErr *updateError
			if !errors.As(err, &updateErr) || updateErr.kind != test.kind || updateErr.changed {
				t.Fatalf("error = %#v, want kind %q precommit", err, test.kind)
			}
			test.file.closeErr = nil
			if cleanupErr := reservation.Cleanup(); cleanupErr != nil {
				t.Fatalf("Cleanup: %v", cleanupErr)
			}
			if ops.removeCalls != 1 || ops.renameCalls > 1 {
				t.Fatalf("remove=%d rename=%d", ops.removeCalls, ops.renameCalls)
			}
		})
	}
}

func TestReplacementReservationFailureDoesNotCreateFallback(t *testing.T) {
	ops := &memoryReplacementOps{createErr: errors.New("exclusive create denied")}
	_, err := (&osReplacementCapability{ops: ops}).Reserve(installation{canonicalTarget: "/fixture/neko", targetParent: "/fixture", targetMode: 0o755})
	var updateErr *updateError
	if !errors.As(err, &updateErr) || updateErr.kind != errorParentNotWritable || ops.removeCalls != 0 {
		t.Fatalf("error=%#v remove=%d", err, ops.removeCalls)
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
	syncErr  error
	closeErr error
}

func (file *faultSyncFile) Sync() error  { return file.syncErr }
func (file *faultSyncFile) Close() error { return file.closeErr }

type faultReplacementFile struct {
	name     string
	writeErr error
	syncErr  error
	closeErr error
	bytes.Buffer
	closed bool
}

func (file *faultReplacementFile) Write(body []byte) (int, error) {
	if file.writeErr != nil {
		return 0, file.writeErr
	}
	return file.Buffer.Write(body)
}

func (file *faultReplacementFile) Name() string { return file.name }
func (file *faultReplacementFile) Sync() error  { return file.syncErr }
func (file *faultReplacementFile) Close() error {
	file.closed = true
	return file.closeErr
}

type memoryReplacementOps struct {
	file        replacementFile
	createErr   error
	chmodErr    error
	openErr     error
	openFile    syncFile
	renameErr   error
	removeErr   error
	removeCalls int
	renameCalls int
}

func (ops *memoryReplacementOps) CreateTemp(string, string) (replacementFile, error) {
	return ops.file, ops.createErr
}
func (ops *memoryReplacementOps) Chmod(string, fs.FileMode) error { return ops.chmodErr }
func (ops *memoryReplacementOps) Open(string) (syncFile, error) {
	if ops.openErr != nil {
		return nil, ops.openErr
	}
	if ops.openFile != nil {
		return ops.openFile, nil
	}
	return &faultSyncFile{}, nil
}
func (ops *memoryReplacementOps) OpenDirectory(string) (syncFile, error) {
	return &faultSyncFile{}, nil
}
func (ops *memoryReplacementOps) Rename(string, string) error {
	ops.renameCalls++
	return ops.renameErr
}
func (ops *memoryReplacementOps) Remove(string) error {
	ops.removeCalls++
	return ops.removeErr
}
