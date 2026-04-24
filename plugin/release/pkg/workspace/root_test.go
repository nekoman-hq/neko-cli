package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestResolveProjectRootPrefersNearestReleaseConfig(t *testing.T) {
	repoRoot := t.TempDir()
	nestedProject := filepath.Join(repoRoot, "apps", "web")
	startDir := filepath.Join(nestedProject, "src", "components")

	mustMkdirAll(t, filepath.Join(repoRoot, gitMarker))
	mustWriteFile(t, filepath.Join(repoRoot, config.FileName), "{}")
	mustWriteFile(t, filepath.Join(nestedProject, config.FileName), "{}")
	mustMkdirAll(t, startDir)

	root, err := ResolveProjectRoot(startDir)
	if err != nil {
		t.Fatalf("ResolveProjectRoot: %v", err)
	}

	if root != nestedProject {
		t.Fatalf("expected nearest config root %s, got %s", nestedProject, root)
	}
}

func TestResolveProjectRootFallsBackToGitRoot(t *testing.T) {
	repoRoot := t.TempDir()
	startDir := filepath.Join(repoRoot, "internal", "handlers")

	mustMkdirAll(t, filepath.Join(repoRoot, gitMarker))
	mustMkdirAll(t, startDir)

	root, err := ResolveProjectRoot(startDir)
	if err != nil {
		t.Fatalf("ResolveProjectRoot: %v", err)
	}

	if root != repoRoot {
		t.Fatalf("expected git root %s, got %s", repoRoot, root)
	}
}

func TestResolveProjectRootReturnsWorkingDirWithoutMarkers(t *testing.T) {
	startDir := filepath.Join(t.TempDir(), "feature", "area")
	mustMkdirAll(t, startDir)

	root, err := ResolveProjectRoot(startDir)
	if err != nil {
		t.Fatalf("ResolveProjectRoot: %v", err)
	}

	if root != startDir {
		t.Fatalf("expected working dir %s, got %s", startDir, root)
	}
}

func TestChangeToProjectRootSwitchesProcessDirectory(t *testing.T) {
	originalDir, getwdErr := os.Getwd()
	if getwdErr != nil {
		t.Fatalf("getwd: %v", getwdErr)
	}

	repoRoot := t.TempDir()
	startDir := filepath.Join(repoRoot, "packages", "release")

	mustMkdirAll(t, filepath.Join(repoRoot, gitMarker))
	mustMkdirAll(t, startDir)

	if chdirErr := os.Chdir(startDir); chdirErr != nil {
		t.Fatalf("chdir start dir: %v", chdirErr)
	}

	t.Cleanup(func() {
		if restoreErr := os.Chdir(originalDir); restoreErr != nil {
			t.Fatalf("restore cwd: %v", restoreErr)
		}
	})

	if err := ChangeToProjectRoot(startDir); err != nil {
		t.Fatalf("ChangeToProjectRoot: %v", err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd after change: %v", err)
	}

	expectedDir, evalExpectedErr := filepath.EvalSymlinks(repoRoot)
	if evalExpectedErr != nil {
		t.Fatalf("eval symlinks for expected dir: %v", evalExpectedErr)
	}

	actualDir, evalActualErr := filepath.EvalSymlinks(currentDir)
	if evalActualErr != nil {
		t.Fatalf("eval symlinks for current dir: %v", evalActualErr)
	}

	if actualDir != expectedDir {
		t.Fatalf("expected cwd %s, got %s", expectedDir, actualDir)
	}
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()

	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0755); err != nil {
		t.Fatalf("mkdir parent %s: %v", parent, err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
