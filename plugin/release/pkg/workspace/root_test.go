//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
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
	mustWriteFile(t, filepath.Join(repoRoot, config.V1FileName), "{}")
	mustWriteFile(t, filepath.Join(nestedProject, config.V1FileName), "{}")
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

func TestResolveProjectRootPrefersGitRootV2OverNestedV1(t *testing.T) {
	repoRoot := t.TempDir()
	nestedProject := filepath.Join(repoRoot, "apps", "web")
	startDir := filepath.Join(nestedProject, "src")

	mustMkdirAll(t, filepath.Join(repoRoot, gitMarker))
	mustWriteFile(t, config.V2ConfigPath(repoRoot), `{"schemaVersion":2,"units":[]}`)
	mustWriteFile(t, config.V2StatePath(repoRoot), `{"schemaVersion":2,"units":{}}`)
	mustWriteFile(t, filepath.Join(nestedProject, config.V1FileName), "{}")
	mustMkdirAll(t, startDir)

	root, err := ResolveProjectRoot(startDir)
	if err != nil {
		t.Fatalf("ResolveProjectRoot: %v", err)
	}
	if root != repoRoot {
		t.Fatalf("expected V2 git root %s, got %s", repoRoot, root)
	}
}

func TestResolveProjectRootRejectsRootV1V2Conflict(t *testing.T) {
	repoRoot := t.TempDir()
	startDir := filepath.Join(repoRoot, "app")

	mustMkdirAll(t, filepath.Join(repoRoot, gitMarker))
	mustMkdirAll(t, startDir)
	mustWriteFile(t, filepath.Join(repoRoot, config.V1FileName), "{}")
	mustWriteFile(t, config.V2ConfigPath(repoRoot), `{"schemaVersion":2,"units":[]}`)
	mustWriteFile(t, config.V2StatePath(repoRoot), `{"schemaVersion":2,"units":{}}`)

	_, err := ResolveProjectRoot(startDir)
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestResolveProjectRootRejectsV2WithoutState(t *testing.T) {
	repoRoot := t.TempDir()
	startDir := filepath.Join(repoRoot, "app")

	mustMkdirAll(t, filepath.Join(repoRoot, gitMarker))
	mustMkdirAll(t, startDir)
	mustWriteFile(t, config.V2ConfigPath(repoRoot), `{"schemaVersion":2,"units":[]}`)

	_, err := ResolveProjectRoot(startDir)
	if err == nil {
		t.Fatal("expected missing state error")
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

func TestResolveProjectRootUsesFileParentAsStartDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	startFile := filepath.Join(repoRoot, "packages", "release", "request.json")

	mustMkdirAll(t, filepath.Join(repoRoot, gitMarker))
	mustWriteFile(t, startFile, "{}")

	root, err := ResolveProjectRoot(startFile)
	if err != nil {
		t.Fatalf("ResolveProjectRoot: %v", err)
	}
	if root != repoRoot {
		t.Fatalf("expected file start to resolve to git root %s, got %s", repoRoot, root)
	}
}

func TestResolveProjectRootPreservesSymlinkRootSpelling(t *testing.T) {
	targetRoot := t.TempDir()
	linkParent := t.TempDir()
	linkRoot := filepath.Join(linkParent, "repo-link")
	startDir := filepath.Join(linkRoot, "packages", "release")

	mustMkdirAll(t, filepath.Join(targetRoot, gitMarker))
	if err := os.Symlink(targetRoot, linkRoot); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	mustMkdirAll(t, startDir)

	root, err := ResolveProjectRoot(startDir)
	if err != nil {
		t.Fatalf("ResolveProjectRoot: %v", err)
	}
	if root != linkRoot {
		t.Fatalf("expected symlink root spelling %s, got %s", linkRoot, root)
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
