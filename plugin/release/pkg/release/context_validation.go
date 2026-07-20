package release

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/contextvalidation"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

// GitObjectFormat identifies the repository object-ID format validated for a
// dispatched release context.
type GitObjectFormat = contextvalidation.GitObjectFormat

const (
	GitObjectFormatSHA1   = contextvalidation.GitObjectFormatSHA1
	GitObjectFormatSHA256 = contextvalidation.GitObjectFormatSHA256
)

// ReleaseContextValidationRequest is the supported typed application input
// for validating one dispatched Release V2 context.
type ReleaseContextValidationRequest = contextvalidation.ReleaseContextValidationRequest

// ValidatedReleaseContext contains canonical local release facts that passed
// source, unit, version, tag, commit, HEAD, and tag-target checks.
type ValidatedReleaseContext = contextvalidation.ValidatedReleaseContext

// ParseReleaseContextValidationRequest isolates the plugin transport's
// untyped flag map and supplies the validated explicit repository root.
func ParseReleaseContextValidationRequest(root workspace.RepositoryRoot, request plugin.Request) ReleaseContextValidationRequest {
	return contextvalidation.ParseReleaseContextValidationRequest(root, request)
}

// MapValidatedReleaseContext maps the typed result to the supported plugin
// response and presentation contract.
func MapValidatedReleaseContext(result *ValidatedReleaseContext, timestamp time.Time) *plugin.Response {
	return contextvalidation.MapValidatedReleaseContext(result, timestamp)
}

// HandleReleaseContextValidation resolves the repository root from the request
// context and validates one dispatched Release V2 context without mutation.
func HandleReleaseContextValidation(request plugin.Request) (*plugin.Response, error) {
	return contextvalidation.HandleReleaseContextValidation(request)
}

// HandleReleaseContextValidationAt validates one dispatched Release V2 context
// against an explicit repository root without changing process cwd.
func HandleReleaseContextValidationAt(root workspace.RepositoryRoot, request plugin.Request) (*plugin.Response, error) {
	return contextvalidation.HandleReleaseContextValidationAt(root, request)
}
