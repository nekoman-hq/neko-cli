package goreleaser

import (
	"os"
	"os/exec"

	release2 "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

type systemV1Process struct{}

func (systemV1Process) Run(repositoryRoot string, args []string, environment []string) ([]byte, error) {
	cmd := exec.Command("goreleaser", args...)
	cmd.Dir = repositoryRoot
	if environment != nil {
		cmd.Env = environment
	}
	output, err := cmd.CombinedOutput()
	return release2.RedactV1ProcessResultFromEnvironment(output, err, environment)
}

type systemV1Environment struct{}

func (systemV1Environment) Environ() []string { return os.Environ() }
