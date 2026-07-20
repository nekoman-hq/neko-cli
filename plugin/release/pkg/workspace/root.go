package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// RepositoryRoot is a resolved release repository root. Construct it through
// ResolveRepositoryRoot or ValidateRepositoryRoot so embedder-facing command
// entry points do not need to rediscover or mutate process cwd.
type RepositoryRoot struct {
	path string
}

// Path returns the resolved repository root path.
func (root RepositoryRoot) Path() string {
	return root.path
}

// String returns the resolved repository root path for diagnostics.
func (root RepositoryRoot) String() string {
	return root.path
}

// ResolveRepositoryRoot resolves a start directory using the Release Plugin's
// existing V2/V1 root discovery rules and returns the typed root for explicit
// command composition.
func ResolveRepositoryRoot(startDir string) (RepositoryRoot, error) {
	root, err := ResolveProjectRoot(startDir)
	if err != nil {
		return RepositoryRoot{}, err
	}
	return RepositoryRoot{path: root}, nil
}

// ValidateRepositoryRoot validates that root is already the resolved repository
// root according to the Release Plugin's existing discovery rules.
func ValidateRepositoryRoot(root string) (RepositoryRoot, error) {
	if root == "" {
		return RepositoryRoot{}, fmt.Errorf("repository root is required")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return RepositoryRoot{}, fmt.Errorf("failed to resolve absolute repository root: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return RepositoryRoot{}, fmt.Errorf("failed to inspect repository root %s: %w", absRoot, err)
	}
	if !info.IsDir() {
		return RepositoryRoot{}, fmt.Errorf("repository root %s is not a directory", absRoot)
	}

	resolved, err := ResolveProjectRoot(absRoot)
	if err != nil {
		return RepositoryRoot{}, err
	}
	if filepath.Clean(resolved) != filepath.Clean(absRoot) {
		return RepositoryRoot{}, fmt.Errorf("repository root %s resolves to %s; pass the resolved root", absRoot, resolved)
	}
	return RepositoryRoot{path: resolved}, nil
}
