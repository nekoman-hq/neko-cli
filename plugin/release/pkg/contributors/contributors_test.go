package contributors

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestHandleContributorsV2UsesSelectedUnitPaths(t *testing.T) {
	withGitRepo(t)
	writeV2Config(t)
	commitFile(t, "api/main.go", "api 1")
	commitFile(t, "web/main.js", "web 1")

	resp, err := HandleContributors(plugin.Request{Flags: map[string]any{"unit": "api"}})
	if err != nil {
		t.Fatalf("HandleContributors: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	items, ok := resp.Data["items"].([]map[string]any)
	if !ok {
		t.Fatalf("unexpected items payload: %#v", resp.Data["items"])
	}
	if len(items) != 1 || items[0]["commits"] != "1" {
		t.Fatalf("expected one api contributor commit, got %#v", items)
	}
}

func writeV2Config(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(".neko", 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	config := `{"schemaVersion":2,"units":[{"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"goreleaser"}},{"id":"web","paths":["web/**"],"workingDirectory":".","tagPrefix":"web/v","executor":{"type":"release-it"}}]}`
	state := `{"schemaVersion":2,"units":{"api":{"version":"0.2.0"},"web":{"version":"0.1.0"}}}`
	if err := os.WriteFile(".neko/release.config.json", []byte(config), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(".neko/release.state.json", []byte(state), 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}
}

func withGitRepo(t *testing.T) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	gitCmd(t, "init")
	gitCmd(t, "config", "user.email", "test@example.com")
	gitCmd(t, "config", "user.name", "Test User")
}

func commitFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	gitCmd(t, "add", path)
	gitCmd(t, "commit", "-m", "change "+path)
}

func gitCmd(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s", args, string(out))
	}
}
