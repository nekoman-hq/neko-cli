package update

import (
	"os"
	"path/filepath"
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
	installation, err := inspector.Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	canonicalTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if installation.symlinkPath != link || installation.canonicalTarget != canonicalTarget {
		t.Fatalf("paths = symlink %q target %q", installation.symlinkPath, installation.canonicalTarget)
	}
	if !installation.parentCreateAllowed || !installation.parentReplaceAllowed {
		t.Fatal("read-only target in writable parent must be replaceable")
	}
	if installation.targetMode != 0o500 {
		t.Fatalf("target mode = %#o", installation.targetMode)
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
	installation, err := inspector.Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if installation.parentCreateAllowed || installation.parentReplaceAllowed {
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

	inspector := newOSInstallationInspector()
	inspector.executable = func() (string, error) { return target, nil }
	inspector.managerPrefixes = []string{prefix}
	installation, err := inspector.Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if installation.classification != installationManagerOwned || installation.manager != "Homebrew" {
		t.Fatalf("classification = %q manager = %q", installation.classification, installation.manager)
	}

	inspector.managerPrefixes = []string{filepath.Join(root, "different-prefix")}
	installation, err = inspector.Inspect()
	if err != nil {
		t.Fatalf("Inspect ambiguous path: %v", err)
	}
	if installation.classification == installationManagerOwned {
		t.Fatal("generic Cellar substring must not imply package-manager ownership")
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
	installation, err := inspector.Inspect()
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if installation.classification != installationUnmanagedPrivileged {
		t.Fatalf("classification = %q", installation.classification)
	}
	if installation.parentCreateAllowed {
		t.Fatal("ordinary identity must not mutate root-owned 0755 parent")
	}
}
