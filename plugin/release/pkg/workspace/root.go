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
	rootDir, err := ResolveProjectRoot(startDir)
	if err != nil {
		return err
	}

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
