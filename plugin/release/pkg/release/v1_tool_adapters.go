package release

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

// SystemV1BinaryLocator preserves the shared legacy executable lookup and
// reporting contract used by all three V1 release systems.
type SystemV1BinaryLocator struct{}

func NewSystemV1BinaryLocator() SystemV1BinaryLocator { return SystemV1BinaryLocator{} }

func (SystemV1BinaryLocator) Require(name string) error {
	log.PluginV(log.Init, "Searching for %s executable: %s", name, log.ColorText(log.ColorGreen, "which "+name))
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("required dependency missing: %s: %w", path, err)
	}
	log.PluginPrint(log.Init, "\uF00C Found required %s executable", log.ColorText(log.ColorCyan, name))
	return nil
}

// SystemV1FileInspector preserves the shared legacy file-existence contract.
type SystemV1FileInspector struct{}

func NewSystemV1FileInspector() SystemV1FileInspector { return SystemV1FileInspector{} }

func (SystemV1FileInspector) Exists(repositoryRoot, path string) (bool, error) {
	_, err := os.Stat(filepath.Join(repositoryRoot, path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
