//nolint:staticcheck // Migration tests intentionally exercise deprecated V1 inputs.
package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestMigrationJournalUnsupportedSchemaIsPreservedForManualRecovery(t *testing.T) {
	root := withGitRepo(t)
	writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)
	plan, err := ResolvePlan(root)
	if err != nil {
		t.Fatalf("ResolvePlan initial: %v", err)
	}
	journal := journalForPlan(t, plan, journalStageStateWritten)
	journal.SchemaVersion = 99
	writeJournalForTest(t, root, journal)
	before, err := os.ReadFile(filepath.Join(root, releaseconfig.V2Directory, journalFileName))
	if err != nil {
		t.Fatalf("read migration journal before recovery: %v", err)
	}

	_, err = ResolvePlan(root)

	if err == nil || !strings.Contains(err.Error(), "does not match repository paths") {
		t.Fatalf("unsupported schema error = %v, want conservative journal mismatch", err)
	}
	after, readErr := os.ReadFile(filepath.Join(root, releaseconfig.V2Directory, journalFileName))
	if readErr != nil {
		t.Fatalf("read migration journal after recovery: %v", readErr)
	}
	if string(after) != string(before) {
		t.Fatalf("unsupported migration journal was mutated:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}
