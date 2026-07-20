package config

import (
	"os"
	"path/filepath"
)

// V2ConfigPath returns the canonical V2 config path for repositoryRoot.
func V2ConfigPath(repositoryRoot string) string {
	return filepath.Join(repositoryRoot, V2Directory, V2ConfigFileName)
}

// V2StatePath returns the canonical V2 state path for repositoryRoot.
func V2StatePath(repositoryRoot string) string {
	return filepath.Join(repositoryRoot, V2Directory, V2StateFileName)
}

// V2ConfigExists checks whether a repository root contains the V2 config.
func V2ConfigExists(repositoryRoot string) bool {
	_, err := os.Stat(V2ConfigPath(repositoryRoot))
	return err == nil
}

// V2StateExists checks whether a repository root contains the V2 state.
func V2StateExists(repositoryRoot string) bool {
	_, err := os.Stat(V2StatePath(repositoryRoot))
	return err == nil
}
