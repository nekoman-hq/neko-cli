package evidence

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestEvidenceDefaultIsConciseAndKeepsEveryActionableFinding(t *testing.T) {
	result := evidencePresentationFixture()
	response := mapEvidenceQueryResponse(result, evidencePresentationTime())
	output := renderEvidencePresentation(t, response, renderer.RenderOptions{
		Format:        renderer.FormatTable,
		WidthProvider: evidenceOutputWidth(96),
	})

	for _, want := range []string{
		"Evidence Summary",
		"Evidence Inventory",
		FamilyReleaseExecution,
		strings.Repeat("a", 64),
		ClassificationResumable,
		"Existing resume policy can continue after local checks.",
		FamilyDispatch,
		ClassificationTerminal,
		"GitHub rejected the workflow dispatch request.",
		"recovery is required.",
		ClassificationCorrupt,
		"Evidence could not be decoded safely. Preserve the file and inspect manually.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("Evidence default omitted %q:\n%s", want, output)
		}
	}
	for _, hidden := range []string{
		"Execution Evidence",
		"Dispatch Evidence",
		"Linkage",
		"Local Git Evidence",
		"Recovery Relevance",
		"Digest SHA-256",
		"/private/tmp/evidence-contract",
	} {
		if strings.Contains(output, hidden) {
			t.Fatalf("Evidence default exposed describe-only or unsafe value %q:\n%s", hidden, output)
		}
	}
}

func TestEvidenceDescribeShowsCompleteSafeInventory(t *testing.T) {
	response := mapEvidenceQueryResponse(evidencePresentationFixture(), evidencePresentationTime())
	output := renderEvidencePresentation(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true,
		WidthProvider: evidenceOutputWidth(96),
	})

	for _, want := range []string{
		"Evidence Summary",
		"Evidence Inventory",
		"Execution Evidence",
		"Dispatch Evidence",
		"Linkage",
		"Local Git Evidence",
		"Classification",
		"Recovery Relevance",
		"Limitations",
		"Digest SHA-256",
		".git/neko/release/executions/",
		".git/neko/release/dispatches/",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("Evidence describe omitted %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "/private/tmp/evidence-contract") {
		t.Fatalf("Evidence describe exposed fixture root:\n%s", output)
	}
}

func TestEvidenceVerboseIsIntentionalNoOpAndJSONIsInvariant(t *testing.T) {
	response := mapEvidenceQueryResponse(evidencePresentationFixture(), evidencePresentationTime())
	defaultOutput := renderEvidencePresentation(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, WidthProvider: evidenceOutputWidth(96),
	})
	verboseOutput := renderEvidencePresentation(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, Verbose: true, WidthProvider: evidenceOutputWidth(96),
	})
	if verboseOutput != defaultOutput {
		t.Fatalf("Evidence verbose changed deterministic human output:\ndefault:\n%s\nverbose:\n%s", defaultOutput, verboseOutput)
	}

	plainJSON := renderEvidencePresentation(t, response, renderer.RenderOptions{Format: renderer.FormatJSON})
	for _, options := range []renderer.RenderOptions{
		{Format: renderer.FormatJSON, Describe: true},
		{Format: renderer.FormatJSON, Verbose: true},
		{Format: renderer.FormatJSON, Describe: true, Verbose: true},
	} {
		if output := renderEvidencePresentation(t, response, options); output != plainJSON {
			t.Fatalf("Evidence JSON changed with presentation mode:\nplain:\n%s\nmode:\n%s", plainJSON, output)
		}
	}
}

func TestEvidencePresentationUsesEssentialResponsiveColumns(t *testing.T) {
	response := mapEvidenceQueryResponse(evidencePresentationFixture(), evidencePresentationTime())
	narrow := renderEvidencePresentation(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, WidthProvider: evidenceOutputWidth(34),
	})
	for _, want := range []string{"Family:", "Identity:", "State:", "Classification:", "Action:"} {
		if !strings.Contains(narrow, want) {
			t.Fatalf("narrow Evidence output omitted %q:\n%s", want, narrow)
		}
	}
	if strings.Contains(narrow, "/private/tmp/evidence-contract") || strings.Contains(narrow, "\x1b[") {
		t.Fatalf("narrow Evidence output is unsafe:\n%s", narrow)
	}

	options := renderer.RenderOptions{Format: renderer.FormatTable, Describe: true}
	first := renderEvidencePresentation(t, response, options)
	second := renderEvidencePresentation(t, response, options)
	if first != second || !strings.Contains(first, "Record 1") {
		t.Fatalf("unknown-width Evidence output is not deterministic vertical output:\n%s", first)
	}
}

