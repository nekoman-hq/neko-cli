package release

import (
	"context"
	"fmt"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

type unitOverviewCommandHandler struct {
	inspector unitOverviewInspector
	clock     ReleaseClock
	root      workspace.RepositoryRoot
}

func parseUnitOverviewRequest(root workspace.RepositoryRoot) unitOverviewRequest {
	return unitOverviewRequest{RepositoryRoot: root.Path()}
}

func (handler unitOverviewCommandHandler) Handle(
	ctx context.Context,
	_ plugin.Request,
) (*plugin.Response, error) {
	result := handler.inspector.Inspect(ctx, parseUnitOverviewRequest(handler.root))
	if result == nil {
		return nil, fmt.Errorf("unit overview did not produce a result")
	}
	return mapUnitOverviewResult(result, handler.clock.Now()), nil
}

// HandleUnits resolves the local repository root and returns the Release V2
// unit inventory without mutation, workflow inspection, Git, tokens, or network.
func HandleUnits(request plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveInspectionRepositoryRoot(request.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleUnitsAt(root, request)
}

// HandleUnitsAt returns the Release V2 unit inventory at an explicit root.
func HandleUnitsAt(root workspace.RepositoryRoot, request plugin.Request) (*plugin.Response, error) {
	handler := unitOverviewCommandHandler{
		inspector: unitOverviewInspectionUseCase{sources: filesystemLocalV2SourceReader{}},
		clock:     systemReleaseClock{},
		root:      root,
	}
	return handler.Handle(context.Background(), request)
}
