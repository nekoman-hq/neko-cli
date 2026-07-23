package release

import (
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

func TestTerminalGitDiagnosticsOmitRootsPathsAndRawOutput(t *testing.T) {
	originalVerbose := log.Verbose
	log.Verbose = true
	t.Cleanup(func() { log.Verbose = originalVerbose })

	const (
		root   = "/private/tmp/lifecycle-diagnostics"
		secret = "diagnostic-output-secret"
	)
	_, stderr := captureReleaseProgressOutput(t, func() {
		diagnostics := terminalGitReleaseDiagnostics{}
		diagnostics.GitTopLevelResolved(root)
		diagnostics.GitCommandRunning(root, []string{"status", "--short"})
		diagnostics.GitCommandFailed([]string{"diff", root + "/.neko/release.state.json"}, secret)
		diagnostics.GitCommandSucceeded([]string{"rev-parse", "HEAD"}, strings.Repeat("a", 40))
	})

	for _, forbidden := range []string{root, secret} {
		if strings.Contains(stderr, forbidden) {
			t.Fatalf("terminal git diagnostics exposed %q:\n%s", forbidden, stderr)
		}
	}
	for _, want := range []string{
		"Git toplevel resolved to the selected repository root",
		"Running repository-local git command",
		"diff (repository-local path omitted)",
		"bytes, 1 lines",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("terminal git diagnostics omitted %q:\n%s", want, stderr)
		}
	}
}
