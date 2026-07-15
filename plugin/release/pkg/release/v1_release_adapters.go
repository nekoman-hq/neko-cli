//nolint:staticcheck // This file temporarily adapts deprecated V1 effects pending focused Stage 9 adapters.
package release

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	pluginerrors "github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/git"
)

type v1LatestTagReader interface {
	LatestTag() string
}

type v1TagRefresher interface {
	RefreshTags()
}

type legacyV1VersionEvidence struct{}

func (legacyV1VersionEvidence) LatestTag() string { return latestVersionTag() }
func (legacyV1VersionEvidence) RefreshTags()      { refreshVersionTags() }

type v1ReleasePlanningOperation struct {
	planner   v1ReleasePlanner
	tags      v1LatestTagReader
	refresher v1TagRefresher
}

func (operation v1ReleasePlanningOperation) BuildPreviewPlan(intent V1ReleaseIntent) (V1ReleasePlan, error) {
	return operation.planner.Plan(V1ReleasePlanningRequest{Intent: intent, LatestTag: operation.tags.LatestTag()})
}

func (operation v1ReleasePlanningOperation) BuildExecutionPlan(intent V1ReleaseIntent) (V1ReleasePlan, error) {
	operation.refresher.RefreshTags()
	return operation.planner.Plan(V1ReleasePlanningRequest{Intent: intent, LatestTag: operation.tags.LatestTag()})
}

type systemV1ReleaseRequirements struct{}

func (systemV1ReleaseRequirements) Validate(intent V1ReleaseIntent) error {
	log.PluginV(
		log.Config,
		"Validating release requirements for %s in %s",
		log.ColorText(log.ColorCyan, string(intent.Config.ReleaseSystem)),
		log.ColorText(log.ColorGreen, intent.RepositoryRoot),
	)
	return validateRequirementsForExecutor(string(intent.Config.ReleaseSystem), intent.RepositoryRoot, true)
}

type v1ReleaseConfigFileMaterializer struct{}

func (v1ReleaseConfigFileMaterializer) WritePlannedVersion(intent V1ReleaseIntent, plan V1ReleasePlan) error {
	intent.Config.Version = plan.NextVersion
	return releaseconfig.V1SaveConfig(*intent.Config)
}

func (v1ReleaseConfigFileMaterializer) RestorePreviousVersion(intent V1ReleaseIntent, plan V1ReleasePlan) error {
	intent.Config.Version = plan.CurrentVersion
	return releaseconfig.V1SaveConfig(*intent.Config)
}

type registeredV1ReleaseExecutorCatalog struct{}

func (registeredV1ReleaseExecutorCatalog) Resolve(name string) (v1ReleaseExecutor, error) {
	tool, err := Get(name)
	if err != nil {
		return nil, err
	}
	return &registeredV1ReleaseExecutor{tool: tool}, nil
}

type registeredV1ReleaseExecutor struct {
	tool Tool
}

func (executor *registeredV1ReleaseExecutor) Name() string { return executor.tool.Name() }

func (executor *registeredV1ReleaseExecutor) Execute(request V1ExecutorRequest) error {
	plan := request.Plan
	tagSpec, err := releaseconfig.NewTagSpec("v")
	if err != nil {
		return err
	}
	return executor.tool.Execute(&ReleaseExecutionContext{
		RepositoryRoot: plan.RepositoryRoot,
		Unit: releaseconfig.ReleaseUnit{
			ID:               plan.UnitID,
			Paths:            []string{"**"},
			WorkingDirectory: ".",
			TagPrefix:        "v",
			ExecutorType:     plan.Executor,
			Delivery:         string(releaseconfig.DeliveryLocal),
			Version:          plan.CurrentVersion,
		},
		UnitRoot:       plan.RepositoryRoot,
		CurrentVersion: plan.CurrentVersion,
		NextVersion:    plan.NextVersion,
		Tag:            plan.Tag,
		TagSpec:        tagSpec,
		ReleaseKind:    plan.ReleaseType,
		Executor:       plan.Executor,
		Delivery:       string(releaseconfig.DeliveryLocal),
		SourceFormat:   releaseconfig.SourceFormatV1,
	})
}

