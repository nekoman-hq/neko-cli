package history

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestHandleHistoryV2UsesOnlySelectedUnitTags(t *testing.T) {
	withGitRepo(t)
	writeV2Config(t)
	commitFile(t, "api/main.go", "api 1")
	gitCmd(t, "tag", "api/v0.1.0")
	commitFile(t, "web/main.js", "web 1")
	gitCmd(t, "tag", "web/v0.1.0")
	commitFile(t, "api/main.go", "api 2")
	gitCmd(t, "tag", "api/v0.2.0")

	resp, err := HandleHistory(plugin.Request{Flags: map[string]any{"unit": "api"}})
	if err != nil {
		t.Fatalf("HandleHistory: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	items, ok := resp.Data["items"].([]map[string]any)
	if !ok {
		t.Fatalf("unexpected items payload: %#v", resp.Data["items"])
	}
	if len(items) != 2 {
		t.Fatalf("expected two api history rows, got %#v", items)
	}
	for _, item := range items {
		if item["unit"] != "api" {
			t.Fatalf("unexpected unit row: %#v", item)
		}
		if item["version"] == "web/v0.1.0" {
			t.Fatalf("web tag leaked into api history: %#v", items)
		}
	}
}

func TestHandleHistoryV2RequiresUnitForMultiUnit(t *testing.T) {
	withGitRepo(t)
	writeV2Config(t)

	resp, err := HandleHistory(plugin.Request{})
	if err != nil {
		t.Fatalf("HandleHistory: %v", err)
	}
	if resp.Status != "error" || resp.Error.Code != "UNIT_RESOLUTION_FAILED" {
		t.Fatalf("expected unit resolution error, got %#v", resp)
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
