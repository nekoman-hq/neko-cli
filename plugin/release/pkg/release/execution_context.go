package release

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// ReleaseExecutionContext is the schema-neutral release execution input.
//
// V2 dry-runs and journaled GitHub Actions releases build this context so
// delivery, executor ownership, state, tag, and workflow facts stay explicit.
//
//nolint:govet // Logical release-domain order is clearer than fieldalignment ordering here.
type ReleaseExecutionContext struct {
	RepositoryRoot string
	Unit           releaseconfig.ReleaseUnit
	UnitRoot       string
	CurrentVersion string
	NextVersion    string
	Tag            string
	TagSpec        releaseconfig.TagSpec
	ReleaseKind    Type
	DryRun         bool
	Executor       string
	Delivery       string
	Workflow       string
	SourceFormat   releaseconfig.SourceFormat
	Capabilities   ExecutorCapabilities
	DeliveryMode   DeliveryContract
}

// BuildReleaseExecutionContext is a compatibility facade for callers that
// still pass a normalized repository containing either source format. Active
// command composition selects the source first and uses the V2-only builder.
//
// Deprecated: use BuildV2ReleaseExecutionContext for V2 release contexts, or
// PlanV1Release for V1 planning.
func BuildReleaseExecutionContext(repository *releaseconfig.ReleaseRepository, unit releaseconfig.ReleaseUnit, releaseType Type, dryRun bool) (*ReleaseExecutionContext, error) {
	if repository == nil {
		return nil, fmt.Errorf("release repository is missing")
	}
	repositoryRoot, err := absoluteExistingDir(repository.RepositoryRoot, "repository root")
	if err != nil {
		return nil, err
	}

	unitRoot, err := resolveUnitRoot(repositoryRoot, repository.SourceFormat, unit)
	if err != nil {
		return nil, err
	}
	return assembleReleaseExecutionContext(repositoryRoot, unitRoot, repository.SourceFormat, unit, releaseType, dryRun)
}

// BuildV2ReleaseExecutionContext builds the selected V2 application context
// without inspecting or branching on source format.
func BuildV2ReleaseExecutionContext(repositoryRoot string, unit releaseconfig.ReleaseUnit, releaseType Type, dryRun bool) (*ReleaseExecutionContext, error) {
	repositoryRoot, err := absoluteExistingDir(repositoryRoot, "repository root")
	if err != nil {
		return nil, err
	}
	unitRoot, err := resolveV2UnitRoot(repositoryRoot, unit)
	if err != nil {
		return nil, err
	}
	return assembleReleaseExecutionContext(repositoryRoot, unitRoot, releaseconfig.SourceFormatV2, unit, releaseType, dryRun)
}

func assembleReleaseExecutionContext(
	repositoryRoot string,
	unitRoot string,
	sourceFormat releaseconfig.SourceFormat,
	unit releaseconfig.ReleaseUnit,
	releaseType Type,
	dryRun bool,
) (*ReleaseExecutionContext, error) {
	plan, err := PlanUnitVersionBump(unit, releaseType)
	if err != nil {
		return nil, err
	}
	tagSpec, err := releaseconfig.NewTagSpec(unit.TagPrefix)
	if err != nil {
		return nil, err
	}
	delivery, err := ResolveDelivery(unit.Delivery)
	if err != nil {
		return nil, err
	}
	capabilities, err := ResolveExecutorCapabilities(unit.ExecutorType)
	if err != nil {
		return nil, err
	}

	return &ReleaseExecutionContext{
		RepositoryRoot: repositoryRoot,
		Unit:           unit,
		UnitRoot:       unitRoot,
		CurrentVersion: plan.CurrentVersion,
		NextVersion:    plan.NextVersion,
		Tag:            plan.Tag,
		TagSpec:        tagSpec,
		ReleaseKind:    releaseType,
		DryRun:         dryRun,
		Executor:       plan.Executor,
		Delivery:       plan.Delivery,
		Workflow:       unit.Workflow,
		SourceFormat:   sourceFormat,
		Capabilities:   capabilities,
		DeliveryMode:   delivery,
	}, nil
}

func absoluteExistingDir(path, label string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%s is missing", label)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%s %q cannot be resolved: %w", label, path, err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s %q does not exist", label, path)
		}
		return "", fmt.Errorf("%s %q cannot be inspected: %w", label, path, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s %q is not a directory", label, path)
	}
	return absolute, nil
}

func resolveUnitRoot(repositoryRoot string, sourceFormat releaseconfig.SourceFormat, unit releaseconfig.ReleaseUnit) (string, error) {
	if sourceFormat == releaseconfig.SourceFormatV1 {
		return repositoryRoot, nil
	}
	return resolveV2UnitRoot(repositoryRoot, unit)
}

func resolveV2UnitRoot(repositoryRoot string, unit releaseconfig.ReleaseUnit) (string, error) {
	workingDirectory := unit.WorkingDirectory
	if workingDirectory == "" {
		workingDirectory = "."
	}
	if filepath.IsAbs(workingDirectory) {
		return "", fmt.Errorf("unit %q workingDirectory %q must be relative", unit.ID, workingDirectory)
	}
	clean := filepath.Clean(workingDirectory)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unit %q workingDirectory %q leaves the repository", unit.ID, workingDirectory)
	}
	unitRoot, err := absoluteExistingDir(filepath.Join(repositoryRoot, clean), fmt.Sprintf("unit %q workingDirectory", unit.ID))
	if err != nil {
		return "", err
	}
	if err := ensureInsideRepository(repositoryRoot, unitRoot, unit.ID); err != nil {
		return "", err
	}
	if err := ensurePhysicallyInsideRepository(repositoryRoot, unitRoot, unit.ID); err != nil {
		return "", err
	}
	return unitRoot, nil
}

func ensureInsideRepository(repositoryRoot, unitRoot, unitID string) error {
	relative, err := filepath.Rel(repositoryRoot, unitRoot)
	if err != nil {
		return fmt.Errorf("unit %q workingDirectory cannot be related to repository root: %w", unitID, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("unit %q workingDirectory leaves the repository", unitID)
	}
	return nil
}

func ensurePhysicallyInsideRepository(repositoryRoot, unitRoot, unitID string) error {
	resolvedRoot, err := filepath.EvalSymlinks(repositoryRoot)
	if err != nil {
		return fmt.Errorf("unit %q repository root cannot be resolved physically: %w", unitID, err)
	}
	resolvedUnitRoot, err := filepath.EvalSymlinks(unitRoot)
	if err != nil {
		return fmt.Errorf("unit %q workingDirectory cannot be resolved physically: %w", unitID, err)
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedUnitRoot)
	if err != nil {
		return fmt.Errorf("unit %q workingDirectory cannot be physically related to repository root: %w", unitID, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("unit %q workingDirectory resolves outside repository", unitID)
	}
	return nil
}
