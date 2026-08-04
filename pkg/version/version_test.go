package version

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/git"
)

func TestDisplayReleaseShowsResolvedStableTagAndNeverTheReleaseTitle(t *testing.T) {
	output := captureStdout(t, func() {
		displayRelease(&github.RepoInfo{Owner: "nekoman-hq", Repo: "neko-cli"}, &github.SelectedCLIRelease{
			Version:     "v3.1.2",
			TagName:     "v3.1.2",
			PublishedAt: "2026-08-01T18:30:00Z",
			HTMLURL:     "https://github.com/nekoman-hq/neko-cli/releases/tag/v3.1.2",
			Author:      "release-bot",
		})
	})

	if !strings.Contains(output, "Latest CLI Release") {
		t.Fatalf("expected Latest CLI Release label, got:\n%s", output)
	}
	if !strings.Contains(output, "3.1.2 (v3.1.2)") {
		t.Fatalf("expected resolved stable version and tag, got:\n%s", output)
	}
	if strings.Contains(output, "plugin-release") || strings.Contains(output, "plugin-ui") || strings.Contains(output, "plugin-registry") {
		t.Fatalf("version output must not mention plugin releases, got:\n%s", output)
	}
}

// TestLatestReportsTheStableCLIReleaseFromAMixedReleaseList reproduces the
// reported defect: the repository-wide newest release is plugin-release/v4.4.5,
// and `neko version` must still report the newest stable CLI release.
func TestLatestReportsTheStableCLIReleaseFromAMixedReleaseList(t *testing.T) {
	server := releaseListServer(t, []map[string]any{
		{"name": "plugin-release 4.4.5", "tag_name": "plugin-release/v4.4.5"},
		{"name": "Neko Plugin Registry", "tag_name": "plugin-registry"},
		{"name": "plugin-ui 1.3.0", "tag_name": "plugin-ui/v1.3.0"},
		{"name": "Nekocli 3.2.0", "tag_name": "v3.2.0", "draft": true},
		{"name": "Nekocli 3.1.3 RC1", "tag_name": "v3.1.3-rc.1", "prerelease": true},
		{"name": "Nekocli 3.1.2", "tag_name": "v3.1.2", "published_at": "2026-08-01T18:30:00Z"},
		{"name": "Nekocli 3.0.10", "tag_name": "v3.0.10"},
	})
	t.Setenv("NEKO_GITHUB_API_BASE", server.URL)

	output := captureStdout(t, func() {
		if err := Latest(context.Background(), github.CLIRepository()); err != nil {
			t.Fatalf("Latest: %v", err)
		}
	})

	if !strings.Contains(output, "3.1.2 (v3.1.2)") {
		t.Fatalf("expected the newest stable CLI release, got:\n%s", output)
	}
	if strings.Contains(output, "4.4.5") || strings.Contains(output, "plugin-release") {
		t.Fatalf("version output must ignore plugin releases, got:\n%s", output)
	}
}

func TestLatestReportsDiscoveryFailureAndNeverPresentsTheInstalledVersionAsLatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	t.Setenv("NEKO_GITHUB_API_BASE", server.URL)

	previousVersion := Version
	Version = "3.0.3"
	t.Cleanup(func() { Version = previousVersion })

	output := captureStdout(t, func() {
		if err := Latest(context.Background(), github.CLIRepository()); err != nil {
			t.Fatalf("Latest: %v", err)
		}
	})

	if strings.Contains(output, "Latest CLI Release\n") && strings.Contains(output, "Repository:") {
		t.Fatalf("failed discovery must not render a Latest CLI Release panel, got:\n%s", output)
	}
	if strings.Contains(output, "3.0.3 (v3.0.3)") {
		t.Fatalf("failed discovery must never present the installed version as the latest release, got:\n%s", output)
	}
}

func releaseListServer(t *testing.T, releases []map[string]any) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.Path, "/releases/latest") {
			t.Errorf("CLI discovery requested %q", request.URL.Path)
		}
		payload := releases
		if request.URL.Query().Get("page") != "1" {
			payload = nil
		}
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(payload); err != nil {
			t.Errorf("encode release list: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writePipe

	var buf bytes.Buffer
	copied := make(chan struct{})
	go func() {
		defer close(copied)
		_, _ = io.Copy(&buf, readPipe)
	}()

	fn()

	if err := writePipe.Close(); err != nil {
		t.Fatalf("close stdout pipe writer: %v", err)
	}
	os.Stdout = originalStdout
	<-copied
	if err := readPipe.Close(); err != nil {
		t.Fatalf("close stdout pipe reader: %v", err)
	}

	return buf.String()
}
