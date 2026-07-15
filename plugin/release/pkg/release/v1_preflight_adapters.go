//nolint:staticcheck // V1 compatibility adapters intentionally use deprecated V1 models.
package release

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	legacygit "github.com/nekoman-hq/neko-cli/plugin/release/pkg/git"
)

type v1RequirementFileInspector interface {
	Exists(string, string) (bool, error)
}

type systemV1ReleaseRequirements struct {
	tokens v1LegacyTokenResolver
	files  v1RequirementFileInspector
}

func newSystemV1ReleaseRequirements() systemV1ReleaseRequirements {
	return systemV1ReleaseRequirements{
		tokens: systemV1LegacyTokenResolver{},
		files:  NewSystemV1FileInspector(),
	}
}

func (requirements systemV1ReleaseRequirements) Validate(intent V1ReleaseIntent) error {
	if intent.Config == nil {
		return fmt.Errorf("release configuration is missing")
	}
	log.PluginV(
		log.Config,
		"Validating release requirements for %s in %s",
		log.ColorText(log.ColorCyan, string(intent.Config.ReleaseSystem)),
		log.ColorText(log.ColorGreen, intent.RepositoryRoot),
	)
	if _, err := requirements.tokens.Resolve(); err != nil {
		return err
	}
	files, err := requiredReleaseSystemFiles(string(intent.Config.ReleaseSystem))
	if err != nil {
		return err
	}
	for _, file := range files {
		target := filepath.Join(intent.RepositoryRoot, file)
		exists, inspectErr := requirements.files.Exists(intent.RepositoryRoot, file)
		if inspectErr != nil {
			return fmt.Errorf("failed to check %s: %w", target, inspectErr)
		}
		if exists {
			return nil
		}
	}
	if len(files) == 1 {
		return fmt.Errorf("required %s configuration missing: %s not found", intent.Config.ReleaseSystem, files[0])
	}
	return fmt.Errorf(
		"required %s configuration missing: none of %s were found",
		intent.Config.ReleaseSystem,
		joinQuotedFiles(files),
	)
}

type v1PreflightRepository interface {
	Observe(string) error
	IsClean(string) error
	EnsureNotDetached(string) error
	OnMainBranch(string) error
	HasUpstream(string) error
	IsUpToDate(string) error
}

type systemV1PreflightRepository struct{}

func (systemV1PreflightRepository) Observe(root string) error {
	return inV1Repository(root, func() error {
		_, err := legacygit.Current()
		return err
	})
}
func (systemV1PreflightRepository) IsClean(root string) error {
	return inV1Repository(root, legacygit.IsClean)
}
func (systemV1PreflightRepository) EnsureNotDetached(root string) error {
	return inV1Repository(root, legacygit.EnsureNotDetached)
}
func (systemV1PreflightRepository) OnMainBranch(root string) error {
	return inV1Repository(root, legacygit.OnMainBranch)
}
func (systemV1PreflightRepository) HasUpstream(root string) error {
	return inV1Repository(root, legacygit.HasUpstream)
}
func (systemV1PreflightRepository) IsUpToDate(root string) error {
	return inV1Repository(root, legacygit.IsUpToDate)
}

func inV1Repository(repositoryRoot string, operation func() error) error {
	if repositoryRoot == "" {
		return operation()
	}
	previous, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to resolve current working directory: %w", err)
	}
	if err := os.Chdir(repositoryRoot); err != nil {
		return fmt.Errorf("failed to enter repository root %s: %w", repositoryRoot, err)
	}
	defer func() { _ = os.Chdir(previous) }()
	return operation()
}

type legacyV1Preflight struct {
	requirements v1ReleaseRequirements
	repository   v1PreflightRepository
}

func (preflight legacyV1Preflight) Check(intent V1ReleaseIntent) *V1ReleaseFailure {
	log.PluginV(log.Preflight, "Running pre-flight checks")
	if err := preflight.requirements.Validate(intent); err != nil {
		return newFatalV1ReleaseFailure("RELEASE_REQUIREMENTS_INVALID", err)
	}
	if err := preflight.repository.IsClean(intent.RepositoryRoot); err != nil {
		return newFatalV1ReleaseFailure("UNCOMMITTED_CHANGES", err)
	}
	if err := preflight.repository.EnsureNotDetached(intent.RepositoryRoot); err != nil {
		return newFatalV1ReleaseFailure("DETACHED_HEAD", err)
	}
	if err := preflight.repository.OnMainBranch(intent.RepositoryRoot); err != nil {
		return newFatalV1ReleaseFailure("INCORRECT_BRANCH", err)
	}
	if err := preflight.repository.HasUpstream(intent.RepositoryRoot); err != nil {
		return newFatalV1ReleaseFailure("NO_UPSTREAM_BRANCH", err)
	}
	if err := preflight.repository.IsUpToDate(intent.RepositoryRoot); err != nil {
		return newFatalV1ReleaseFailure("BRANCH_OUT_OF_DATE", err)
	}
	log.PluginV(log.Preflight, "\uF00C Preflight checks succeeded!")
	return nil
}

type systemV1ReleasePreflight struct {
	requirements v1ReleaseRequirements
	repository   v1PreflightRepository
}

func (preflight systemV1ReleasePreflight) Check(intent V1ReleaseIntent) *V1ReleaseFailure {
	_ = preflight.repository.Observe(intent.RepositoryRoot)
	return legacyV1Preflight{
		requirements: preflight.requirements,
		repository:   preflight.repository,
	}.Check(intent)
}

func currentV1RepositoryRoot() string {
	root, err := os.Getwd()
	if err != nil {
		return "."
	}
	return root
}
