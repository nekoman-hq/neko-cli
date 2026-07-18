package evidence

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
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
		if _, ok := items[0][key].(string); !ok {
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
