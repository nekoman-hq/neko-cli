//nolint:staticcheck // Migration planner tests intentionally use deprecated V1 data.
package migrate

import (
	"os"
	"reflect"
	"testing"
)

func TestConstructMigrationPlanProducesCompleteDeterministicTarget(t *testing.T) {
	root := "/path/that/does/not/exist"
	paths := migrationPaths(root)
	sourceData := []byte(v1Fixture)
	source := newMigrationFileSnapshot(paths.source, sourceData, 0600)

	first, err := constructMigrationPlan(root, paths, source, migrationFileSnapshot{path: paths.backup}, newMigrationPlan)
	if err != nil {
		t.Fatalf("construct first plan: %v", err)
	}
	second, err := constructMigrationPlan(root, paths, source, migrationFileSnapshot{path: paths.backup}, newMigrationPlan)
	if err != nil {
		t.Fatalf("construct second plan: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("planner output is not deterministic:\nfirst  %#v\nsecond %#v", first, second)
	}
	if first.sourceFormat != migrationSourceV1 || first.unitID != "default" || first.version != "1.2.3" || first.executor != "jreleaser" || first.delivery != "github-actions" {
		t.Fatalf("planned migration facts changed: %#v", first)
	}
	if string(first.target.configJSON) != validConfigJSON() || string(first.target.stateJSON) != validStateJSON() {
		t.Fatalf("planned target bytes changed: config=%q state=%q", first.target.configJSON, first.target.stateJSON)
	}
	if first.target.config.Units[0].ID != "default" || first.target.state.Units["default"].Version != "1.2.3" {
		t.Fatalf("typed target is incomplete: %#v", first.target)
	}

	sourceData[0] = '!'
	if first.source.data[0] != '{' {
		t.Fatal("planner retained mutable source input")
	}
}

func TestConstructMigrationPlanRejectsSourceAndBackupFailures(t *testing.T) {
	paths := migrationPaths(t.TempDir())
	tests := []struct {
		name   string
		source migrationFileSnapshot
		backup migrationFileSnapshot
	}{
		{name: "missing source", source: migrationFileSnapshot{path: paths.source}, backup: migrationFileSnapshot{path: paths.backup}},
		{name: "malformed source", source: newMigrationFileSnapshot(paths.source, []byte("{"), 0644), backup: migrationFileSnapshot{path: paths.backup}},
		{name: "different backup", source: newMigrationFileSnapshot(paths.source, []byte(v1Fixture), 0644), backup: newMigrationFileSnapshot(paths.backup, []byte("{}"), 0600)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := constructMigrationPlan(paths.source, paths, test.source, test.backup, newMigrationPlan); err == nil {
				t.Fatal("invalid planning input was accepted")
			}
		})
	}
}

func TestMigrationFileSnapshotClonesExactBytesAndMode(t *testing.T) {
	data := []byte("source")
	snapshot := newMigrationFileSnapshot("source.json", data, os.FileMode(0600))
	data[0] = 'X'
	if string(snapshot.data) != "source" || snapshot.mode != 0600 || !snapshot.exists {
		t.Fatalf("snapshot changed: %#v", snapshot)
	}
}
