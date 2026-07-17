package release

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestGitHubActionsReleaseProgressCharacterization(t *testing.T) {
	_, _, ctx := newActiveGitHubActionsReleaseRepository(t)
	client := &recordingWorkflowDispatchClient{response: GitHubActionsDispatchResponse{
		State:      DispatchJournalAccepted,
		HTTPStatus: 204,
	}}
	runner := NewGitHubActionsReleaseRunner(
		WithGitHubActionsReleaseTokenResolver(staticDispatchTokenResolver{token: "dx1-secret-token"}),
		WithGitHubActionsReleaseDispatchClient(client),
		WithGitHubActionsReleaseProgress(newTerminalReleaseProgress()),
	)

	_, stderr := captureReleaseProgressOutput(t, func() {
		result, runErr := runner.Run(context.Background(), ctx)
		if runErr != nil {
			t.Fatalf("Run: %v", runErr)
		}
		if result.ExecutionState != ReleaseExecutionHandoffReady {
			t.Fatalf("execution state = %s, want %s", result.ExecutionState, ReleaseExecutionHandoffReady)
		}
	})

	assertOrderedSubstrings(t, stderr,
		"Repository root: "+ctx.RepositoryRoot,
		"Release source format: v2",
		"Selected unit: api",
		"Planning V2 release: current=0.2.0 next=0.2.1 tag=api/v0.2.1",
		"Executor=goreleaser delivery=github-actions workflow=.github/workflows/release-api.yml tagPrefix=api/v",
		"GitHub token preflight: resolving token without printing it",
		"GitHub token preflight: token available",
		"Planning materialized files",
		"Planned materialized files: none",
		"Known release files: .neko/release.state.json",
		"Running git preflight checks",
		"Git preflight: branch=",
		"Git preflight: remote URL=https://github.com/nekoman/repo.git",
		"Git preflight: workflow validation passed for .github/workflows/release-api.yml",
		"Execution journal preflight: unresolved journals=0",
		"Preparing execution journal",
		"Execution phase: preflight-validated",
		"Capturing materialization snapshots",
		"Applied materialized files: none",
		"Capturing V2 state snapshot",
		"Writing V2 state update: api -> 0.2.1",
		"State update written",
		"Staging targeted release files: .neko/release.state.json",
		"Targeted release files staged",
		"Creating release commit: chore(release): api api/v0.2.1",
		"Release commit created:",
		"Creating unit tag: api/v0.2.1",
		"Preparing dispatch journal",
		"Dispatch inputs: release_sha=",
		"Pushing release commit ",
		"Release commit push succeeded",
		"Pushing unit tag api/v0.2.1",
		"Unit tag push succeeded",
		"Dispatching workflow .github/workflows/release-api.yml for ref api/v0.2.1",
		"GitHub Actions target resolved: nekoman/repo workflow=.github/workflows/release-api.yml ref=api/v0.2.1",
		"Preparing GitHub Actions dispatch journal",
		"GitHub Actions dispatch token available",
		"Recording dispatch request-started before HTTP call",
		"Sending workflow_dispatch request",
		"workflow_dispatch response state=accepted status=204",
		"Dispatch journal finalized with state: accepted",
		"Dispatch state: accepted",
		"Dispatch run: not resolved",
		"Execution state: handoff-ready",
		"Recovery guidance: GitHub Actions dispatch accepted. GitHub Actions owns build and publish from the pushed tag.",
	)
	if strings.Contains(stderr, "dx1-secret-token") || strings.Contains(stderr, "Bearer") {
		t.Fatalf("progress output exposed a secret:\n%s", stderr)
	}
}

