package migrate

import (
	"errors"
	"reflect"
	"testing"
)

func TestMigrationUseCasePlansOnceAndExecutesOnce(t *testing.T) {
	plan := completedMigrationPlan("/repo", migrationPaths("/repo"))
	plan.kind = newMigrationPlan
	roots := &fakeMigrationRootResolver{root: "/repo"}
	plans := &fakeMigrationPlanResolver{plan: plan}
	executor := &fakeMigrationPlanExecutor{}
	useCase := migrationUseCase{roots: roots, plans: plans, executor: executor}

	result, failure := useCase.Migrate(migrationCommandRequest{startDirectory: "/work"})
	if failure != nil {
		t.Fatalf("Migrate: %v", failure)
	}
	if roots.received != "/work" || plans.received != "/repo" || executor.calls != 1 {
		t.Fatalf("unexpected dependency calls: roots=%#v plans=%#v executor=%#v", roots, plans, executor)
	}
	if result.outcome != migrationCompleted {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestMigrationPreviewStopsBeforeExecution(t *testing.T) {
	plan := completedMigrationPlan("/repo", migrationPaths("/repo"))
	plan.kind = recoveryMigrationPlan
	plan.actions = []string{"validate migration journal"}
	executor := &fakeMigrationPlanExecutor{}
	useCase := migrationUseCase{
		roots:    &fakeMigrationRootResolver{root: "/repo"},
		plans:    &fakeMigrationPlanResolver{plan: plan},
		executor: executor,
	}

	result, failure := useCase.Migrate(migrationCommandRequest{preview: true})
	if failure != nil {
		t.Fatalf("Migrate preview: %v", failure)
	}
	wantActions := []string{"preview recovery of interrupted migration", "validate migration journal"}
	if result.outcome != migrationPreviewed || !reflect.DeepEqual(result.plan.actions, wantActions) {
		t.Fatalf("unexpected preview result: %#v", result)
	}
	if executor.calls != 0 {
		t.Fatalf("preview executed %d mutations", executor.calls)
	}
	if !reflect.DeepEqual(plan.actions, []string{"validate migration journal"}) {
		t.Fatalf("preview mutated source plan: %#v", plan.actions)
	}
}

func TestMigrationUseCaseStopsAtDependencyFailure(t *testing.T) {
	sentinel := errors.New("sentinel")

	t.Run("root", func(t *testing.T) {
		plans := &fakeMigrationPlanResolver{}
		useCase := migrationUseCase{
			roots:    &fakeMigrationRootResolver{err: sentinel},
			plans:    plans,
			executor: &fakeMigrationPlanExecutor{},
		}
		_, failure := useCase.Migrate(migrationCommandRequest{})
		if !errors.Is(failure, sentinel) || plans.calls != 0 {
			t.Fatalf("root failure = %v, plan calls = %d", failure, plans.calls)
		}
	})

	t.Run("plan", func(t *testing.T) {
		executor := &fakeMigrationPlanExecutor{}
		useCase := migrationUseCase{
			roots:    &fakeMigrationRootResolver{root: "/repo"},
			plans:    &fakeMigrationPlanResolver{err: sentinel},
			executor: executor,
		}
		_, failure := useCase.Migrate(migrationCommandRequest{})
		if !errors.Is(failure, sentinel) || executor.calls != 0 {
			t.Fatalf("plan failure = %v, executor calls = %d", failure, executor.calls)
		}
	})

	t.Run("execute", func(t *testing.T) {
		plan := completedMigrationPlan("/repo", migrationPaths("/repo"))
		plan.kind = newMigrationPlan
		useCase := migrationUseCase{
			roots:    &fakeMigrationRootResolver{root: "/repo"},
			plans:    &fakeMigrationPlanResolver{plan: plan},
			executor: &fakeMigrationPlanExecutor{err: sentinel},
		}
		_, failure := useCase.Migrate(migrationCommandRequest{})
		if !errors.Is(failure, sentinel) {
			t.Fatalf("execute failure = %v", failure)
		}
	})
}

type fakeMigrationRootResolver struct { //nolint:govet // Fake fields follow observed input, output, and failure order.
	received string
	root     string
	err      error
}

func (resolver *fakeMigrationRootResolver) Resolve(startDirectory string) (string, error) {
	resolver.received = startDirectory
	return resolver.root, resolver.err
}

type fakeMigrationPlanResolver struct { //nolint:govet // Fake fields follow observed input, output, and call state order.
	received string
	plan     migrationPlan
	err      error
	calls    int
}

func (resolver *fakeMigrationPlanResolver) Resolve(root string) (migrationPlan, error) {
	resolver.calls++
	resolver.received = root
	return resolver.plan, resolver.err
}

type fakeMigrationPlanExecutor struct { //nolint:govet // Fake fields follow observed input, failure, and call state order.
	received migrationPlan
	err      error
	calls    int
}

func (executor *fakeMigrationPlanExecutor) Execute(plan migrationPlan) error {
	executor.calls++
	executor.received = plan
	return executor.err
}
