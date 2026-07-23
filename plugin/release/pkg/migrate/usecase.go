package migrate

import "github.com/nekoman-hq/neko-cli/pkg/log"

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

func newMigrationUseCaseAt(root string) migrationUseCase {
	return migrationUseCase{
		roots:    fixedMigrationRootResolver{root: root},
		plans:    filesystemMigrationPlanResolver{},
		executor: newMigrationPlanExecution(),
	}
}

func (useCase migrationUseCase) Migrate(request migrationCommandRequest) (migrationCommandResult, *migrationFailure) {
	log.PluginV(log.Config, "Resolving migration repository root")
	root, err := useCase.roots.Resolve(request.startDirectory)
	if err != nil {
		return migrationCommandResult{}, &migrationFailure{kind: migrationPlanningFailure, cause: err}
	}
	plan, err := useCase.plans.Resolve(root)
	if err != nil {
		return migrationCommandResult{}, &migrationFailure{kind: migrationPlanningFailure, cause: err}
	}
	log.PluginV(log.Config, "Evaluating migration dry-run decision")
	if request.preview {
		log.PluginV(log.Exec, "Dry-run selected; no migration files written")
		log.PluginV(log.Exec, "Migration planning completed")
		return migrationCommandResult{plan: previewMigrationPlan(plan), outcome: migrationPreviewed}, nil
	}
	if plan.kind == completedMigrationPlanKind {
		log.PluginV(log.Exec, "Migration already complete; no files written")
		log.PluginV(log.Exec, "Migration command completed")
		return migrationCommandResult{plan: plan, outcome: migrationAlreadyCompleted}, nil
	}
	if err := useCase.executor.Execute(plan); err != nil {
		return migrationCommandResult{}, migrationFailureFromExecution(err)
	}
	log.PluginV(log.Exec, "Migration command completed")
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

type fixedMigrationRootResolver struct {
	root string
}

func (resolver fixedMigrationRootResolver) Resolve(string) (string, error) {
	return resolver.root, nil
}
