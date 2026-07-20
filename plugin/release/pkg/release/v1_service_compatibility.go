// Package release includes all neko cli release logic.
//
//nolint:staticcheck // Service is a direct compatibility facade over the isolated V1 application.
package release

import (
	"context"
	"os"

	"github.com/Masterminds/semver/v3"
	pluginerrors "github.com/nekoman-hq/neko-cli/pkg/errors"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// Service preserves the former exported V1 entry points. Active command
// composition no longer uses this type; both methods delegate to the isolated
// V1 planner/application and contain no release orchestration.
//
// Deprecated: use HandleReleaseWithV1Executors with explicit V1Executor values
// for release execution, or PlanV1Release for version planning.
type Service struct {
	cfg *releaseconfig.V1ReleaseConfig
	ctx *ReleaseExecutionContext
}

// NewReleaseService constructs the legacy V1 service facade.
//
// Deprecated: use HandleReleaseWithV1Executors with explicit V1Executor values
// for release execution, or PlanV1Release for version planning.
func NewReleaseService(cfg *releaseconfig.V1ReleaseConfig) *Service {
	return &Service{cfg: cfg}
}

// NewReleaseServiceWithContext constructs the legacy V1 service facade with a
// compatibility execution context.
//
// Deprecated: use HandleReleaseWithV1Executors with explicit V1Executor values
// for release execution, or PlanV1Release for version planning.
func NewReleaseServiceWithContext(cfg *releaseconfig.V1ReleaseConfig, ctx *ReleaseExecutionContext) *Service {
	return &Service{cfg: cfg, ctx: ctx}
}

// Run executes a release through the legacy service facade.
//
// Deprecated: use HandleReleaseWithV1Executors with explicit V1Executor values
// instead.
func (service *Service) Run(releaseType Type) error {
	root := service.repositoryRoot()
	repository := releaseconfig.NormalizeV1Repository(root, service.cfg)
	executors := v1ReleaseExecutorCatalog(directV1ReleaseExecutorCatalog{})
	if service.ctx != nil {
		executors = registeredV1ReleaseExecutorCatalog{}
	}
	_, failure := composeV1ReleaseCommandApplication(executors).Start(
		context.Background(),
		repository,
		ReleaseCommandRequest{ReleaseType: releaseType},
	)
	if failure == nil {
		return nil
	}
	if failure.Boundary == CommandFailureFatal {
		pluginerrors.WriteError(failure.Code, failure.responseMessage())
	}
	if failure.Cause != nil {
		return failure.Cause
	}
	return &FatalCommandError{failure: failure}
}

// GetNewVersion returns the current and next V1 versions through the legacy
// service facade.
//
// Deprecated: use PlanV1Release with explicit latest-tag evidence instead.
func (service *Service) GetNewVersion(releaseType Type) (*semver.Version, *semver.Version, error) {
	root := service.repositoryRoot()
	repository := releaseconfig.NormalizeV1Repository(root, service.cfg)
	intent := V1ReleaseIntent{
		RepositoryRoot: root,
		Unit:           repository.Units[0],
		Config:         service.cfg,
		ReleaseType:    releaseType,
	}
	reporter := systemV1ReleaseReporter{}
	reporter.PlanningStarted()
	plan, err := (v1ReleasePlanningOperation{
		planner: pureV1ReleasePlanner{},
		tags:    legacyV1VersionEvidence{},
	}).BuildPreviewPlan(intent)
	if err != nil {
		return nil, nil, err
	}
	reporter.PlanningCompleted(plan)
	current, err := semver.NewVersion(plan.CurrentVersion)
	if err != nil {
		return nil, nil, err
	}
	next, err := semver.NewVersion(plan.NextVersion)
	if err != nil {
		return nil, nil, err
	}
	return current, next, nil
}

func (service *Service) repositoryRoot() string {
	if service.ctx != nil && service.ctx.RepositoryRoot != "" {
		return service.ctx.RepositoryRoot
	}
	root, err := os.Getwd()
	if err != nil {
		return "."
	}
	return root
}
