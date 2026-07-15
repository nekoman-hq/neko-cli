//nolint:staticcheck // V1 compatibility test intentionally exercises the legacy config contract.
package history

import (
	"os"
	"os/exec"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestHandleHistoryCharacterizesGitFailureBoundaries(t *testing.T) {
	t.Run("v2 failure is structured", func(t *testing.T) {
		withHistoryDirectory(t)
		writeV2Config(t)

		resp, err := HandleHistory(plugin.Request{Flags: map[string]any{"unit": "api"}})
		if err != nil {
			t.Fatalf("HandleHistory returned Go error: %v", err)
		}
		assertHistoryError(t, resp, "GIT_HISTORY_FAILED")
	})

	t.Run("v1 tag failure remains empty success", func(t *testing.T) {
		withHistoryDirectory(t)
		writeHistoryV1Config(t)

		resp, err := HandleHistory(plugin.Request{})
		if err != nil {
			t.Fatalf("HandleHistory returned Go error: %v", err)
		}
		if resp.Status != "success" || resp.Metadata.Command != "history" || resp.Metadata.Timestamp.IsZero() {
			t.Fatalf("unexpected response: %#v", resp)
		}
		items, ok := resp.Data["items"].([]map[string]any)
		if !ok || len(items) != 0 {
			t.Fatalf("items = %#v, want empty history", resp.Data["items"])
		}
	})
}

func TestHandleHistoryV2OrderingAndReadOnlyContract(t *testing.T) {
	withGitRepo(t)
	writeV2Config(t)
	commitFile(t, "api/main.go", "api 1")
	gitCmd(t, "tag", "api/v0.2.0")
	gitCmd(t, "tag", "api/v0.1.0")

	statusBefore := historyGitOutput(t, "status", "--short")
	headBefore := historyGitOutput(t, "rev-parse", "HEAD")
	refsBefore := historyGitOutput(t, "show-ref", "--tags")

	resp, err := HandleHistory(plugin.Request{Flags: map[string]any{"unit": "api"}})
	if err != nil {
		t.Fatalf("HandleHistory returned Go error: %v", err)
	}
	if resp.Status != "success" || resp.Metadata.Command != "history" || resp.Metadata.Timestamp.IsZero() {
		t.Fatalf("unexpected response: %#v", resp)
	}
	items, ok := resp.Data["items"].([]map[string]any)
	if !ok || len(items) != 2 {
		t.Fatalf("items = %#v, want two rows", resp.Data["items"])
	}
	want := []map[string]any{
		{"unit": "api", "version": "api/v0.1.0", "from": "", "commits": 1},
		{"unit": "api", "version": "api/v0.2.0", "from": "api/v0.1.0", "commits": 0},
	}
	for i := range want {
		for _, key := range []string{"unit", "version", "from", "commits"} {
			if items[i][key] != want[i][key] {
				t.Fatalf("item %d %s = %#v, want %#v; item=%#v", i, key, items[i][key], want[i][key], items[i])
			}
		}
	}

	if got := historyGitOutput(t, "status", "--short"); got != statusBefore {
		t.Fatalf("history mutated worktree/index\nbefore:\n%s\nafter:\n%s", statusBefore, got)
	}
	if got := historyGitOutput(t, "rev-parse", "HEAD"); got != headBefore {
		t.Fatalf("history moved HEAD: before %q after %q", headBefore, got)
	}
	if got := historyGitOutput(t, "show-ref", "--tags"); got != refsBefore {
		t.Fatalf("history mutated tags\nbefore:\n%s\nafter:\n%s", refsBefore, got)
	}
}

func assertHistoryError(t *testing.T, resp *plugin.Response, code string) {
	t.Helper()
	if resp.Status != "error" || resp.Error == nil || resp.Error.Code != code {
		t.Fatalf("response = %#v, want structured %s", resp, code)
	}
	if resp.Metadata.Command != "history" || resp.Metadata.Timestamp.IsZero() {
		t.Fatalf("unexpected metadata: %#v", resp.Metadata)
	}
}

func withHistoryDirectory(t *testing.T) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}

func writeHistoryV1Config(t *testing.T) {
	t.Helper()
	data := `{
  "project-name": "neko-cli",
  "project-owner": "nekoman-hq",
  "project-type": "backend",
  "release-system": "goreleaser",
  "version": "1.2.3"
}`
	if err := os.WriteFile(releaseconfig.V1FileName, []byte(data), 0644); err != nil {
		t.Fatalf("write V1 config: %v", err)
	}
}

func historyGitOutput(t *testing.T, args ...string) string {
	t.Helper()
	output, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}
