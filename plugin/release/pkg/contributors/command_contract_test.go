//nolint:staticcheck // V1 compatibility test intentionally exercises the legacy config contract.
package contributors

import (
	"os"
	"os/exec"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestHandleContributorsCharacterizesGitFailureBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		arrange func(t *testing.T)
		request plugin.Request
	}{
		{
			name: "v2",
			arrange: func(t *testing.T) {
				writeV2Config(t)
			},
			request: plugin.Request{Flags: map[string]any{"unit": "api"}},
		},
		{
			name: "v1",
			arrange: func(t *testing.T) {
				writeContributorsV1Config(t)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withContributorsDirectory(t)
			tt.arrange(t)

			resp, err := HandleContributors(tt.request)
			if err != nil {
				t.Fatalf("HandleContributors returned Go error: %v", err)
			}
			if resp.Status != "error" || resp.Error == nil || resp.Error.Code != "GIT_CONTRIBUTORS_FAILED" {
				t.Fatalf("response = %#v, want structured Git failure", resp)
			}
			if resp.Metadata.Command != "contributors" || resp.Metadata.Timestamp.IsZero() {
				t.Fatalf("unexpected metadata: %#v", resp.Metadata)
			}
		})
	}
}

func TestHandleContributorsV2EmptyResultIsReadOnly(t *testing.T) {
	withGitRepo(t)
	writeV2Config(t)
	commitFile(t, "web/main.js", "web 1")

	statusBefore := contributorsGitOutput(t, "status", "--short")
	headBefore := contributorsGitOutput(t, "rev-parse", "HEAD")

	resp, err := HandleContributors(plugin.Request{Flags: map[string]any{"unit": "api"}})
	if err != nil {
		t.Fatalf("HandleContributors returned Go error: %v", err)
	}
	if resp.Status != "success" || resp.Metadata.Command != "contributors" || resp.Metadata.Timestamp.IsZero() {
		t.Fatalf("unexpected response: %#v", resp)
	}
	items, ok := resp.Data["items"].([]map[string]any)
	if !ok || len(items) != 0 {
		t.Fatalf("items = %#v, want empty result", resp.Data["items"])
	}

	if got := contributorsGitOutput(t, "status", "--short"); got != statusBefore {
		t.Fatalf("contributors mutated worktree/index\nbefore:\n%s\nafter:\n%s", statusBefore, got)
	}
	if got := contributorsGitOutput(t, "rev-parse", "HEAD"); got != headBefore {
		t.Fatalf("contributors moved HEAD: before %q after %q", headBefore, got)
	}
}

func withContributorsDirectory(t *testing.T) {
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

func writeContributorsV1Config(t *testing.T) {
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

func contributorsGitOutput(t *testing.T, args ...string) string {
	t.Helper()
	output, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}
