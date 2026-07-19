package evidence

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

func TestEvidenceQueryResponsePreservesUnfilteredJSONContract(t *testing.T) {
	timestamp := time.Date(2026, time.July, 18, 12, 30, 0, 0, time.UTC)
	result := evidenceQueryResult{
		Records: []EvidenceRecord{{
			CreatedAt:             "2026-07-18T12:00:00Z",
			UpdatedAt:             "2026-07-18T12:15:00Z",
			Family:                FamilyDispatch,
			Identity:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Owner:                 "github-actions dispatch",
			Unit:                  "api",
			Version:               "1.2.4",
			Tag:                   "api/v1.2.4",
			State:                 "accepted",
			PendingAction:         "none",
			Classification:        ClassificationCompleted,
			SafeToResume:          false,
			AutomaticContinuation: true,
			ManualRecovery:        false,
			LifecycleAllowed:      false,
			LifecycleOperation:    "inspect-only",
			Guidance:              "GitHub accepted the dispatch.",
			Path:                  "/repo/.git/neko/release/dispatch/aaaaaaaa.json",
			DigestSHA256:          "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}},
		Diagnostics: []EvidenceDiagnostic{{
			Family:         FamilyMigration,
			Path:           "/repo/.neko/release.migration.json",
			Classification: ClassificationCorrupt,
			Code:           "invalid-json",
			Guidance:       "Preserve the file and inspect it manually.",
		}},
	}

	response := mapEvidenceQueryResponse(result, timestamp)
	if response.Status != "success" || response.RendererHint != "table" {
		t.Fatalf("unexpected response envelope: %#v", response)
	}
	if response.Metadata.Plugin != "release" || response.Metadata.Version != "dev" || response.Metadata.Command != CommandName || !response.Metadata.Timestamp.Equal(timestamp) {
		t.Fatalf("unexpected response metadata: %#v", response.Metadata)
	}

	items, ok := response.Data["items"].([]map[string]any)
	if !ok || len(items) != 2 {
		t.Fatalf("items = %#v, want record followed by diagnostic", response.Data["items"])
	}
	for _, key := range []string{"safe_to_resume", "automatic_continuation", "manual_recovery"} {
		if _, itemOK := items[0][key].(string); !itemOK {
			t.Fatalf("items[0][%q] type = %T, want string", key, items[0][key])
		}
	}
	records, ok := response.Data["evidence"].([]EvidenceRecord)
	if !ok || len(records) != 1 || records[0].SafeToResume || !records[0].AutomaticContinuation || records[0].ManualRecovery {
		t.Fatalf("typed Evidence records changed: %#v", response.Data["evidence"])
	}
	diagnostics, ok := response.Data["diagnostics"].([]EvidenceDiagnostic)
	if !ok || len(diagnostics) != 1 || diagnostics[0].Code != "invalid-json" {
		t.Fatalf("typed diagnostics changed: %#v", response.Data["diagnostics"])
	}

	var output bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatJSON, &output); err != nil {
		t.Fatalf("RenderTo: %v", err)
	}
	want := "{\n" +
		"  \"status\": \"success\",\n" +
		"  \"metadata\": {\n" +
		"    \"timestamp\": \"2026-07-18T12:30:00Z\",\n" +
		"    \"plugin\": \"release\",\n" +
		"    \"version\": \"dev\",\n" +
		"    \"command\": \"evidence\"\n" +
		"  },\n" +
		"  \"data\": {\n" +
		"    \"diagnostics\": [\n" +
		"      {\n" +
		"        \"Family\": \"migration\",\n" +
		"        \"Path\": \"/repo/.neko/release.migration.json\",\n" +
		"        \"Classification\": \"corrupt\",\n" +
		"        \"Code\": \"invalid-json\",\n" +
		"        \"Guidance\": \"Preserve the file and inspect it manually.\"\n" +
		"      }\n" +
		"    ],\n" +
		"    \"evidence\": [\n" +
		"      {\n" +
		"        \"CreatedAt\": \"2026-07-18T12:00:00Z\",\n" +
		"        \"UpdatedAt\": \"2026-07-18T12:15:00Z\",\n" +
		"        \"Family\": \"dispatch\",\n" +
		"        \"Identity\": \"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\",\n" +
		"        \"Owner\": \"github-actions dispatch\",\n" +
		"        \"Unit\": \"api\",\n" +
		"        \"Version\": \"1.2.4\",\n" +
		"        \"Tag\": \"api/v1.2.4\",\n" +
		"        \"State\": \"accepted\",\n" +
		"        \"PendingAction\": \"none\",\n" +
		"        \"Classification\": \"completed\",\n" +
		"        \"SafeToResume\": false,\n" +
		"        \"AutomaticContinuation\": true,\n" +
		"        \"ManualRecovery\": false,\n" +
		"        \"LifecycleAllowed\": false,\n" +
		"        \"LifecycleOperation\": \"inspect-only\",\n" +
		"        \"Guidance\": \"GitHub accepted the dispatch.\",\n" +
		"        \"Path\": \"/repo/.git/neko/release/dispatch/aaaaaaaa.json\",\n" +
		"        \"DigestSHA256\": \"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\"\n" +
		"      }\n" +
		"    ],\n" +
		"    \"items\": [\n" +
		"      {\n" +
		"        \"automatic_continuation\": \"true\",\n" +
		"        \"classification\": \"completed\",\n" +
		"        \"digest\": \"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\",\n" +
		"        \"family\": \"dispatch\",\n" +
		"        \"guidance\": \"GitHub accepted the dispatch.\",\n" +
		"        \"identity\": \"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\",\n" +
		"        \"lifecycle\": \"blocked\",\n" +
		"        \"manual_recovery\": \"false\",\n" +
		"        \"owner\": \"github-actions dispatch\",\n" +
		"        \"path\": \"/repo/.git/neko/release/dispatch/aaaaaaaa.json\",\n" +
		"        \"pending_action\": \"none\",\n" +
		"        \"safe_to_resume\": \"false\",\n" +
		"        \"state\": \"accepted\",\n" +
		"        \"tag\": \"api/v1.2.4\",\n" +
		"        \"unit\": \"api\",\n" +
		"        \"version\": \"1.2.4\"\n" +
		"      },\n" +
		"      {\n" +
		"        \"automatic_continuation\": \"false\",\n" +
		"        \"classification\": \"corrupt\",\n" +
		"        \"digest\": \"not applicable\",\n" +
		"        \"family\": \"migration\",\n" +
		"        \"guidance\": \"Preserve the file and inspect it manually.\",\n" +
		"        \"identity\": \"diagnostic\",\n" +
		"        \"lifecycle\": \"blocked\",\n" +
		"        \"manual_recovery\": \"true\",\n" +
		"        \"owner\": \"evidence inspection\",\n" +
		"        \"path\": \"/repo/.neko/release.migration.json\",\n" +
		"        \"pending_action\": \"manual-inspection\",\n" +
		"        \"safe_to_resume\": \"false\",\n" +
		"        \"state\": \"invalid-json\",\n" +
		"        \"tag\": \"not applicable\",\n" +
		"        \"unit\": \"not applicable\",\n" +
		"        \"version\": \"not applicable\"\n" +
		"      }\n" +
		"    ]\n" +
		"  },\n" +
		"  \"renderer_hint\": \"table\"\n" +
		"}\n"
	if output.String() != want {
		t.Fatalf("unfiltered Evidence JSON changed:\nwant:\n%s\ngot:\n%s", want, output.String())
	}
}

func TestEvidenceHandlerReturnsMappedResponseWithoutGoError(t *testing.T) {
	timestamp := time.Date(2026, time.July, 18, 13, 0, 0, 0, time.UTC)
	runner := &recordingEvidenceQueryRunner{result: evidenceQueryResult{Records: []EvidenceRecord{{Family: FamilyDispatch}}}}
	handler := evidenceCommandHandler{query: runner, clock: fixedEvidenceResponseClock{timestamp: timestamp}}

	response, err := handler.Handle(plugin.Request{Context: plugin.Context{WorkingDir: "/repo"}})
	if err != nil {
		t.Fatalf("Handle returned Go error: %v", err)
	}
	if response == nil || response.Status != "success" || response.Metadata.Command != CommandName {
		t.Fatalf("unexpected response: %#v", response)
	}
	if runner.request.RepositoryRoot != "/repo" || runner.request.Family != "" || runner.request.Unit != "" {
		t.Fatalf("unfiltered request changed: %#v", runner.request)
	}
}

func TestEvidenceHandlerRoutesIdentityInspectionToCompleteDetail(t *testing.T) {
	identity := strings.Repeat("a", 64)
	runner := &recordingEvidenceQueryRunner{result: evidenceQueryResult{Records: []EvidenceRecord{{
		Family:   FamilyDispatch,
		Identity: identity,
	}}}}
	handler := evidenceCommandHandler{
		query: runner,
		clock: fixedEvidenceResponseClock{timestamp: time.Date(2026, time.July, 18, 13, 0, 0, 0, time.UTC)},
	}

	response, err := handler.Handle(plugin.Request{
		Flags:   map[string]any{"identity": identity[:8], "family": FamilyDispatch, "unit": "api"},
		Context: plugin.Context{WorkingDir: "/repo"},
	})
	if err != nil {
		t.Fatalf("Handle detail: %v", err)
	}
	if runner.request.IdentityPrefix != identity[:8] || runner.request.Family != FamilyDispatch || runner.request.Unit != "api" {
		t.Fatalf("detail query request changed: %#v", runner.request)
	}
	if response.PresentationTable != nil {
		t.Fatalf("detail response unexpectedly opted into summary table: %#v", response.PresentationTable)
	}
	if items, ok := response.Data["items"].([]map[string]any); !ok || len(items) == 0 || items[1]["value"] != identity {
		t.Fatalf("detail response omitted complete identity: %#v", response.Data["items"])
	}
}

func TestEvidenceHandlerReturnsNilResponseWithQueryError(t *testing.T) {
	wantErr := errors.New("evidence query failed")
	handler := evidenceCommandHandler{
		query: &recordingEvidenceQueryRunner{err: wantErr},
		clock: fixedEvidenceResponseClock{timestamp: time.Date(2026, time.July, 18, 13, 0, 0, 0, time.UTC)},
	}

	response, err := handler.Handle(plugin.Request{Context: plugin.Context{WorkingDir: "/repo"}})
	if response != nil || !errors.Is(err, wantErr) {
		t.Fatalf("Handle = (%#v, %v), want (nil, %v)", response, err, wantErr)
	}
}

func TestEvidenceSummaryDeclaresSemanticColumnPriority(t *testing.T) {
	response := mapEvidenceQueryResponse(evidenceQueryResult{Records: []EvidenceRecord{{Family: FamilyDispatch}}}, time.Now())
	want := []presentation.Column{
		{Key: "family", Label: "Family", Essential: true},
		{Key: "state", Label: "State", Essential: true},
		{Key: "classification", Label: "Classification", Essential: true},
		{Key: "safe_to_resume", Label: "Resume", Essential: true},
		{Key: "manual_recovery", Label: "Recovery", Essential: true},
		{Key: "unit", Label: "Unit"},
		{Key: "version", Label: "Version"},
		{Key: "tag", Label: "Tag"},
		{Key: "pending_action", Label: "Pending action"},
		{Key: "automatic_continuation", Label: "Automatic"},
		{Key: "lifecycle", Label: "Lifecycle"},
	}
	if response.PresentationTable == nil || !reflect.DeepEqual(response.PresentationTable.Columns, want) {
		t.Fatalf("human table columns = %#v, want %#v", response.PresentationTable, want)
	}
}

func TestEvidenceDetailResponseShowsEverySafeFieldAndCompleteTypedRecord(t *testing.T) {
	record := EvidenceRecord{
		CreatedAt:             "2026-07-18T12:00:00Z",
		UpdatedAt:             "2026-07-18T12:15:00Z",
		Family:                FamilyReleaseExecution,
		Identity:              strings.Repeat("a", 64),
		Owner:                 "release execution",
		Unit:                  "api",
		Version:               "1.2.4",
		Tag:                   "api/v1.2.4",
		State:                 "handoff-ready",
		PendingAction:         "none",
		Classification:        ClassificationCompleted,
		SafeToResume:          false,
		AutomaticContinuation: false,
		ManualRecovery:        false,
		LifecycleAllowed:      true,
		LifecycleOperation:    "archive-completed",
		Guidance:              "The handoff is complete.",
		Path:                  "/repo/.git/neko/release/executions/record.json",
		DigestSHA256:          strings.Repeat("b", 64),
	}
	result := evidenceQueryResult{
		Records: []EvidenceRecord{record},
		Diagnostics: []EvidenceDiagnostic{{
			Family:         FamilyReleaseExecution,
			Classification: ClassificationCorrupt,
			Code:           "example",
		}},
	}

	response := mapEvidenceDetailResponse(result, time.Date(2026, time.July, 18, 12, 30, 0, 0, time.UTC))
	if response.PresentationTable != nil || response.RendererHint != "table" {
		t.Fatalf("detail response should use existing property/value rendering: %#v", response)
	}
	items, ok := response.Data["items"].([]map[string]any)
	if !ok {
		t.Fatalf("detail items type = %T", response.Data["items"])
	}
	wantProperties := []string{
		"Family", "Identity", "Owner", "Unit", "Version", "Tag", "State", "Pending Action", "Classification",
		"Safe To Resume", "Automatic Continuation", "Manual Recovery", "Lifecycle Allowed", "Lifecycle Operation",
		"Guidance", "Path", "Digest SHA-256", "Created At", "Updated At",
	}
	if len(items) != len(wantProperties) {
		t.Fatalf("detail item count = %d, want %d", len(items), len(wantProperties))
	}
	for index, property := range wantProperties {
		if items[index]["property"] != property {
			t.Fatalf("detail property %d = %#v, want %q", index, items[index]["property"], property)
		}
	}
	typedRecords, recordsOK := response.Data["evidence"].([]EvidenceRecord)
	if !recordsOK || !reflect.DeepEqual(typedRecords, []EvidenceRecord{record}) {
		t.Fatalf("detail typed record changed: %#v", response.Data["evidence"])
	}
	typedDiagnostics, diagnosticsOK := response.Data["diagnostics"].([]EvidenceDiagnostic)
	if !diagnosticsOK || !reflect.DeepEqual(typedDiagnostics, result.Diagnostics) {
		t.Fatalf("detail diagnostics changed: %#v", response.Data["diagnostics"])
	}
}

func TestEvidenceSummaryRendersResponsiveLayoutsWithoutForensicColumns(t *testing.T) {
	response := mapEvidenceQueryResponse(evidenceQueryResult{Records: []EvidenceRecord{{
		Family:                FamilyDispatch,
		Identity:              strings.Repeat("a", 64),
		Owner:                 "github-actions dispatch",
		Unit:                  "api",
		Version:               "1.2.4",
		Tag:                   "api/v1.2.4",
		State:                 "accepted",
		PendingAction:         "none",
		Classification:        ClassificationCompleted,
		AutomaticContinuation: true,
		Path:                  "/repo/private/evidence.json",
		DigestSHA256:          strings.Repeat("b", 64),
		Guidance:              "Complete guidance remains available in detail.",
	}}}, time.Now())

	wide := renderEvidenceAtWidth(t, response, renderer.FormatWide, 240)
	wideHeader := strings.Split(ansi.Strip(wide), "\n")[0]
	for _, label := range []string{"Family", "State", "Classification", "Resume", "Recovery", "Unit", "Version", "Tag", "Pending action", "Automatic", "Lifecycle"} {
		if !strings.Contains(wideHeader, label) {
			t.Fatalf("wide Evidence header missing %q: %q", label, wideHeader)
		}
	}
	for _, forensic := range []string{"Identity", "Digest", "Owner", "Path", "Guidance", strings.Repeat("a", 16)} {
		if strings.Contains(ansi.Strip(wide), forensic) {
			t.Fatalf("Evidence summary leaked detail-only value %q:\n%s", forensic, ansi.Strip(wide))
		}
	}

	narrow := ansi.Strip(renderEvidenceAtWidth(t, response, renderer.FormatTable, 24))
	if !strings.Contains(narrow, "Family: dispatch") || !strings.Contains(narrow, "Recovery: false") || strings.Contains(narrow, "────") {
		t.Fatalf("narrow Evidence output is not a vertical summary:\n%s", narrow)
	}
}

func renderEvidenceAtWidth(t *testing.T, response *plugin.Response, format renderer.OutputFormat, width int) string {
	t.Helper()
	var output bytes.Buffer
	options := renderer.RenderOptions{Format: format, WidthProvider: evidenceOutputWidth(width)}
	if err := renderer.RenderWithOptionsTo(response, options, &output); err != nil {
		t.Fatalf("RenderWithOptionsTo: %v", err)
	}
	return output.String()
}

type evidenceOutputWidth int

func (width evidenceOutputWidth) Width(io.Writer) (int, bool) {
	return int(width), true
}

//nolint:govet // Field order follows request/result/error test behavior.
type recordingEvidenceQueryRunner struct {
	request evidenceQueryRequest
	result  evidenceQueryResult
	err     error
}

func (runner *recordingEvidenceQueryRunner) Query(_ context.Context, request evidenceQueryRequest) (evidenceQueryResult, error) {
	runner.request = request
	return runner.result, runner.err
}

type fixedEvidenceResponseClock struct {
	timestamp time.Time
}

func (clock fixedEvidenceResponseClock) Now() time.Time {
	return clock.timestamp
}
