//nolint:staticcheck // V1 executor adapters intentionally use the deprecated token contract.
package jreleaser

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	coreconfig "github.com/nekoman-hq/neko-cli/pkg/config"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	release2 "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

type v1TokenResolver interface {
	Resolve() (string, error)
}

type systemV1TokenResolver struct{}

func (systemV1TokenResolver) Resolve() (string, error) { return coreconfig.GetPAT() }

type v1Environment interface {
	Environ() []string
}

type systemV1Environment struct{}

func (systemV1Environment) Environ() []string { return os.Environ() }

type v1CommandProcess interface {
	Run(string, []string, []string) ([]byte, error)
}

type systemV1CommandProcess struct{}

func (systemV1CommandProcess) Run(repositoryRoot string, args, environment []string) ([]byte, error) {
	cmd := exec.Command("jreleaser", args...)
	cmd.Dir = repositoryRoot
	cmd.Env = environment
	return cmd.CombinedOutput()
}

type systemV1JReleaserCommand struct {
	tokens      v1TokenResolver
	environment v1Environment
	process     v1CommandProcess
}

func newSystemV1JReleaserCommand() systemV1JReleaserCommand {
	return systemV1JReleaserCommand{
		tokens:      systemV1TokenResolver{},
		environment: systemV1Environment{},
		process:     systemV1CommandProcess{},
	}
}

func (command systemV1JReleaserCommand) Run(repositoryRoot string, args ...string) ([]byte, error) {
	token, err := command.tokens.Resolve()
	if err != nil {
		return nil, err
	}
	log.PluginV(log.Init, "Executing command: JRELEASER_GITHUB_TOKEN=***** jreleaser %s", strings.Join(args, " "))
	output, processErr := command.process.Run(
		repositoryRoot,
		args,
		append(command.environment.Environ(), "JRELEASER_GITHUB_TOKEN="+token),
	)
	redactedOutput, redactedError := release2.RedactV1ProcessResult(output, processErr, token)
	if redactedError != nil {
		return redactedOutput, fmt.Errorf("failed to execute command: %w", redactedError)
	}
	return redactedOutput, nil
}

type systemV1ConfigStore struct{}

func (systemV1ConfigStore) Load(repositoryRoot string) (*Config, error) {
	return LoadConfigAt(repositoryRoot)
}
func (systemV1ConfigStore) Save(repositoryRoot string, config *Config) error {
	return SaveConfigAt(repositoryRoot, config)
}

type systemV1Clock struct{}

func (systemV1Clock) Year() int { return time.Now().Year() }
