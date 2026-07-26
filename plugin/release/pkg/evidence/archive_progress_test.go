package evidence

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/log"
)

func TestEvidenceArchiveProgressFollowsSuccessfulFilesystemOrder(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "executions", strings.Repeat("a", 64)+".json")
	data := []byte("{\"completed\":true}\n")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	if err := os.WriteFile(source, data, 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	record := EvidenceRecord{
		Family: FamilyReleaseExecution, Identity: strings.Repeat("a", 64),
		Classification: ClassificationCompleted, LifecycleAllowed: true,
		LifecycleOperation: "archive-completed", Path: source, DigestSHA256: sha256Hex(data),
	}
	progress := &recordingEvidenceArchiveProgress{}
	useCase := evidenceArchiveUseCase{
		query:    &recordingEvidenceQueryRunner{result: evidenceQueryResult{Records: []EvidenceRecord{record}}},
		progress: progress,
	}
	result, err := useCase.Archive(context.Background(), evidenceArchiveRequest{
		RepositoryRoot: root, Family: record.Family, Identity: record.Identity,
		DigestSHA256: record.DigestSHA256, ConfirmArchive: true,
	})
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	want := []evidenceArchiveProgressKind{
		evidenceArchiveResolvingFamily,
		evidenceArchiveReadingEvidence,
		evidenceArchiveResolvingIdentity,
		evidenceArchiveVerifyingDigest,
		evidenceArchiveDigestVerified,
		evidenceArchiveCheckingTarget,
		evidenceArchiveTargetAvailable,
		evidenceArchivePreparingWrite,
		evidenceArchiveWriteCompleted,
		evidenceArchiveVerifyingResult,
		evidenceArchiveResultVerified,
		evidenceArchiveRemovingSource,
		evidenceArchiveSourceRemoved,
		evidenceArchiveCompleted,
	}
	if !reflect.DeepEqual(progress.kinds(), want) {
		t.Fatalf("archive progress = %#v, want %#v", progress.kinds(), want)
	}
	if _, statErr := os.Stat(source); !os.IsNotExist(statErr) {
		t.Fatalf("source still exists after completion: %v", statErr)
	}
	archived, readErr := os.ReadFile(result.ArchivePath)
	if readErr != nil || string(archived) != string(data) {
		t.Fatalf("archive bytes = %q, err=%v", archived, readErr)
	}
}

func TestEvidenceArchiveTargetConflictStopsBeforeWriteAndSourceRemoval(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "executions", strings.Repeat("a", 64)+".json")
	data := []byte("{\"completed\":true}\n")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	if err := os.WriteFile(source, data, 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	record := EvidenceRecord{
		Family: FamilyReleaseExecution, Identity: strings.Repeat("a", 64),
		Classification: ClassificationCompleted, LifecycleAllowed: true,
		LifecycleOperation: "archive-completed", Path: source, DigestSHA256: sha256Hex(data),
	}
	archivePath := filepath.Join(
		filepath.Dir(source), "archived", record.Identity+"-"+record.DigestSHA256+".json",
	)
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o700); err != nil {
		t.Fatalf("create archive directory: %v", err)
	}
	conflict := []byte("existing archive\n")
	if err := os.WriteFile(archivePath, conflict, 0o600); err != nil {
		t.Fatalf("write archive conflict: %v", err)
	}
	progress := &recordingEvidenceArchiveProgress{}
	useCase := evidenceArchiveUseCase{
		query:    &recordingEvidenceQueryRunner{result: evidenceQueryResult{Records: []EvidenceRecord{record}}},
		progress: progress,
	}
	_, err := useCase.Archive(context.Background(), evidenceArchiveRequest{
		RepositoryRoot: root, Family: record.Family, Identity: record.Identity,
		DigestSHA256: record.DigestSHA256, ConfirmArchive: true,
	})
	if err == nil || !strings.Contains(err.Error(), "archive already exists") {
		t.Fatalf("Archive target conflict error = %v", err)
	}
	for _, forbidden := range []evidenceArchiveProgressKind{
		evidenceArchiveTargetAvailable,
		evidenceArchivePreparingWrite,
		evidenceArchiveWriteCompleted,
		evidenceArchiveRemovingSource,
		evidenceArchiveSourceRemoved,
		evidenceArchiveCompleted,
	} {
		if progress.contains(forbidden) {
			t.Fatalf("target conflict emitted later phase %q: %#v", forbidden, progress.kinds())
		}
	}
	if got, readErr := os.ReadFile(source); readErr != nil || string(got) != string(data) {
		t.Fatalf("target conflict changed source: %q, err=%v", got, readErr)
	}
	if got, readErr := os.ReadFile(archivePath); readErr != nil || string(got) != string(conflict) {
		t.Fatalf("target conflict changed archive: %q, err=%v", got, readErr)
	}
}

