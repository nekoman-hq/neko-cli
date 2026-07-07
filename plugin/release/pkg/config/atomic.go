package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWriteFile writes data through a temp file in the target directory and
// renames it into place only after the content has been fully written and
// fsynced. Future V2 migration/state writers can use this without exposing
// partially-written JSON at the final path.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	temp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("atomic write %s: create temp file: %w", path, err)
	}
	tempName := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("atomic write %s: write temp file: %w", path, err)
	}
	if err := temp.Chmod(perm); err != nil {
		_ = temp.Close()
		return fmt.Errorf("atomic write %s: chmod temp file: %w", path, err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("atomic write %s: fsync temp file: %w", path, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("atomic write %s: close temp file: %w", path, err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("atomic write %s: rename temp file: %w", path, err)
	}
	cleanup = false

	if dirFile, err := os.Open(dir); err == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
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
