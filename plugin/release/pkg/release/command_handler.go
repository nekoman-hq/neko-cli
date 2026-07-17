package release

import (
	"context"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

type releaseCommandStarter interface {
	Start(context.Context, ReleaseCommandRequest) (ReleaseCommandOutcome, *CommandFailure)
}

type releaseResumer interface {
	Resume(context.Context, ResumeCommandRequest) (ResumeCommandOutcome, *CommandFailure)
}

type releasePlanInspector interface {
	Inspect(context.Context, ReleasePlanInspectionRequest) (*ReleasePlanInspection, *CommandFailure)
}

type releaseCommandHandler struct {
	starter     releaseCommandStarter
	clock       ReleaseClock
	releaseType Type
}

func (handler releaseCommandHandler) Handle(ctx context.Context, req plugin.Request) (*plugin.Response, error) {
	request := ParseReleaseCommandRequest(req, handler.releaseType)
	outcome, failure := handler.starter.Start(ctx, request)
	timestamp := handler.clock.Now()
	if failure != nil {
		if failure.Boundary == CommandFailureFatal {
			return nil, &FatalCommandError{failure: failure}
		}
		return MapCommandFailure(string(request.ReleaseType), failure, timestamp), nil
	}
	return MapReleaseCommandOutcome(string(request.ReleaseType), outcome, timestamp)
}

type resumeCommandHandler struct {
	resumer releaseResumer
	clock   ReleaseClock
}

type releasePlanCommandHandler struct {
	inspector releasePlanInspector
	clock     ReleaseClock
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

func (handler releasePlanCommandHandler) Handle(ctx context.Context, req plugin.Request) (*plugin.Response, error) {
	request, parseFailure := ParsePlanCommandRequest(req)
	timestamp := handler.clock.Now()
	if parseFailure != nil {
		return MapCommandFailure("plan", parseFailure, timestamp), nil
	}
	inspection, failure := handler.inspector.Inspect(ctx, ReleasePlanInspectionRequest(request))
	if failure != nil {
		return MapCommandFailure("plan", failure, timestamp), nil
	}
	return MapReleasePlanInspection(inspection, timestamp), nil
}

// HandleRelease handles the patch, minor, and major release commands.
func HandleRelease(req plugin.Request, releaseType Type) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleReleaseAt(root, req, releaseType)
}

// HandleReleaseAt handles the patch, minor, and major release commands at an
// explicit repository root without changing process cwd.
func HandleReleaseAt(root workspace.RepositoryRoot, req plugin.Request, releaseType Type) (*plugin.Response, error) {
	return handleReleaseWithStarter(req, releaseType, newReleaseStartOperationAt(root))
}

// HandleReleaseWithV1Executors is the production composition entry point. It
// keeps V1 executor selection explicit and independent from the compatibility
// registry retained for direct callers of HandleRelease.
func HandleReleaseWithV1Executors(req plugin.Request, releaseType Type, executors ...V1Executor) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleReleaseWithV1ExecutorsAt(root, req, releaseType, executors...)
}

// HandleReleaseWithV1ExecutorsAt is the canonical embedder entry point for
// release command composition. The supplied root is the only repository root
// used by the command.
func HandleReleaseWithV1ExecutorsAt(root workspace.RepositoryRoot, req plugin.Request, releaseType Type, executors ...V1Executor) (*plugin.Response, error) {
	starter := newReleaseStartOperationWithV1ExecutorsAt(root, newFixedV1ReleaseExecutorCatalog(executors...))
	return handleReleaseWithStarter(req, releaseType, starter)
}

func handleReleaseWithStarter(req plugin.Request, releaseType Type, starter releaseCommandStarter) (*plugin.Response, error) {
	handler := releaseCommandHandler{
		starter:     starter,
		clock:       systemReleaseClock{},
		releaseType: releaseType,
	}
	return handler.Handle(context.Background(), req)
}

// HandleResume handles read-only recovery assessment and conservative resume.
func HandleResume(req plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleResumeAt(root, req)
}

// HandleResumeAt handles read-only recovery assessment and conservative resume
// at an explicit repository root without changing process cwd.
func HandleResumeAt(root workspace.RepositoryRoot, req plugin.Request) (*plugin.Response, error) {
	handler := resumeCommandHandler{
		resumer: newResumeReleaseUseCase(root.Path()),
		clock:   systemReleaseClock{},
	}
	return handler.Handle(context.Background(), req)
}

// HandlePlan handles read-only release-plan inspection.
func HandlePlan(req plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandlePlanAt(root, req)
}

// HandlePlanAt handles read-only release-plan inspection at an explicit
// repository root without changing process cwd.
func HandlePlanAt(root workspace.RepositoryRoot, req plugin.Request) (*plugin.Response, error) {
	handler := releasePlanCommandHandler{
		inspector: newReleasePlanInspectionUseCase(root.Path()),
		clock:     systemReleaseClock{},
	}
	return handler.Handle(context.Background(), req)
}
