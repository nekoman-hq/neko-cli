package release

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type githubWorkflowSelection struct {
	TargetPath         string
	SelectedUnit       string
	UnitsUsingWorkflow []string
}

func resolveGitHubWorkflowSelection(repository *releaseconfig.ReleaseRepository, request githubWorkflowScaffoldRequest) (githubWorkflowSelection, *CommandFailure) {
	if repository == nil || repository.SourceFormat != releaseconfig.SourceFormatV2 {
		return githubWorkflowSelection{}, failureFromMessage("UNSUPPORTED_RELEASE_SOURCE", "GitHub Actions workflow scaffolding requires a Release V2 repository")
	}
	if request.TargetPath != "" {
		if err := releaseconfig.ValidateV2WorkflowPath(request.TargetPath); err != nil {
			return githubWorkflowSelection{}, failureFromMessage("WORKFLOW_TARGET_INVALID", "--path must be a canonical .github/workflows/*.yml or .yaml path")
		}
	}

	if request.UnitID != "" {
		unit, err := releaseconfig.ResolveReleaseUnit(repository, request.UnitID, releaseconfig.UnitResolutionOptions{})
		if err != nil {
			return githubWorkflowSelection{}, failureFromError("RELEASE_UNIT_NOT_FOUND", err)
		}
		if strings.TrimSpace(unit.Workflow) == "" {
			return githubWorkflowSelection{}, failureFromMessage("WORKFLOW_NOT_CONFIGURED", fmt.Sprintf("release unit %q does not configure a GitHub Actions workflow", unit.ID))
		}
		if request.TargetPath != "" && request.TargetPath != unit.Workflow {
			return githubWorkflowSelection{}, failureFromMessage("WORKFLOW_TARGET_NOT_CONFIGURED", fmt.Sprintf("--path %q does not match release unit %q workflow %q", request.TargetPath, unit.ID, unit.Workflow))
		}
		return workflowSelectionForPath(repository, unit.Workflow, unit.ID), nil
	}

	if request.TargetPath != "" {
		selection := workflowSelectionForPath(repository, request.TargetPath, "")
		if len(selection.UnitsUsingWorkflow) == 0 {
			return githubWorkflowSelection{}, failureFromMessage("WORKFLOW_TARGET_NOT_CONFIGURED", fmt.Sprintf("--path %q is not configured by any Release V2 unit", request.TargetPath))
		}
		return selection, nil
	}

	paths := make(map[string]struct{})
	for _, unit := range repository.Units {
		if unit.Workflow != "" {
			paths[unit.Workflow] = struct{}{}
		}
	}
	if len(paths) == 0 {
		return githubWorkflowSelection{}, failureFromMessage("WORKFLOW_NOT_CONFIGURED", "Release V2 config does not define a GitHub Actions workflow path")
	}
	if len(paths) > 1 {
		configured := make([]string, 0, len(paths))
		for path := range paths {
			configured = append(configured, path)
		}
		sort.Strings(configured)
		return githubWorkflowSelection{}, &CommandFailure{
			Code:    "AMBIGUOUS_WORKFLOW_TARGET",
			Message: "Release V2 units configure multiple workflow paths; select one with --unit or --path",
			Details: map[string]any{"configured_workflows": configured},
		}
	}
	for path := range paths {
		return workflowSelectionForPath(repository, path, ""), nil
	}
	return githubWorkflowSelection{}, failureFromMessage("WORKFLOW_NOT_CONFIGURED", "Release V2 config does not define a GitHub Actions workflow path")
}

func workflowSelectionForPath(repository *releaseconfig.ReleaseRepository, targetPath, selectedUnit string) githubWorkflowSelection {
	units := make([]string, 0)
	for _, unit := range repository.Units {
		if unit.Workflow == targetPath {
			units = append(units, unit.ID)
		}
	}
	sort.Strings(units)
	return githubWorkflowSelection{TargetPath: targetPath, SelectedUnit: selectedUnit, UnitsUsingWorkflow: units}
}

func inspectGitHubWorkflowOutputTarget(root, relativePath string) (githubWorkflowOutputTarget, []byte, bool, *CommandFailure) {
	if err := releaseconfig.ValidateV2WorkflowPath(relativePath); err != nil {
		return githubWorkflowOutputTarget{}, nil, false, failureFromMessage("WORKFLOW_TARGET_INVALID", "workflow target must be a canonical .github/workflows/*.yml or .yaml path")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return githubWorkflowOutputTarget{}, nil, false, failureFromMessage("WORKFLOW_TARGET_INVALID", "repository root could not be resolved")
	}
	target := githubWorkflowOutputTarget{
		RepositoryRoot: absoluteRoot,
		RelativePath:   relativePath,
		AbsolutePath:   filepath.Join(absoluteRoot, filepath.FromSlash(relativePath)),
	}
	if failure := validateGitHubWorkflowOutputParent(target); failure != nil {
		return githubWorkflowOutputTarget{}, nil, false, failure
	}
	info, err := os.Lstat(target.AbsolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return target, nil, false, nil
		}
		return githubWorkflowOutputTarget{}, nil, false, failureFromMessage("WORKFLOW_TARGET_INVALID", "workflow target could not be inspected")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return githubWorkflowOutputTarget{}, nil, false, failureFromMessage("WORKFLOW_TARGET_SYMLINK_ESCAPE", "workflow target must not be a symlink")
	}
	if !info.Mode().IsRegular() {
		return githubWorkflowOutputTarget{}, nil, false, failureFromMessage("WORKFLOW_TARGET_INVALID", "workflow target must be a regular file")
	}
	content, err := os.ReadFile(target.AbsolutePath)
	if err != nil {
		return githubWorkflowOutputTarget{}, nil, false, failureFromMessage("WORKFLOW_PREVIEW_FAILED", "existing workflow target could not be read safely")
	}
	return target, content, true, nil
}

func validateGitHubWorkflowOutputParent(target githubWorkflowOutputTarget) *CommandFailure {
	resolvedRoot, err := filepath.EvalSymlinks(target.RepositoryRoot)
	if err != nil {
		return failureFromMessage("WORKFLOW_TARGET_INVALID", "repository root could not be resolved physically")
	}
	existingParent, err := nearestExistingWorkflowParent(filepath.Dir(target.AbsolutePath))
	if err != nil {
		return failureFromMessage("WORKFLOW_TARGET_INVALID", "workflow target parent could not be inspected")
	}
	resolvedParent, err := filepath.EvalSymlinks(existingParent)
	if err != nil {
		return failureFromMessage("WORKFLOW_TARGET_INVALID", "workflow target parent could not be resolved physically")
	}
	if !pathWithinRoot(resolvedRoot, resolvedParent) {
		return failureFromMessage("WORKFLOW_TARGET_SYMLINK_ESCAPE", "workflow target parent resolves outside the repository root")
	}
	return nil
}

func nearestExistingWorkflowParent(start string) (string, error) {
	for current := start; ; current = filepath.Dir(current) {
		info, err := os.Stat(current)
		if err == nil {
			if !info.IsDir() {
				return "", fmt.Errorf("workflow output parent %s is not a directory", current)
			}
			return current, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		next := filepath.Dir(current)
		if next == current {
			return "", fmt.Errorf("workflow output parent does not exist")
		}
	}
}

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative))
}
