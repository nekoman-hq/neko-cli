package release

import (
	"os"
	"strings"
	"testing"
)

func TestCommandHandlersRemainPresentationBoundaries(t *testing.T) {
	source := readCommandBoundarySource(t, "command_handler.go")
	for _, forbidden := range []string{
		"LoadReleaseRepository",
		"ResolveReleaseUnit",
		"BuildReleaseExecutionContext",
		"NewGitHubActionsReleaseRunner",
		"ReleaseExecutionJournalStore",
		"resumeJournal",
		"GITHUB_TOKEN",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("command_handler.go contains application responsibility %q", forbidden)
		}
	}
}

func TestCommandApplicationOperationsDoNotConstructPluginResponses(t *testing.T) {
	for _, path := range []string{"handler.go", "resume.go", "github_actions_release_runner.go", "github_actions_release_use_case.go"} {
		source := readCommandBoundarySource(t, path)
		if strings.Contains(source, "github.com/nekoman-hq/neko-cli/pkg/plugin") || strings.Contains(source, "plugin.Response") {
			t.Fatalf("%s depends on plugin response presentation", path)
		}
	}
}

func TestGitHubActionsReleaseRunnerRemainsFacade(t *testing.T) {
	source := readCommandBoundarySource(t, "github_actions_release_runner.go")
	for _, forbidden := range []string{
		"BeginPending",
		"ConfirmPhase",
		"BuildReleaseExecutionJournal",
		"NewReleaseExecutionJournalStore",
		"ResolveVersionMaterializer",
		"PushCommit",
		"NewGitHubActionsDispatcher",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("github_actions_release_runner.go contains extracted responsibility %q", forbidden)
		}
	}
}

func TestResumeArchitectureHasNoUniversalPlaybookOrBooleanModes(t *testing.T) {
	for _, path := range []string{"resume.go", "resume_policy.go", "resume_operations.go"} {
		source := readCommandBoundarySource(t, path)
		for _, forbidden := range []string{
			"func resumeJournal",
			"loadOnly bool",
			"pushed bool",
			"ResumeManager",
			"ResumeService",
			"RecoveryCoordinator",
			"TransitionEngine",
			"WorkflowPipeline",
			"PhaseProcessor",
			"DependencyBag",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains prohibited resume architecture %q", path, forbidden)
			}
		}
	}
}

func TestResumeOperationsReuseFocusedActiveReleaseCapabilities(t *testing.T) {
	source := readCommandBoundarySource(t, "resume.go")
	for _, capability := range []string{
		"createGitHubActionsReleaseTag",
		"prepareGitHubActionsReleaseDispatch",
		"pushGitHubActionsReleaseCommit",
		"pushGitHubActionsReleaseTag",
		"dispatchGitHubActionsReleaseWorkflow",
		"confirmGitHubActionsReleaseHandoff",
	} {
		if !strings.Contains(source, capability) {
			t.Fatalf("resume composition does not reuse active release capability %q", capability)
		}
	}
}

func readCommandBoundarySource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
