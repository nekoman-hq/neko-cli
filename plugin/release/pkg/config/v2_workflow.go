package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const v2WorkflowPrefix = ".github/workflows/"

var v2WorkflowFilenamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.(yml|yaml)$`)

// ValidateV2ReleaseConfigStructure validates V2 config semantics without
// requiring repository files to exist.
func ValidateV2ReleaseConfigStructure(cfg *V2ReleaseConfig) error {
	if cfg == nil {
		return fmt.Errorf("v2 config is missing")
	}
	return validateV2ConfigAndState("", cfg, nil, false)
}

// ValidateV2ConfigStateStructure validates the complete V2 config/state
// contract without requiring repository-owned files to exist yet.
func ValidateV2ConfigStateStructure(cfg *V2ReleaseConfig, state *V2ReleaseState) error {
	return validateV2ConfigAndState("", cfg, state, true)
}

// ValidateV2WorkflowPath validates one canonical repository-relative GitHub
// Actions workflow path without requiring the target file to exist.
func ValidateV2WorkflowPath(workflow string) error {
	return validateV2WorkflowStructure("selected", DeliveryGitHubActions, workflow)
}

// ValidateV2ReleaseConfigAtRepositoryRoot validates V2 config semantics and
// repository-root workflow files when a real repository is available.
func ValidateV2ReleaseConfigAtRepositoryRoot(repositoryRoot string, cfg *V2ReleaseConfig, state *V2ReleaseState) error {
	return validateV2ConfigAndState(repositoryRoot, cfg, state, true)
}

func validateV2Executor(repositoryRoot, unitID string, executor V2Executor) error {
	if !executor.Type.IsValid() {
		return fmt.Errorf("v2 config unit %q has unknown executor %q", unitID, executor.Type)
	}
	delivery := executor.Delivery
	if err := validateV2SupportedDelivery(unitID, delivery); err != nil {
		return err
	}
	if err := validateV2WorkflowStructure(unitID, delivery, executor.Workflow); err != nil {
		return err
	}
	if repositoryRoot != "" {
		if err := validateV2WorkflowAtRepositoryRoot(repositoryRoot, unitID, executor.Workflow); err != nil {
			return err
		}
	}
	return nil
}

func validateV2SupportedDelivery(unitID string, delivery DeliveryType) error {
	if delivery == "" {
		return fmt.Errorf("v2 config unit %q executor.delivery is required and must be github-actions", unitID)
	}
	if !delivery.IsValid() {
		return fmt.Errorf("v2 config unit %q has unknown delivery %q", unitID, delivery)
	}
	if delivery == DeliveryLocal {
		return fmt.Errorf("v2 config unit %q local delivery is not supported for V2 releases; use github-actions delivery with a workflow", unitID)
	}
	if delivery != DeliveryGitHubActions {
		return fmt.Errorf("v2 config unit %q delivery %q is not supported for V2 releases; use github-actions", unitID, delivery)
	}
	return nil
}

func validateV2WorkflowStructure(unitID string, delivery DeliveryType, workflow string) error {
	trimmed := strings.TrimSpace(workflow)
	if delivery != DeliveryGitHubActions {
		return nil
	}
	if workflow == "" {
		return fmt.Errorf("v2 config unit %q github-actions delivery requires workflow", unitID)
	}
	if trimmed == "" {
		return fmt.Errorf("v2 config unit %q github-actions workflow must not be empty", unitID)
	}
	if workflow != trimmed {
		return fmt.Errorf("v2 config unit %q github-actions workflow must not have leading or trailing whitespace", unitID)
	}
	if strings.Contains(workflow, `\`) {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q must use forward slashes", unitID, workflow)
	}
	if strings.HasPrefix(workflow, "/") || filepath.IsAbs(workflow) {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q must be repository-root-relative", unitID, workflow)
	}
	if strings.Contains(workflow, "://") || strings.ContainsAny(workflow, "?#@") {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q must not be a URL, query, fragment, or ref", unitID, workflow)
	}
	if strings.ContainsAny(workflow, "$`~{}[]!*") {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q must not contain shell-like expansion syntax", unitID, workflow)
	}
	if !strings.HasPrefix(workflow, v2WorkflowPrefix) {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q must begin with %s", unitID, workflow, v2WorkflowPrefix)
	}
	if strings.Contains(workflow, "//") {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q must not contain duplicate separators", unitID, workflow)
	}
	if strings.Contains(workflow, "..") {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q must not contain traversal", unitID, workflow)
	}
	if strings.Contains(workflow, "/./") || strings.Contains(workflow, "./") {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q must not contain ./ segments", unitID, workflow)
	}
	if strings.HasSuffix(workflow, "/") {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q must point to a workflow file, not a directory", unitID, workflow)
	}
	filename := strings.TrimPrefix(workflow, v2WorkflowPrefix)
	if filename == "" {
		return fmt.Errorf("v2 config unit %q github-actions workflow must include a file below %s", unitID, v2WorkflowPrefix)
	}
	if strings.Contains(filename, "/") {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q must point directly to one file below %s", unitID, workflow, v2WorkflowPrefix)
	}
	if strings.TrimSpace(filename) != filename || strings.ContainsAny(filename, " \t\r\n") {
		return fmt.Errorf("v2 config unit %q github-actions workflow filename %q must not contain whitespace", unitID, filename)
	}
	if !strings.HasSuffix(filename, ".yml") && !strings.HasSuffix(filename, ".yaml") {
		return fmt.Errorf("v2 config unit %q github-actions workflow filename %q must end with lowercase .yml or .yaml", unitID, filename)
	}
	if !v2WorkflowFilenamePattern.MatchString(filename) {
		return fmt.Errorf("v2 config unit %q github-actions workflow filename %q must match [A-Za-z0-9][A-Za-z0-9._-]*.(yml|yaml)", unitID, filename)
	}
	return nil
}

