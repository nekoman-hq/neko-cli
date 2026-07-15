package main

import (
	"strings"
	"testing"
)

func TestMigrationCommandPlanningAndExecutionBoundariesRemainSeparated(t *testing.T) {
	handler := readQueryArchitectureFile(t, "pkg/migrate/handler.go")
	for _, required := range []string{
		"parseMigrationCommandRequest(req.Flags, req.Context.WorkingDir)",
		"handler.useCase.Migrate(request)",
		"mapMigrationCommandResponse(",
	} {
		if !strings.Contains(handler, required) {
			t.Fatalf("migration handler must contain %q", required)
		}
	}
	for _, forbidden := range []string{
		"LoadV2Repository(",
		"os.ReadFile(",
		"os.WriteFile(",
		"os.Remove(",
		"os.Rename(",
		"exec.Command(",
		"&plugin.Response{",
	} {
		if strings.Contains(handler, forbidden) {
			t.Fatalf("migration handler must not contain %q", forbidden)
		}
	}

	useCase := readQueryArchitectureFile(t, "pkg/migrate/usecase.go")
	for _, forbidden := range []string{
		"pkg/plugin",
		"plugin.Response",
		"map[string]any",
		"os.ReadFile(",
		"os.WriteFile(",
		"os.Remove(",
		"os.Rename(",
		"exec.Command(",
	} {
		if strings.Contains(useCase, forbidden) {
			t.Fatalf("migration use case must not contain %q", forbidden)
		}
	}

	planner := readQueryArchitectureFile(t, "pkg/migrate/planner.go")
	for _, forbidden := range []string{
		"\"os\"",
		"pkg/plugin",
		"plugin.Response",
		"AtomicWrite",
		"CreateAtomicFileReplacement",
	} {
		if strings.Contains(planner, forbidden) {
			t.Fatalf("migration planner must not contain %q", forbidden)
		}
	}

	policy := readQueryArchitectureFile(t, "pkg/migrate/policy.go")
	for _, forbidden := range []string{
		"\"os\"",
		"path/filepath",
		"pkg/config",
		"pkg/plugin",
		"plugin.Response",
		"map[",
		"func()",
	} {
		if strings.Contains(policy, forbidden) {
			t.Fatalf("migration policy must not contain %q", forbidden)
		}
	}
}

func TestMigrationExecutionReusesPairPersistenceAndRetainsSafeCleanupOrder(t *testing.T) {
	execution := readQueryArchitectureFile(t, "pkg/migrate/execution.go")
	verifyIndex := strings.Index(execution, "execution.targetVerifier.Verify(plan)")
	archiveIndex := strings.Index(execution, "execution.sourceArchiver.Archive(plan)")
	if verifyIndex < 0 || archiveIndex < 0 || verifyIndex >= archiveIndex {
		t.Fatalf("target verification must remain before source archive")
	}
	for _, forbidden := range []string{
		"pkg/plugin",
		"plugin.Response",
		"os.ReadFile(",
		"os.WriteFile(",
		"os.Remove(",
		"os.Rename(",
		"AtomicWrite",
		"map[string]",
	} {
		if strings.Contains(execution, forbidden) {
			t.Fatalf("migration execution must not contain %q", forbidden)
		}
	}

	adapters := readQueryArchitectureFile(t, "pkg/migrate/execution_adapters.go")
	if !strings.Contains(adapters, "releaseconfig.NewV2ReleasePairPersister(root)") {
		t.Fatal("migration target persistence must reuse the Stage 6 pair persister")
	}
	for _, forbidden := range []string{
		"CreateAtomicFileReplacement",
		"AtomicWriteFile(",
		"plugin.Response",
	} {
		if strings.Contains(adapters, forbidden) {
			t.Fatalf("migration adapters must not contain duplicate target persistence %q", forbidden)
		}
	}

	initRepository := readQueryArchitectureFile(t, "pkg/init/repository.go")
	if !strings.Contains(initRepository, "config.NewV2ReleasePairPersister(root)") {
		t.Fatal("init must retain the shared pair persister")
	}
}

func TestMigrationRefactorIntroducesNoGenericWorkflowFramework(t *testing.T) {
	for _, path := range []string{
		"pkg/migrate/model.go",
		"pkg/migrate/policy.go",
		"pkg/migrate/planner.go",
		"pkg/migrate/usecase.go",
		"pkg/migrate/execution.go",
		"pkg/migrate/execution_adapters.go",
	} {
		source := readQueryArchitectureFile(t, path)
		for _, forbidden := range []string{
			"MigrationService",
			"MigrationManager",
			"MigrationCoordinator",
			"RecoveryManager",
			"RecoveryEngine",
			"MigrationProcessor",
			"MigrationContext",
			"MigrationDependencies",
			"TransitionEngine",
			"StateMachine",
			"Pipeline",
			"[]Step",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s introduces forbidden workflow abstraction %q", path, forbidden)
			}
		}
	}
}
