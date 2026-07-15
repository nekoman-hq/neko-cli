package pluginindex

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicPluginIndexOutputPersisterCreatesAndReplacesCanonicalTarget(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "nested", "registry")
	outputPath := filepath.Join(outputDir, "plugin-index.json")
	unrelatedPath := filepath.Join(root, "unrelated.txt")
	if err := os.WriteFile(unrelatedPath, []byte("untouched"), 0600); err != nil {
		t.Fatalf("write unrelated: %v", err)
	}
	persister := newPluginIndexOutputPersister(pluginIndexPersistenceDisk{})

	if err := persister.Persist(outputPath, []byte("first output\n")); err != nil {
		t.Fatalf("Persist create: %v", err)
	}
	assertPersistedPluginIndex(t, outputPath, "first output\n", 0644)
	assertPluginIndexPathMode(t, outputDir, 0755)
	if err := os.Chmod(outputPath, 0600); err != nil {
		t.Fatalf("chmod output: %v", err)
	}
	if err := persister.Persist(outputPath, []byte("replacement output\n")); err != nil {
		t.Fatalf("Persist replace: %v", err)
	}
	assertPersistedPluginIndex(t, outputPath, "replacement output\n", 0600)
	assertNoPluginIndexTemporaryFiles(t, outputDir, outputPath)
	assertPersistedPluginIndex(t, unrelatedPath, "untouched", 0600)
}

func TestAtomicPluginIndexOutputPersisterPreservesOriginalOnRecoverableFailures(t *testing.T) {
	tests := []struct {
		writeErr   error
		replaceErr error
		name       string
	}{
		{name: "write", writeErr: errors.New("injected write failure")},
		{name: "replace", replaceErr: errors.New("injected replace failure")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			outputPath := filepath.Join(root, "plugin-index.json")
			unrelatedPath := filepath.Join(root, "unrelated.txt")
			if err := os.WriteFile(outputPath, []byte("original output\n"), 0600); err != nil {
				t.Fatalf("write original: %v", err)
			}
			if err := os.WriteFile(unrelatedPath, []byte("untouched"), 0644); err != nil {
				t.Fatalf("write unrelated: %v", err)
			}
			filesystem := faultInjectPluginIndexFilesystem{
				delegate:   pluginIndexPersistenceDisk{},
				writeErr:   tt.writeErr,
				replaceErr: tt.replaceErr,
			}
			persister := newPluginIndexOutputPersister(filesystem)

			err := persister.Persist(outputPath, []byte("new output\n"))
			if err == nil || !errors.Is(err, firstPluginIndexError(tt.writeErr, tt.replaceErr)) {
				t.Fatalf("Persist error = %v", err)
			}
			assertPersistedPluginIndex(t, outputPath, "original output\n", 0600)
			assertPersistedPluginIndex(t, unrelatedPath, "untouched", 0644)
			assertNoPluginIndexTemporaryFiles(t, root, outputPath)
		})
	}
}

func TestAtomicPluginIndexOutputPersisterStopsAtFilesystemFailures(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		filesystem := &recordingPluginIndexPersistenceFilesystem{mkdirErr: errors.New("mkdir failed")}
		err := newPluginIndexOutputPersister(filesystem).Persist("out/plugin-index.json", []byte("output"))
		if err == nil || !errors.Is(err, filesystem.mkdirErr) {
			t.Fatalf("Persist error = %v", err)
		}
		if filesystem.statCalls != 0 || filesystem.createCalls != 0 {
			t.Fatalf("later filesystem calls occurred: %#v", filesystem)
		}
	})

	t.Run("inspect", func(t *testing.T) {
		filesystem := &recordingPluginIndexPersistenceFilesystem{statErr: errors.New("stat failed")}
		err := newPluginIndexOutputPersister(filesystem).Persist("out/plugin-index.json", []byte("output"))
		if err == nil || !errors.Is(err, filesystem.statErr) {
			t.Fatalf("Persist error = %v", err)
		}
		if filesystem.createCalls != 0 {
			t.Fatalf("create called after inspect failure: %#v", filesystem)
		}
	})

	t.Run("create", func(t *testing.T) {
		filesystem := &recordingPluginIndexPersistenceFilesystem{
			statErr:   fs.ErrNotExist,
			createErr: errors.New("create failed"),
		}
		err := newPluginIndexOutputPersister(filesystem).Persist("out/plugin-index.json", []byte("output"))
		if err == nil || !errors.Is(err, filesystem.createErr) {
			t.Fatalf("Persist error = %v", err)
		}
		if filesystem.createCalls != 1 {
			t.Fatalf("create calls = %d, want 1", filesystem.createCalls)
		}
	})
}

