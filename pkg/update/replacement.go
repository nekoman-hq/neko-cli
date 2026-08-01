package update

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

type replacementFile interface {
	io.Writer
	Name() string
	Sync() error
	Close() error
}

type syncFile interface {
	Sync() error
	Close() error
}

type replacementOps interface {
	CreateTemp(dir, pattern string) (replacementFile, error)
	Chmod(path string, mode fs.FileMode) error
	Open(path string) (syncFile, error)
	OpenDirectory(path string) (syncFile, error)
	Rename(oldPath, newPath string) error
	Remove(path string) error
}

type osReplacementOps struct{}

func (osReplacementOps) CreateTemp(dir, pattern string) (replacementFile, error) {
	return os.CreateTemp(dir, pattern)
}

func (osReplacementOps) Chmod(path string, mode fs.FileMode) error {
	return os.Chmod(path, mode)
}

func (osReplacementOps) Open(path string) (syncFile, error) {
	return os.Open(path)
}

func (osReplacementOps) OpenDirectory(path string) (syncFile, error) {
	return os.Open(path)
}

func (osReplacementOps) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (osReplacementOps) Remove(path string) error {
	return os.Remove(path)
}

type replacementCapability interface {
	Reserve(installation) (*replacementReservation, error)
}

type osReplacementCapability struct {
	ops replacementOps
}

func newOSReplacementCapability() *osReplacementCapability {
	return &osReplacementCapability{ops: osReplacementOps{}}
}

type replacementReservation struct {
	ops       replacementOps
	file      replacementFile
	path      string
	target    string
	parent    string
	closed    bool
	committed bool
}

func (capability *osReplacementCapability) Reserve(target installation) (*replacementReservation, error) {
	if capability == nil || capability.ops == nil {
		return nil, newUpdateError(errorReservation, "replacement capability is unavailable", nil)
	}
	file, err := capability.ops.CreateTemp(target.targetParent, ".neko-update-*")
	if err != nil {
		return nil, newUpdateError(
			errorParentNotWritable,
			fmt.Sprintf(
				"cannot reserve an update file beside %s; parent directory %s must allow create, rename, and remove operations; no archive was downloaded and the installed executable is unchanged",
				target.canonicalTarget,
				target.targetParent,
			),
			err,
		)
	}
	reservedPath := filepath.Clean(file.Name())
	if filepath.Dir(reservedPath) != filepath.Clean(target.targetParent) {
		_ = file.Close()
		_ = capability.ops.Remove(reservedPath)
		return nil, newUpdateError(errorReservation, "reserved update file is not beside the target executable", nil)
	}
	return &replacementReservation{
		ops:    capability.ops,
		file:   file,
		path:   reservedPath,
		target: target.canonicalTarget,
		parent: target.targetParent,
	}, nil
}

func (reservation *replacementReservation) Path() string {
	return reservation.path
}

func (reservation *replacementReservation) Commit(binary []byte, targetMode fs.FileMode) error {
	if reservation == nil || reservation.file == nil {
		return newUpdateError(errorReservation, "replacement reservation is unavailable", nil)
	}
	mode := targetMode.Perm()
	if mode&0o111 == 0 {
		return newUpdateError(errorMode, fmt.Sprintf("refusing replacement mode %#o without executable bits", mode), nil)
	}

	written, writeErr := io.Copy(reservation.file, bytes.NewReader(binary))
	if writeErr != nil {
		return newUpdateError(errorSiblingWrite, fmt.Sprintf("cannot write reserved update file beside %s", reservation.target), writeErr)
	}
	if written != int64(len(binary)) {
		return newUpdateError(errorSiblingWrite, fmt.Sprintf("reserved update file for %s was written incompletely", reservation.target), io.ErrShortWrite)
	}
	if syncErr := reservation.file.Sync(); syncErr != nil {
		return newUpdateError(errorFileSync, fmt.Sprintf("cannot flush reserved update file for %s", reservation.target), syncErr)
	}
	if closeErr := reservation.file.Close(); closeErr != nil {
		return newUpdateError(errorSiblingWrite, fmt.Sprintf("cannot close reserved update file for %s", reservation.target), closeErr)
	}
	reservation.closed = true

	if chmodErr := reservation.ops.Chmod(reservation.path, mode); chmodErr != nil {
		return newUpdateError(errorMode, fmt.Sprintf("cannot apply mode %#o to reserved update file for %s", mode, reservation.target), chmodErr)
	}
	stagedFile, err := reservation.ops.Open(reservation.path)
	if err != nil {
		return newUpdateError(errorFileSync, fmt.Sprintf("cannot reopen reserved update file for %s", reservation.target), err)
	}
	if err := stagedFile.Sync(); err != nil {
		_ = stagedFile.Close()
		return newUpdateError(errorFileSync, fmt.Sprintf("cannot fsync reserved update file for %s", reservation.target), err)
	}
	if err := stagedFile.Close(); err != nil {
		return newUpdateError(errorFileSync, fmt.Sprintf("cannot close fsynced update file for %s", reservation.target), err)
	}

	if err := reservation.ops.Rename(reservation.path, reservation.target); err != nil {
		return newUpdateError(
			errorRename,
			fmt.Sprintf("cannot atomically replace %s; the installed executable is unchanged", reservation.target),
			err,
		)
	}
	reservation.committed = true

	parentDirectory, err := reservation.ops.OpenDirectory(reservation.parent)
	if err != nil {
		committedError := newUpdateError(errorCommittedSync, fmt.Sprintf("updated %s, but cannot open its parent directory to confirm durability; replacement committed", reservation.target), err)
		committedError.changed = true
		return committedError
	}
	if err := parentDirectory.Sync(); err != nil && !directorySyncUnsupported(err) {
		_ = parentDirectory.Close()
		committedError := newUpdateError(errorCommittedSync, fmt.Sprintf("updated %s, but cannot fsync its parent directory; replacement committed", reservation.target), err)
		committedError.changed = true
		return committedError
	}
	if err := parentDirectory.Close(); err != nil {
		committedError := newUpdateError(errorCommittedSync, fmt.Sprintf("updated %s, but cannot close its parent directory after fsync; replacement committed", reservation.target), err)
		committedError.changed = true
		return committedError
	}
	return nil
}

func (reservation *replacementReservation) Cleanup() error {
	if reservation == nil {
		return nil
	}
	var cleanupErrors []error
	if reservation.file != nil && !reservation.closed {
		if err := reservation.file.Close(); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("close reserved update file: %w", err))
		}
		reservation.closed = true
	}
	if !reservation.committed && reservation.path != "" {
		if err := reservation.ops.Remove(reservation.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove reserved update file %s: %w", reservation.path, err))
		}
	}
	return errors.Join(cleanupErrors...)
}

func directorySyncUnsupported(err error) bool {
	return errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP)
}
