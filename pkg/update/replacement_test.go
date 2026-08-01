package update

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
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
	beforeInfo, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	beforeHash := sha256.Sum256(oldContent)
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
	afterHash := sha256.Sum256(content)
	if os.SameFile(beforeInfo, info) {
		t.Fatal("atomic replacement retained the original target inode")
	}
	t.Logf("atomic-commit before_sha256=%x after_sha256=%x before_mode=%#o after_mode=%#o inode_changed=true", beforeHash, afterHash, beforeInfo.Mode().Perm(), info.Mode().Perm())
	if _, err := os.Stat(target + ".backup"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("fixed backup exists or stat failed: %v", err)
	}
	if _, err := os.Stat(reservedPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("renamed reservation still exists: %v", err)
	}
}

func TestReplacementPrecommitStagesPreserveHashModeAndInode(t *testing.T) {
	for _, stage := range []string{"write", "initial-fsync", "close", "chmod", "reopen", "second-fsync", "rename"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "neko")
			oldContent := []byte("authoritative old executable")
			if err := os.WriteFile(target, oldContent, 0o700); err != nil {
				t.Fatal(err)
			}
			beforeInfo, beforeHash := fileEvidence(t, target)
			ops := &filesystemFaultOps{stage: stage}
			reservation, err := (&osReplacementCapability{ops: ops}).Reserve(installation{
				canonicalTarget: target,
				targetParent:    root,
				targetMode:      0o700,
			})
			if err != nil {
				t.Fatalf("Reserve: %v", err)
			}
			commitErr := reservation.Commit([]byte("candidate replacement executable"), 0o700)
			if commitErr == nil || !strings.Contains(commitErr.Error(), stage) {
				t.Fatalf("Commit error=%v, want stage %q", commitErr, stage)
			}
			if cleanupErr := reservation.Cleanup(); cleanupErr != nil {
				t.Fatalf("Cleanup: %v", cleanupErr)
			}
			afterInfo, afterHash := fileEvidence(t, target)
			if beforeHash != afterHash || beforeInfo.Mode().Perm() != afterInfo.Mode().Perm() || !os.SameFile(beforeInfo, afterInfo) {
				t.Fatalf("precommit mutation: before_hash=%x after_hash=%x before_mode=%#o after_mode=%#o same_inode=%t", beforeHash, afterHash, beforeInfo.Mode().Perm(), afterInfo.Mode().Perm(), os.SameFile(beforeInfo, afterInfo))
			}
			temporary, globErr := filepath.Glob(filepath.Join(root, ".neko-update-*"))
			if globErr != nil || len(temporary) != 0 {
				t.Fatalf("temporary=%v err=%v", temporary, globErr)
			}
			t.Logf("stage=%s before_sha256=%x after_sha256=%x before_mode=%#o after_mode=%#o same_inode=true temporary=0", stage, beforeHash, afterHash, beforeInfo.Mode().Perm(), afterInfo.Mode().Perm())
		})
	}
}

func fileEvidence(t *testing.T, path string) (os.FileInfo, [sha256.Size]byte) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info, sha256.Sum256(content)
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

type filesystemFaultOps struct {
	osReplacementOps
	stage string
}

func (ops *filesystemFaultOps) CreateTemp(dir, pattern string) (replacementFile, error) {
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	return &filesystemFaultFile{File: file, stage: ops.stage}, nil
}

func (ops *filesystemFaultOps) Chmod(path string, mode fs.FileMode) error {
	if ops.stage == "chmod" {
		return fmt.Errorf("frozen %s failure", ops.stage)
	}
	return ops.osReplacementOps.Chmod(path, mode)
}

func (ops *filesystemFaultOps) Open(path string) (syncFile, error) {
	if ops.stage == "reopen" {
		return nil, fmt.Errorf("frozen %s failure", ops.stage)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &filesystemFaultSyncFile{File: file, stage: ops.stage}, nil
}

func (ops *filesystemFaultOps) Rename(oldPath, newPath string) error {
	if ops.stage == "rename" {
		return fmt.Errorf("frozen %s failure", ops.stage)
	}
	return ops.osReplacementOps.Rename(oldPath, newPath)
}

type filesystemFaultFile struct {
	*os.File
	stage       string
	closeFailed bool
}

func (file *filesystemFaultFile) Write(body []byte) (int, error) {
	if file.stage == "write" {
		return 0, fmt.Errorf("frozen %s failure", file.stage)
	}
	return file.File.Write(body)
}

func (file *filesystemFaultFile) Sync() error {
	if file.stage == "initial-fsync" {
		return fmt.Errorf("frozen %s failure", file.stage)
	}
	return file.File.Sync()
}

func (file *filesystemFaultFile) Close() error {
	if file.stage == "close" && !file.closeFailed {
		file.closeFailed = true
		if err := file.File.Close(); err != nil {
			return err
		}
		return fmt.Errorf("frozen %s failure", file.stage)
	}
	if file.closeFailed {
		return nil
	}
	return file.File.Close()
}

type filesystemFaultSyncFile struct {
	*os.File
	stage string
}

func (file *filesystemFaultSyncFile) Sync() error {
	if file.stage == "second-fsync" {
		return fmt.Errorf("frozen %s failure", file.stage)
	}
	return file.File.Sync()
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