type faultInjectPluginIndexFilesystem struct {
	delegate   pluginIndexPersistenceDisk
	writeErr   error
	replaceErr error
}

func (filesystem faultInjectPluginIndexFilesystem) MkdirAll(path string, mode os.FileMode) error {
	return filesystem.delegate.MkdirAll(path, mode)
}

func (filesystem faultInjectPluginIndexFilesystem) Stat(path string) (fs.FileInfo, error) {
	return filesystem.delegate.Stat(path)
}

func (filesystem faultInjectPluginIndexFilesystem) CreateAtomicReplacement(path string) (pluginIndexPreparedOutput, error) {
	prepared, err := filesystem.delegate.CreateAtomicReplacement(path)
	if err != nil {
		return nil, err
	}
	return &faultInjectPluginIndexPreparedOutput{
		delegate:   prepared,
		writeErr:   filesystem.writeErr,
		replaceErr: filesystem.replaceErr,
	}, nil
}

type faultInjectPluginIndexPreparedOutput struct {
	delegate   pluginIndexPreparedOutput
	writeErr   error
	replaceErr error
}

func (prepared *faultInjectPluginIndexPreparedOutput) Write(output []byte, mode os.FileMode) error {
	if prepared.writeErr != nil {
		return prepared.writeErr
	}
	return prepared.delegate.Write(output, mode)
}

func (prepared *faultInjectPluginIndexPreparedOutput) Replace() error {
	if prepared.replaceErr != nil {
		return prepared.replaceErr
	}
	return prepared.delegate.Replace()
}

func (prepared *faultInjectPluginIndexPreparedOutput) Discard() {
	prepared.delegate.Discard()
}

//nolint:govet // Test fake groups configured failures before call counters.
type recordingPluginIndexPersistenceFilesystem struct {
	mkdirErr    error
	statErr     error
	createErr   error
	prepared    pluginIndexPreparedOutput
	statInfo    fs.FileInfo
	statCalls   int
	createCalls int
}

func (filesystem *recordingPluginIndexPersistenceFilesystem) MkdirAll(string, os.FileMode) error {
	return filesystem.mkdirErr
}

func (filesystem *recordingPluginIndexPersistenceFilesystem) Stat(string) (fs.FileInfo, error) {
	filesystem.statCalls++
	return filesystem.statInfo, filesystem.statErr
}

func (filesystem *recordingPluginIndexPersistenceFilesystem) CreateAtomicReplacement(string) (pluginIndexPreparedOutput, error) {
	filesystem.createCalls++
	return filesystem.prepared, filesystem.createErr
}

func assertPersistedPluginIndex(t *testing.T, path, want string, mode fs.FileMode) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("content %s = %q, want %q", path, data, want)
	}
	assertPluginIndexPathMode(t, path, mode)
}

func assertPluginIndexPathMode(t *testing.T, path string, want fs.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode %s = %#o, want %#o", path, got, want)
	}
}

func assertNoPluginIndexTemporaryFiles(t *testing.T, directory, outputPath string) {
	t.Helper()
	pattern := filepath.Join(directory, "."+filepath.Base(outputPath)+".tmp-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob temporary files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %#v", matches)
	}
}

func firstPluginIndexError(first, second error) error {
	if first != nil {
		return first
	}
	return second
}
