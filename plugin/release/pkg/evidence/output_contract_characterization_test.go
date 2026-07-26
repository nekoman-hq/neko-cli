package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

func TestEvidenceClassificationVocabularyRemainsStable(t *testing.T) {
	dispatchTests := []struct {
		state          release.DispatchJournalState
		classification string
		manual         bool
	}{
		{state: release.DispatchJournalPrepared, classification: ClassificationActive},
		{state: release.DispatchJournalRequestStarted, classification: ClassificationUncertain, manual: true},
		{state: release.DispatchJournalAccepted, classification: ClassificationCompleted},
		{state: release.DispatchJournalRejected, classification: ClassificationTerminal, manual: true},
		{state: release.DispatchJournalUnknown, classification: ClassificationUncertain, manual: true},
	}
	for _, test := range dispatchTests {
		classification, manual, guidance := classifyDispatch(test.state)
		if classification != test.classification || manual != test.manual || strings.TrimSpace(guidance) == "" {
			t.Fatalf("classifyDispatch(%q) = (%q, %t, %q)", test.state, classification, manual, guidance)
		}
	}

	executionTests := []struct {
		state          release.ReleaseExecutionJournalState
		pending        release.ReleaseExecutionPendingAction
		commit         string
		classification string
		safe           bool
		manual         bool
	}{
		{
			state: release.ReleaseExecutionPrepared, pending: release.ReleaseExecutionPendingNone,
			classification: ClassificationActive,
		},
		{
			state: release.ReleaseExecutionCommitCreated, pending: release.ReleaseExecutionPendingNone,
			commit: strings.Repeat("a", 40), classification: ClassificationResumable, safe: true,
		},
		{
			state: release.ReleaseExecutionCommitPushed, pending: release.ReleaseExecutionPendingNone,
			commit: strings.Repeat("a", 40), classification: ClassificationManualRecoveryRequired, manual: true,
		},
		{
			state: release.ReleaseExecutionHandoffReady, pending: release.ReleaseExecutionPendingNone,
			classification: ClassificationCompleted,
		},
		{
			state: release.ReleaseExecutionDispatchJournalPrepared, pending: release.ReleaseExecutionPendingPushReleaseCommit,
			classification: ClassificationUncertain, manual: true,
		},
	}
	for _, test := range executionTests {
		classification, safe, _, manual, guidance := classifyReleaseExecution(release.ReleaseExecutionJournal{
			State: test.state, PendingAction: test.pending, ReleaseCommitSHA: test.commit,
		})
		if classification != test.classification || safe != test.safe || manual != test.manual ||
			strings.TrimSpace(guidance) == "" {
			t.Fatalf(
				"classifyReleaseExecution(%q, %q) = (%q, %t, %t, %q)",
				test.state, test.pending, classification, safe, manual, guidance,
			)
		}
	}
}

