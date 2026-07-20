//nolint:staticcheck // Migration root discovery intentionally recognizes the deprecated V1 file.
package migrate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type gitMigrationRootResolver struct{}

func (gitMigrationRootResolver) Resolve(startDirectory string) (string, error) {
	return gitRoot(startDirectory)
}

func gitRoot(startDir string) (string, error) {
	if startDir == "" {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("determine working directory: %w", err)
		}
	}
	command := exec.Command("git", "rev-parse", "--show-toplevel")
	command.Dir = startDir
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("determine git repository root from %s: %s", startDir, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func findNestedV1(root, rootV1 string) (string, bool, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if entry.IsDir() || entry.Name() != releaseconfig.V1FileName || path == rootV1 {
			return nil
		}
		found = path
		return filepath.SkipAll
	})
	if err != nil {
		return "", false, fmt.Errorf("scan for nested V1 configs: %w", err)
	}
	return found, found != "", nil
}
