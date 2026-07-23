package release

import (
	"bytes"
	"encoding/json"
	"reflect"
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
	if result.Summary != (integrationDoctorSummary{NotVerifiable: 21, Verified: 15}) {
		t.Fatalf("summary = %#v", result.Summary)
	}
	if len(result.Units) != 3 || len(result.Workflows) != 3 || len(result.Verifications) != 24 || len(result.Diagnostics) != 21 {
		t.Fatalf("units=%d workflows=%d facts=%d diagnostics=%d", len(result.Units), len(result.Workflows), len(result.Verifications), len(result.Diagnostics))
	}
	for _, workflow := range result.Workflows {
		if !workflow.Exists || workflow.Classification != "configured_custom" {
			t.Errorf("workflow = %#v", workflow)
		}
	}
}

func TestRepositoryDoctorUnitScopedDogfoodContract(t *testing.T) {
	wantCodes := []string{
		"CONSUMER_BUILD_NOT_VERIFIABLE",
		"INSTALLATION_ARTIFACTS_NOT_VERIFIABLE",
		"PUBLICATION_CREDENTIALS_NOT_VERIFIABLE",
		"PUBLICATION_TARGET_NOT_VERIFIABLE",
		"REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE",
		"REMOTE_WORKFLOW_NOT_VERIFIABLE",
		"REPOSITORY_VARIABLES_NOT_VERIFIABLE",
	}
	for _, unit := range []string{"cli", "plugin-release", "plugin-ui"} {
		t.Run(unit, func(t *testing.T) {
			response := runIntegrationDoctor(t, repositoryInspectionRoot(t), map[string]any{"unit": unit})
			result := integrationDoctorResultFromResponse(t, response)
			if result.Readiness != integrationDoctorReady || response.ExitCode != 0 ||
				result.Summary != (integrationDoctorSummary{NotVerifiable: 7, Verified: 5}) {
				t.Fatalf("readiness=%q exit=%d summary=%#v", result.Readiness, response.ExitCode, result.Summary)
			}
			if len(result.Units) != 1 || result.Units[0].ID != unit || len(result.Workflows) != 1 ||
				len(result.Verifications) != 8 || len(result.Diagnostics) != 7 {
				t.Fatalf("unit=%#v workflows=%#v facts=%d diagnostics=%#v", result.Units, result.Workflows, len(result.Verifications), result.Diagnostics)
			}
			gotCodes := make([]string, 0, len(result.Diagnostics))
			for _, diagnostic := range result.Diagnostics {
				gotCodes = append(gotCodes, diagnostic.Code)
			}
			if !reflect.DeepEqual(gotCodes, wantCodes) {
				t.Fatalf("codes=%v want=%v", gotCodes, wantCodes)
			}
		})
	}
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
		"Readiness", "ready", "Locally verified", "15", "Not verifiable", "21",
		"locally verified", "local workflow was inspected", "locally identified",
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
