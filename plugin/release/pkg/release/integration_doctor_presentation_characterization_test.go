package release

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestIntegrationDoctorHighVolumePresentationFixturePreservesMachineContract(t *testing.T) {
	result := integrationDoctorHighVolumePresentationFixture()
	response := mapIntegrationDoctorResult(&result, time.Date(2026, time.July, 19, 9, 30, 0, 0, time.UTC))

	if response.Status != "success" || response.ExitCode != 1 {
		t.Fatalf("response status=%q exit=%d, want success/1", response.Status, response.ExitCode)
	}
	if got := integrationDoctorResultFromResponse(t, response); !reflect.DeepEqual(got, result) {
		t.Fatalf("mapped result changed:\ngot  %#v\nwant %#v", got, result)
	}

	wantData := integrationDoctorDataForTest(t, response.Data)
	assertIntegrationDoctorRenderedDataForTest(t, response, renderer.FormatJSON, wantData)

	rawResponse := *response
	rawResponse.RendererHint = "raw-json"
	assertIntegrationDoctorRenderedDataForTest(t, &rawResponse, renderer.FormatTable, wantData)
}

func TestIntegrationDoctorHighVolumePresentationFixtureCharacterizesReadinessAndOrdering(t *testing.T) {
	result := integrationDoctorHighVolumePresentationFixture()
	if result.Readiness != integrationDoctorNotReady {
		t.Fatalf("readiness = %q, want not_ready", result.Readiness)
	}
	if result.Summary != (integrationDoctorSummary{Errors: 3, Warnings: 2, NotVerifiable: 4}) {
		t.Fatalf("summary = %#v", result.Summary)
	}

	wantCodes := []string{
		"V2_RECOVERY_BLOCKED",
		"CHECKOUT_REF_INVALID",
		"CONTEXT_OUTPUT_FILE_INVALID",
		"PERMISSIONS_BROAD",
		"CONCURRENCY_MISSING",
		"REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE",
		"REMOTE_WORKFLOW_NOT_VERIFIABLE",
		"CONSUMER_BUILD_NOT_VERIFIABLE",
		"PUBLICATION_TARGET_NOT_VERIFIABLE",
	}
	gotCodes := make([]string, 0, len(result.Diagnostics))
	for _, diagnostic := range result.Diagnostics {
		gotCodes = append(gotCodes, diagnostic.Code)
	}
	if !reflect.DeepEqual(gotCodes, wantCodes) {
		t.Fatalf("diagnostic order = %#v, want %#v", gotCodes, wantCodes)
	}
}

func integrationDoctorHighVolumePresentationFixture() integrationDoctorResult {
	const (
		apiWorkflow = ".github/workflows/release-api-production-with-a-deliberately-long-name.yml"
		webWorkflow = ".github/workflows/release-web-production-with-a-deliberately-long-name.yml"
	)
	result := integrationDoctorResult{
		Verifications: []integrationDoctorVerification{},
		Units: []integrationDoctorUnit{
			{ID: "api", Version: "2.4.1", TagPrefix: "api-v", Executor: "goreleaser", Delivery: "github-actions", Workflow: apiWorkflow, WorkingDirectory: "services/api"},
			{ID: "web", Version: "3.8.2", TagPrefix: "web-v", Executor: "release-it", Delivery: "github-actions", Workflow: webWorkflow, WorkingDirectory: "apps/web"},
		},
		Workflows: []integrationDoctorWorkflow{
			{Path: apiWorkflow, Units: []string{"api"}, Classification: "custom", Exists: true},
			{Path: webWorkflow, Units: []string{"web"}, Classification: "custom", Exists: true},
		},
		Diagnostics: []integrationDoctorDiagnostic{
			{Severity: integrationDoctorNotVerifiable, Scope: "workflow", Unit: "web", Workflow: webWorkflow, Code: "PUBLICATION_TARGET_NOT_VERIFIABLE", Message: "The publication target cannot be proven by local structural inspection.", Remediation: "Verify the remote publication target, artifact ownership, and accepted release policy in the consumer repository."},
			{Severity: integrationDoctorWarning, Scope: "workflow", Unit: "web", Workflow: webWorkflow, Code: "CONCURRENCY_MISSING", Message: "The workflow has no explicit release concurrency policy.", Remediation: "Add a group containing unit and tag identity with cancel-in-progress: false."},
			{Severity: integrationDoctorError, Scope: "workflow", Unit: "api", Workflow: apiWorkflow, Code: "CONTEXT_OUTPUT_FILE_INVALID", Message: "The release context validator does not write its complete result to the GitHub output file.", Remediation: "Pass --github-output-file \"$GITHUB_OUTPUT\" so downstream release steps consume the validated unit, version, tag, and commit identity."},
			{Severity: integrationDoctorNotVerifiable, Scope: "remote", Code: "REMOTE_WORKFLOW_NOT_VERIFIABLE", Message: "The workflow content on the remote default branch is not verified by this local inspection.", Remediation: "Review the remote workflow at the repository-relative configured path before dispatching a release."},
			{Severity: integrationDoctorError, Scope: "source", Code: "V2_RECOVERY_BLOCKED", Message: "Release V2 source facts are blocked by unresolved pair-recovery evidence.", Remediation: "Resolve the existing local pair-recovery evidence with the owning Release V2 command before relying on integration readiness."},
			{Severity: integrationDoctorWarning, Scope: "workflow", Unit: "api", Workflow: apiWorkflow, Code: "PERMISSIONS_BROAD", Message: "The workflow grants one or more write permissions.", Remediation: "Keep contents read-only by default and grant only the consumer step's required permissions."},
			{Severity: integrationDoctorNotVerifiable, Scope: "workflow", Unit: "api", Workflow: apiWorkflow, Code: "CONSUMER_BUILD_NOT_VERIFIABLE", Message: "Custom consumer build and publication commands cannot be proven correct by local structural inspection.", Remediation: "Review the consumer commands, package credentials, artifact identity, and unit scoping in repository policy."},
			{Severity: integrationDoctorError, Scope: "workflow", Unit: "api", Workflow: apiWorkflow, Code: "CHECKOUT_REF_INVALID", Message: "Checkout does not use the dispatched release commit identity.", Remediation: "Set checkout ref to inputs.release_sha or github.event.inputs.release_sha and keep the full repository history available."},
			{Severity: integrationDoctorNotVerifiable, Scope: "remote", Code: "REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE", Message: "Remote workflow dispatch authorization cannot be verified locally.", Remediation: "Verify repository Actions permissions and the dispatching token's workflow authorization before release handoff."},
		},
	}
	finalizeIntegrationDoctorResult(&result)
	return result
}

func mapIntegrationDoctorResultForTest(result integrationDoctorResult) *plugin.Response {
	return mapIntegrationDoctorResult(&result, time.Date(2026, time.July, 19, 9, 30, 0, 0, time.UTC))
}

func integrationDoctorDataForTest(t *testing.T, data map[string]any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal Doctor data: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode Doctor data: %v", err)
	}
	return decoded
}

func assertIntegrationDoctorRenderedDataForTest(
	t *testing.T,
	response *plugin.Response,
	format renderer.OutputFormat,
	want map[string]any,
) {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{Format: format}, &output); err != nil {
		t.Fatalf("render %s: %v", format, err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode rendered %s: %v\n%s", format, err, output.String())
	}
	if format == renderer.FormatJSON {
		value, ok := decoded["data"].(map[string]any)
		if !ok {
			t.Fatalf("public JSON data type = %T", decoded["data"])
		}
		decoded = value
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("rendered %s data changed:\ngot  %#v\nwant %#v", format, decoded, want)
	}
}
