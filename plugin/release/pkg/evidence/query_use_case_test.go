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
	if writeErr := os.WriteFile(corruptPath, []byte("{not-json"), 0600); writeErr != nil {
		t.Fatalf("write corrupt dispatch journal: %v", writeErr)
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
	detailOutput, err := json.Marshal(mapEvidenceDetailResponse(evidenceQueryResult{Records: result.Records[:1]}, evidenceTestTime))
	if err != nil {
		t.Fatalf("marshal detail response: %v", err)
	}
	for _, forbidden := range []string{"secret-token", "Authorization", "Bearer"} {
		if strings.Contains(string(output), forbidden) {
			t.Fatalf("evidence output leaked %q:\n%s", forbidden, output)
		}
		if strings.Contains(string(detailOutput), forbidden) {
			t.Fatalf("Evidence detail leaked %q:\n%s", forbidden, detailOutput)
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

func TestEvidenceIdentityPrefixAppliesFiltersBeforeUniqueMatchingWithoutMutation(t *testing.T) {
	root := newEvidenceGitRepository(t)
	executionDir, err := release.NewReleaseExecutionJournalStore(root).JournalDirectory()
	if err != nil {
		t.Fatalf("execution directory: %v", err)
	}
	dispatchDir, err := release.NewDispatchJournalStore(root).JournalDirectory()
	if err != nil {
		t.Fatalf("dispatch directory: %v", err)
	}
	apiIdentity := strings.Repeat("a", 64)
	workerIdentity := "aaaaaaaa" + strings.Repeat("b", 56)
	dispatchIdentity := "aaaaaaaa" + strings.Repeat("c", 56)
	apiPath := filepath.Join(executionDir, apiIdentity+".json")
	workerPath := filepath.Join(executionDir, workerIdentity+".json")
	dispatchPath := filepath.Join(dispatchDir, dispatchIdentity+".json")
	writeEvidenceJSON(t, apiPath, release.ReleaseExecutionJournal{
		SchemaVersion: releaseExecutionSchemaVersionForTest,
		Identity:      release.ReleaseExecutionIdentity{SHA256: apiIdentity},
		UnitID:        "api",
		NextVersion:   "1.2.4",
		Tag:           "api/v1.2.4",
		State:         release.ReleaseExecutionPrepared,
		PendingAction: release.ReleaseExecutionPendingNone,
		CreatedAt:     evidenceTestTime,
		UpdatedAt:     evidenceTestTime,
	})
	writeEvidenceJSON(t, workerPath, release.ReleaseExecutionJournal{
		SchemaVersion: releaseExecutionSchemaVersionForTest,
		Identity:      release.ReleaseExecutionIdentity{SHA256: workerIdentity},
		UnitID:        "worker",
		NextVersion:   "2.0.1",
		Tag:           "worker/v2.0.1",
		State:         release.ReleaseExecutionPrepared,
		PendingAction: release.ReleaseExecutionPendingNone,
		CreatedAt:     evidenceTestTime,
		UpdatedAt:     evidenceTestTime,
	})
	writeEvidenceJSON(t, dispatchPath, release.DispatchJournal{
		SchemaVersion: dispatchSchemaVersionForTest,
		Identity:      release.ReleaseDispatchIdentity{SHA256: dispatchIdentity},
		UnitID:        "api",
		Version:       "1.2.4",
		Tag:           "api/v1.2.4",
		State:         release.DispatchJournalAccepted,
		CreatedAt:     evidenceTestTime,
		UpdatedAt:     evidenceTestTime,
	})
	before := readEvidenceFiles(t, apiPath, workerPath, dispatchPath)

	filtered, err := newEvidenceQueryUseCase().Query(context.Background(), evidenceQueryRequest{
		RepositoryRoot: root,
		Family:         FamilyReleaseExecution,
		Unit:           "api",
		IdentityPrefix: "aaaaaaaa",
	})
	if err != nil {
		t.Fatalf("filtered prefix Query: %v", err)
	}
	if len(filtered.Records) != 1 || filtered.Records[0].Identity != apiIdentity {
		t.Fatalf("filtered prefix result = %#v, want API execution record", filtered.Records)
	}

	_, err = newEvidenceQueryUseCase().Query(context.Background(), evidenceQueryRequest{
		RepositoryRoot: root,
		Family:         FamilyReleaseExecution,
		IdentityPrefix: "aaaaaaaa",
	})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "2 matches") {
		t.Fatalf("ambiguous prefix error = %v", err)
	}

	_, err = newEvidenceQueryUseCase().Query(context.Background(), evidenceQueryRequest{
		RepositoryRoot: root,
		Family:         FamilyReleaseExecution,
		Unit:           "missing",
		IdentityPrefix: "aaaaaaaa",
	})
	if err == nil || !strings.Contains(err.Error(), "no evidence identity matches") {
		t.Fatalf("zero-match prefix error = %v", err)
	}

	exact, err := newEvidenceQueryUseCase().Query(context.Background(), evidenceQueryRequest{
		RepositoryRoot: root,
		IdentityPrefix: apiIdentity,
	})
	if err != nil || len(exact.Records) != 1 || exact.Records[0].Identity != apiIdentity {
		t.Fatalf("full identity Query = (%#v, %v)", exact.Records, err)
	}

	after := readEvidenceFiles(t, apiPath, workerPath, dispatchPath)
	for path, contents := range before {
		if string(after[path]) != string(contents) {
			t.Fatalf("identity inspection mutated %s", path)
		}
	}
}

func TestEvidenceIdentityPrefixValidationIsCanonicalAndArchiveRemainsExact(t *testing.T) {
	fullIdentity := strings.Repeat("a", 64)
	for _, test := range []struct {
		name     string
		identity string
		valid    bool
	}{
		{name: "minimum prefix", identity: "0123abcd", valid: true},
		{name: "full identity", identity: fullIdentity, valid: true},
		{name: "too short", identity: "0123abc"},
		{name: "too long", identity: fullIdentity + "a"},
		{name: "uppercase rejected", identity: "0123ABCD"},
		{name: "non hexadecimal rejected", identity: "0123abcg"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := parseEvidenceQueryRequest(map[string]any{"identity": test.identity}, "/repo")
			if test.valid {
				if err != nil || request.IdentityPrefix != test.identity {
					t.Fatalf("parseEvidenceQueryRequest = (%#v, %v)", request, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "8 to 64 lowercase hexadecimal") {
				t.Fatalf("invalid identity error = %v", err)
			}
		})
	}

	_, err := parseEvidenceArchiveRequest(map[string]any{
		"family":          FamilyReleaseExecution,
		"identity":        "0123abcd",
		"digest-sha256":   strings.Repeat("b", 64),
		"confirm-archive": true,
	}, "/repo")
	if err == nil || !strings.Contains(err.Error(), "sha256 evidence identity") {
		t.Fatalf("archive accepted identity prefix: %v", err)
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
	if _, archiveErr := newEvidenceArchiveUseCase().Archive(context.Background(), request); archiveErr == nil || !strings.Contains(archiveErr.Error(), "digest changed") {
		t.Fatalf("wrong digest archive error = %v, want digest changed", archiveErr)
	}
	if _, statErr := os.Stat(sourcePath); statErr != nil {
		t.Fatalf("wrong digest removed source evidence: %v", statErr)
	}

	request.DigestSHA256 = query.Records[0].DigestSHA256
	result, err := newEvidenceArchiveUseCase().Archive(context.Background(), request)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if _, statErr := os.Stat(sourcePath); !os.IsNotExist(statErr) {
		t.Fatalf("source evidence still exists after archive: %v", statErr)
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

func readEvidenceFiles(t *testing.T, paths ...string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte, len(paths))
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read evidence file %s: %v", path, err)
		}
		files[path] = contents
	}
	return files
}

func gitEvidenceCommand(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
