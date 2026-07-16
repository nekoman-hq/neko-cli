package evidence

import (
	"context"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

type evidenceQueryRunner interface {
	Query(context.Context, evidenceQueryRequest) (evidenceQueryResult, error)
}

type evidenceCommandHandler struct {
	query evidenceQueryRunner
	clock evidenceResponseClock
}

// HandleEvidence inspects release evidence without mutating journals or files.
func HandleEvidence(req plugin.Request) (*plugin.Response, error) {
	handler := evidenceCommandHandler{
		query: newEvidenceQueryUseCase(),
		clock: systemEvidenceResponseClock{},
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
