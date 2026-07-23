package release

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestReleaseDryRunPresentationUsesOneSharedVocabulary(t *testing.T) {
	timestamp := time.Date(2026, time.July, 22, 12, 30, 0, 0, time.UTC)
	outcomes := []struct { //nolint:govet // Test rows follow name, command, fixture order.
		name    string
		command string
		outcome ReleaseCommandOutcome
	}{
		{
			name: "V1 patch", command: "patch",
			outcome: &V1ReleasePreview{
				ReleaseType: Patch, CurrentVersion: "1.2.3", NextVersion: "1.2.4", ReleaseSystem: "goreleaser",
			},
		},
		{
			name: "V2 minor", command: "minor",
			outcome: lifecycleV2PreviewFixture(Minor, "2.4.0", "2.5.0"),
		},
		{
			name: "V2 major", command: "major",
			outcome: lifecycleV2PreviewFixture(Major, "2.4.0", "3.0.0"),
		},
	}

	for _, test := range outcomes {
		t.Run(test.name, func(t *testing.T) {
			response, err := MapReleaseCommandOutcome(test.command, test.outcome, timestamp)
			if err != nil {
				t.Fatalf("MapReleaseCommandOutcome: %v", err)
			}
			before := canonicalLifecycleData(t, response)
			concise := ansi.Strip(renderLifecycleResponse(t, response, renderer.RenderOptions{
				Format: renderer.FormatTable, WidthProvider: releasePlanOutputWidth{width: 88, available: true},
			}))
			for _, want := range []string{
				"Release Summary",
				"Requested change",
				strings.ToUpper(test.command[:1]) + test.command[1:],
				"Dry run",
				"Operations",
				"Materialized Files",
				"Mutation boundary",
			} {
				if !strings.Contains(concise, want) {
					t.Fatalf("concise lifecycle output omitted %q:\n%s", want, concise)
				}
			}
			for _, hidden := range []string{"Source and Configuration", "Execution Evidence", "Git and Handoff"} {
				if strings.Contains(concise, hidden) {
					t.Fatalf("concise lifecycle output exposed describe-only %q:\n%s", hidden, concise)
				}
			}

			described := ansi.Strip(renderLifecycleResponse(t, response, renderer.RenderOptions{
				Format: renderer.FormatTable, Describe: true,
				WidthProvider: releasePlanOutputWidth{width: 88, available: true},
			}))
			for _, want := range []string{
				"Release Summary",
				"Operations",
				"Materialized Files",
				"Source and Configuration",
				"Execution Evidence",
				"Git and Handoff",
				"Limitations",
			} {
				if !strings.Contains(described, want) {
					t.Fatalf("described lifecycle output omitted %q:\n%s", want, described)
				}
			}
			if after := canonicalLifecycleData(t, response); !bytes.Equal(before, after) {
				t.Fatalf("rendering changed lifecycle domain data:\nbefore=%s\nafter=%s", before, after)
			}
		})
	}
}

