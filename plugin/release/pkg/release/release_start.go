// Package release includes the Release command orchestration boundaries.
package release

import (
	"context"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

type releaseRepositoryReader interface {
	Load(string) (*config.ReleaseRepository, error)
}

type releaseConfigRepositoryReader struct{}

func (releaseConfigRepositoryReader) Load(root string) (*config.ReleaseRepository, error) {
	return loadReleaseRepository(root, config.LoadReleaseRepository)
}

type releasePlanConfigRepositoryReader struct{}

func (releasePlanConfigRepositoryReader) Load(root string) (*config.ReleaseRepository, error) {
	return loadReleaseRepository(root, config.LoadReleaseRepositoryForInspection)
}

func loadReleaseRepository(
	root string,
	load func(string) (*config.ReleaseRepository, error),
) (*config.ReleaseRepository, error) {
	repository, err := load(root)
	if err != nil {
		return nil, err
	}
	absoluteRoot, err := absoluteExistingDir(repository.RepositoryRoot, "repository root")
	if err != nil {
		return nil, err
	}
	repository.RepositoryRoot = absoluteRoot
	return repository, nil
}

type v1ReleaseApplication interface {
	Start(context.Context, *config.ReleaseRepository, ReleaseCommandRequest) (ReleaseCommandOutcome, *CommandFailure)
}

type v2ReleaseApplication interface {
	Start(context.Context, *config.ReleaseRepository, ReleaseCommandRequest) (ReleaseCommandOutcome, *CommandFailure)
}

type releaseStartOperation struct {
	repositories   releaseRepositoryReader
	v1             v1ReleaseApplication
	v2             v2ReleaseApplication
	progress       ReleaseProgress
	repositoryRoot string
}

func newReleaseStartOperationAt(root workspace.RepositoryRoot) releaseStartOperation {
	return newReleaseStartOperationWithV1ExecutorsAt(root, registeredV1ReleaseExecutorCatalog{})
}

func newReleaseStartOperationWithV1ExecutorsAt(root workspace.RepositoryRoot, executors v1ReleaseExecutorCatalog) releaseStartOperation {
	return newReleaseStartOperationWithRepositoryRoot(root.Path(), executors)
}

func newReleaseStartOperationWithRepositoryRoot(repositoryRoot string, executors v1ReleaseExecutorCatalog) releaseStartOperation {
	progress := newTerminalReleaseProgress()
	return releaseStartOperation{
		repositoryRoot: repositoryRoot,
		repositories:   releaseConfigRepositoryReader{},
		v1:             composeV1ReleaseCommandApplication(executors),
		v2:             v2ReleaseCommandApplication{progress: progress},
		progress:       progress,
	}
}

func (operation releaseStartOperation) Start(ctx context.Context, request ReleaseCommandRequest) (ReleaseCommandOutcome, *CommandFailure) {
	reportReleaseProgress(operation.progress, ReleaseProgressEvent{
		Kind:        ReleaseProgressReleaseStarted,
		ReleaseType: string(request.ReleaseType),
	})

	repository, err := operation.repositories.Load(operation.repositoryRoot)
	if err != nil {
		if config.V2ConfigExists(operation.repositoryRoot) || config.V2StateExists(operation.repositoryRoot) {
			return nil, &CommandFailure{
				Code:  "CONFIG_INVALID",
				Cause: err,
				Details: map[string]any{
					"hint": "Fix .neko/release.config.json and .neko/release.state.json before running release commands",
				},
			}
		}
		return nil, &CommandFailure{
			Code:  "CONFIG_NOT_FOUND",
			Cause: err,
			Details: map[string]any{
				"hint": "Run 'neko release init' to create V2 config/state, or 'neko release migrate' to convert an existing V1 config",
			},
		}
	}

	path, err := selectReleaseApplicationPath(repository.SourceFormat)
	if err != nil {
		return nil, failureFromError("SOURCE_FORMAT_UNSUPPORTED", err)
	}

	switch path {
	case config.SourceFormatV1:
		return operation.v1.Start(ctx, repository, request)
	case config.SourceFormatV2:
		return operation.v2.Start(ctx, repository, request)
	default:
		return nil, failureFromMessage("SOURCE_FORMAT_UNSUPPORTED", "release source selection returned no application path")
	}
}
