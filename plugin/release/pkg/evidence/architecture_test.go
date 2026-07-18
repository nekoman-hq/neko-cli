package evidence

import (
	"os"
	"strings"
	"testing"
)

func TestEvidenceQueryCannotReachJournalMutationStores(t *testing.T) {
	source, err := os.ReadFile("query.go")
	if err != nil {
		t.Fatalf("read query.go: %v", err)
	}
	text := string(source)
	for _, forbidden := range []string{
		"NewReleaseExecutionJournalStore",
		"NewDispatchJournalStore",
		"NewV1CompensationEvidenceStore",
		"BeginPending(",
		"ConfirmPhase(",
		"RecordLastError(",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("read-only Evidence query reaches mutation capability %q", forbidden)
		}
	}
	if !strings.Contains(text, "ResolveReleaseEvidenceLocations") {
		t.Fatal("read-only Evidence query does not use the canonical read-only location boundary")
	}
}
