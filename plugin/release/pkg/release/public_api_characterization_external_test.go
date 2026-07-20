package release_test

import (
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestIndependentCommandCompatibilitySurface(t *testing.T) {
	var (
		doctor                 func(plugin.Request) (*plugin.Response, error)                                         = release.HandleDoctor
		doctorAt               func(workspace.RepositoryRoot, plugin.Request) (*plugin.Response, error)               = release.HandleDoctorAt
		units                  func(plugin.Request) (*plugin.Response, error)                                         = release.HandleUnits
		unitsAt                func(workspace.RepositoryRoot, plugin.Request) (*plugin.Response, error)               = release.HandleUnitsAt
		workflowInit           func(plugin.Request) (*plugin.Response, error)                                         = release.HandleGitHubWorkflowInit
		workflowInitAt         func(workspace.RepositoryRoot, plugin.Request) (*plugin.Response, error)               = release.HandleGitHubWorkflowInitAt
		contextValidation      func(plugin.Request) (*plugin.Response, error)                                         = release.HandleReleaseContextValidation
		contextValidationAt    func(workspace.RepositoryRoot, plugin.Request) (*plugin.Response, error)               = release.HandleReleaseContextValidationAt
		parseContextValidation func(workspace.RepositoryRoot, plugin.Request) release.ReleaseContextValidationRequest = release.ParseReleaseContextValidationRequest
		mapValidatedContext    func(*release.ValidatedReleaseContext, time.Time) *plugin.Response                     = release.MapValidatedReleaseContext
		renderWorkflow         func() ([]byte, error)                                                                 = release.RenderCanonicalGitHubActionsReleaseWorkflow
		resolveRepository      func(string, string) (release.GitHubRepositoryTarget, error)                           = release.ResolveGitHubRepositoryTarget
	)
	for name, available := range map[string]bool{
		"doctor":                   doctor != nil,
		"doctor-at":                doctorAt != nil,
		"units":                    units != nil,
		"units-at":                 unitsAt != nil,
		"workflow-init":            workflowInit != nil,
		"workflow-init-at":         workflowInitAt != nil,
		"context-validation":       contextValidation != nil,
		"context-validation-at":    contextValidationAt != nil,
		"parse-context-validation": parseContextValidation != nil,
		"map-validated-context":    mapValidatedContext != nil,
		"render-workflow":          renderWorkflow != nil,
		"resolve-repository":       resolveRepository != nil,
	} {
		if !available {
			t.Errorf("public compatibility entry point %s is unavailable", name)
		}
	}

	if release.GitObjectFormatSHA1 != "sha1" || release.GitObjectFormatSHA256 != "sha256" {
		t.Fatalf("git object format wire values changed: %q %q", release.GitObjectFormatSHA1, release.GitObjectFormatSHA256)
	}
	if release.GitHubActionsReleaseWorkflowContractVersion != 1 {
		t.Fatalf("workflow contract version = %d, want 1", release.GitHubActionsReleaseWorkflowContractVersion)
	}
}
