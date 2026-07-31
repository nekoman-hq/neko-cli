package workflowinit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

const githubWorkflowInitCommandName = "github-workflow-init"

type githubWorkflowScaffoldPreviewer interface {
	Preview(context.Context, githubWorkflowScaffoldRequest) (*githubWorkflowScaffoldResult, *commandFailure)
}

type githubWorkflowScaffoldCreator interface {
	Create(context.Context, githubWorkflowScaffoldRequest) (*githubWorkflowScaffoldResult, *commandFailure)
}

type githubWorkflowScaffoldCommandHandler struct {
	previewer githubWorkflowScaffoldPreviewer
	creator   githubWorkflowScaffoldCreator
	clock     workflowInitClock
	root      workspace.RepositoryRoot
}

func parseGitHubWorkflowScaffoldCommandRequest(root workspace.RepositoryRoot, request plugin.Request) (githubWorkflowScaffoldCommandRequest, *commandFailure) {
	unitID, failure := optionalWorkflowFlagString(request.Flags, "unit")
	if failure != nil {
		return githubWorkflowScaffoldCommandRequest{}, failure
	}
	targetPath, failure := optionalWorkflowFlagString(request.Flags, "path")
	if failure != nil {
		return githubWorkflowScaffoldCommandRequest{}, failure
	}
	dryRun, failure := optionalWorkflowFlagBool(request.Flags, "dry-run")
	if failure != nil {
		return githubWorkflowScaffoldCommandRequest{}, failure
	}
	intent := githubWorkflowScaffoldCreateIntent
	if dryRun {
		intent = githubWorkflowScaffoldPreviewIntent
	}
	return githubWorkflowScaffoldCommandRequest{
		Scaffold: githubWorkflowScaffoldRequest{
			RepositoryRoot: root.Path(),
			UnitID:         unitID,
			TargetPath:     targetPath,
		},
		Intent: intent,
	}, nil
}

func optionalWorkflowFlagString(flags map[string]any, name string) (string, *commandFailure) {
	raw, present := flags[name]
	if !present {
		return "", nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", invalidWorkflowScaffoldFlag(name, "string")
	}
	if value != strings.TrimSpace(value) {
		return "", invalidWorkflowScaffoldFlag(name, "string without leading or trailing whitespace")
	}
	return value, nil
}

func optionalWorkflowFlagBool(flags map[string]any, name string) (bool, *commandFailure) {
	raw, present := flags[name]
	if !present {
		return false, nil
	}
	value, ok := raw.(bool)
	if !ok {
		return false, invalidWorkflowScaffoldFlag(name, "boolean")
	}
	return value, nil
}

func invalidWorkflowScaffoldFlag(name, expected string) *commandFailure {
	return failureFromMessage("INVALID_WORKFLOW_SCAFFOLD_REQUEST", fmt.Sprintf("--%s must be a %s", name, expected))
}

func (handler githubWorkflowScaffoldCommandHandler) Handle(ctx context.Context, request plugin.Request) (*plugin.Response, error) {
	log.PluginV(log.Config, "Validating workflow initialization request")
	typedRequest, failure := parseGitHubWorkflowScaffoldCommandRequest(handler.root, request)
	if failure != nil {
		log.PluginV(log.Exec, "Workflow initialization stopped during request validation")
		return workflowScaffoldFailureResponse(failure, handler.clock.Now()), nil
	}
	log.PluginV(log.Config, "Selecting workflow initialization mode")
	var result *githubWorkflowScaffoldResult
	switch typedRequest.Intent {
	case githubWorkflowScaffoldPreviewIntent:
		result, failure = handler.previewer.Preview(ctx, typedRequest.Scaffold)
	case githubWorkflowScaffoldCreateIntent:
		result, failure = handler.creator.Create(ctx, typedRequest.Scaffold)
	default:
		failure = failureFromMessage("INVALID_WORKFLOW_SCAFFOLD_REQUEST", "workflow scaffolding intent is invalid")
	}
	if failure != nil {
		log.PluginV(log.Exec, "Workflow initialization stopped before completion")
		return workflowScaffoldFailureResponse(failure, handler.clock.Now()), nil
	}
	if !result.Preview {
		log.PluginV(log.Exec, "Workflow initialization completed")
	}
	return mapGitHubWorkflowScaffoldResult(result, handler.clock.Now()), nil
}

func workflowScaffoldFailureResponse(failure *commandFailure, timestamp time.Time) *plugin.Response {
	response := mapCommandFailure(githubWorkflowInitCommandName, failure, timestamp)
	response.PresentationTable = githubWorkflowFailurePresentation(failure)
	return response
}

// HandleGitHubWorkflowInit resolves the repository root and creates or
// previews the configured canonical GitHub Actions release workflow.
func HandleGitHubWorkflowInit(request plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(request.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleGitHubWorkflowInitAt(root, request)
}

// HandleGitHubWorkflowInitAt creates or previews the configured canonical
// GitHub Actions release workflow at an explicit repository root.
func HandleGitHubWorkflowInitAt(root workspace.RepositoryRoot, request plugin.Request) (*plugin.Response, error) {
	planner := newGitHubWorkflowGenerationPlanner()
	handler := githubWorkflowScaffoldCommandHandler{
		previewer: githubWorkflowScaffoldPreviewUseCase{planner: planner},
		creator: githubWorkflowScaffoldCreateUseCase{
			planner: planner,
			writer:  atomicGitHubWorkflowOutputCreator{},
		},
		clock: systemWorkflowInitClock{},
		root:  root,
	}
	return handler.Handle(context.Background(), request)
}
