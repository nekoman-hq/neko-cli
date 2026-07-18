//nolint:staticcheck // Migration planning intentionally converts deprecated V1 values.
package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const defaultMigratedWorkflow = ".github/workflows/release-default.yml"

func constructMigrationPlan(
	root string,
	paths migrationPathSet,
	source migrationFileSnapshot,
	backup migrationFileSnapshot,
	kind migrationPlanKind,
) (migrationPlan, error) {
	if kind != newMigrationPlan && kind != recoveryMigrationPlan {
		return migrationPlan{}, fmt.Errorf("unsupported migration plan kind %d", kind)
	}
	if !source.exists {
		return migrationPlan{}, fmt.Errorf("migration source %s is missing", paths.source)
	}
	if backup.exists && !bytes.Equal(backup.data, source.data) {
		return migrationPlan{}, fmt.Errorf("migration conflict: existing backup %s differs from active V1 config", paths.backup)
	}

	var v1 releaseconfig.V1ReleaseConfig
	if err := json.Unmarshal(source.data, &v1); err != nil {
		return migrationPlan{}, fmt.Errorf("parse V1 config %s: %w", paths.source, err)
	}
	if err := releaseconfig.V1Validate(&v1); err != nil {
		return migrationPlan{}, err
	}

	target := migrationTarget{
		config: releaseconfig.V2ReleaseConfig{
			SchemaVersion: 2,
			Units: []releaseconfig.V2Unit{
				{
					ID:               "default",
					DisplayName:      v1.ProjectName,
					Paths:            []string{"**"},
					WorkingDirectory: ".",
					TagPrefix:        "v",
					Executor: releaseconfig.V2Executor{
						Type:     releaseconfig.ExecutorType(v1.ReleaseSystem),
						Delivery: releaseconfig.DeliveryGitHubActions,
						Workflow: defaultMigratedWorkflow,
					},
				},
			},
		},
		state: releaseconfig.V2ReleaseState{
			SchemaVersion: 2,
			Units: map[string]releaseconfig.V2UnitState{
				"default": {Version: v1.Version},
			},
		},
	}
	if err := releaseconfig.ValidateV2("", &target.config, &target.state); err != nil {
		return migrationPlan{}, fmt.Errorf("validate planned V2 configuration: %w", err)
	}

	configJSON, err := releaseconfig.CanonicalV2Config(target.config)
	if err != nil {
		return migrationPlan{}, err
	}
	stateJSON, err := releaseconfig.CanonicalV2State(target.state)
	if err != nil {
		return migrationPlan{}, err
	}
	target.configJSON = append([]byte(nil), configJSON...)
	target.stateJSON = append([]byte(nil), stateJSON...)

	return migrationPlan{
		repositoryRoot:  root,
		sourceFormat:    migrationSourceV1,
		source:          newMigrationFileSnapshot(source.path, source.data, source.mode),
		backup:          cloneMigrationFileSnapshot(backup),
		paths:           paths,
		target:          target,
		kind:            kind,
		targetOperation: persistMigrationTarget,
		sourceOperation: archiveMigrationSource,
		unitID:          "default",
		version:         v1.Version,
		tagPrefix:       "v",
		executor:        string(v1.ReleaseSystem),
		delivery:        string(releaseconfig.DeliveryGitHubActions),
		actions: []string{
			"create .neko directory",
			"write migration journal",
			"write .neko/release.config.json",
			"write .neko/release.state.json",
			"archive .release.neko.json to .release.neko.json.v1.bak",
			"validate migrated V2 configuration",
			"remove migration journal",
		},
	}, nil
}

func completedMigrationPlan(root string, paths migrationPathSet) migrationPlan {
	return migrationPlan{
		repositoryRoot: root,
		sourceFormat:   migrationSourceV2,
		paths:          paths,
		kind:           completedMigrationPlanKind,
		actions:        []string{"already migrated; no changes required"},
	}
}

func cloneMigrationFileSnapshot(snapshot migrationFileSnapshot) migrationFileSnapshot {
	if !snapshot.exists {
		return migrationFileSnapshot{path: snapshot.path}
	}
	return newMigrationFileSnapshot(snapshot.path, snapshot.data, snapshot.mode)
}
