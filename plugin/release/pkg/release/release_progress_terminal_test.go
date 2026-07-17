package release

import (
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

func TestTerminalReleaseProgressRendersTypedEventsToStderr(t *testing.T) {
	progress := newTerminalReleaseProgress()

	stdout, stderr := captureReleaseProgressOutput(t, func() {
		progress.ReportReleaseProgress(ReleaseProgressEvent{
			Kind:        ReleaseProgressReleaseStarted,
			ReleaseType: "patch",
		})
		progress.ReportReleaseProgress(ReleaseProgressEvent{
			Kind:           ReleaseProgressRepositoryContext,
			RepositoryRoot: "/repo",
			SourceFormat:   "v2",
			UnitID:         "api",
			ConfigPath:     "/repo/.neko/release.config.json",
			StatePath:      "/repo/.neko/release.state.json",
		})
		progress.ReportReleaseProgress(ReleaseProgressEvent{
			Kind:           ReleaseProgressReleasePlan,
			CurrentVersion: "1.2.3",
			NextVersion:    "1.2.4",
			Tag:            "api/v1.2.4",
			Executor:       "goreleaser",
			Delivery:       "github-actions",
			Workflow:       ".github/workflows/release-api.yml",
			TagPrefix:      "api/v",
		})
	})

	if stdout != "" {
		t.Fatalf("terminal release progress wrote to stdout:\n%s", stdout)
	}
	assertOrderedSubstrings(t, stderr,
		"Starting patch release",
		"Repository root: /repo",
		"Release source format: v2",
		"Selected unit: api",
		"Config path: /repo/.neko/release.config.json",
		"State path: /repo/.neko/release.state.json",
		"Planning V2 release: current=1.2.3 next=1.2.4 tag=api/v1.2.4",
		"Executor=goreleaser delivery=github-actions workflow=.github/workflows/release-api.yml tagPrefix=api/v",
	)
}

func TestTerminalReleaseProgressHonorsVerboseSuppression(t *testing.T) {
	originalVerbose := log.Verbose
	t.Cleanup(func() { log.Verbose = originalVerbose })
	progress := newTerminalReleaseProgress()

	log.Verbose = false
	_, quiet := captureReleaseProgressOutput(t, func() {
		progress.ReportReleaseProgress(ReleaseProgressEvent{Kind: ReleaseProgressPendingActionStarting, PendingAction: "write-state"})
	})
	if quiet != "" {
		t.Fatalf("verbose progress rendered while disabled:\n%s", quiet)
	}

	log.Verbose = true
	_, verbose := captureReleaseProgressOutput(t, func() {
		progress.ReportReleaseProgress(ReleaseProgressEvent{Kind: ReleaseProgressPendingActionStarting, PendingAction: "write-state"})
	})
	if !strings.Contains(verbose, "Starting release action: write-state") {
		t.Fatalf("verbose progress did not render expected text:\n%s", verbose)
	}
}

func TestTerminalReleaseProgressUnknownEventIsNoop(t *testing.T) {
	stdout, stderr := captureReleaseProgressOutput(t, func() {
		newTerminalReleaseProgress().ReportReleaseProgress(ReleaseProgressEvent{Kind: "unknown"})
	})
	if stdout != "" || stderr != "" {
		t.Fatalf("unknown progress event rendered output stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestTerminalReleaseProgressCannotRenderSecretFields(t *testing.T) {
	const secret = "dx1-terminal-secret"
	_, stderr := captureReleaseProgressOutput(t, func() {
		newTerminalReleaseProgress().ReportReleaseProgress(ReleaseProgressEvent{
			Kind:          ReleaseProgressTokenPreflightAvailable,
			SafeRemoteURL: "https://user:" + secret + "@github.com/owner/repo.git",
			Guidance:      "do not print " + secret,
		})
	})
	if strings.Contains(stderr, secret) {
		t.Fatalf("terminal progress rendered secret-bearing unused fields:\n%s", stderr)
	}
}
