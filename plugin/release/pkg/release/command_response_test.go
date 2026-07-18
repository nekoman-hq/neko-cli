package release

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestMapCommandFailurePreservesMetadataDetailsAndInjectedTimestamp(t *testing.T) {
	timestamp := time.Date(2026, time.July, 14, 12, 0, 0, 123, time.FixedZone("test", 2*60*60))
	failure := &CommandFailure{
		Code:    "CONFIG_NOT_FOUND",
		Message: "missing release config",
		Details: map[string]any{"hint": "run init"},
	}

	resp := MapCommandFailure("patch", failure, timestamp)

	if resp.Status != "error" || resp.Error.Code != failure.Code || resp.Error.Message != failure.Message {
		t.Fatalf("unexpected error mapping: %#v", resp)
	}
	if resp.Metadata.Command != "patch" || resp.Metadata.Plugin == "" || resp.Metadata.Version == "" || !resp.Metadata.Timestamp.Equal(timestamp) {
		t.Fatalf("unexpected error metadata: %#v", resp.Metadata)
	}
	if resp.Error.Details["hint"] != "run init" {
		t.Fatalf("details changed: %#v", resp.Error.Details)
	}
	failure.Details["hint"] = "changed after mapping"
	if resp.Error.Details["hint"] != "run init" {
		t.Fatal("mapped error details alias the application failure")
	}
}

func TestMapReleaseCommandOutcomePreservesStableRows(t *testing.T) {
	timestamp := time.Date(2026, time.July, 14, 13, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		command    string
		outcome    ReleaseCommandOutcome
		properties []string
	}{
		{
			name:    "legacy preview",
			command: "minor",
			outcome: &LegacyReleasePreview{ReleaseType: Minor, CurrentVersion: "1.2.3", NextVersion: "1.3.0", ReleaseSystem: "goreleaser"},
			properties: []string{
				"Release Type", "Current Version", "New Version", "Release System", "Dry Run", "Status",
			},
		},
		{
			name:    "legacy completion",
			command: "patch",
			outcome: &LegacyReleaseCompleted{ReleaseType: Patch, PreviousVersion: "1.2.3", NextVersion: "1.2.4", ReleaseSystem: "goreleaser"},
			properties: []string{
				"Release Type", "Previous Version", "New Version", "Release System", "Status",
			},
		},
		{
			name:    "v2 preview",
			command: "major",
			outcome: &V2ReleasePreview{
				UnitID:                "api",
				CurrentVersion:        "1.2.3",
				NextVersion:           "2.0.0",
				Tag:                   "api/v2.0.0",
				Executor:              "goreleaser",
				Delivery:              "github-actions",
				Workflow:              ".github/workflows/release-api.yml",
				WorkingDirectory:      ".",
				UnitRoot:              "/repo",
				StateChange:           "api: 1.2.3 -> 2.0.0",
				KnownReleaseFilePaths: []string{".neko/release.state.json"},
				CommitMessage:         "chore(release): api v2.0.0",
				OwnershipSummary:      "neko owns release state and tag",
				V2GitOwnership:        "neko owns git coordination",
				StateGuarantee:        "state is committed with release files",
				Dispatch: &ReleaseDispatchDryRunSummary{
					Ref:             "api/v2.0.0",
					Inputs:          map[string]string{"version": "2.0.0", "unit": "api"},
					JournalIdentity: "pending release commit",
					JournalLocation: "pending release commit",
					Status:          "planned after release commit and tag push",
				},
			},
			properties: []string{
				"Release Type", "Unit", "Current Version", "New Version", "Tag", "Executor", "Delivery", "Workflow",
				"Dispatch", "Working Directory", "Unit Root", "State Change", "Materialized Files", "Known Release Files",
				"Planned Release Commit", "Planned Tag", "Planned Push Order", "Tool Ownership", "V2 Git Ownership",
				"State Commit Guarantee", "Executor Start", "Dry Run", "Status", "Dispatch Ref", "Dispatch Inputs",
				"Journal Identity", "Journal Location", "Dispatch Status",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := MapReleaseCommandOutcome(tt.command, tt.outcome, timestamp)
			if err != nil {
				t.Fatalf("MapReleaseCommandOutcome: %v", err)
			}
			if resp.Status != "success" || resp.RendererHint != "table" || resp.Metadata.Command != tt.command || !resp.Metadata.Timestamp.Equal(timestamp) {
				t.Fatalf("unexpected response envelope: %#v", resp)
			}
			if got := responseProperties(t, resp.Data["items"]); !slices.Equal(got, tt.properties) {
				t.Fatalf("properties = %#v, want %#v", got, tt.properties)
			}
		})
	}
}

