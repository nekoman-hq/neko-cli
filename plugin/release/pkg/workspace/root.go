//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package workspace

//lint:file-ignore SA1019 Root resolution must detect the deprecated V1 file during compatibility

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const gitMarker = ".git"

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

// ResolveInspectionRepositoryRoot resolves the local repository
// boundary without requiring release source files to be mutually valid. It is
// intended for read-only diagnostics that must report missing or conflicting
// V2 config/state files rather than fail during root discovery.
func ResolveInspectionRepositoryRoot(startDir string) (RepositoryRoot, error) {
	absStartDir, err := inspectionStartDirectory(startDir)
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

func inspectionStartDirectory(startDir string) (string, error) {
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to determine working directory: %w", err)
		}
	}
	absStartDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute working directory: %w", err)
	}
	info, err := os.Stat(absStartDir)
	if err != nil {
		return "", fmt.Errorf("failed to inspect working directory %s: %w", absStartDir, err)
	}
	if !info.IsDir() {
		return filepath.Dir(absStartDir), nil
	}
	return absStartDir, nil
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

// ResolveProjectRoot finds the best project root for release operations.
// V2 is repository-root scoped: once a git root contains .neko config, nested
// V1 files cannot override it. Without V2, the legacy nearest-V1 behavior is
// preserved for existing repositories.
func ResolveProjectRoot(startDir string) (string, error) {
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to determine working directory: %w", err)
		}
	}

	absStartDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute working directory: %w", err)
	}

	info, err := os.Stat(absStartDir)
	if err != nil {
		return "", fmt.Errorf("failed to inspect working directory %s: %w", absStartDir, err)
	}
	if !info.IsDir() {
		absStartDir = filepath.Dir(absStartDir)
	}

	if gitRoot, found, err := findAncestorWithMarker(absStartDir, gitMarker); err != nil {
		return "", err
	} else if found {
		v2Config := config.V2ConfigPath(gitRoot)
		if _, err := os.Stat(v2Config); err == nil {
			if config.V1ConfigExistsAt(gitRoot) {
				return "", fmt.Errorf("release configuration conflict: %s and %s both exist at repository root", filepath.Join(gitRoot, config.V1FileName), v2Config)
			}
			if !config.V2StateExists(gitRoot) {
				return "", fmt.Errorf("release schema v2 config exists at %s, but required state file %s is missing", v2Config, config.V2StatePath(gitRoot))
			}
			return gitRoot, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to inspect %s: %w", v2Config, err)
		}

		if root, found, err := findAncestorWithMarker(absStartDir, config.V1FileName); err != nil {
			return "", err
		} else if found {
			return root, nil
		}

		return gitRoot, nil
	}

	if root, found, err := findAncestorWithMarker(absStartDir, config.V1FileName); err != nil {
		return "", err
	} else if found {
		return root, nil
	}

	return absStartDir, nil
}

// ChangeToProjectRoot switches the process working directory to the resolved
// project root so relative config and tool files work from nested subfolders.
func ChangeToProjectRoot(startDir string) error {
	root, err := ResolveRepositoryRoot(startDir)
	if err != nil {
		return err
	}
	rootDir := root.Path()

	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to determine current process directory: %w", err)
	}

	if currentDir == rootDir {
		log.PluginV(log.Config, "Using project root %s", log.ColorText(log.ColorGreen, rootDir))
		return nil
	}

	log.PluginV(
		log.Config,
		"Switching to project root %s (from %s)",
		log.ColorText(log.ColorGreen, rootDir),
		log.ColorText(log.ColorYellow, currentDir),
	)

	if err := os.Chdir(rootDir); err != nil {
		return fmt.Errorf("failed to switch to project root %s: %w", rootDir, err)
	}

	return nil
}

func findAncestorWithMarker(startDir string, marker string) (string, bool, error) {
	for dir := startDir; ; dir = filepath.Dir(dir) {
		markerPath := filepath.Join(dir, marker)
		if _, err := os.Stat(markerPath); err == nil {
			return dir, true, nil
		} else if !os.IsNotExist(err) {
			return "", false, fmt.Errorf("failed to inspect %s: %w", markerPath, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false, nil
		}
	}
}