func TestV2DryRunProgressCharacterizationKeepsMachineResponseSeparate(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	expectedRoot := root
	if strings.HasPrefix(expectedRoot, "/var/") {
		expectedRoot = "/private" + expectedRoot
	}
	withWorkingDirectoryRoot(t, root)
	t.Setenv("GITHUB_TOKEN", "dx1-dry-run-secret")

	stdout, stderr := captureReleaseProgressOutput(t, func() {
		resp, err := HandleRelease(plugin.Request{
			Command: "patch",
			Flags: map[string]any{
				"dry-run": true,
				"unit":    "api",
			},
		}, Patch)
		if err != nil {
			t.Fatalf("HandleRelease: %v", err)
		}
		if resp.Status != "success" {
			t.Fatalf("expected success, got %#v", resp.Error)
		}
		body, marshalErr := json.Marshal(resp)
		if marshalErr != nil {
			t.Fatalf("marshal response: %v", marshalErr)
		}
		for _, forbidden := range []string{
			"Starting patch release",
			"Planning V2 dry-run",
			"dx1-dry-run-secret",
		} {
			if strings.Contains(string(body), forbidden) {
				t.Fatalf("machine response contains progress or secret %q:\n%s", forbidden, body)
			}
		}
	})

	if stdout != "" {
		t.Fatalf("HandleRelease wrote progress to stdout:\n%s", stdout)
	}
	assertOrderedSubstrings(t, stderr,
		"Starting patch release",
		"Repository root: "+expectedRoot,
		"Release source format: v2",
		"Selected unit: api",
		"Planning V2 dry-run: current=0.2.0 next=0.2.1 tag=api/v0.2.1",
		"Executor=goreleaser delivery=github-actions workflow=.github/workflows/release-api.yml tagPrefix=api/v",
		"Planned materialized files: none",
		"Known release files: .neko/release.state.json",
		"Dry run only: no token required, no journal created, no commit/tag/push/dispatch",
		"Planned dispatch ref: api/v0.2.1",
		"Planned dispatch inputs: release_sha=pending release commit tag=api/v0.2.1 unit=api version=0.2.1",
	)
	if strings.Contains(stderr, "dx1-dry-run-secret") || strings.Contains(stderr, "Execution state: handoff-ready") {
		t.Fatalf("dry-run progress exposed secret or execution progress:\n%s", stderr)
	}
}

func TestReleaseVerboseProgressCharacterization(t *testing.T) {
	originalVerbose := log.Verbose
	t.Cleanup(func() { log.Verbose = originalVerbose })

	run := func(verbose bool) string {
		log.Verbose = verbose
		_, stderr := captureReleaseProgressOutput(t, func() {
			_, err := (applyGitHubActionsReleaseMaterialization{
				journal:      recordingVerboseProgressJournal{},
				transactions: recordingVerboseProgressTransactions{},
				progress:     newTerminalReleaseProgress(),
			}).Apply(preparedGitHubActionsReleaseExecution{Identity: ReleaseExecutionIdentity{SHA256: "execution"}}, &MaterializationPlan{})
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
		})
		return stderr
	}

	withoutVerbose := run(false)
	if strings.Contains(withoutVerbose, "Starting release action: apply-materialization") {
		t.Fatalf("verbose progress rendered while disabled:\n%s", withoutVerbose)
	}
	withVerbose := run(true)
	assertOrderedSubstrings(t, withVerbose,
		"Capturing materialization snapshots",
		"Materialization snapshots captured",
		"Starting release action: apply-materialization",
		"Execution journal pending action recorded: apply-materialization",
		"Release action completed: apply-materialization",
		"Execution phase confirmed: materialization-applied",
		"Applied materialized files: none",
	)
}

type recordingVerboseProgressJournal struct{}

func (recordingVerboseProgressJournal) BeginPending(ReleaseExecutionIdentity, ReleaseExecutionPendingAction) (*ReleaseExecutionJournalResolution, error) {
	return &ReleaseExecutionJournalResolution{}, nil
}

func (recordingVerboseProgressJournal) ConfirmPhase(ReleaseExecutionIdentity, ReleaseExecutionJournalState, ReleaseExecutionJournalUpdate) (*ReleaseExecutionJournalResolution, error) {
	return &ReleaseExecutionJournalResolution{}, nil
}

func (recordingVerboseProgressJournal) RecordLastError(ReleaseExecutionIdentity, string) (*ReleaseExecutionJournalResolution, error) {
	return &ReleaseExecutionJournalResolution{}, nil
}

type recordingVerboseProgressTransactions struct{}

func (recordingVerboseProgressTransactions) New(*MaterializationPlan) releaseMaterializationTransaction {
	return recordingVerboseProgressTransaction{}
}

type recordingVerboseProgressTransaction struct{}

func (recordingVerboseProgressTransaction) CaptureSnapshots() error { return nil }
func (recordingVerboseProgressTransaction) Apply() (*AppliedMaterialization, error) {
	return &AppliedMaterialization{}, nil
}
func (recordingVerboseProgressTransaction) Restore() error { return nil }

func captureReleaseProgressOutput(t *testing.T, run func()) (string, string) {
	t.Helper()
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	originalStdout := os.Stdout
	originalStderr := os.Stderr
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	run()

	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	os.Stdout = originalStdout
	os.Stderr = originalStderr

	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if err := stdoutReader.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	if err := stderrReader.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}
	return string(stdout), string(stderr)
}

func assertOrderedSubstrings(t *testing.T, output string, ordered ...string) {
	t.Helper()
	searchFrom := 0
	for _, want := range ordered {
		index := strings.Index(output[searchFrom:], want)
		if index < 0 {
			t.Fatalf("output missing ordered substring %q after offset %d:\n%s", want, searchFrom, output)
		}
		searchFrom += index + len(want)
	}
}