func TestReleasePresentationLeavesCanonicalJSONUnchanged(t *testing.T) {
	timestamp := time.Date(2026, time.July, 22, 12, 31, 0, 0, time.UTC)
	outcomes := []struct { //nolint:govet // Test rows follow command and fixture order.
		command string
		outcome ReleaseCommandOutcome
	}{
		{command: "patch", outcome: &V1ReleasePreview{
			ReleaseType: Patch, CurrentVersion: "1.2.3", NextVersion: "1.2.4", ReleaseSystem: "goreleaser",
		}},
		{command: "minor", outcome: lifecycleV2PreviewFixture(Minor, "2.4.0", "2.5.0")},
		{command: "major", outcome: &V1ReleaseCompleted{
			ReleaseType: Major, PreviousVersion: "1.2.3", NextVersion: "2.0.0", ReleaseSystem: "goreleaser",
		}},
		{command: "patch", outcome: &GitHubActionsReleaseResult{
			Unit: "api", Version: "2.4.1", Tag: "api/v2.4.1", CommitSHA: strings.Repeat("a", 40),
			Workflow: ".github/workflows/release-api.yml", ExecutionJournalPath: "/private/tmp/repo/.git/neko/release/executions/id.json",
			DispatchJournalPath: "/private/tmp/repo/.git/neko/release/dispatches/id.json",
			ExecutionState:      ReleaseExecutionHandoffReady, DispatchState: DispatchJournalAccepted,
			RecoveryGuidance: "GitHub Actions owns build and publish from the pushed tag.",
		}},
	}

	for _, test := range outcomes {
		response, err := MapReleaseCommandOutcome(test.command, test.outcome, timestamp)
		if err != nil {
			t.Fatalf("MapReleaseCommandOutcome(%s): %v", test.command, err)
		}
		plain := renderLifecycleResponse(t, response, renderer.RenderOptions{Format: renderer.FormatJSON})
		for _, options := range []renderer.RenderOptions{
			{Format: renderer.FormatJSON, Describe: true},
			{Format: renderer.FormatJSON, Verbose: true},
			{Format: renderer.FormatJSON, Describe: true, Verbose: true},
		} {
			if got := renderLifecycleResponse(t, response, options); got != plain {
				t.Fatalf("%s JSON changed with presentation mode:\nplain=%s\nmode=%s", test.command, plain, got)
			}
		}
		for _, forbidden := range []string{"human_table", "human_properties", "describe_only", "\x1b["} {
			if strings.Contains(plain, forbidden) {
				t.Fatalf("%s JSON exposed presentation metadata %q:\n%s", test.command, forbidden, plain)
			}
		}
	}
}

func TestReleasePresentationIsResponsiveAndPathSafe(t *testing.T) {
	response, err := MapReleaseCommandOutcome(
		"patch",
		lifecycleV2PreviewFixture(Patch, "2.4.0", "2.4.1"),
		time.Date(2026, time.July, 22, 12, 32, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("MapReleaseCommandOutcome: %v", err)
	}
	narrow := renderLifecycleResponse(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true,
		WidthProvider: releasePlanOutputWidth{width: 36, available: true},
	})
	narrowPlain := ansi.Strip(narrow)
	for _, want := range []string{"Release Summary", "Requested", "Patch", "Operations", "Action:"} {
		if !strings.Contains(narrowPlain, want) {
			t.Fatalf("narrow lifecycle output omitted %q:\n%s", want, narrowPlain)
		}
	}
	assertReleasePlanLinesFit(t, narrow, 36)
	if strings.Contains(narrowPlain, "/private/tmp/lifecycle-contract") {
		t.Fatalf("narrow lifecycle output exposed an absolute path:\n%s", narrowPlain)
	}

	options := renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true,
		WidthProvider: releasePlanOutputWidth{},
	}
	first := renderLifecycleResponse(t, response, options)
	second := renderLifecycleResponse(t, response, options)
	if first != second || !strings.Contains(ansi.Strip(first), "Record 1") {
		t.Fatalf("unknown-width lifecycle presentation is not deterministic and vertical:\nfirst=%s\nsecond=%s", first, second)
	}
}

func TestLifecycleFailuresRemainSpecificAndActionable(t *testing.T) {
	failures := []struct {
		command string
		code    string
		message string
		title   string
	}{
		{command: "patch", code: "CONFIG_NOT_FOUND", message: "release configuration is missing", title: "Release Refused"},
		{command: "minor", code: "UNIT_RESOLUTION_FAILED", message: "selected unit is ambiguous", title: "Release Refused"},
		{command: "major", code: "V2_GITHUB_ACTIONS_RELEASE_FAILED", message: "release tag already exists", title: "Release Rejected"},
		{command: "resume", code: "NO_RESUMABLE_JOURNAL", message: "No resumable release execution journal was found", title: "Resume Refused"},
	}
	for _, test := range failures {
		response := MapCommandFailure(
			test.command,
			failureFromMessage(test.code, test.message),
			time.Date(2026, time.July, 22, 12, 33, 0, 0, time.UTC),
		)
		output := ansi.Strip(renderLifecycleResponse(t, response, renderer.RenderOptions{Format: renderer.FormatTable}))
		for _, want := range []string{test.title, test.code, test.message, "Next safe action"} {
			if !strings.Contains(output, want) {
				t.Fatalf("%s failure output omitted %q:\n%s", test.command, want, output)
			}
		}
		if response.Error.Code != test.code || response.Error.Message != test.message {
			t.Fatalf("%s failure envelope changed: %#v", test.command, response.Error)
		}
	}
}

