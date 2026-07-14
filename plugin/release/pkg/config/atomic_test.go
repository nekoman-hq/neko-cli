package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicWriteFileSuccess(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "release.state.json")

	if err := AtomicWriteFile(target, []byte("new"), 0644); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("expected target content new, got %q", string(data))
	}
	assertNoAtomicTemps(t, root)
}

func TestPrepareAtomicFileDoesNotReplaceUntilRequested(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "release.state.json")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatalf("write target: %v", err)
	}

	prepared, err := PrepareAtomicFile(target, []byte("new"), 0644)
	if err != nil {
		t.Fatalf("PrepareAtomicFile: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read unchanged target: %v", err)
	}
	if string(data) != "old" {
		t.Fatalf("prepare changed target to %q", data)
	}

	prepared.Discard()
	assertNoAtomicTemps(t, root)
	data, err = os.ReadFile(target)
	if err != nil {
		t.Fatalf("read discarded target: %v", err)
	}
	if string(data) != "old" {
		t.Fatalf("discard changed target to %q", data)
	}
}

func TestAtomicWriteFileRenameErrorKeepsExistingTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "state")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}

	err := AtomicWriteFile(target, []byte("new"), 0644)
	if err == nil {
		t.Fatal("expected rename error")
	}

	info, statErr := os.Stat(target)
	if statErr != nil {
		t.Fatalf("stat target: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatal("target directory was replaced")
	}
	assertNoAtomicTemps(t, root)
}

func TestAtomicWriteJSONFormatsCanonicalJSON(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "config.json")

	if err := AtomicWriteJSON(target, map[string]any{"schemaVersion": 2}, 0644); err != nil {
		t.Fatalf("AtomicWriteJSON: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != "{\n  \"schemaVersion\": 2\n}\n" {
		t.Fatalf("unexpected JSON:\n%s", string(data))
	}
}

func assertNoAtomicTemps(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temporary file was not cleaned up: %s", entry.Name())
		}
	}
}
