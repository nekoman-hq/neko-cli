package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallationInspectorUsesParentCapabilityAndPreservesSymlink(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "target")
	linkDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(targetDir, "neko")
	if err := os.WriteFile(target, []byte("fixture"), 0o500); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(linkDir, "neko")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	inspector := newOSInstallationInspector()
	inspector.executable = func() (string, error) { return link, nil }
	inspector.managerPrefixes = nil
	inspected, err := inspector.Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	canonicalTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.symlinkPath != link || inspected.canonicalTarget != canonicalTarget {
		t.Fatalf("paths = symlink %q target %q", inspected.symlinkPath, inspected.canonicalTarget)
	}
	if !inspected.parentCreateAllowed || !inspected.parentReplaceAllowed {
		t.Fatal("read-only target in writable parent must be replaceable")
	}
	if inspected.targetMode != 0o500 {
		t.Fatalf("target mode = %#o", inspected.targetMode)
	}
}

func TestInstallationInspectorClassifiesNonWritableParent(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "neko")
	if err := os.WriteFile(target, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	inspector := newOSInstallationInspector()
	inspector.executable = func() (string, error) { return target, nil }
	inspector.managerPrefixes = nil
	inspected, err := inspector.Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if inspected.parentCreateAllowed || inspected.parentReplaceAllowed {
		t.Fatal("non-writable parent reported mutation capability")
	}
}

func TestInstallationInspectorRequiresPositiveHomebrewEvidence(t *testing.T) {
	root := t.TempDir()
	prefix := filepath.Join(root, "homebrew")
	target := filepath.Join(prefix, "Cellar", "neko-cli", "1.2.3", "bin", "neko")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(prefix, "bin")
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(linkDir, "neko")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	inspector := newOSInstallationInspector()
	inspector.executable = func() (string, error) { return link, nil }
	inspector.managerPrefixes = []string{prefix}
	inspected, err := inspector.Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if inspected.classification != installationManagerOwned || inspected.manager != "Homebrew" {
		t.Fatalf("classification = %q manager = %q", inspected.classification, inspected.manager)
	}
	if inspected.symlinkPath != link {
		t.Fatalf("manager symlink path = %q", inspected.symlinkPath)
	}

	inspector.managerPrefixes = []string{filepath.Join(root, "different-prefix")}
	inspected, err = inspector.Inspect()
	if err != nil {
		t.Fatalf("Inspect ambiguous path: %v", err)
	}
	if inspected.classification == installationManagerOwned {
		t.Fatal("generic Cellar substring must not imply package-manager ownership")
	}
}

func TestInstallationInspectorRejectsMissingLoopedAndUnsupportedTargets(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct { //nolint:govet // Field order keeps the target-construction table readable.
		name string
		path func() string
	}{
		{name: "missing", path: func() string { return filepath.Join(root, "missing") }},
		{name: "symlink loop", path: func() string {
			first := filepath.Join(root, "first")
			second := filepath.Join(root, "second")
			if err := os.Symlink(second, first); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(first, second); err != nil {
				t.Fatal(err)
			}
			return first
		}},
		{name: "directory", path: func() string {
			directory := filepath.Join(root, "directory")
			if err := os.Mkdir(directory, 0o755); err != nil {
				t.Fatal(err)
			}
			return directory
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := test.path()
			inspector := newOSInstallationInspector()
			inspector.executable = func() (string, error) { return path, nil }
			if _, err := inspector.Inspect(); err == nil {
				t.Fatal("expected unsupported target error")
			}
		})
	}
}

func TestInstallationInspectorReportsUnreadableTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "neko")
	if err := os.WriteFile(target, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	inspector := newOSInstallationInspector()
	inspector.executable = func() (string, error) { return target, nil }
	inspector.open = func(string) (*os.File, error) { return nil, os.ErrPermission }
	_, err := inspector.Inspect()
	if err == nil || !strings.Contains(err.Error(), "not readable") {
		t.Fatalf("error = %v", err)
	}
}

func TestInstallationInspectorSupportsInjectedPrivilegedOwnership(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "neko")
	if err := os.WriteFile(target, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}

	inspector := newOSInstallationInspector()
	inspector.executable = func() (string, error) { return target, nil }
	inspector.identity = identity{uid: 501, groups: []int{20}, known: true}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	inspector.owner = func(path string, _ os.FileInfo) (int, int, bool) {
		if filepath.Base(path) == "neko" || path == canonicalRoot {
			return 0, 0, true
		}
		return 501, 20, true
	}
	inspector.managerPrefixes = nil
	inspected, err := inspector.Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if inspected.classification != installationUnmanagedPrivileged {
		t.Fatalf("classification = %q", inspected.classification)
	}
	if inspected.parentCreateAllowed {
		t.Fatal("ordinary identity must not mutate root-owned 0755 parent")
	}
}
