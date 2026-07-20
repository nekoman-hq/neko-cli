package doctor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

type integrationDoctorClock interface {
	Now() time.Time
}

type systemIntegrationDoctorClock struct{}

func (systemIntegrationDoctorClock) Now() time.Time {
	return time.Now()
}

type integrationDoctorRequestFailure struct {
	Code    string
	Message string
}

type integrationDoctorCommandHandler struct {
	inspector integrationDoctorInspector
	clock     integrationDoctorClock
	root      workspace.RepositoryRoot
}

func parseIntegrationDoctorRequest(
	root workspace.RepositoryRoot,
	request plugin.Request,
) (integrationDoctorRequest, *integrationDoctorRequestFailure) {
	unitID := ""
	if raw, present := request.Flags["unit"]; present {
		value, ok := raw.(string)
		if !ok || value == "" || value != strings.TrimSpace(value) {
			return integrationDoctorRequest{}, &integrationDoctorRequestFailure{
				Code: "INVALID_DOCTOR_REQUEST", Message: "--unit must be a non-empty string without leading or trailing whitespace",
			}
		}
		unitID = value
	}
	verifyRemote := false
	if raw, present := request.Flags["verify-remote"]; present {
		value, ok := raw.(bool)
		if !ok {
			return integrationDoctorRequest{}, &integrationDoctorRequestFailure{
				Code: "INVALID_DOCTOR_REQUEST", Message: "--verify-remote must be a boolean",
			}
		}
		verifyRemote = value
	}
	return integrationDoctorRequest{
		RepositoryRoot: root.Path(),
		UnitID:         unitID,
		VerifyRemote:   verifyRemote,
	}, nil
}

func (handler integrationDoctorCommandHandler) Handle(
	ctx context.Context,
	request plugin.Request,
) (*plugin.Response, error) {
	typedRequest, failure := parseIntegrationDoctorRequest(handler.root, request)
	if failure != nil {
		return mapIntegrationDoctorRequestFailure(failure, handler.clock.Now()), nil
	}
	result := handler.inspector.Inspect(ctx, typedRequest)
	if result == nil {
		return nil, fmt.Errorf("integration doctor did not produce a result")
	}
	return mapIntegrationDoctorResult(result, handler.clock.Now()), nil
}

func mapIntegrationDoctorRequestFailure(
	failure *integrationDoctorRequestFailure,
	timestamp time.Time,
) *plugin.Response {
	if failure == nil {
		return nil
	}
	return &plugin.Response{
		Status:   "error",
		Metadata: integrationDoctorResponseMetadata(timestamp),
		Error: &plugin.ResponseError{
			Code: failure.Code, Message: failure.Message,
		},
		ExitCode: 1,
	}
}

func integrationDoctorResponseMetadata(timestamp time.Time) plugin.ResponseMetadata {
	return plugin.ResponseMetadata{
		Plugin: metadata.PluginName, Version: metadata.Version,
		Command: integrationDoctorCommandName, Timestamp: timestamp,
	}
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
// an explicit repository root. It remains offline unless the request explicitly
// enables the focused read-only remote verifier.
func HandleDoctorAt(root workspace.RepositoryRoot, request plugin.Request) (*plugin.Response, error) {
	readClient, err := newIntegrationDoctorGitHubReadClient()
	if err != nil {
		return nil, fmt.Errorf("construct integration doctor GitHub read client: %w", err)
	}
	handler := integrationDoctorCommandHandler{
		inspector: integrationDoctorInspectionUseCase{
			sources:    filesystemIntegrationDoctorSourceReader{},
			workflows:  filesystemIntegrationDoctorWorkflowReader{},
			files:      filesystemIntegrationDoctorRepositoryFileReader{},
			identities: filesystemIntegrationDoctorRepositoryIdentityReader{},
			remote: integrationDoctorGitHubRemoteInspector{
				reader: readClient,
				tokens: environmentGitHubReadTokenResolver{},
			},
		},
		clock: systemIntegrationDoctorClock{},
		root:  root,
	}
	return handler.Handle(context.Background(), request)
}