func TestLifecycleFailurePresentationRedactsPathsWithoutChangingMachineError(t *testing.T) {
	const message = "multiple resumable journals found: /private/tmp/lifecycle-failure/.git/neko/release/executions/one.json; inspect manually"
	response := MapCommandFailure(
		"resume",
		failureFromMessage("MULTIPLE_RESUMABLE_JOURNALS", message),
		time.Date(2026, time.July, 22, 12, 34, 0, 0, time.UTC),
	)
	human := ansi.Strip(renderLifecycleResponse(t, response, renderer.RenderOptions{Format: renderer.FormatTable}))
	if strings.Contains(human, "/private/tmp/lifecycle-failure") ||
		!strings.Contains(human, "repository-local path") ||
		!strings.Contains(human, "inspect manually") {
		t.Fatalf("failure presentation did not safely preserve the actionable reason:\n%s", human)
	}
	jsonOutput := renderLifecycleResponse(t, response, renderer.RenderOptions{Format: renderer.FormatJSON})
	if !strings.Contains(jsonOutput, message) || response.Error.Message != message {
		t.Fatalf("machine error message changed:\n%s", jsonOutput)
	}
}

func lifecycleV2PreviewFixture(releaseType Type, currentVersion, nextVersion string) *V2ReleasePreview {
	return &V2ReleasePreview{
		UnitID: "api", CurrentVersion: currentVersion, NextVersion: nextVersion,
		Tag: "api/v" + nextVersion, Executor: "goreleaser", Delivery: string(config.DeliveryGitHubActions),
		Workflow: ".github/workflows/release-api.yml", WorkingDirectory: ".",
		UnitRoot:    "/private/tmp/lifecycle-contract/services/api",
		StateChange: ".neko/release.state.json: api " + currentVersion + " -> " + nextVersion,
		MaterializedFilePaths: []string{
			".neko/release.state.json",
			"services/api/package.json",
		},
		KnownReleaseFilePaths: []string{
			".goreleaser.yml",
			".neko/release.state.json",
			"services/api/package.json",
		},
		CommitMessage:    "chore(release): api api/v" + nextVersion,
		OwnershipSummary: "release lifecycle owns state and handoff",
		V2GitOwnership:   "release lifecycle owns commit, tag, and push",
		StateGuarantee:   "state is committed with the release",
		Dispatch: &ReleaseDispatchDryRunSummary{
			Ref: "api/v" + nextVersion,
			Inputs: map[string]string{
				"release_sha": "pending release commit",
				"tag":         "api/v" + nextVersion,
				"unit":        "api",
				"version":     nextVersion,
			},
			JournalIdentity: "dry-run",
			JournalLocation: "/private/tmp/lifecycle-contract/.git/neko/release/dispatches/dry-run.json",
			Status:          "not created during dry-run",
		},
	}
}

func renderLifecycleResponse(t *testing.T, response *plugin.Response, options renderer.RenderOptions) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, options, &output); err != nil {
		t.Fatalf("RenderWithOptionsTo: %v", err)
	}
	return output.String()
}

func canonicalLifecycleData(t *testing.T, response *plugin.Response) []byte {
	t.Helper()
	data, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatalf("marshal lifecycle data: %v", err)
	}
	return data
}
