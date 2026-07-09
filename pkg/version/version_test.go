package version

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/git"
)

func TestDisplayReleaseLabelsLatestAsCLIRelease(t *testing.T) {
	output := captureStdout(t, func() {
		displayRelease(&github.RepoInfo{Owner: "nekoman-hq", Repo: "neko-cli"}, &github.Release{
			Name:        "Nekocli 3.0.4",
			TagName:     "v3.0.4",
			PublishedAt: "2026-07-09T17:11:02Z",
			HTMLURL:     "https://github.com/nekoman-hq/neko-cli/releases/tag/v3.0.4",
		})
	})

	if !strings.Contains(output, "Latest CLI Release") {
		t.Fatalf("expected Latest CLI Release label, got:\n%s", output)
	}
	if strings.Contains(output, "plugin-release/v") || strings.Contains(output, "plugin-ui/v") {
		t.Fatalf("version output must not mention plugin release tags, got:\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writePipe

	fn()

	if err := writePipe.Close(); err != nil {
		t.Fatalf("close stdout pipe writer: %v", err)
	}
	os.Stdout = originalStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, readPipe); err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	if err := readPipe.Close(); err != nil {
		t.Fatalf("close stdout pipe reader: %v", err)
	}

	return buf.String()
}