func TestMapReleasePlanInspectionResponseContract(t *testing.T) {
	timestamp := time.Date(2026, time.July, 15, 9, 0, 0, 0, time.UTC)
	resp := MapReleasePlanInspection(&ReleasePlanInspection{
		Source:           "v2",
		Unit:             ReleasePlanInspectionUnit{ID: "api", DisplayName: "API"},
		CurrentVersion:   "1.2.3",
		RequestedChange:  Patch,
		NextVersion:      "1.2.4",
		Tag:              "api/v1.2.4",
		Executor:         "goreleaser",
		Delivery:         "github-actions",
		Workflow:         ".github/workflows/release-api.yml",
		WorkingDirectory: ".",
		UnitRoot:         "/repo",
		MaterializedOutputs: []PlannedMaterializedOutput{
			{Path: "plugin/release/manifest.json", Reason: "sync plugin manifest version with release plan"},
		},
		KnownReleaseFiles: []InspectedKnownReleaseFile{
			{Path: ".neko/release.state.json", Reason: "v2 release state"},
		},
		Readiness: LocalPlanReady,
		Limitations: []ReleasePlanLimitation{
			{Category: "local-only", Message: "No execution is started."},
		},
	}, timestamp)

	if resp.Status != "success" || resp.RendererHint != "table" || resp.Metadata.Command != "plan" || !resp.Metadata.Timestamp.Equal(timestamp) {
		t.Fatalf("unexpected plan response envelope: %#v", resp)
	}
	want := []string{
		"Source", "Unit", "Display Name", "Current Version", "Requested Change", "Next Version", "Tag",
		"Executor", "Delivery", "Workflow", "Working Directory", "Unit Root", "Planned Materialized Files",
		"Known Release Files", "Local Readiness", "Local Blockers", "Limitations", "Status",
	}
	if got := responseProperties(t, resp.Data["items"]); !slices.Equal(got, want) {
		t.Fatalf("plan response properties = %#v, want %#v", got, want)
	}
	if got := responseValueForProperty(t, resp.Data["items"], "Planned Materialized Files"); !strings.Contains(got, "sync plugin manifest") {
		t.Fatalf("materialized output reason missing: %q", got)
	}
	if got := responseValueForProperty(t, resp.Data["items"], "Local Blockers"); got != "none" {
		t.Fatalf("empty blocker value = %q", got)
	}
}

func TestReleasePlanMappingPreservesTypedLimitationsAndMachineRow(t *testing.T) {
	limitations := []ReleasePlanLimitation{
		{Category: "local-only", Message: "This inspection uses local facts only."},
		{Category: "no-remote-checks", Message: "Remote release state is not inspected."},
		{Category: "token-free", Message: "Tokens are not read or reported."},
	}
	result := &ReleasePlanInspection{
		Source:      "v2",
		Unit:        ReleasePlanInspectionUnit{ID: "cli"},
		Limitations: append([]ReleasePlanLimitation(nil), limitations...),
	}

	response := MapReleasePlanInspection(result, time.Date(2026, time.July, 18, 18, 0, 0, 0, time.UTC))

	wantMachineValue := "local-only: This inspection uses local facts only. | " +
		"no-remote-checks: Remote release state is not inspected. | " +
		"token-free: Tokens are not read or reported."
	if got := responseValueForProperty(t, response.Data["items"], "Limitations"); got != wantMachineValue {
		t.Fatalf("machine-readable limitations = %q, want %q", got, wantMachineValue)
	}
	if !slices.Equal(result.Limitations, limitations) {
		t.Fatalf("typed limitations changed during mapping: %#v", result.Limitations)
	}
}

