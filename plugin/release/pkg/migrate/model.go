package migrate

import (
	"os"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type migrationSourceFormat string

const (
	migrationSourceV1 migrationSourceFormat = "v1"
	migrationSourceV2 migrationSourceFormat = "v2"
)

type migrationFileSnapshot struct {
	path   string
	data   []byte
	mode   os.FileMode
	exists bool
}

func newMigrationFileSnapshot(path string, data []byte, mode os.FileMode) migrationFileSnapshot {
	return migrationFileSnapshot{
		path:   path,
		data:   append([]byte(nil), data...),
		mode:   mode,
		exists: true,
	}
}

type migrationTarget struct { //nolint:govet // Config and state values precede their matching serialized bytes.
	config     releaseconfig.V2ReleaseConfig
	state      releaseconfig.V2ReleaseState
	configJSON []byte
	stateJSON  []byte
}

type migrationPlanKind uint8

const (
	newMigrationPlan migrationPlanKind = iota + 1
	recoveryMigrationPlan
	completedMigrationPlanKind
)

type migrationPlan struct { //nolint:govet // The plan follows source, target, and presentation domain order.
	repositoryRoot string
	sourceFormat   migrationSourceFormat
	source         migrationFileSnapshot
	backup         migrationFileSnapshot
	paths          migrationPathSet
	target         migrationTarget
	kind           migrationPlanKind
	actions        []string
	unitID         string
	version        string
	tagPrefix      string
	executor       string
	delivery       string
}

func (plan migrationPlan) compatibilityPlan() *Plan {
	return &Plan{
		RepositoryRoot: plan.repositoryRoot,
		SourceType:     string(plan.sourceFormat),
		SourcePath:     plan.paths.source,
		ConfigPath:     plan.paths.config,
		StatePath:      plan.paths.state,
		BackupPath:     plan.paths.backup,
		JournalPath:    plan.paths.journal,
		UnitID:         plan.unitID,
		Version:        plan.version,
		TagPrefix:      plan.tagPrefix,
		Executor:       plan.executor,
		Delivery:       plan.delivery,
		ConfigJSON:     string(plan.target.configJSON),
		StateJSON:      string(plan.target.stateJSON),
		Actions:        append([]string(nil), plan.actions...),
		AlreadyDone:    plan.kind == completedMigrationPlanKind,
		Recovery:       plan.kind == recoveryMigrationPlan,
	}
}

type migrationCommandOutcome uint8

const (
	migrationPreviewed migrationCommandOutcome = iota + 1
	migrationCompleted
	migrationAlreadyCompleted
)

type migrationCommandResult struct {
	plan    migrationPlan
	outcome migrationCommandOutcome
}

type migrationFailure struct {
	cause error
}

func (failure *migrationFailure) Error() string {
	return failure.cause.Error()
}

func (failure *migrationFailure) Unwrap() error {
	return failure.cause
}
