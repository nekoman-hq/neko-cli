package releaseit

import (
	"os"
	"os/exec"

	release2 "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

type v1Environment interface {
	Environ() []string
}

type systemV1Environment struct{}

func (systemV1Environment) Environ() []string { return os.Environ() }

type systemV1Process struct {
	environment v1Environment
}

func newSystemV1Process() systemV1Process {
	return systemV1Process{environment: systemV1Environment{}}
}

func (process systemV1Process) Run(repositoryRoot, executable string, args ...string) ([]byte, error) {
	cmd := exec.Command(executable, args...)
	cmd.Dir = repositoryRoot
	environment := process.environment.Environ()
	cmd.Env = environment
	output, err := cmd.CombinedOutput()
	return release2.RedactV1ProcessResultFromEnvironment(output, err, environment)
}

type systemV1ConfigStore struct{}

func (systemV1ConfigStore) Save(repositoryRoot string, config *Config) error {
	return SaveConfigAt(repositoryRoot, config)
}
