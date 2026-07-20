// Package migrate implements the conservative V1-to-V2 migration command.
package migrate

// Plan describes the exact migration actions and content.
type Plan struct {
	RepositoryRoot string
	SourceType     string
	SourcePath     string
	ConfigPath     string
	StatePath      string
	BackupPath     string
	JournalPath    string
	UnitID         string
	Version        string
	TagPrefix      string
	Executor       string
	Delivery       string
	ConfigJSON     string
	StateJSON      string
	Actions        []string
	AlreadyDone    bool
	Recovery       bool
}

// ResolvePlan returns the current migration plan without writing files.
func ResolvePlan(startDir string) (*Plan, error) {
	root, err := (gitMigrationRootResolver{}).Resolve(startDir)
	if err != nil {
		return nil, err
	}
	plan, err := (filesystemMigrationPlanResolver{}).Resolve(root)
	if err != nil {
		return nil, err
	}
	return plan.compatibilityPlan(), nil
}

// Run executes or previews the V1-to-V2 migration.
func Run(startDir string, dryRun bool) (*Plan, error) {
	result, failure := newMigrationUseCase().Migrate(migrationCommandRequest{
		startDirectory: startDir,
		preview:        dryRun,
	})
	if failure != nil {
		return nil, failure
	}
	plan := result.plan.compatibilityPlan()
	if result.outcome == migrationCompleted {
		plan.Actions = append(plan.Actions, "migration completed")
	}
	return plan, nil
}
