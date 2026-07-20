package release

import (
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	internaldoctor "github.com/nekoman-hq/neko-cli/plugin/release/internal/doctor"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

// HandleDoctor resolves a repository root for local inspection and reports
// Release V2 GitHub Actions integration readiness without mutation.
func HandleDoctor(request plugin.Request) (*plugin.Response, error) {
	return internaldoctor.HandleDoctor(request)
}

// HandleDoctorAt reports Release V2 GitHub Actions integration readiness at
// an explicit repository root. It remains offline unless the request explicitly
// enables the focused read-only remote verifier.
func HandleDoctorAt(root workspace.RepositoryRoot, request plugin.Request) (*plugin.Response, error) {
	return internaldoctor.HandleDoctorAt(root, request)
}