func TestEvidenceArchiveDefaultDescribeAndJSONContracts(t *testing.T) {
	result := evidenceArchiveResult{
		Family:       FamilyReleaseExecution,
		Identity:     strings.Repeat("a", 64),
		DigestSHA256: strings.Repeat("b", 64),
		SourcePath:   "/private/tmp/evidence-contract/.git/neko/release/executions/source.json",
		ArchivePath:  "/private/tmp/evidence-contract/.git/neko/release/executions/archived/target.json",
	}
	response := mapEvidenceArchiveResponse(result, evidencePresentationTime())
	defaultOutput := renderEvidencePresentation(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, WidthProvider: evidenceOutputWidth(88),
	})
	for _, want := range []string{
		"Evidence Archive Result",
		FamilyReleaseExecution,
		result.Identity,
		"Confirmed",
		"Matched",
		"Archived",
		".git/neko/release/executions/source.json",
		".git/neko/release/executions/archived/target.json",
	} {
		if !strings.Contains(defaultOutput, want) {
			t.Fatalf("Archive default omitted %q:\n%s", want, defaultOutput)
		}
	}
	for _, hidden := range []string{"Archive Validation", "Guarded Write Plan", "Limitations"} {
		if strings.Contains(defaultOutput, hidden) {
			t.Fatalf("Archive default exposed describe-only %q:\n%s", hidden, defaultOutput)
		}
	}
	if strings.Contains(defaultOutput, "/private/tmp/evidence-contract") {
		t.Fatalf("Archive default exposed fixture root:\n%s", defaultOutput)
	}

	describeOutput := renderEvidencePresentation(t, response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: true, WidthProvider: evidenceOutputWidth(88),
	})
	for _, want := range []string{"Archive Validation", "Guarded Write Plan", "Limitations"} {
		if !strings.Contains(describeOutput, want) {
			t.Fatalf("Archive describe omitted %q:\n%s", want, describeOutput)
		}
	}

	plainJSON := renderEvidencePresentation(t, response, renderer.RenderOptions{Format: renderer.FormatJSON})
	for _, options := range []renderer.RenderOptions{
		{Format: renderer.FormatJSON, Describe: true},
		{Format: renderer.FormatJSON, Verbose: true},
		{Format: renderer.FormatJSON, Describe: true, Verbose: true},
	} {
		if output := renderEvidencePresentation(t, response, options); output != plainJSON {
			t.Fatalf("Archive JSON changed with presentation mode:\nplain:\n%s\nmode:\n%s", plainJSON, output)
		}
	}
}

func evidencePresentationFixture() evidenceQueryResult {
	return evidenceQueryResult{
		Records: []EvidenceRecord{
			{
				Family: FamilyDispatch, Identity: strings.Repeat("b", 64), Owner: "dispatch journal",
				Unit: "api", Version: "1.2.4", Tag: "api/v1.2.4", State: "rejected",
				Classification: ClassificationTerminal, ManualRecovery: true,
				Guidance:     "GitHub rejected the workflow dispatch request. Manual recovery is required.",
				Path:         "/private/tmp/evidence-contract/.git/neko/release/dispatches/" + strings.Repeat("b", 64) + ".json",
				DigestSHA256: strings.Repeat("d", 64),
			},
			{
				Family: FamilyReleaseExecution, Identity: strings.Repeat("a", 64), Owner: "release execution journal",
				Unit: "api", Version: "1.2.4", Tag: "api/v1.2.4", State: "commit-created",
				PendingAction: "none", Classification: ClassificationResumable,
				SafeToResume: true, AutomaticContinuation: true,
				Guidance:     "Existing resume policy can continue after local checks.",
				Path:         "/private/tmp/evidence-contract/.git/neko/release/executions/" + strings.Repeat("a", 64) + ".json",
				DigestSHA256: strings.Repeat("c", 64),
			},
		},
		Diagnostics: []EvidenceDiagnostic{{
			Family:         FamilyDispatch,
			Path:           "/private/tmp/evidence-contract/.git/neko/release/dispatches/" + strings.Repeat("e", 64) + ".json",
			Classification: ClassificationCorrupt,
			Code:           "corrupt-json",
			Guidance:       "Evidence could not be decoded safely. Preserve the file and inspect manually.",
		}},
	}
}

func evidencePresentationTime() time.Time {
	return time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
}

func renderEvidencePresentation(t *testing.T, response *plugin.Response, options renderer.RenderOptions) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, options, &output); err != nil {
		t.Fatalf("RenderWithOptionsTo: %v", err)
	}
	return ansi.Strip(output.String())
}
