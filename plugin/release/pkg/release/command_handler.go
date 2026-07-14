package release

import (
	"context"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

type releaseCommandStarter interface {
	Start(context.Context, ReleaseCommandRequest) (ReleaseCommandOutcome, *CommandFailure)
}

type releaseResumer interface {
	Resume(context.Context, ResumeCommandRequest) (ResumeCommandOutcome, *CommandFailure)
}

type releaseCommandHandler struct {
	starter     releaseCommandStarter
	clock       responseClock
	releaseType Type
}

func (handler releaseCommandHandler) Handle(ctx context.Context, req plugin.Request) (*plugin.Response, error) {
	request := ParseReleaseCommandRequest(req, handler.releaseType)
	outcome, failure := handler.starter.Start(ctx, request)
	timestamp := handler.clock.Now()
	if failure != nil {
		return MapCommandFailure(string(request.ReleaseType), failure, timestamp), nil
	}
	return MapReleaseCommandOutcome(string(request.ReleaseType), outcome, timestamp)
}

type resumeCommandHandler struct {
	resumer releaseResumer
	clock   responseClock
}

func (handler resumeCommandHandler) Handle(ctx context.Context, req plugin.Request) (*plugin.Response, error) {
	request := ParseResumeCommandRequest(req)
	outcome, failure := handler.resumer.Resume(ctx, request)
	timestamp := handler.clock.Now()
	if failure != nil {
		return MapCommandFailure("resume", failure, timestamp), nil
	}
	return MapResumeCommandOutcome(outcome, timestamp)
}

// HandleRelease handles the patch, minor, and major release commands.
func HandleRelease(req plugin.Request, releaseType Type) (*plugin.Response, error) {
	handler := releaseCommandHandler{
		starter:     releaseStartOperation{},
		clock:       systemResponseClock{},
		releaseType: releaseType,
	}
	return handler.Handle(context.Background(), req)
}

// HandleResume handles read-only recovery assessment and conservative resume.
func HandleResume(req plugin.Request) (*plugin.Response, error) {
	handler := resumeCommandHandler{
		resumer: resumeReleaseOperation{},
		clock:   systemResponseClock{},
	}
	return handler.Handle(context.Background(), req)
}
