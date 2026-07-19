package release

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestIntegrationDoctorCharacterizesBroadLocalVerificationGaps(t *testing.T) {
	root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	workflowPath := writeIntegrationDoctorWorkflow(
		t,
		root,
		".github/workflows/release.yml",
		customIntegrationDoctorWorkflow(t),
	)
	writeIntegrationDoctorBytes(t, filepath.Join(root.Path(), ".goreleaser.yaml"), []byte("version: 2\n"))
	writeIntegrationDoctorBytes(t, filepath.Join(root.Path(), "install.sh"), []byte("#!/usr/bin/env bash\n"))

	snapshot := (filesystemIntegrationDoctorWorkflowReader{}).Read(root.Path(), ".github/workflows/release.yml")
	repository, err := releaseconfig.LoadReleaseRepository(root.Path())
	if err != nil {
		t.Fatalf("load release repository: %v", err)
	}
	_, _, diagnostics := inspectIntegrationDoctorWorkflow(
		root.Path(),
		".github/workflows/release.yml",
		repository.Units,
		repository.Units,
		snapshot,
		filesystemIntegrationDoctorRepositoryFileReader{},
		integrationDoctorRepositoryIdentity{},
		os.ErrNotExist,
	)
	if got, want := integrationDoctorNotVerifiableCodes(diagnostics), []string{
		"CONSUMER_BUILD_NOT_VERIFIABLE",
		"INSTALLATION_ARTIFACTS_NOT_VERIFIABLE",
		"REMOTE_WORKFLOW_NOT_VERIFIABLE",
		"REPOSITORY_VARIABLES_NOT_VERIFIABLE",
		"PUBLICATION_CREDENTIALS_NOT_VERIFIABLE",
		"REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE",
		"PUBLICATION_TARGET_NOT_VERIFIABLE",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("not-verifiable codes = %v, want %v", got, want)
	}

	placeholderPath := writeIntegrationDoctorWorkflow(
		t,
		root,
		".github/workflows/release.yml",
		canonicalIntegrationDoctorWorkflow(t),
	)
	placeholderSnapshot := (filesystemIntegrationDoctorWorkflowReader{}).Read(root.Path(), ".github/workflows/release.yml")
	_, _, placeholderDiagnostics := inspectIntegrationDoctorWorkflow(
		root.Path(),
		".github/workflows/release.yml",
		repository.Units,
		repository.Units,
		placeholderSnapshot,
		filesystemIntegrationDoctorRepositoryFileReader{},
		integrationDoctorRepositoryIdentity{},
		os.ErrNotExist,
	)
	if got, want := integrationDoctorNotVerifiableCodes(placeholderDiagnostics), []string{
		"INSTALLATION_ARTIFACTS_NOT_VERIFIABLE",
		"REMOTE_WORKFLOW_NOT_VERIFIABLE",
		"REPOSITORY_VARIABLES_NOT_VERIFIABLE",
		"PUBLICATION_CREDENTIALS_NOT_VERIFIABLE",
		"REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE",
		"PUBLICATION_TARGET_NOT_VERIFIABLE",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("placeholder not-verifiable codes = %v, want %v", got, want)
	}

	if err := os.Remove(placeholderPath); err != nil {
		t.Fatalf("remove workflow: %v", err)
	}
	_, _, missingDiagnostics := inspectIntegrationDoctorWorkflow(
		root.Path(),
		".github/workflows/release.yml",
		repository.Units,
		repository.Units,
		(filesystemIntegrationDoctorWorkflowReader{}).Read(root.Path(), ".github/workflows/release.yml"),
		filesystemIntegrationDoctorRepositoryFileReader{},
		integrationDoctorRepositoryIdentity{},
		os.ErrNotExist,
	)
	if got := integrationDoctorNotVerifiableCodes(missingDiagnostics); len(got) != 0 {
		t.Fatalf("missing workflow unexpectedly emitted limitations: %v", got)
	}

	if workflowPath == "" {
		t.Fatal("workflow fixture path is empty")
	}
}

func integrationDoctorNotVerifiableCodes(diagnostics []integrationDoctorDiagnostic) []string {
	codes := make([]string, 0)
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == integrationDoctorNotVerifiable {
			codes = append(codes, diagnostic.Code)
		}
	}
	return codes
}
