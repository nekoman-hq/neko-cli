package release

import (
	"os"
	"strings"
	"testing"
)

func TestGitHubWorkflowScaffoldPlanningHasNoMutationCapability(t *testing.T) {
	source := readWorkflowScaffoldArchitectureSource(t, "github_workflow_scaffold_plan.go")
	for _, forbidden := range []string{
		"githubWorkflowOutputCreator",
		"atomicGitHubWorkflowOutputCreator",
		"os.WriteFile",
		"os.Create",
		"os.Mkdir",
		"exec.Command",
		"plugin.Response",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("workflow generation planning contains prohibited capability %q", forbidden)
		}
	}
}

func TestGitHubWorkflowRendererHasNoFilesystemCapability(t *testing.T) {
	source := readWorkflowScaffoldArchitectureSource(t, "github_actions_workflow_spec.go")
	for _, forbidden := range []string{"\"os\"", "\"path/filepath\"", "io.Writer", "os.File", "githubWorkflowOutputCreator"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("canonical workflow renderer contains prohibited capability %q", forbidden)
		}
	}
}

func TestGitHubWorkflowScaffoldHasNoGitNetworkTokenOrDispatchCapability(t *testing.T) {
	paths := []string{
		"github_workflow_scaffold_command.go",
		"github_workflow_scaffold_model.go",
		"github_workflow_scaffold_plan.go",
		"github_workflow_scaffold_response.go",
		"github_workflow_scaffold_source.go",
		"github_workflow_scaffold_target.go",
		"github_workflow_scaffold_writer.go",
	}
	for _, path := range paths {
		source := readWorkflowScaffoldArchitectureSource(t, path)
		for _, forbidden := range []string{
			"GITHUB_TOKEN",
			"TokenResolver",
			"GitHubActionsDispatcher",
			"GitHubActionsDispatchClient",
			"NewGitReleaseCoordinator",
			"exec.Command",
			"net/http",
			"http.Client",
			"PushCommit",
			"PushTag",
			"DispatchWorkflow",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains prohibited side-effect capability %q", path, forbidden)
			}
		}
	}
}

func TestGitHubWorkflowScaffoldKeepsResponseMappingAtCommandBoundary(t *testing.T) {
	for _, path := range []string{
		"github_workflow_scaffold_model.go",
		"github_workflow_scaffold_plan.go",
		"github_workflow_scaffold_source.go",
		"github_workflow_scaffold_target.go",
		"github_workflow_scaffold_writer.go",
		"github_actions_workflow_spec.go",
	} {
		source := readWorkflowScaffoldArchitectureSource(t, path)
		if strings.Contains(source, "github.com/nekoman-hq/neko-cli/pkg/plugin") || strings.Contains(source, "plugin.Response") {
			t.Fatalf("%s depends on command response presentation", path)
		}
	}
}

func TestGitHubWorkflowScaffoldAvoidsSpeculativeFrameworks(t *testing.T) {
	paths := []string{
		"github_actions_workflow_spec.go",
		"github_workflow_scaffold_command.go",
		"github_workflow_scaffold_model.go",
		"github_workflow_scaffold_plan.go",
		"github_workflow_scaffold_response.go",
		"github_workflow_scaffold_source.go",
		"github_workflow_scaffold_target.go",
		"github_workflow_scaffold_writer.go",
	}
	for _, path := range paths {
		source := readWorkflowScaffoldArchitectureSource(t, path)
		for _, forbidden := range []string{
			"ProviderRegistry",
			"WorkflowDSL",
			"WorkflowStateMachine",
			"TransitionEngine",
			"DependencyBag",
			"ServiceLocator",
			"force bool",
			"managed bool",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains prohibited workflow architecture %q", path, forbidden)
			}
		}
	}
}

func readWorkflowScaffoldArchitectureSource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
