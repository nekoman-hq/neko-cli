package release

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestRepositoryDoctorDogfoodContract(t *testing.T) {
	response := runIntegrationDoctor(t, repositoryInspectionRoot(t), nil)
	result := integrationDoctorResultFromResponse(t, response)
	if result.Readiness != integrationDoctorReady || response.ExitCode != 0 {
		t.Fatalf("readiness=%q exit=%d", result.Readiness, response.ExitCode)
	}
	if result.Summary != (integrationDoctorSummary{NotVerifiable: 19, Verified: 15}) {
		t.Fatalf("summary = %#v", result.Summary)
	}
	if len(result.Units) != 3 || len(result.Workflows) != 3 || len(result.Verifications) != 23 || len(result.Diagnostics) != 19 {
		t.Fatalf("units=%d workflows=%d facts=%d diagnostics=%d", len(result.Units), len(result.Workflows), len(result.Verifications), len(result.Diagnostics))
	}
	for _, workflow := range result.Workflows {
		if !workflow.Exists || workflow.Classification != "configured_custom" {
			t.Errorf("workflow = %#v", workflow)
		}
	}
	assertRepositoryDoctorSemantics(t, result, "cli", true)
	assertRepositoryDoctorSemantics(t, result, "plugin-ui", true)
	assertRepositoryDoctorSemantics(t, result, "plugin-release", false)
	assertRepositoryDoctorSourceToolchain(t, result)
}

func TestRepositoryDoctorUnitScopedDogfoodContract(t *testing.T) {
	standardCodes := []string{
		"CONSUMER_BUILD_NOT_VERIFIABLE",
		"INSTALLATION_ARTIFACTS_NOT_VERIFIABLE",
		"PUBLICATION_CREDENTIALS_NOT_VERIFIABLE",
		"PUBLICATION_TARGET_NOT_VERIFIABLE",
		"REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE",
		"REMOTE_WORKFLOW_NOT_VERIFIABLE",
		"REPOSITORY_VARIABLES_NOT_VERIFIABLE",
	}
	tests := []struct {
		unit            string
		codes           []string
		summary         integrationDoctorSummary
		facts           int
		hasVariableFact bool
	}{
		{unit: "cli", summary: integrationDoctorSummary{NotVerifiable: 7, Verified: 5}, facts: 8, codes: standardCodes, hasVariableFact: true},
		{unit: "plugin-release", summary: integrationDoctorSummary{NotVerifiable: 5, Verified: 5}, facts: 7, codes: []string{
			"CONSUMER_BUILD_NOT_VERIFIABLE",
			"PUBLICATION_CREDENTIALS_NOT_VERIFIABLE",
			"PUBLICATION_TARGET_NOT_VERIFIABLE",
			"REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE",
			"REMOTE_WORKFLOW_NOT_VERIFIABLE",
		}, hasVariableFact: false},
		{unit: "plugin-ui", summary: integrationDoctorSummary{NotVerifiable: 7, Verified: 5}, facts: 8, codes: standardCodes, hasVariableFact: true},
	}
	for _, test := range tests {
		t.Run(test.unit, func(t *testing.T) {
			response := runIntegrationDoctor(t, repositoryInspectionRoot(t), map[string]any{"unit": test.unit})
			result := integrationDoctorResultFromResponse(t, response)
			if result.Readiness != integrationDoctorReady || response.ExitCode != 0 ||
				result.Summary != test.summary {
				t.Fatalf("readiness=%q exit=%d summary=%#v", result.Readiness, response.ExitCode, result.Summary)
			}
			if len(result.Units) != 1 || result.Units[0].ID != test.unit || len(result.Workflows) != 1 ||
				len(result.Verifications) != test.facts || len(result.Diagnostics) != len(test.codes) {
				t.Fatalf("unit=%#v workflows=%#v facts=%d diagnostics=%#v", result.Units, result.Workflows, len(result.Verifications), result.Diagnostics)
			}
			gotCodes := make([]string, 0, len(result.Diagnostics))
			for _, diagnostic := range result.Diagnostics {
				gotCodes = append(gotCodes, diagnostic.Code)
			}
			if !reflect.DeepEqual(gotCodes, test.codes) {
				t.Fatalf("codes=%v want=%v", gotCodes, test.codes)
			}
			assertRepositoryDoctorSemantics(t, result, test.unit, test.hasVariableFact)
			if test.unit == "plugin-release" {
				assertRepositoryDoctorSourceToolchain(t, result)
				assertIntegrationDoctorCodeAbsent(t, result.Diagnostics, "INSTALLATION_ARTIFACTS_NOT_VERIFIABLE")
				assertIntegrationDoctorCodeAbsent(t, result.Diagnostics, "REPOSITORY_VARIABLES_NOT_VERIFIABLE")
			}
		})
	}
}