func validateV2WorkflowAtRepositoryRoot(repositoryRoot, unitID, workflow string) error {
	absoluteRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return fmt.Errorf("v2 config unit %q repository root %q cannot be resolved: %w", unitID, repositoryRoot, err)
	}
	workflowPath := filepath.Join(absoluteRoot, filepath.FromSlash(workflow))
	info, err := os.Lstat(workflowPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("v2 config unit %q github-actions workflow %q does not exist", unitID, workflow)
		}
		return fmt.Errorf("v2 config unit %q github-actions workflow %q cannot be inspected: %w", unitID, workflow, err)
	}
	if info.IsDir() {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q is a directory, expected a file", unitID, workflow)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return fmt.Errorf("v2 config unit %q repository root %q cannot be resolved physically: %w", unitID, absoluteRoot, err)
	}
	resolvedWorkflow, err := filepath.EvalSymlinks(workflowPath)
	if err != nil {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q cannot be resolved physically: %w", unitID, workflow, err)
	}
	workflowsDir := filepath.Join(absoluteRoot, ".github", "workflows")
	resolvedWorkflowsDir, err := filepath.EvalSymlinks(workflowsDir)
	if err != nil {
		return fmt.Errorf("v2 config unit %q github-actions workflows directory cannot be resolved: %w", unitID, err)
	}
	if !pathInside(resolvedRoot, resolvedWorkflow) {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q resolves outside repository root", unitID, workflow)
	}
	if !pathInside(resolvedWorkflowsDir, resolvedWorkflow) {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q resolves outside .github/workflows", unitID, workflow)
	}
	resolvedInfo, err := os.Stat(resolvedWorkflow)
	if err != nil {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q cannot be inspected after symlink resolution: %w", unitID, workflow, err)
	}
	if !resolvedInfo.Mode().IsRegular() {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q is not a regular file", unitID, workflow)
	}
	if ext := filepath.Ext(resolvedWorkflow); ext != ".yml" && ext != ".yaml" {
		return fmt.Errorf("v2 config unit %q github-actions workflow %q must resolve to a .yml or .yaml file", unitID, workflow)
	}
	return nil
}

func pathInside(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative))
}
