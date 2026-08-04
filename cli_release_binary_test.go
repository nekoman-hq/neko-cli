package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// buildTestCLI builds a throwaway neko binary that reports the given version
// through the production ldflags contract. It never replaces an installed
// binary and never contacts GitHub.
func buildTestCLI(t *testing.T, version string) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "neko")
	build := exec.Command("go", "build",
		"-ldflags", "-X github.com/nekoman-hq/neko-cli/pkg/version.Version="+version,
		"-o", binary,
		".",
	)
	build.Dir = mustGetwd(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build test CLI: %v\n%s", err, output)
	}
	return binary
}

type recordingReleaseServer struct {
	*httptest.Server
	requested []string
	mu        sync.Mutex
}

func (server *recordingReleaseServer) paths() []string {
	server.mu.Lock()
	defer server.mu.Unlock()
	return append([]string(nil), server.requested...)
}

// newReleaseServer serves the shared multi-unit release fixture on page one and
// an empty page afterwards, mirroring the real paginated GitHub response.
func newReleaseServer(t *testing.T, status int) *recordingReleaseServer {
	t.Helper()

	releases := readContractFixture(t, "mixed-releases.json")
	server := &recordingReleaseServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		server.mu.Lock()
		server.requested = append(server.requested, request.URL.RequestURI())
		server.mu.Unlock()

		if status != http.StatusOK {
			writer.WriteHeader(status)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("page") == "1" {
			_, _ = writer.Write(releases)
			return
		}
		_, _ = writer.Write([]byte("[]"))
	}))
	t.Cleanup(server.Close)
	return server
}

func runTestCLI(t *testing.T, binary, apiBase string, args ...string) (string, error) {
	t.Helper()

	command := exec.Command(binary, args...)
	command.Env = append(os.Environ(),
		"NO_COLOR=1",
		"NEKO_GITHUB_API_BASE="+apiBase,
		"NEKO_REPOSITORY=nekoman-hq/neko-cli",
		"NEKO_PLUGIN_DIR="+filepath.Join(t.TempDir(), "plugins"),
		"GITHUB_TOKEN=",
	)
	output, err := command.CombinedOutput()
	return string(output), err
}

// TestVersionAndUpdateResolveTheSameStableCLIRelease reproduces all three
// reported CLI-side defects against one controlled fixture: the repository-wide
// newest release is plugin-release/v4.4.5, the installed build is 3.0.3, and the
// newest stable CLI release is v3.1.2.
func TestVersionAndUpdateResolveTheSameStableCLIRelease(t *testing.T) {
	binary := buildTestCLI(t, "3.0.3")
	server := newReleaseServer(t, http.StatusOK)

	versionOutput, err := runTestCLI(t, binary, server.URL, "version")
	if err != nil {
		t.Fatalf("neko version: %v\n%s", err, versionOutput)
	}
	if !strings.Contains(versionOutput, "3.1.2 (v3.1.2)") {
		t.Fatalf("neko version must report the newest stable CLI release:\n%s", versionOutput)
	}
	for _, forbidden := range []string{"plugin-release", "plugin-ui", "plugin-registry", "4.4.5"} {
		if strings.Contains(versionOutput, forbidden) {
			t.Fatalf("neko version leaked %q:\n%s", forbidden, versionOutput)
		}
	}

	updateOutput, err := runTestCLI(t, binary, server.URL, "update", "--dry-run")
	if err != nil {
		t.Fatalf("neko update --dry-run: %v\n%s", err, updateOutput)
	}
	if !strings.Contains(updateOutput, "would upgrade 3.0.3 to 3.1.2") {
		t.Fatalf("neko update must offer the newest stable CLI release:\n%s", updateOutput)
	}
	if strings.Contains(updateOutput, "already running the latest version") {
		t.Fatalf("neko update reported an out-of-date build as current:\n%s", updateOutput)
	}

	for _, path := range server.paths() {
		if !strings.HasPrefix(path, "/repos/nekoman-hq/neko-cli/releases?per_page=100&page=") {
			t.Fatalf("unexpected CLI discovery request %q", path)
		}
	}
}

func TestUpdateReportsDiscoveryFailureInsteadOfAlreadyLatest(t *testing.T) {
	binary := buildTestCLI(t, "3.0.3")
	server := newReleaseServer(t, http.StatusInternalServerError)

	output, err := runTestCLI(t, binary, server.URL, "update")
	if err == nil {
		t.Fatalf("neko update unexpectedly succeeded:\n%s", output)
	}
	if strings.Contains(output, "already running the latest version") {
		t.Fatalf("a discovery failure was rendered as already-latest:\n%s", output)
	}
	if !strings.Contains(output, "unable to determine the latest CLI release") {
		t.Fatalf("expected an actionable discovery error:\n%s", output)
	}
	if !strings.Contains(output, "the installed executable is unchanged") {
		t.Fatalf("expected the unchanged-executable statement:\n%s", output)
	}
}

func TestVersionReportsDiscoveryFailureWithoutClaimingALatestRelease(t *testing.T) {
	binary := buildTestCLI(t, "3.0.3")
	server := newReleaseServer(t, http.StatusInternalServerError)

	output, err := runTestCLI(t, binary, server.URL, "version")
	if err != nil {
		t.Fatalf("neko version: %v\n%s", err, output)
	}
	if !strings.Contains(output, "unable to determine the latest CLI release") {
		t.Fatalf("expected an actionable discovery warning:\n%s", output)
	}
	if strings.Contains(output, "3.0.3 (v3.0.3)") {
		t.Fatalf("neko version presented the installed build as the latest release:\n%s", output)
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return dir
}
