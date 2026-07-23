package doctor

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestDoctorDefaultKeepsOnlySummaryAndActionableFindings(t *testing.T) {
	result := doctorDescribeContractFixture()
	response := mapIntegrationDoctorResult(result, time.Time{})

	if response.PresentationProperties == nil {
		t.Fatal("doctor response omitted summary")
	}
	if response.PresentationTable == nil || response.PresentationTable.Title != "Findings" {
		t.Fatalf("doctor default table = %#v, want Findings", response.PresentationTable)
	}
	if got := len(response.PresentationTable.Rows); got != 3 {
		t.Fatalf("default finding rows = %d, want three actionable facts", got)
	}
	for _, row := range response.PresentationTable.Rows {
		if row["status"] == "Verified" || row["check"] == "Consumer Workflow" {
			t.Fatalf("healthy fact leaked into default findings: %#v", row)
		}
	}

	output := ansi.Strip(renderDoctorContract(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, WidthProvider: releasePlanOutputWidth{width: 80, available: true},
	}))
	for _, expected := range []string{"Release Integration Doctor", "Findings", "Workflow Missing", "Unauthorized"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("doctor default omitted %q:\n%s", expected, output)
		}
	}
	for _, hidden := range []string{"Healthy workflow contract", "Configured Units", "Verification Facts", "Complete Diagnostics"} {
		if strings.Contains(output, hidden) {
			t.Fatalf("doctor default exposed describe-only value %q:\n%s", hidden, output)
		}
	}
}

func TestDoctorDescribeShowsCompleteSafeStructuredInventory(t *testing.T) {
	result := doctorDescribeContractFixture()
	response := mapIntegrationDoctorResult(result, time.Time{})
	output := ansi.Strip(renderDoctorContract(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true,
		WidthProvider: releasePlanOutputWidth{width: 100, available: true},
	}))

	for _, expected := range []string{
		"Complete Diagnostics",
		"Verification Facts",
		"Configured Units",
		"Configured Workflows",
		"Limitations",
		"Healthy workflow contract",
		"consumer workflow",
		"Not Attempted",
		"Release workflow availability is intentionally deferred.",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("doctor describe omitted %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "/Users/example/private/repository") {
		t.Fatalf("doctor describe exposed an absolute local path:\n%s", output)
	}
}

func renderDoctorContract(t *testing.T, response *plugin.Response, options renderer.RenderOptions) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, options, &output); err != nil {
		t.Fatalf("render doctor contract: %v", err)
	}
	return output.String()
}

func TestDoctorPresentationDoesNotChangeMachineDataOrExitPolicy(t *testing.T) {
	result := doctorDescribeContractFixture()
	response := mapIntegrationDoctorResult(result, time.Time{})

	if response.ExitCode != 1 {
		t.Fatalf("doctor exit code = %d, want existing not-ready exit 1", response.ExitCode)
	}
	if got := response.Data["diagnostics"]; !reflect.DeepEqual(got, result.Diagnostics) {
		t.Fatalf("diagnostic machine data changed: %#v", got)
	}
	if got := response.Data["verifications"]; !reflect.DeepEqual(got, result.Verifications) {
		t.Fatalf("verification machine data changed: %#v", got)
	}
	if response.PresentationTable == nil || response.PresentationTable.Following == nil ||
		!response.PresentationTable.Following.DescribeOnly {
		t.Fatalf("complete diagnostic inventory is not describe-only: %#v", response.PresentationTable)
	}
}

func doctorDescribeContractFixture() *integrationDoctorResult {
	result := &integrationDoctorResult{
		RemoteVerification: integrationDoctorRemoteSummary{
			Status: integrationDoctorRemotePartial, Requested: true, Verified: 1, Unresolved: 1, Failed: 1,
		},
		Units: []integrationDoctorUnit{{
			ID: "cli", Version: "3.0.4", TagPrefix: "cli-v", Executor: "goreleaser",
			Delivery: "github-actions", Workflow: ".github/workflows/release-cli.yml",
			WorkingDirectory: "/Users/example/private/repository",
		}},
		Workflows: []integrationDoctorWorkflow{{
			Path: ".github/workflows/release-cli.yml", Units: []string{"cli"}, Classification: "consumer", Exists: true,
		}},
		Verifications: []integrationDoctorVerification{
			{
				Subject: "consumer workflow", Category: "workflow_contract", State: integrationDoctorVerified,
				Evidence: "Healthy workflow contract", Unit: "cli", Workflow: ".github/workflows/release-cli.yml",
			},
			{
				Subject: "release workflow", Category: "workflow_identity", State: integrationDoctorUnauthorized,
				Evidence: "GitHub did not authorize the exact workflow read.", Unit: "cli",
				Workflow: ".github/workflows/release-cli.yml", Remote: true,
			},
			{
				Subject: "future workflow run", Category: "workflow_run", State: integrationDoctorNotAttempted,
				Evidence:        "Release workflow availability is intentionally deferred.",
				LimitationClass: integrationDoctorRuntimeLimitation, Remote: true,
			},
		},
		Diagnostics: []integrationDoctorDiagnostic{
			{
				Severity: integrationDoctorError, Scope: "workflow", Unit: "cli",
				Workflow: ".github/workflows/release-cli.yml", Code: "WORKFLOW_MISSING",
				Message: "The configured release workflow is missing.", Remediation: "Create the canonical release workflow.",
			},
			{
				Severity: integrationDoctorWarning, Scope: "remote", Unit: "cli",
				Code: "REMOTE_AUTHORIZATION_UNRESOLVED", Message: "Remote authorization could not be verified.",
				Remediation: "Review repository Actions access.",
			},
		},
	}
	finalizeIntegrationDoctorResult(result)
	return result
}
