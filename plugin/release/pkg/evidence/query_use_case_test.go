package evidence

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

func TestEvidenceQuerySummarizesValidJournalsAndCorruptDiagnosticsWithoutSecrets(t *testing.T) {
	root := newEvidenceGitRepository(t)
	executionIdentity := strings.Repeat("a", 64)
	dispatchIdentity := strings.Repeat("b", 64)
	executionDir, err := release.NewReleaseExecutionJournalStore(root).JournalDirectory()
	if err != nil {
		t.Fatalf("execution directory: %v", err)
	}
	dispatchDir, err := release.NewDispatchJournalStore(root).JournalDirectory()
	if err != nil {
		t.Fatalf("dispatch directory: %v", err)
	}
	writeEvidenceJSON(t, filepath.Join(executionDir, executionIdentity+".json"), release.ReleaseExecutionJournal{
		SchemaVersion: releaseExecutionSchemaVersionForTest,
		Identity: release.ReleaseExecutionIdentity{
			SHA256: executionIdentity,
		},
		RepositoryRemote: "https://github.com/nekoman/repo.git",
		UnitID:           "api",
		NextVersion:      "1.2.4",
		Tag:              "api/v1.2.4",
		State:            release.ReleaseExecutionHandoffReady,
		PendingAction:    release.ReleaseExecutionPendingNone,
		LastError:        "Authorization: Bearer secret-token",
		CreatedAt:        evidenceTestTime,
		UpdatedAt:        evidenceTestTime,
	})
	writeEvidenceJSON(t, filepath.Join(dispatchDir, dispatchIdentity+".json"), release.DispatchJournal{
		SchemaVersion: dispatchSchemaVersionForTest,
		Identity: release.ReleaseDispatchIdentity{
			SHA256: dispatchIdentity,
		},
		UnitID:           "api",
		Version:          "1.2.4",
		Tag:              "api/v1.2.4",
		Inputs:           map[string]string{"token": "secret-token"},
		State:            release.DispatchJournalAccepted,
		LastError:        "Bearer secret-token",
		CreatedAt:        evidenceTestTime,
		UpdatedAt:        evidenceTestTime,
		RecoveryGuidance: "accepted",
	})
	corruptPath := filepath.Join(dispatchDir, strings.Repeat("c", 64)+".json")
	if err := os.WriteFile(corruptPath, []byte("{not-json"), 0600); err != nil {
		t.Fatalf("write corrupt dispatch journal: %v", err)
	}

	result, err := newEvidenceQueryUseCase().Query(context.Background(), evidenceQueryRequest{RepositoryRoot: root})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if len(result.Records) != 2 {
		t.Fatalf("records = %#v, want 2", result.Records)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Classification != ClassificationCorrupt {
		t.Fatalf("diagnostics = %#v, want one corrupt diagnostic", result.Diagnostics)
	}
	output, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	for _, forbidden := range []string{"secret-token", "Authorization", "Bearer"} {
		if strings.Contains(string(output), forbidden) {
			t.Fatalf("evidence output leaked %q:\n%s", forbidden, output)
		}
	}
	if result.Records[0].Family != FamilyDispatch || result.Records[1].Family != FamilyReleaseExecution {
		t.Fatalf("records are not deterministically sorted: %#v", result.Records)
	}
}

func TestEvidenceQueryPreservesUnsupportedMigrationJournal(t *testing.T) {
	root := newEvidenceGitRepository(t)
	path := filepath.Join(root, ".neko", "release.migration.json")
	before := []byte(`{"schemaVersion":99,"stage":"prepared"}` + "\n")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	if err := os.WriteFile(path, before, 0644); err != nil {
		t.Fatalf("write migration journal: %v", err)
	}

	result, err := newEvidenceQueryUseCase().Query(context.Background(), evidenceQueryRequest{RepositoryRoot: root, Family: FamilyMigration})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if len(result.Records) != 0 || len(result.Diagnostics) != 1 || result.Diagnostics[0].Classification != ClassificationUnsupported {
		t.Fatalf("result = %#v, want unsupported diagnostic only", result)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration journal: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("inspection mutated unsupported migration journal:\nbefore=%s\nafter=%s", before, after)
	}
}

func TestEvidenceArchiveRequiresFreshDigestAndCreatesPrivateExactArchive(t *testing.T) {
	root := newEvidenceGitRepository(t)
	identity := strings.Repeat("d", 64)
	executionDir, err := release.NewReleaseExecutionJournalStore(root).JournalDirectory()
	if err != nil {
		t.Fatalf("execution directory: %v", err)
	}
	sourcePath := filepath.Join(executionDir, identity+".json")
	writeEvidenceJSON(t, sourcePath, release.ReleaseExecutionJournal{
		SchemaVersion: releaseExecutionSchemaVersionForTest,
		Identity: release.ReleaseExecutionIdentity{
			SHA256: identity,
		},
		RepositoryRemote: "https://github.com/nekoman/repo.git",
		UnitID:           "api",
		NextVersion:      "1.2.4",
		Tag:              "api/v1.2.4",
		State:            release.ReleaseExecutionHandoffReady,
		PendingAction:    release.ReleaseExecutionPendingNone,
		CreatedAt:        evidenceTestTime,
		UpdatedAt:        evidenceTestTime,
	})
	before, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source evidence: %v", err)
	}
	query, err := newEvidenceQueryUseCase().Query(context.Background(), evidenceQueryRequest{RepositoryRoot: root, Family: FamilyReleaseExecution})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(query.Records) != 1 || !query.Records[0].LifecycleAllowed || query.Records[0].LifecycleOperation != "archive-completed" {
		t.Fatalf("completed evidence was not marked archivable: %#v", query.Records)
	}
	request := evidenceArchiveRequest{
		RepositoryRoot: root,
		Family:         FamilyReleaseExecution,
		Identity:       identity,
		DigestSHA256:   strings.Repeat("0", 64),
		ConfirmArchive: true,
	}
	if _, err := newEvidenceArchiveUseCase().Archive(context.Background(), request); err == nil || !strings.Contains(err.Error(), "digest changed") {
		t.Fatalf("wrong digest archive error = %v, want digest changed", err)
	}
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatalf("wrong digest removed source evidence: %v", err)
	}

	request.DigestSHA256 = query.Records[0].DigestSHA256
	result, err := newEvidenceArchiveUseCase().Archive(context.Background(), request)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("source evidence still exists after archive: %v", err)
	}
	archived, err := os.ReadFile(result.ArchivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if string(archived) != string(before) {
		t.Fatalf("archive bytes changed:\n%s", archived)
	}
	info, err := os.Stat(filepath.Dir(result.ArchivePath))
	if err != nil {
		t.Fatalf("stat archive dir: %v", err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("archive dir mode = %04o, want 0700", info.Mode().Perm())
	}
}

func TestEvidenceArchiveRefusesUnresolvedEvidence(t *testing.T) {
	root := newEvidenceGitRepository(t)
	identity := strings.Repeat("e", 64)
	executionDir, err := release.NewReleaseExecutionJournalStore(root).JournalDirectory()
	if err != nil {
		t.Fatalf("execution directory: %v", err)
	}
	sourcePath := filepath.Join(executionDir, identity+".json")
	writeEvidenceJSON(t, sourcePath, release.ReleaseExecutionJournal{
		SchemaVersion: releaseExecutionSchemaVersionForTest,
		Identity: release.ReleaseExecutionIdentity{
			SHA256: identity,
		},
		RepositoryRemote: "https://github.com/nekoman/repo.git",
		UnitID:           "api",
		NextVersion:      "1.2.4",
		Tag:              "api/v1.2.4",
		State:            release.ReleaseExecutionPrepared,
		PendingAction:    release.ReleaseExecutionPendingNone,
		CreatedAt:        evidenceTestTime,
		UpdatedAt:        evidenceTestTime,
	})
	query, err := newEvidenceQueryUseCase().Query(context.Background(), evidenceQueryRequest{RepositoryRoot: root, Family: FamilyReleaseExecution})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(query.Records) != 1 || query.Records[0].LifecycleAllowed {
		t.Fatalf("unresolved evidence was marked archivable: %#v", query.Records)
	}

	_, err = newEvidenceArchiveUseCase().Archive(context.Background(), evidenceArchiveRequest{
		RepositoryRoot: root,
		Family:         FamilyReleaseExecution,
		Identity:       identity,
		DigestSHA256:   query.Records[0].DigestSHA256,
		ConfirmArchive: true,
	})
	if err == nil || !strings.Contains(err.Error(), "not eligible") {
		t.Fatalf("unresolved archive error = %v, want not eligible", err)
	}
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatalf("unresolved archive removed source evidence: %v", err)
	}
}

const (
	releaseExecutionSchemaVersionForTest = 1
	dispatchSchemaVersionForTest         = 1
)

var evidenceTestTime = time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)

func newEvidenceGitRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitEvidenceCommand(t, root, "init")
	gitEvidenceCommand(t, root, "config", "user.email", "test@example.com")
	gitEvidenceCommand(t, root, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitEvidenceCommand(t, root, "add", "README.md")
	gitEvidenceCommand(t, root, "commit", "-m", "initial")
	return root
}

func writeEvidenceJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("mkdir evidence dir: %v", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
}

func gitEvidenceCommand(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
