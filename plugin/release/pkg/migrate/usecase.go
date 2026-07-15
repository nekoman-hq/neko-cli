package migrate

type migrationRootResolver interface {
	Resolve(startDirectory string) (string, error)
}

type migrationPlanResolver interface {
	Resolve(root string) (migrationPlan, error)
}

type migrationPlanExecutor interface {
	Execute(plan migrationPlan) error
}

type migrationUseCase struct {
	roots    migrationRootResolver
	plans    migrationPlanResolver
	executor migrationPlanExecutor
}

func newMigrationUseCase() migrationUseCase {
	return migrationUseCase{
		roots:    gitMigrationRootResolver{},
		plans:    filesystemMigrationPlanResolver{},
		executor: newMigrationPlanExecution(),
	}
}

func (useCase migrationUseCase) Migrate(request migrationCommandRequest) (migrationCommandResult, *migrationFailure) {
	root, err := useCase.roots.Resolve(request.startDirectory)
	if err != nil {
		return migrationCommandResult{}, &migrationFailure{kind: migrationPlanningFailure, cause: err}
	}
	plan, err := useCase.plans.Resolve(root)
	if err != nil {
		return migrationCommandResult{}, &migrationFailure{kind: migrationPlanningFailure, cause: err}
	}
	if request.preview {
		return migrationCommandResult{plan: previewMigrationPlan(plan), outcome: migrationPreviewed}, nil
	}
	if plan.kind == completedMigrationPlanKind {
		return migrationCommandResult{plan: plan, outcome: migrationAlreadyCompleted}, nil
	}
	if err := useCase.executor.Execute(plan); err != nil {
		return migrationCommandResult{}, migrationFailureFromExecution(err)
	}
	return migrationCommandResult{plan: plan, outcome: migrationCompleted}, nil
}

func previewMigrationPlan(plan migrationPlan) migrationPlan {
	preview := plan
	preview.actions = append([]string(nil), plan.actions...)
	if plan.kind == recoveryMigrationPlan {
		preview.actions = append([]string{"preview recovery of interrupted migration"}, preview.actions...)
	}
	return preview
}