func TestEvidenceArchiveRefusalsDoNotMutateSourceOrCreateArchive(t *testing.T) {
	data := []byte("{\"completed\":true}\n")
	tests := []struct {
		name        string
		configure   func(t *testing.T, root, source string, record *EvidenceRecord, request *evidenceArchiveRequest)
		wantError   string
		sourceExist bool
	}{
		{
			name: "unknown identity",
			configure: func(_ *testing.T, _ string, _ string, _ *EvidenceRecord, request *evidenceArchiveRequest) {
				request.Identity = strings.Repeat("b", 64)
			},
			wantError:   "matching evidence record was not found",
			sourceExist: true,
		},
		{
			name: "digest mismatch",
			configure: func(_ *testing.T, _ string, _ string, _ *EvidenceRecord, request *evidenceArchiveRequest) {
				request.DigestSHA256 = strings.Repeat("b", 64)
			},
			wantError:   "evidence digest changed since inspection",
			sourceExist: true,
		},
		{
			name: "source missing",
			configure: func(t *testing.T, _ string, source string, _ *EvidenceRecord, _ *evidenceArchiveRequest) {
				if err := os.Remove(source); err != nil {
					t.Fatalf("remove source fixture: %v", err)
				}
			},
			wantError: "re-observe evidence file",
		},
		{
			name: "filesystem failure",
			configure: func(t *testing.T, _ string, source string, _ *EvidenceRecord, _ *evidenceArchiveRequest) {
				archiveBlocker := filepath.Join(filepath.Dir(source), "archived")
				if err := os.WriteFile(archiveBlocker, []byte("not a directory\n"), 0o600); err != nil {
					t.Fatalf("write archive directory blocker: %v", err)
				}
			},
			wantError:   "create evidence archive directory",
			sourceExist: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "executions", strings.Repeat("a", 64)+".json")
			if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
				t.Fatalf("create source directory: %v", err)
			}
			if err := os.WriteFile(source, data, 0o600); err != nil {
				t.Fatalf("write source: %v", err)
			}
			record := EvidenceRecord{
				Family: FamilyReleaseExecution, Identity: strings.Repeat("a", 64),
				Classification: ClassificationCompleted, LifecycleAllowed: true,
				LifecycleOperation: "archive-completed", Path: source, DigestSHA256: sha256Hex(data),
			}
			request := evidenceArchiveRequest{
				RepositoryRoot: root, Family: record.Family, Identity: record.Identity,
				DigestSHA256: record.DigestSHA256, ConfirmArchive: true,
			}
			test.configure(t, root, source, &record, &request)
			progress := &recordingEvidenceArchiveProgress{}
			useCase := evidenceArchiveUseCase{
				query:    &recordingEvidenceQueryRunner{result: evidenceQueryResult{Records: []EvidenceRecord{record}}},
				progress: progress,
			}
			_, err := useCase.Archive(context.Background(), request)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Archive error = %v, want %q", err, test.wantError)
			}
			if progress.contains(evidenceArchiveCompleted) {
				t.Fatalf("refusal emitted completion: %#v", progress.kinds())
			}
			if test.sourceExist {
				got, readErr := os.ReadFile(source)
				if readErr != nil || string(got) != string(data) {
					t.Fatalf("refusal changed source: %q, err=%v", got, readErr)
				}
			} else if _, statErr := os.Stat(source); !os.IsNotExist(statErr) {
				t.Fatalf("source-missing fixture was recreated: %v", statErr)
			}
			archivePath := filepath.Join(
				filepath.Dir(source), "archived", record.Identity+"-"+record.DigestSHA256+".json",
			)
			if _, statErr := os.Stat(archivePath); statErr == nil {
				t.Fatalf("refusal created exact archive")
			}
		})
	}
}

func TestEvidenceArchiveTerminalProgressRedactsPathsCredentialsAndDigests(t *testing.T) {
	originalVerbose := log.Verbose
	log.Verbose = true
	t.Cleanup(func() { log.Verbose = originalVerbose })

	const (
		root   = "/private/tmp/evidence-progress"
		secret = "evidence-progress-secret"
	)
	digest := strings.Repeat("b", 64)
	stderr := ansi.Strip(captureEvidenceProgress(t, func() {
		progress := terminalEvidenceArchiveProgress{}
		for _, event := range []evidenceArchiveProgressEvent{
			{Kind: evidenceArchiveRequestValidated, Family: FamilyReleaseExecution, Identity: strings.Repeat("a", 64)},
			{Kind: evidenceArchiveVerifyingDigest, Digest: digest},
			{Kind: evidenceArchiveCheckingTarget, Archive: root + "/.git/neko/release/executions/archived/target.json"},
			{Kind: evidenceArchiveRemovingSource, Source: root + "/.git/neko/release/executions/" + secret + ".json"},
			{Kind: evidenceArchiveCompleted},
		} {
			progress.ReportEvidenceArchiveProgress(event)
		}
	}))
	for _, forbidden := range []string{root, secret, digest, "\x1b["} {
		if strings.Contains(stderr, forbidden) {
			t.Fatalf("archive progress exposed %q:\n%s", forbidden, stderr)
		}
	}
	for _, want := range []string{
		"Archive request validated",
		"Verifying current evidence digest: " + digest[:12],
		".git/neko/release/executions/archived/target.json",
		"Evidence archive operation completed",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("archive progress omitted %q:\n%s", want, stderr)
		}
	}
}

type recordingEvidenceArchiveProgress struct {
	events []evidenceArchiveProgressEvent
}

func (progress *recordingEvidenceArchiveProgress) ReportEvidenceArchiveProgress(event evidenceArchiveProgressEvent) {
	progress.events = append(progress.events, event)
}

func (progress *recordingEvidenceArchiveProgress) kinds() []evidenceArchiveProgressKind {
	kinds := make([]evidenceArchiveProgressKind, 0, len(progress.events))
	for _, event := range progress.events {
		kinds = append(kinds, event.Kind)
	}
	return kinds
}

func (progress *recordingEvidenceArchiveProgress) contains(kind evidenceArchiveProgressKind) bool {
	for _, event := range progress.events {
		if event.Kind == kind {
			return true
		}
	}
	return false
}

func captureEvidenceProgress(t *testing.T, run func()) string {
	t.Helper()
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = writeEnd
	run()
	if closeErr := writeEnd.Close(); closeErr != nil {
		t.Fatalf("close stderr writer: %v", closeErr)
	}
	os.Stderr = oldStderr
	output, err := io.ReadAll(readEnd)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if err := readEnd.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}
	return string(output)
}
