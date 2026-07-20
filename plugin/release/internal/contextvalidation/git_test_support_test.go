package contextvalidation

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func newContextCharacterizationRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitCmd(t, root, "init")
	gitCmd(t, root, "config", "user.email", "release-context@example.invalid")
	gitCmd(t, root, "config", "user.name", "Release Context")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("release context\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitCmd(t, root, "add", "README.md")
	gitCmd(t, root, "commit", "-m", "initial")
	return root
}

func gitCmd(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, string(output), err)
	}
}

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, string(output), err)
	}
	return string(output)
}
