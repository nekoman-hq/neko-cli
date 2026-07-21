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
	root   workspace.RepositoryRoot
	stages []LifecycleStage
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
		if name != "unit" && name != "output" {
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
	return pipelineRequest{RepositoryRoot: root.Path(), UnitID: unitID}, nil
}

func (handler pipelineCommandHandler) Handle(_ context.Context, request plugin.Request) (*plugin.Response, error) {
	typedRequest, failure := parsePipelineRequest(handler.root, request)
	if failure != nil {
		return mapPipelineFailure(failure), nil
	}
	result, failure := inspectConfiguredPipeline(typedRequest, handler.stages)
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
