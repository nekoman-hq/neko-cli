package release

import (
	"context"
	"fmt"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

type integrationDoctorCommandHandler struct {
	inspector integrationDoctorInspector
	clock     ReleaseClock
	root      workspace.RepositoryRoot
}

func parseIntegrationDoctorRequest(
	root workspace.RepositoryRoot,
	request plugin.Request,
) (integrationDoctorRequest, *CommandFailure) {
	unitID := ""
	if raw, present := request.Flags["unit"]; present {
		value, ok := raw.(string)
		if !ok || value == "" || value != strings.TrimSpace(value) {
			return integrationDoctorRequest{}, failureFromMessage(
				"INVALID_DOCTOR_REQUEST",
				"--unit must be a non-empty string without leading or trailing whitespace",
			)
		}
		unitID = value
	}
	return integrationDoctorRequest{RepositoryRoot: root.Path(), UnitID: unitID}, nil
}

func (handler integrationDoctorCommandHandler) Handle(
	ctx context.Context,
	request plugin.Request,
) (*plugin.Response, error) {
	typedRequest, failure := parseIntegrationDoctorRequest(handler.root, request)
	if failure != nil {
		response := MapCommandFailure(integrationDoctorCommandName, failure, handler.clock.Now())
		response.ExitCode = 1
		return response, nil
	}
	result := handler.inspector.Inspect(ctx, typedRequest)
	if result == nil {
		return nil, fmt.Errorf("integration doctor did not produce a result")
	}
	return mapIntegrationDoctorResult(result, handler.clock.Now()), nil
}

// HandleDoctor resolves a repository root for local inspection and reports
// Release V2 GitHub Actions integration readiness without mutation.
func HandleDoctor(request plugin.Request) (*plugin.Response, error) {
	root, err := workspace.ResolveInspectionRepositoryRoot(request.Context.WorkingDir)
	if err != nil {
		return nil, err
	}
	return HandleDoctorAt(root, request)
}

// HandleDoctorAt reports Release V2 GitHub Actions integration readiness at
// an explicit repository root without mutation, tokens, Git, or network use.
func HandleDoctorAt(root workspace.RepositoryRoot, request plugin.Request) (*plugin.Response, error) {
	handler := integrationDoctorCommandHandler{
		inspector: integrationDoctorInspectionUseCase{
			sources:    filesystemIntegrationDoctorSourceReader{},
			workflows:  filesystemIntegrationDoctorWorkflowReader{},
			files:      filesystemIntegrationDoctorRepositoryFileReader{},
			identities: filesystemIntegrationDoctorRepositoryIdentityReader{},
		},
		clock: systemReleaseClock{},
		root:  root,
	}
	return handler.Handle(context.Background(), request)
}
