package pluginindex

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const (
	pluginIndexDirectoryMode fs.FileMode = 0755
	pluginIndexFileMode      fs.FileMode = 0644
)

type pluginIndexPreparedOutput interface {
	Write([]byte, os.FileMode) error
	Replace() error
	Discard()
}

type pluginIndexPersistenceFilesystem interface {
	MkdirAll(string, os.FileMode) error
	Stat(string) (fs.FileInfo, error)
	CreateAtomicReplacement(string) (pluginIndexPreparedOutput, error)
}

type atomicPluginIndexOutputPersister struct {
	filesystem pluginIndexPersistenceFilesystem
}

func newPluginIndexOutputPersister(filesystem pluginIndexPersistenceFilesystem) atomicPluginIndexOutputPersister {
	return atomicPluginIndexOutputPersister{filesystem: filesystem}
}

func (persister atomicPluginIndexOutputPersister) Persist(outputPath string, output []byte) error {
	if err := persister.filesystem.MkdirAll(filepath.Dir(outputPath), pluginIndexDirectoryMode); err != nil {
		return fmt.Errorf("create plugin index output directory: %w", err)
	}
	mode, err := persister.outputMode(outputPath)
	if err != nil {
		return err
	}
	prepared, err := persister.filesystem.CreateAtomicReplacement(outputPath)
	if err != nil {
		return fmt.Errorf("write plugin index %s: %w", outputPath, err)
	}
	defer prepared.Discard()
	if err := prepared.Write(output, mode); err != nil {
		return fmt.Errorf("write plugin index %s: %w", outputPath, err)
	}
	if err := prepared.Replace(); err != nil {
		return fmt.Errorf("write plugin index %s: %w", outputPath, err)
	}
	return nil
}

func (persister atomicPluginIndexOutputPersister) outputMode(outputPath string) (os.FileMode, error) {
	info, err := persister.filesystem.Stat(outputPath)
	if err == nil {
		return info.Mode(), nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return pluginIndexFileMode, nil
	}
	return 0, fmt.Errorf("inspect plugin index output %s: %w", outputPath, err)
}

type pluginIndexPersistenceDisk struct{}

func (pluginIndexPersistenceDisk) MkdirAll(path string, mode os.FileMode) error {
	return os.MkdirAll(path, mode)
}

func (pluginIndexPersistenceDisk) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (pluginIndexPersistenceDisk) CreateAtomicReplacement(path string) (pluginIndexPreparedOutput, error) {
	return releaseconfig.CreateAtomicFileReplacement(path)
}
