package pipelineinspection

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

type pipelineCommandHandler struct {
	root         workspace.RepositoryRoot
	stages       []LifecycleStage
	runtime      RuntimeSnapshot
	verification VerificationSnapshot
}

func parsePipelineRequest(root workspace.RepositoryRoot, request plugin.Request) (pipelineRequest, *commandFailure) {
	if len(request.Args) != 0 {
		return pipelineRequest{}, &commandFailure{Code: "INVALID_PIPELINE_REQUEST", Message: "pipeline inspection does not accept positional arguments"}
	}
	flagNames := make([]string, 0, len(request.Flags))
	for name := range request.Flags {
		flagNames = append(flagNames, name)
	}
	sort.Strings(flagNames)
	for _, name := range flagNames {
		if name != "unit" && name != "output" && name != "verify-remote" {
			return pipelineRequest{}, &commandFailure{Code: "INVALID_PIPELINE_REQUEST", Message: fmt.Sprintf("pipeline inspection does not support --%s", name)}
		}
	}
	unitID := ""
	if raw, present := request.Flags["unit"]; present {
		value, ok := raw.(string)
		if !ok || value == "" || value != strings.TrimSpace(value) {
			return pipelineRequest{}, &commandFailure{Code: "INVALID_PIPELINE_REQUEST", Message: "--unit must be a non-empty string without leading or trailing whitespace"}
		}
		unitID = value
	}
	verifyRemote := false
	if raw, present := request.Flags["verify-remote"]; present {
		value, ok := raw.(bool)
		if !ok {
			return pipelineRequest{}, &commandFailure{Code: "INVALID_PIPELINE_REQUEST", Message: "--verify-remote must be a boolean"}
		}
		verifyRemote = value
	}
	return pipelineRequest{RepositoryRoot: root.Path(), UnitID: unitID, VerifyRemote: verifyRemote}, nil
}

// RequestsRemoteVerification selects the explicit remote composition path only
// for a structurally valid Pipeline request. The command parser remains the
// authoritative error boundary.
func RequestsRemoteVerification(request plugin.Request) bool {
	if len(request.Args) != 0 {
		return false
	}
	for name := range request.Flags {
		if name != "unit" && name != "output" && name != "verify-remote" {
			return false
		}
	}
	if raw, present := request.Flags["unit"]; present {
		value, ok := raw.(string)
		if !ok || value == "" || value != strings.TrimSpace(value) {
			return false
		}
	}
	value, ok := request.Flags["verify-remote"].(bool)
	return ok && value
}

func (handler pipelineCommandHandler) Handle(_ context.Context, request plugin.Request) (*plugin.Response, error) {
	typedRequest, failure := parsePipelineRequest(handler.root, request)
	if failure != nil {
		return mapPipelineFailure(failure), nil
	}
	result, failure := inspectConfiguredPipeline(typedRequest, handler.stages, handler.runtime, handler.verification)
	if failure != nil {
		return mapPipelineFailure(failure), nil
	}
	if result == nil {
		return nil, fmt.Errorf("pipeline inspection did not produce a result")
	}
	return mapPipelineResult(result), nil
}

// HandlePipeline resolves the local repository root and projects one Release
// V2 pipeline without mutation, Git, subprocesses, tokens, or network access.
func HandlePipeline(request plugin.Request, stages []LifecycleStage) (*plugin.Response, error) {
	root, err := workspace.ResolveInspectionRepositoryRoot(request.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandlePipelineAt(root, request, stages)
}

// HandlePipelineAt projects one Release V2 pipeline at an explicit root.
func HandlePipelineAt(root workspace.RepositoryRoot, request plugin.Request, stages []LifecycleStage) (*plugin.Response, error) {
	handler := pipelineCommandHandler{root: root, stages: append([]LifecycleStage(nil), stages...)}
	return handler.Handle(context.Background(), request)
}

// HandlePipelineRuntimeAt projects one Release V2 pipeline with immutable
// authoritative runtime facts supplied by pkg/release.
func HandlePipelineRuntimeAt(root workspace.RepositoryRoot, request plugin.Request, stages []LifecycleStage, runtime RuntimeSnapshot) (*plugin.Response, error) {
	handler := pipelineCommandHandler{
		root: root, stages: append([]LifecycleStage(nil), stages...),
		runtime: cloneRuntimeSnapshot(runtime),
	}
	return handler.Handle(context.Background(), request)
}

// HandlePipelineRuntimeVerificationAt projects one Release V2 pipeline with
// immutable authoritative runtime and verification facts supplied by
// pkg/release.
func HandlePipelineRuntimeVerificationAt(
	root workspace.RepositoryRoot,
	request plugin.Request,
	stages []LifecycleStage,
	runtime RuntimeSnapshot,
	verification VerificationSnapshot,
) (*plugin.Response, error) {
	handler := pipelineCommandHandler{
		root: root, stages: append([]LifecycleStage(nil), stages...),
		runtime: cloneRuntimeSnapshot(runtime), verification: cloneVerificationSnapshot(verification),
	}
	return handler.Handle(context.Background(), request)
}

func cloneRuntimeSnapshot(snapshot RuntimeSnapshot) RuntimeSnapshot {
	clone := snapshot
	clone.Executions = append([]RuntimeExecutionObservation(nil), snapshot.Executions...)
	for index := range clone.Executions {
		clone.Executions[index].ConfirmedStageIDs = append([]string(nil), snapshot.Executions[index].ConfirmedStageIDs...)
		clone.Executions[index].CurrentStageIDs = append([]string(nil), snapshot.Executions[index].CurrentStageIDs...)
	}
	clone.Dispatches = append([]RuntimeDispatchObservation(nil), snapshot.Dispatches...)
	clone.Problems = append([]RuntimeProblem(nil), snapshot.Problems...)
	return clone
}