func (executor *registeredV1ReleaseExecutor) Rollback() error { return executor.tool.RevertRelease() }

type directV1ReleaseExecutorCatalog struct{}

func (directV1ReleaseExecutorCatalog) Resolve(name string) (v1ReleaseExecutor, error) {
	tool, err := Get(name)
	if err != nil {
		return nil, err
	}
	return &directV1ReleaseExecutor{tool: tool}, nil
}

type directV1ReleaseExecutor struct {
	tool Tool
}

func (executor *directV1ReleaseExecutor) Name() string { return executor.tool.Name() }

func (executor *directV1ReleaseExecutor) Execute(request V1ExecutorRequest) error {
	version, err := semver.NewVersion(request.Plan.NextVersion)
	if err != nil {
		return err
	}
	return executor.tool.Release(version)
}

func (executor *directV1ReleaseExecutor) Rollback() error { return executor.tool.RevertRelease() }

type systemV1ReleaseReporter struct{}

func (systemV1ReleaseReporter) PlanningStarted() {
	log.PluginV(log.Guard, "Running Version Guard checks")
}

func (systemV1ReleaseReporter) PlanningCompleted(plan V1ReleasePlan) {
	if plan.IgnoredLatestTag != "" {
		pluginerrors.WriteWarning(
			"Latest Git tag %s is not a valid semantic version, skipping comparison",
			plan.IgnoredLatestTag,
		)
		log.PluginV(log.Guard, "Using local version %s", plan.CurrentVersion)
		return
	}
	log.PluginV(
		log.Guard,
		"Local version %s is >= latest tag %s, proceeding.",
		plan.CurrentVersion,
		plan.LatestVersion,
	)
}

func (systemV1ReleaseReporter) PreviewCompleted(V1ReleasePlan) {
	log.PluginPrint(log.Exec, "Dry run mode - no changes will be made")
}

func (systemV1ReleaseReporter) ExecutionReady(plan V1ReleasePlan, executorName string) {
	log.PluginPrint(log.Exec, "Release system detected: %s", log.ColorText(log.ColorPurple, executorName))
	log.PluginPrint(
		log.Exec,
		"Latest version tag extracted successfully \uF178 %s",
		log.ColorText(log.ColorCyan, plan.CurrentVersion),
	)
	log.PluginPrint(
		log.Exec,
		"Applying %s (%s \uF178 %s)",
		log.ColorText(log.ColorPurple, string(plan.ReleaseType)),
		plan.CurrentVersion,
		log.ColorText(log.ColorCyan, plan.NextVersion),
	)
	log.PluginPrint(
		log.Guard,
		"\uF00C All checks have succeeded. %s",
		log.ColorText(log.ColorGreen, "Starting release now!"),
	)
}

func (systemV1ReleaseReporter) ConfigWriteFailed(err error) {
	pluginerrors.WriteWarning(
		"Failed to update local config",
		fmt.Sprintf("Updating version in .release.neko.json failed. Attempting to proceed with release: %s", err.Error()),
	)
}

func (systemV1ReleaseReporter) ConfigRestoreFailed(err error) {
	log.PluginPrint(log.Guard, "Warning: Failed to revert config: %s", err.Error())
}

func (systemV1ReleaseReporter) RollbackStarted() {
	log.PluginPrint(log.Guard, "Encountered error while releasing. Trying to undo changes...")
}

func (systemV1ReleaseReporter) RollbackCompleted() {
	log.PluginPrint(log.Guard, "Successfully undid changes.")
}

func (systemV1ReleaseReporter) ReleaseCompleted(plan V1ReleasePlan) {
	log.PluginPrint(
		log.Exec,
		"\uF00C Successfully released version %s",
		log.ColorText(log.ColorCyan, plan.NextVersion),
	)
}

type systemV1ReleasePreflight struct{}

func (systemV1ReleasePreflight) Check(intent V1ReleaseIntent) *V1ReleaseFailure {
	_, _ = git.Current()
	return checkV1ReleasePreflight(intent.Config)
}
