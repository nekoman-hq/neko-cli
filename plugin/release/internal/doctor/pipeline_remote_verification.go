package doctor

import (
	"context"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

// InspectRemoteVerification returns Doctor-owned local and explicit remote
// verification facts through the existing bounded GitHub GET-only reader.
func InspectRemoteVerification(
	ctx context.Context,
	root workspace.RepositoryRoot,
	unitID string,
) (VerificationSnapshot, error) {
	readClient, err := newIntegrationDoctorGitHubReadClient()
	if err != nil {
		return VerificationSnapshot{}, err
	}
	return inspectRemoteVerification(
		ctx,
		root,
		unitID,
		integrationDoctorGitHubRemoteInspector{
			reader: readClient,
			tokens: environmentGitHubReadTokenResolver{},
		},
	), nil
}

func inspectRemoteVerification(
	ctx context.Context,
	root workspace.RepositoryRoot,
	unitID string,
	remote integrationDoctorRemoteInspector,
) VerificationSnapshot {
	result := (integrationDoctorInspectionUseCase{
		sources:    filesystemIntegrationDoctorSourceReader{},
		workflows:  filesystemIntegrationDoctorWorkflowReader{},
		files:      filesystemIntegrationDoctorRepositoryFileReader{},
		identities: filesystemIntegrationDoctorRepositoryIdentityReader{},
		remote:     remote,
	}).Inspect(ctx, integrationDoctorRequest{
		RepositoryRoot: root.Path(),
		UnitID:         unitID,
		VerifyRemote:   true,
	})
	return pipelineVerificationSnapshot(result)
}
