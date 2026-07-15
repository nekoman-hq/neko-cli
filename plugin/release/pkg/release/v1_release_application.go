//nolint:staticcheck // This file is the explicit application boundary for deprecated V1 compatibility.
package release

import (
	"context"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type v1ReleasePreviewer interface {
	Preview(V1ReleasePreviewRequest) (V1ReleaseResult, *V1ReleaseFailure)
}

type v1ReleaseExecutorUseCase interface {
	Execute(V1ReleaseExecutionRequest) (V1ReleaseResult, *V1ReleaseFailure)
}

type v1ReleaseCommandApplication struct {
	preview   v1ReleasePreviewer
	execution v1ReleaseExecutorUseCase
}

func newV1ReleaseCommandApplication() v1ReleaseCommandApplication {
	return composeV1ReleaseCommandApplication(registeredV1ReleaseExecutorCatalog{})
}

func composeV1ReleaseCommandApplication(executors v1ReleaseExecutorCatalog) v1ReleaseCommandApplication {
	evidence := legacyV1VersionEvidence{}
	planning := v1ReleasePlanningOperation{
		planner:   pureV1ReleasePlanner{},
		tags:      evidence,
		refresher: evidence,
	}
	reporter := systemV1ReleaseReporter{}
	return v1ReleaseCommandApplication{
		preview: v1ReleasePreviewUseCase{
			plans:    planning,
			reporter: reporter,
		},
		execution: v1ReleaseExecutionUseCase{
			previewPlans:   planning,
			executionPlans: planning,
			requirements:   systemV1ReleaseRequirements{},
			preflight:      systemV1ReleasePreflight{},
			materializer:   v1ReleaseConfigFileMaterializer{},
			executors:      executors,
			reporter:       reporter,
		},
	}
}

func (application v1ReleaseCommandApplication) Start(
	_ context.Context,
	repository *releaseconfig.ReleaseRepository,
	request ReleaseCommandRequest,
) (ReleaseCommandOutcome, *CommandFailure) {
	unit, err := releaseconfig.ResolveReleaseUnit(
		repository,
		request.UnitID,
		releaseconfig.UnitResolutionOptions{RequireExplicitForMulti: true},
	)
	if err != nil {
		return nil, failureFromError("UNIT_RESOLUTION_FAILED", err)
	}
	intent := V1ReleaseIntent{
		RepositoryRoot: repository.RepositoryRoot,
		Unit:           *unit,
		Config:         repository.Legacy,
		ReleaseType:    request.ReleaseType,
	}

	if request.DryRun {
		result, failure := application.preview.Preview(V1ReleasePreviewRequest{Intent: intent})
		return result, commandFailureFromV1(failure)
	}
	result, failure := application.execution.Execute(V1ReleaseExecutionRequest{Intent: intent})
	return result, commandFailureFromV1(failure)
}
