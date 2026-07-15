package release

import (
	"os"
	"strings"
	"testing"
)

func TestV1ApplicationAndPlannerRemainInfrastructureFree(t *testing.T) {
	for _, path := range []string{"v1_release_use_case.go", "v1_release_planner.go", "v1_release_model.go"} {
		source := readCommandBoundarySource(t, path)
		for _, forbidden := range []string{
			"plugin.Response",
			"pkg/plugin",
			"exec.Command",
			"os.ReadFile",
			"os.WriteFile",
			"GITHUB_TOKEN",
			"SourceFormatV2",
			"ReleaseExecutionJournal",
			"Dispatch",
			"flags map[string]any",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains forbidden application dependency %q", path, forbidden)
			}
		}
	}
}

func TestActiveReleaseSelectionDoesNotUseMixedV1Orchestration(t *testing.T) {
	source := readCommandBoundarySource(t, "handler.go")
	for _, forbidden := range []string{
		"repository.SourceFormat ==",
		"BuildReleaseExecutionContext(repository",
		"NewReleaseService",
		"return startLegacyRelease",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("handler.go retains mixed active orchestration %q", forbidden)
		}
	}
	if !strings.Contains(source, "BuildV2ReleaseExecutionContext") {
		t.Fatal("selected V2 application does not use the V2-only context builder")
	}
}

func TestV1SubsystemHasNoGenericCompatibilityFramework(t *testing.T) {
	for _, path := range []string{
		"release_source_selector.go",
		"v1_release_application.go",
		"v1_release_use_case.go",
		"v1_release_model.go",
		"v1_release_planner.go",
	} {
		source := readCommandBoundarySource(t, path)
		for _, forbidden := range []string{
			"CompatibilityManager",
			"VersionedReleaseEngine",
			"TransitionEngine",
			"WorkflowPipeline",
			"DependencyBag",
			"[]func(",
			"legacy bool",
			"compatibility bool",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains prohibited compatibility framework %q", path, forbidden)
			}
		}
	}
}

func TestMigrationDoesNotDependOnV1ExecutionSubsystem(t *testing.T) {
	entries, err := os.ReadDir("../migrate")
	if err != nil {
		t.Fatalf("read migrate package: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		data, err := os.ReadFile("../migrate/" + entry.Name())
		if err != nil {
			t.Fatalf("read migrate/%s: %v", entry.Name(), err)
		}
		source := string(data)
		for _, forbidden := range []string{
			"plugin/release/pkg/release\"",
			"V1ReleaseExecution",
			"V1ExecutorRequest",
			"RevertGitRelease",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("migrate/%s depends on V1 execution through %q", entry.Name(), forbidden)
			}
		}
	}
}

func TestProductionV1CompositionUsesFixedConcreteExecutors(t *testing.T) {
	source := readCommandBoundarySource(t, "../../main.go")
	for _, required := range []string{
		"goreleaser.NewV1Executor()",
		"jreleaser.NewV1Executor()",
		"releaseit.NewV1Executor()",
		"HandleReleaseWithV1Executors",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("main.go is missing explicit V1 composition %q", required)
		}
	}
	for _, forbidden := range []string{
		"_ \"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release/tool\"",
		"release.HandleRelease(req",
		"release.Register(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("main.go retains mutable V1 lookup through %q", forbidden)
		}
	}
}

func TestV1ExecutorOrchestrationUsesOnlyFocusedPorts(t *testing.T) {
	for _, path := range []string{
		"tool/goreleaser/goreleaser.go",
		"tool/jreleaser/jreleaser.go",
		"tool/releaseit/release_it.go",
	} {
		source := readCommandBoundarySource(t, path)
		for _, forbidden := range []string{
			"exec.Command",
			"os.Stat",
			"os.Environ",
			"GetPAT",
			"time.Now",
			"SourceFormatV2",
			"releasePrepared",
			"release2.Register",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains hidden V1 infrastructure or mixed selection %q", path, forbidden)
			}
		}
	}
}

func TestLegacyRegistrationIsConfinedToCompatibilityAggregator(t *testing.T) {
	for _, path := range []string{
		"tool/goreleaser/goreleaser.go",
		"tool/jreleaser/jreleaser.go",
		"tool/releaseit/release_it.go",
	} {
		if source := readCommandBoundarySource(t, path); strings.Contains(source, "Register(") {
			t.Fatalf("%s self-registers outside the compatibility aggregator", path)
		}
	}
	aggregator := readCommandBoundarySource(t, "tool/register.go")
	if count := strings.Count(aggregator, "release.Register("); count != 3 {
		t.Fatalf("compatibility registrations = %d, want 3", count)
	}
}
