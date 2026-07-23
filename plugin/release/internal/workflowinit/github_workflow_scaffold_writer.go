package workflowinit

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

const (
	githubWorkflowDirectoryMode fs.FileMode = 0755
	githubWorkflowFileMode      fs.FileMode = 0644
)

type githubWorkflowOutputCreator interface {
	Create(githubWorkflowOutputTarget, []byte) *commandFailure
}

type atomicGitHubWorkflowOutputCreator struct{}

func (atomicGitHubWorkflowOutputCreator) Create(target githubWorkflowOutputTarget, content []byte) *commandFailure {
	if err := os.MkdirAll(filepath.Dir(target.AbsolutePath), githubWorkflowDirectoryMode); err != nil {
		return failureFromMessage("WORKFLOW_WRITE_FAILED", "workflow target directory could not be created")
	}
	if failure := validateGitHubWorkflowOutputParent(target); failure != nil {
		return failure
	}
	if _, err := os.Lstat(target.AbsolutePath); err == nil {
		return workflowTargetAppearedFailure(target.RelativePath)
	} else if !os.IsNotExist(err) {
		return failureFromMessage("WORKFLOW_WRITE_FAILED", "workflow target could not be inspected before creation")
	}

	temporary, err := os.CreateTemp(filepath.Dir(target.AbsolutePath), ".neko-release-workflow-*")
	if err != nil {
		return failureFromMessage("WORKFLOW_WRITE_FAILED", "atomic workflow candidate could not be created")
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if _, err := temporary.Write(content); err != nil {
		return failureFromMessage("WORKFLOW_WRITE_FAILED", "generated workflow could not be written completely")
	}
	if err := temporary.Chmod(githubWorkflowFileMode); err != nil {
		return failureFromMessage("WORKFLOW_WRITE_FAILED", "generated workflow permissions could not be applied")
	}
	if err := temporary.Sync(); err != nil {
		return failureFromMessage("WORKFLOW_WRITE_FAILED", "generated workflow could not be synchronized")
	}
	if err := temporary.Close(); err != nil {
		return failureFromMessage("WORKFLOW_WRITE_FAILED", "generated workflow could not be closed")
	}
	if err := os.Link(temporaryPath, target.AbsolutePath); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return workflowTargetAppearedFailure(target.RelativePath)
		}
		return failureFromMessage("WORKFLOW_ATOMIC_CREATE_FAILED", "generated workflow could not be atomically created")
	}
	_ = os.Remove(temporaryPath)
	if directory, err := os.Open(filepath.Dir(target.AbsolutePath)); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func workflowTargetAppearedFailure(relativePath string) *commandFailure {
	return &commandFailure{
		Code:    "WORKFLOW_TARGET_CONFLICT",
		Message: fmt.Sprintf("workflow target %q appeared or changed before atomic creation; it was not overwritten", relativePath),
		Details: map[string]any{"target": relativePath, "guidance": "Run with --dry-run to compare the canonical generated workflow."},
	}
}

type githubWorkflowScaffoldCreateUseCase struct {
	planner githubWorkflowScaffoldPlanner
	writer  githubWorkflowOutputCreator
}

func (useCase githubWorkflowScaffoldCreateUseCase) Create(ctx context.Context, request githubWorkflowScaffoldRequest) (*githubWorkflowScaffoldResult, *commandFailure) {
	plan, failure := useCase.planner.Plan(ctx, request)
	if failure != nil {
		return nil, failure
	}
	switch plan.Classification {
	case githubWorkflowTargetUnchanged:
		log.PluginV(log.Exec, "Existing canonical workflow accepted; no write required")
		return &githubWorkflowScaffoldResult{
			Plan:      *plan,
			Action:    "none",
			Guidance:  workflowScaffoldGuidance(plan.Classification, false),
			Unchanged: true,
		}, nil
	case githubWorkflowTargetConflict:
		log.PluginV(log.Exec, "Workflow content conflict detected; overwrite refused")
		return nil, &commandFailure{
			Code:    "WORKFLOW_TARGET_CONFLICT",
			Message: fmt.Sprintf("workflow target %q already exists with different content and was not overwritten", plan.Target.RelativePath),
			Details: map[string]any{
				"target":           plan.Target.RelativePath,
				"classification":   string(plan.Classification),
				"contract_version": plan.ContractVersion,
				"guidance":         "Run with --dry-run to view the canonical generated workflow and resolve the conflict manually.",
			},
		}
	case githubWorkflowTargetCreate:
		log.PluginV(log.Exec, "Writing missing workflow file")
		if failure := useCase.writer.Create(plan.Target, plan.GeneratedContent); failure != nil {
			return nil, failure
		}
		log.PluginV(log.Exec, "Workflow file created")
		return &githubWorkflowScaffoldResult{
			Plan:     *plan,
			Action:   "created",
			Guidance: workflowScaffoldGuidance(plan.Classification, false),
			Written:  true,
		}, nil
	default:
		return nil, failureFromMessage("WORKFLOW_CREATE_FAILED", "workflow generation plan returned an unsupported classification")
	}
}