func TestMapResumeCommandOutcomePreservesAssessmentRowsAndTimestamp(t *testing.T) {
	timestamp := time.Date(2026, time.July, 14, 14, 0, 0, 0, time.UTC)
	outcome := &ResumeAssessment{
		UnitID:               "api",
		NextVersion:          "1.2.4",
		Tag:                  "api/v1.2.4",
		ExecutionJournalPath: ".git/neko/release/executions/example.json",
		State:                ReleaseExecutionPrepared,
		PendingAction:        ReleaseExecutionPendingNone,
		RecoveryStatus:       ReleaseExecutionRecoveryNotStarted,
		SafeToContinue:       false,
		KnownFilePaths:       []string{".neko/release.state.json", "plugin/release/manifest.json"},
		Guidance:             "Inspect before continuing.",
	}

	resp, err := MapResumeCommandOutcome(outcome, timestamp)

	if err != nil {
		t.Fatalf("MapResumeCommandOutcome: %v", err)
	}
	want := []string{
		"Unit", "Version", "Tag", "Execution Journal", "State", "Pending Action", "Recovery Status",
		"Safe To Continue", "Known Files", "Next Step",
	}
	if got := responseProperties(t, resp.Data["items"]); !slices.Equal(got, want) {
		t.Fatalf("properties = %#v, want %#v", got, want)
	}
	if resp.Metadata.Command != "resume" || !resp.Metadata.Timestamp.Equal(timestamp) || resp.RendererHint != "table" {
		t.Fatalf("unexpected resume response envelope: %#v", resp)
	}
}

func TestMapGitHubActionsOutcomePreservesHandoffRowsForReleaseAndResume(t *testing.T) {
	timestamp := time.Date(2026, time.July, 14, 15, 0, 0, 0, time.UTC)
	outcome := &GitHubActionsReleaseResult{
		Unit:                 "api",
		Version:              "1.2.4",
		Tag:                  "api/v1.2.4",
		CommitSHA:            "abc123",
		Workflow:             ".github/workflows/release-api.yml",
		ExecutionJournalPath: ".git/neko/release/executions/example.json",
		DispatchJournalPath:  ".git/neko/release/dispatch/example.json",
		ExecutionState:       ReleaseExecutionHandoffReady,
		DispatchState:        DispatchJournalAccepted,
		RecoveryGuidance:     "GitHub Actions owns publication.",
	}
	want := []string{
		"Unit", "Version", "Tag", "Release Commit", "Workflow", "Execution Journal", "Dispatch Journal",
		"Execution State", "Dispatch State", "Dispatch Run", "Status",
	}

	releaseResp, err := MapReleaseCommandOutcome("patch", outcome, timestamp)
	if err != nil {
		t.Fatalf("MapReleaseCommandOutcome: %v", err)
	}
	resumeResp, err := MapResumeCommandOutcome(outcome, timestamp)
	if err != nil {
		t.Fatalf("MapResumeCommandOutcome: %v", err)
	}
	for command, mapped := range map[string]*plugin.Response{"patch": releaseResp, "resume": resumeResp} {
		if got := responseProperties(t, mapped.Data["items"]); !slices.Equal(got, want) {
			t.Fatalf("%s properties = %#v, want %#v", command, got, want)
		}
		if mapped.Metadata.Command != command || !mapped.Metadata.Timestamp.Equal(timestamp) {
			t.Fatalf("%s response metadata changed: %#v", command, mapped.Metadata)
		}
		if got := responseValueForProperty(t, mapped.Data["items"], "Dispatch Run"); got != "not resolved" {
			t.Fatalf("%s dispatch fallback = %q", command, got)
		}
	}
}
