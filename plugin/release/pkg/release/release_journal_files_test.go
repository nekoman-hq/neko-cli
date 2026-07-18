package release

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveReleaseEvidenceLocationsDoesNotCreateStorage(t *testing.T) {
	root := t.TempDir()
	gitCmd(t, root, "init")
	releaseDirectory := filepath.Join(root, ".git", "neko", "release")

	locations, err := ResolveReleaseEvidenceLocations(root)
	if err != nil {
		t.Fatalf("ResolveReleaseEvidenceLocations: %v", err)
	}
	want := ReleaseEvidenceLocations{
		ExecutionJournalDirectory: filepath.Join(releaseDirectory, "executions"),
		DispatchJournalDirectory:  filepath.Join(releaseDirectory, "dispatches"),
		V1CompensationPath:        filepath.Join(releaseDirectory, "v1-compensation", "current.json"),
	}
	if !reflect.DeepEqual(locations, want) {
		t.Fatalf("locations = %#v, want %#v", locations, want)
	}
	if _, err := os.Stat(releaseDirectory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only location resolution created storage: %v", err)
	}
}
