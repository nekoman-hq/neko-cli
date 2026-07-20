package evidence

import (
	"os"
	"strings"
	"testing"
)

func TestEvidenceQueryCannotReachJournalMutationStores(t *testing.T) {
	for _, path := range []string{
		"query.go",
		"query_release_execution.go",
		"query_dispatch.go",
		"query_v1_compensation.go",
		"query_migration.go",
		"query_v2_pair_recovery.go",
		"query_files.go",
	} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, forbidden := range []string{
			"NewReleaseExecutionJournalStore",
			"NewDispatchJournalStore",
			"NewV1CompensationEvidenceStore",
			"BeginPending(",
			"ConfirmPhase(",
			"RecordLastError(",
		} {
			if strings.Contains(string(source), forbidden) {
				t.Fatalf("read-only Evidence source %s reaches mutation capability %q", path, forbidden)
			}
		}
	}
	query, err := os.ReadFile("query.go")
	if err != nil {
		t.Fatalf("read query.go: %v", err)
	}
	if !strings.Contains(string(query), "ResolveReleaseEvidenceLocations") {
		t.Fatal("read-only Evidence query does not use the canonical read-only location boundary")
	}
}
