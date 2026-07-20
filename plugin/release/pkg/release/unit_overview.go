package release

import (
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/unitoverview"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

// HandleUnits resolves the local repository root and returns the Release V2
// unit inventory without mutation, workflow inspection, Git, tokens, or network.
func HandleUnits(request plugin.Request) (*plugin.Response, error) {
	return unitoverview.HandleUnits(request)
}

// HandleUnitsAt returns the Release V2 unit inventory at an explicit root.
func HandleUnitsAt(root workspace.RepositoryRoot, request plugin.Request) (*plugin.Response, error) {
	return unitoverview.HandleUnitsAt(root, request)
}
