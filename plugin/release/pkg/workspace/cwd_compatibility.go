package workspace

import (
	"fmt"
	"os"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

// ChangeToProjectRoot switches the process working directory to the resolved
// project root so relative config and tool files work from nested subfolders.
//
// Deprecated: resolve a RepositoryRoot and pass it explicitly to root-aware
// command boundaries instead.
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
