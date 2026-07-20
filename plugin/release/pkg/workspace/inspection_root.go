//nolint:staticcheck // Inspection root discovery intentionally recognizes the deprecated V1 file.
package workspace

import "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"

// ResolveInspectionRepositoryRoot resolves the local repository boundary
// without requiring release source files to be mutually valid. It is intended
// for read-only diagnostics that must report missing or conflicting V2 files.
func ResolveInspectionRepositoryRoot(startDir string) (RepositoryRoot, error) {
	absStartDir, err := absoluteStartDirectory(startDir)
	if err != nil {
		return RepositoryRoot{}, err
	}
	if gitRoot, found, findErr := findAncestorWithMarker(absStartDir, gitMarker); findErr != nil {
		return RepositoryRoot{}, findErr
	} else if found {
		return RepositoryRoot{path: gitRoot}, nil
	}
	if root, found, findErr := findAncestorWithMarker(absStartDir, config.V1FileName); findErr != nil {
		return RepositoryRoot{}, findErr
	} else if found {
		return RepositoryRoot{path: root}, nil
	}
	return RepositoryRoot{path: absStartDir}, nil
}
