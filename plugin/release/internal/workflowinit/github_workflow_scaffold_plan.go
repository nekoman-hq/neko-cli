package workflowinit

import (
	"bytes"
	"context"
	"fmt"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
	"gopkg.in/yaml.v3"
)

type githubActionsWorkflowRenderer interface {
	Render() ([]byte, error)
}

type canonicalGitHubActionsWorkflowRenderer struct{}

func (canonicalGitHubActionsWorkflowRenderer) Render() ([]byte, error) {
	return releaseworkflow.RenderCanonicalGitHubActionsReleaseWorkflow()
}

type githubWorkflowScaffoldPlanner interface {
	Plan(context.Context, githubWorkflowScaffoldRequest) (*githubWorkflowGenerationPlan, *commandFailure)
}

type githubWorkflowGenerationPlanner struct {
	sources  githubWorkflowScaffoldSourceReader
	renderer githubActionsWorkflowRenderer
}

func newGitHubWorkflowGenerationPlanner() githubWorkflowGenerationPlanner {
	return githubWorkflowGenerationPlanner{
		sources:  filesystemGitHubWorkflowScaffoldSourceReader{},
		renderer: canonicalGitHubActionsWorkflowRenderer{},
	}
}

func (planner githubWorkflowGenerationPlanner) Plan(_ context.Context, request githubWorkflowScaffoldRequest) (*githubWorkflowGenerationPlan, *commandFailure) {
	repository, failure := planner.sources.Read(request.RepositoryRoot)
	if failure != nil {
		return nil, failure
	}
	selection, failure := resolveGitHubWorkflowSelection(repository, request)
	if failure != nil {
		return nil, failure
	}
	target, existingContent, exists, failure := inspectGitHubWorkflowOutputTarget(request.RepositoryRoot, selection.TargetPath)
	if failure != nil {
		return nil, failure
	}
	generated, err := planner.renderer.Render()
	if err != nil {
		return nil, failureFromMessage("WORKFLOW_CONTENT_INVALID", "canonical GitHub Actions workflow could not be rendered")
	}
	if err := validateGeneratedGitHubWorkflow(generated); err != nil {
		return nil, failureFromMessage("WORKFLOW_CONTENT_INVALID", "generated GitHub Actions workflow is invalid")
	}

	plan := &githubWorkflowGenerationPlan{
		Target:             target,
		Classification:     githubWorkflowTargetCreate,
		SelectedUnit:       selection.SelectedUnit,
		UnitsUsingWorkflow: append([]string(nil), selection.UnitsUsingWorkflow...),
		GeneratedContent:   append([]byte(nil), generated...),
		ContractVersion:    releaseworkflow.GitHubActionsReleaseWorkflowContractVersion,
	}
	if !exists {
		return plan, nil
	}
	if bytes.Equal(existingContent, generated) {
		plan.Classification = githubWorkflowTargetUnchanged
		return plan, nil
	}
	plan.Classification = githubWorkflowTargetConflict
	plan.ConflictReason = "target exists with content that differs from the canonical generated workflow"
	return plan, nil
}

func validateGeneratedGitHubWorkflow(content []byte) error {
	if len(content) == 0 || content[len(content)-1] != '\n' {
		return fmt.Errorf("generated workflow must be non-empty and end with a newline")
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return fmt.Errorf("parse generated workflow: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("generated workflow must contain one YAML mapping document")
	}
	return nil
}

type githubWorkflowScaffoldPreviewUseCase struct {
	planner githubWorkflowScaffoldPlanner
}

func (useCase githubWorkflowScaffoldPreviewUseCase) Preview(ctx context.Context, request githubWorkflowScaffoldRequest) (*githubWorkflowScaffoldResult, *commandFailure) {
	plan, failure := useCase.planner.Plan(ctx, request)
	if failure != nil {
		return nil, failure
	}
	return &githubWorkflowScaffoldResult{
		Plan:      *plan,
		Action:    previewWorkflowAction(plan.Classification),
		Guidance:  workflowScaffoldGuidance(plan.Classification, true),
		Unchanged: plan.Classification == githubWorkflowTargetUnchanged,
		Preview:   true,
	}, nil
}

func previewWorkflowAction(classification githubWorkflowTargetClassification) string {
	switch classification {
	case githubWorkflowTargetCreate:
		return "would-create"
	case githubWorkflowTargetUnchanged:
		return "none"
	default:
		return "blocked"
	}
}

func workflowScaffoldGuidance(classification githubWorkflowTargetClassification, preview bool) string {
	switch classification {
	case githubWorkflowTargetCreate:
		if preview {
			return "Review the generated workflow, then run without --dry-run to create it."
		}
		return "Set the required repository version variables and replace the consumer-owned failing placeholder before releasing."
	case githubWorkflowTargetUnchanged:
		return "Workflow already current; no file was rewritten."
	default:
		return "The target differs and will not be overwritten; compare it with this preview and resolve the file manually."
	}
}