func assertRepositoryDoctorSemantics(
	t *testing.T,
	result integrationDoctorResult,
	unit string,
	hasVariableFact bool,
) {
	t.Helper()
	workflow := ""
	for _, candidate := range result.Workflows {
		for _, inspectedUnit := range candidate.Units {
			if inspectedUnit == unit {
				workflow = candidate.Path
			}
		}
	}
	if workflow == "" {
		t.Fatalf("workflow for unit %q is missing from %#v", unit, result.Workflows)
	}

	verified := make([]string, 0, 5)
	limitations := map[string]integrationDoctorVerification{}
	for _, fact := range result.Verifications {
		if fact.Workflow != workflow {
			continue
		}
		if fact.Subject == "" || fact.Category == "" || fact.State == "" || fact.Evidence == "" || len(fact.References) == 0 {
			t.Fatalf("incomplete Doctor fact identity/evidence for %s: %#v", unit, fact)
		}
		switch fact.State {
		case integrationDoctorVerified:
			verified = append(verified, fact.Category)
		case integrationDoctorUnverifiable:
			if fact.LimitationClass == "" {
				t.Fatalf("Doctor limitation fact omits class: %#v", fact)
			}
			limitations[fact.Category] = fact
		}
	}
	wantVerified := []string{
		"consumer_structure", "credential_wiring", "goreleaser_configuration", "installation_wiring", "publication_identity",
	}
	sort.Strings(verified)
	sort.Strings(wantVerified)
	if !reflect.DeepEqual(verified, wantVerified) {
		t.Fatalf("verified categories for %s = %v, want %v", unit, verified, wantVerified)
	}
	for _, category := range []string{"dispatch_authorization", "remote_workflow_identity"} {
		if _, ok := limitations[category]; !ok {
			t.Fatalf("Doctor limitation %q missing for %s: %#v", category, unit, limitations)
		}
	}
	_, variableFact := limitations["repository_variable_values"]
	if variableFact != hasVariableFact {
		t.Fatalf("repository-variable limitation for %s present=%t, want %t: %#v", unit, variableFact, hasVariableFact, limitations)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Workflow != workflow {
			continue
		}
		if diagnostic.Severity != integrationDoctorNotVerifiable || diagnostic.Scope == "" ||
			diagnostic.Code == "" || diagnostic.Message == "" || diagnostic.Remediation == "" {
			t.Fatalf("incomplete actionable Doctor diagnostic for %s: %#v", unit, diagnostic)
		}
	}
}

func assertRepositoryDoctorSourceToolchain(t *testing.T, result integrationDoctorResult) {
	t.Helper()
	for _, fact := range result.Verifications {
		if fact.Unit != "plugin-release" || fact.Category != "installation_wiring" {
			continue
		}
		if fact.State != integrationDoctorVerified ||
			!strings.Contains(fact.Evidence, "builds and wires the CLI and candidate plugin from the exact checked-out commit") ||
			!reflect.DeepEqual(fact.References, []string{
				".github/workflows/release-plugin-release.yml", "go.mod", "plugin/release/main.go", "plugin/release/manifest.json",
			}) {
			t.Fatalf("Release Plugin exact-source validation fact = %#v", fact)
		}
		return
	}
	t.Fatal("Release Plugin exact-source installation_wiring fact is missing")
}

func TestRepositoryDoctorMachineOutputsContainTypedLocalEvidenceOnly(t *testing.T) {
	const secretValue = "dogfood-secret-value-must-not-appear"
	t.Setenv("GITHUB_TOKEN", secretValue)
	root := repositoryInspectionRoot(t)
	response := runIntegrationDoctor(t, root, nil)

	var publicJSON bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatJSON, &publicJSON); err != nil {
		t.Fatal(err)
	}
	rawResponse := *response
	rawResponse.RendererHint = "raw-json"
	var rawJSON bytes.Buffer
	if err := renderer.RenderWithOptionsTo(&rawResponse, renderer.RenderOptions{Format: renderer.FormatTable}, &rawJSON); err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string]string{"json": publicJSON.String(), "raw-json": rawJSON.String()} {
		if !json.Valid([]byte(output)) {
			t.Errorf("%s is invalid JSON", name)
		}
		for _, forbidden := range []string{"\x1b[", secretValue, root.Path(), `"verifications": null`, `"diagnostics": null`} {
			if strings.Contains(output, forbidden) {
				t.Errorf("%s contains forbidden value %q", name, forbidden)
			}
		}
		for _, required := range []string{`"verifications"`, `"limitation_class"`, `"mutation_required"`, `"verified"`} {
			if !strings.Contains(output, required) {
				t.Errorf("%s omits %q", name, required)
			}
		}
	}
}

func TestRepositoryDoctorHumanOutputJustifiesNarrowedLimitations(t *testing.T) {
	response := runIntegrationDoctor(t, repositoryInspectionRoot(t), nil)
	var rendered bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true,
		WidthProvider: releasePlanOutputWidth{width: 140, available: true},
	}, &rendered); err != nil {
		t.Fatalf("render described Doctor output: %v", err)
	}
	output := ansi.Strip(rendered.String())
	for _, required := range []string{
		"Readiness", "ready", "Locally verified", "15", "Not verifiable", "19",
		"Remote workflow not verifiable",
		"The local workflow was inspected",
		"Remote dispatch authorization not verifiable",
		"only a real remote authorization decision can prove",
		"Verify authorization in a separately approved remote operation",
	} {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(required)) {
			t.Errorf("human output omits %q", required)
		}
	}
	if strings.Contains(output, "\x1b[") {
		t.Fatal("redirected-style human output contains ANSI")
	}
}

func TestIntegrationDoctorKeepsRepositoryRootsIsolatedInOneProcess(t *testing.T) {
	first := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release-api.yml"})
	second := newIntegrationDoctorRepository(t, map[string]string{"web": ".github/workflows/release-web.yml"})
	writeIntegrationDoctorWorkflow(t, first, ".github/workflows/release-api.yml", customIntegrationDoctorWorkflow(t))
	writeIntegrationDoctorWorkflow(t, second, ".github/workflows/release-web.yml", customIntegrationDoctorWorkflow(t))

	firstResult := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, first, nil))
	secondResult := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, second, nil))
	if len(firstResult.Units) != 1 || firstResult.Units[0].ID != "api" ||
		len(secondResult.Units) != 1 || secondResult.Units[0].ID != "web" {
		t.Fatalf("first=%#v second=%#v", firstResult.Units, secondResult.Units)
	}
	if firstResult.Workflows[0].Path == secondResult.Workflows[0].Path {
		t.Fatal("repository-local workflow identities were mixed")
	}
}
