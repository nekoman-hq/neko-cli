package release

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestIntegrationDoctorColorlessPresentationBaseline(t *testing.T) {
	result := integrationDoctorResult{
		Diagnostics: []integrationDoctorDiagnostic{
			{
				Severity:    integrationDoctorError,
				Scope:       "source",
				Code:        "V2_CONFIG_INVALID",
				Message:     "Release V2 configuration is invalid.",
				Remediation: "Correct the local Release V2 configuration.",
			},
			{
				Severity:    integrationDoctorWarning,
				Scope:       "workflow",
				Unit:        "api",
				Workflow:    ".github/workflows/release-api.yml",
				Code:        "CONCURRENCY_MISSING",
				Message:     "The workflow has no explicit release concurrency policy.",
				Remediation: "Add a release concurrency group.",
			},
		},
	}
	finalizeIntegrationDoctorResult(&result)

	output := ansi.Strip(renderReleasePlanForTest(
		t,
		mapIntegrationDoctorResultForTest(result),
		renderer.FormatTable,
		releasePlanOutputWidth{width: 72, available: true},
	))
	want := `Release Integration Doctor

Readiness            not_ready
Errors               1
Warnings             1
Recommendations      0
Not verifiable       0
Locally verified     0
Inspected units      0
Inspected workflows  0

Diagnostics

Severity  Code                 Target                 Scope
────────────────────────────────────────────────────────────────
ERROR     V2_CONFIG_INVALID    source                 source
WARNING   CONCURRENCY_MISSING  api · release-api.yml  workflow

ERROR · V2_CONFIG_INVALID

Scope
  source

Message
  Release V2 configuration is invalid.

Remediation
  Correct the local Release V2 configuration.

────────────────────────────────

WARNING · CONCURRENCY_MISSING

Scope
  workflow

Unit
  api

Workflow
  .github/workflows/release-api.yml

Message
  The workflow has no explicit release concurrency policy.

Remediation
  Add a release concurrency group.`
	if got := trimDoctorBaselineLinePadding(output); got != want {
		t.Fatalf("colorless Doctor presentation changed:\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func trimDoctorBaselineLinePadding(output string) string {
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	for index := range lines {
		lines[index] = strings.TrimRight(lines[index], " ")
	}
	return strings.Join(lines, "\n")
}