func TestEvidenceArchiveRequestGuardErrorsRemainExact(t *testing.T) {
	identity := strings.Repeat("a", 64)
	digest := strings.Repeat("b", 64)
	tests := []struct {
		name  string
		flags map[string]any
		want  string
	}{
		{name: "missing family", flags: map[string]any{}, want: "--family must name a supported evidence family"},
		{
			name:  "inspect-only family",
			flags: map[string]any{"family": FamilyDispatch},
			want:  `evidence family "dispatch" has no archival lifecycle operation`,
		},
		{
			name:  "invalid family",
			flags: map[string]any{"family": "unknown"},
			want:  "--family must name a supported evidence family",
		},
		{
			name:  "missing identity",
			flags: map[string]any{"family": FamilyReleaseExecution},
			want:  "--identity must be a sha256 evidence identity from inspection output",
		},
		{
			name: "invalid identity",
			flags: map[string]any{
				"family": FamilyReleaseExecution, "identity": "not-an-identity",
			},
			want: "--identity must be a sha256 evidence identity from inspection output",
		},
		{
			name:  "missing digest",
			flags: map[string]any{"family": FamilyReleaseExecution, "identity": identity},
			want:  "--digest-sha256 must be the current digest from inspection output",
		},
		{
			name: "invalid digest",
			flags: map[string]any{
				"family": FamilyReleaseExecution, "identity": identity, "digest-sha256": "not-a-digest",
			},
			want: "--digest-sha256 must be the current digest from inspection output",
		},
		{
			name: "missing confirmation",
			flags: map[string]any{
				"family": FamilyReleaseExecution, "identity": identity, "digest-sha256": digest,
			},
			want: "--confirm-archive is required",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseEvidenceArchiveRequest(test.flags, "/repo")
			if err == nil || err.Error() != test.want {
				t.Fatalf("parseEvidenceArchiveRequest error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestEvidenceArchiveResponseJSONRemainsCanonical(t *testing.T) {
	timestamp := time.Date(2026, time.July, 26, 10, 0, 0, 0, time.UTC)
	result := evidenceArchiveResult{
		Family:       FamilyReleaseExecution,
		Identity:     strings.Repeat("a", 64),
		DigestSHA256: strings.Repeat("b", 64),
		SourcePath:   "/repo/.git/neko/release/executions/source.json",
		ArchivePath:  "/repo/.git/neko/release/executions/archived/target.json",
	}
	response := mapEvidenceArchiveResponse(result, timestamp)
	var output bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatJSON, &output); err != nil {
		t.Fatalf("RenderTo: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode archive JSON: %v", err)
	}
	wantItems := []any{
		map[string]any{"property": "Family", "value": FamilyReleaseExecution},
		map[string]any{"property": "Identity", "value": result.Identity},
		map[string]any{"property": "Digest", "value": result.DigestSHA256},
		map[string]any{"property": "Source", "value": result.SourcePath},
		map[string]any{"property": "Archive", "value": result.ArchivePath},
		map[string]any{"property": "Status", "value": "archived"},
	}
	data, ok := decoded["data"].(map[string]any)
	if !ok || !reflect.DeepEqual(data["items"], wantItems) {
		t.Fatalf("archive items changed:\n%#v", decoded["data"])
	}
	if decoded["status"] != "success" || decoded["renderer_hint"] != "table" {
		t.Fatalf("archive envelope changed:\n%#v", decoded)
	}
}

func TestEvidenceArchiveHandlerStopsBeforeMutationOnInvalidRequest(t *testing.T) {
	identity := strings.Repeat("a", 64)
	digest := strings.Repeat("b", 64)
	//nolint:govet // Table fields follow readable name/input order.
	tests := []struct {
		name  string
		flags map[string]any
	}{
		{name: "missing family", flags: map[string]any{}},
		{name: "invalid family", flags: map[string]any{"family": "unknown"}},
		{name: "missing identity", flags: map[string]any{"family": FamilyReleaseExecution}},
		{
			name: "invalid identity",
			flags: map[string]any{
				"family": FamilyReleaseExecution, "identity": "invalid",
			},
		},
		{
			name: "missing digest",
			flags: map[string]any{
				"family": FamilyReleaseExecution, "identity": identity,
			},
		},
		{
			name: "digest invalid",
			flags: map[string]any{
				"family": FamilyReleaseExecution, "identity": identity, "digest-sha256": "invalid",
			},
		},
		{
			name: "missing confirmation",
			flags: map[string]any{
				"family": FamilyReleaseExecution, "identity": identity, "digest-sha256": digest,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archiver := &recordingEvidenceArchiver{}
			handler := evidenceArchiveCommandHandler{
				archive: archiver,
				clock: fixedEvidenceResponseClock{
					timestamp: time.Date(2026, time.July, 26, 10, 1, 0, 0, time.UTC),
				},
			}
			response, err := handler.Handle(plugin.Request{
				Flags: test.flags,
				Context: plugin.Context{
					WorkingDir: "/repo",
				},
			})
			if response != nil || err == nil || archiver.called {
				t.Fatalf("invalid request = (%#v, %v), archiver called=%t", response, err, archiver.called)
			}
		})
	}
}

//nolint:govet // Test fake fields follow call/result/error observation order.
type recordingEvidenceArchiver struct {
	result evidenceArchiveResult
	err    error
	called bool
}

func (archiver *recordingEvidenceArchiver) Archive(_ context.Context, _ evidenceArchiveRequest) (evidenceArchiveResult, error) {
	archiver.called = true
	return archiver.result, archiver.err
}

func TestEvidenceArchiveHandlerPreservesArchiverError(t *testing.T) {
	wantErr := errors.New("archive target conflict")
	archiver := &recordingEvidenceArchiver{err: wantErr}
	handler := evidenceArchiveCommandHandler{
		archive: archiver,
		clock:   fixedEvidenceResponseClock{timestamp: time.Date(2026, time.July, 26, 10, 2, 0, 0, time.UTC)},
	}
	response, err := handler.Handle(plugin.Request{
		Flags: map[string]any{
			"family":          FamilyReleaseExecution,
			"identity":        strings.Repeat("a", 64),
			"digest-sha256":   strings.Repeat("b", 64),
			"confirm-archive": true,
		},
		Context: plugin.Context{WorkingDir: "/repo"},
	})
	if response != nil || !errors.Is(err, wantErr) {
		t.Fatalf("archive failure = (%#v, %v), want (nil, %v)", response, err, wantErr)
	}
}
