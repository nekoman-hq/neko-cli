package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AtomicFileReplacement owns one target-local temporary file. After Write fully
// writes and fsyncs it, Replace can rename it over the target. Discard must be
// called when it is no longer needed; it is safe to call after Replace.
type AtomicFileReplacement struct {
	file     *os.File
	path     string
	tempName string
}

// CreateAtomicFileReplacement creates an empty temporary file in the target
// directory. Write must complete before Replace can be called.
func CreateAtomicFileReplacement(path string) (*AtomicFileReplacement, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	temp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return nil, fmt.Errorf("atomic write %s: create temp file: %w", path, err)
	}
	return &AtomicFileReplacement{path: path, tempName: temp.Name(), file: temp}, nil
}

// Write fills, chmods, fsyncs, and closes the temporary file without changing
// the target.
func (prepared *AtomicFileReplacement) Write(data []byte, perm os.FileMode) error {
	if prepared == nil || prepared.file == nil || prepared.tempName == "" {
		return fmt.Errorf("atomic write: temporary file is unavailable")
	}
	if _, err := prepared.file.Write(data); err != nil {
		prepared.Discard()
		return fmt.Errorf("atomic write %s: write temp file: %w", prepared.path, err)
	}
	if err := prepared.file.Chmod(perm); err != nil {
		prepared.Discard()
		return fmt.Errorf("atomic write %s: chmod temp file: %w", prepared.path, err)
	}
	if err := prepared.file.Sync(); err != nil {
		prepared.Discard()
		return fmt.Errorf("atomic write %s: fsync temp file: %w", prepared.path, err)
	}
	if err := prepared.file.Close(); err != nil {
		prepared.file = nil
		_ = os.Remove(prepared.tempName)
		prepared.tempName = ""
		return fmt.Errorf("atomic write %s: close temp file: %w", prepared.path, err)
	}
	prepared.file = nil
	return nil
}

// PrepareAtomicFile creates, writes, and fsyncs a temporary file in the target
// directory without changing the target itself.
func PrepareAtomicFile(path string, data []byte, perm os.FileMode) (*AtomicFileReplacement, error) {
	prepared, err := CreateAtomicFileReplacement(path)
	if err != nil {
		return nil, err
	}
	if err := prepared.Write(data, perm); err != nil {
		prepared.Discard()
		return nil, err
	}
	return prepared, nil
}

// Replace renames the prepared file over its target and fsyncs the target
// directory. Each prepared file may be replaced at most once.
func (prepared *AtomicFileReplacement) Replace() error {
	if prepared == nil || prepared.tempName == "" || prepared.file != nil {
		return fmt.Errorf("atomic write: prepared file is unavailable")
	}
	if err := os.Rename(prepared.tempName, prepared.path); err != nil {
		return fmt.Errorf("atomic write %s: rename temp file: %w", prepared.path, err)
	}
	prepared.tempName = ""

	dir := filepath.Dir(prepared.path)
	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}

	return nil
}

// Discard removes an unconsumed prepared temporary file.
func (prepared *AtomicFileReplacement) Discard() {
	if prepared == nil || prepared.tempName == "" {
		return
	}
	if prepared.file != nil {
		_ = prepared.file.Close()
		prepared.file = nil
	}
	_ = os.Remove(prepared.tempName)
	prepared.tempName = ""
}

// AtomicWriteFile writes data through a temp file in the target directory and
// renames it into place only after the content has been fully written and
// fsynced. Future V2 migration/state writers can use this without exposing
// partially-written JSON at the final path.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	prepared, err := PrepareAtomicFile(path, data, perm)
	if err != nil {
		return err
	}
	defer prepared.Discard()
	if err := prepared.Replace(); err != nil {
		return err
	}
	return nil
}

// AtomicWriteJSON writes canonical, formatted JSON atomically.
func AtomicWriteJSON(path string, value any, perm os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("atomic write json %s: marshal: %w", path, err)
	}
	data = append(data, '\n')
	return AtomicWriteFile(path, data, perm)
}
