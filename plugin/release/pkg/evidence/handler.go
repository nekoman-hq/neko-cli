package evidence

import (
	"context"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

type evidenceQueryRunner interface {
	Query(context.Context, evidenceQueryRequest) (evidenceQueryResult, error)
}

type evidenceArchiver interface {
	Archive(context.Context, evidenceArchiveRequest) (evidenceArchiveResult, error)
}

type evidenceCommandHandler struct {
	query evidenceQueryRunner
	clock evidenceResponseClock
}

type evidenceArchiveCommandHandler struct {
	archive evidenceArchiver
	clock   evidenceResponseClock
}

// HandleEvidence inspects release evidence without mutating journals or files.
func HandleEvidence(req plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleEvidenceAt(root, req)
}

// HandleEvidenceAt inspects release evidence at an explicit repository root.
func HandleEvidenceAt(root workspace.RepositoryRoot, req plugin.Request) (*plugin.Response, error) {
	req.Context.WorkingDir = root.Path()
	handler := evidenceCommandHandler{
		query: newEvidenceQueryUseCase(),
		clock: systemEvidenceResponseClock{},
	}
	return handler.Handle(req)
}

// HandleEvidenceArchive archives one completed evidence file after explicit confirmation.
func HandleEvidenceArchive(req plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleEvidenceArchiveAt(root, req)
}

// HandleEvidenceArchiveAt archives one completed evidence file at an explicit
// repository root.
func HandleEvidenceArchiveAt(root workspace.RepositoryRoot, req plugin.Request) (*plugin.Response, error) {
	req.Context.WorkingDir = root.Path()
	handler := evidenceArchiveCommandHandler{
		archive: newEvidenceArchiveUseCase(),
		clock:   systemEvidenceResponseClock{},
	}
	return handler.Handle(req)
}

func (handler evidenceCommandHandler) Handle(req plugin.Request) (*plugin.Response, error) {
	request, err := parseEvidenceQueryRequest(req.Flags, req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	result, err := handler.query.Query(context.Background(), request)
	if err != nil {
		return nil, err
	}
	return mapEvidenceQueryResponse(result, handler.clock.Now()), nil
}

func (handler evidenceArchiveCommandHandler) Handle(req plugin.Request) (*plugin.Response, error) {
	request, err := parseEvidenceArchiveRequest(req.Flags, req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	result, err := handler.archive.Archive(context.Background(), request)
	if err != nil {
		return nil, err
	}
	return mapEvidenceArchiveResponse(result, handler.clock.Now()), nil
}
