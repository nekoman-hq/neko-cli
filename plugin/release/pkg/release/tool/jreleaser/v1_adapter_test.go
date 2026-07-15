//nolint:staticcheck // These tests protect the deprecated V1 compatibility boundary.
package jreleaser

import (
	"errors"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	release2 "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

type recordingV1GitWriter struct{ events *[]string }

func (git recordingV1GitWriter) Head(root string) (string, error) {
	*git.events = append(*git.events, "head "+root)
	return "abc123", nil
}
func (git recordingV1GitWriter) CreateReleaseCommit(root string, version *semver.Version) error {
	*git.events = append(*git.events, "commit "+root+" "+version.String())
	return nil
}
func (git recordingV1GitWriter) PushCommits(root string) error {
	*git.events = append(*git.events, "push "+root)
	return nil
}

type recordingV1Commands struct{ events *[]string }

func (commands recordingV1Commands) Run(root string, args ...string) ([]byte, error) {
	*commands.events = append(*commands.events, "command "+root+" "+strings.Join(args, " "))
	return nil, nil
}

type existingV1Files struct{ exists bool }

func (files existingV1Files) Exists(_, _ string) (bool, error) { return files.exists, nil }

type recordingV1Configs struct {
	events *[]string
	loaded *Config
	saved  *Config
}

func (configs *recordingV1Configs) Load(root string) (*Config, error) {
	*configs.events = append(*configs.events, "load "+root)
	copy := *configs.loaded
	return &copy, nil
}
func (configs *recordingV1Configs) Save(root string, config *Config) error {
	*configs.events = append(*configs.events, "save "+root+" "+config.Project.Version)
	configs.saved = config
	return nil
}

func TestV1ExecutorUsesExplicitRepositoryGitConfigAndCommands(t *testing.T) {
	events := []string{}
	configs := &recordingV1Configs{events: &events, loaded: &Config{Project: Project{Version: "1.2.3"}}}
	executor := &JReleaser{
		git:      recordingV1GitWriter{events: &events},
		commands: recordingV1Commands{events: &events},
		files:    existingV1Files{exists: true},
		configs:  configs,
	}
	request := release2.V1ExecutorRequest{Plan: release2.V1ReleasePlan{RepositoryRoot: "/repo", NextVersion: "1.2.4"}}
	if err := executor.Run(request); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []string{
		"head /repo",
		"load /repo",
		"save /repo 1.2.4",
		"commit /repo 1.2.4",
		"head /repo",
		"push /repo",
		"command /repo full-release --dry-run",
		"command /repo full-release",
	}
	if strings.Join(events, "\n") != strings.Join(want, "\n") {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

type fixedV1Token struct{ value string }

func (token fixedV1Token) Resolve() (string, error) { return token.value, nil }

type fixedV1Environment struct{}

func (fixedV1Environment) Environ() []string { return []string{"VISIBLE=yes"} }

type failingV1CommandProcess struct {
	cause       error
	secret      string
	environment []string
}

func (process *failingV1CommandProcess) Run(_ string, _, environment []string) ([]byte, error) {
	process.environment = environment
	return []byte("output " + process.secret), process.cause
}

func TestV1JReleaserCommandRedactsSecretAndPreservesCause(t *testing.T) {
	secret := "jreleaser-sentinel-secret"
	cause := errors.New("process exposed " + secret)
	process := &failingV1CommandProcess{cause: cause, secret: secret}
	command := systemV1JReleaserCommand{
		tokens:      fixedV1Token{value: secret},
		environment: fixedV1Environment{},
		process:     process,
	}
	output, err := command.Run("/repo", "full-release")
	if err == nil || strings.Contains(string(output), secret) || strings.Contains(err.Error(), secret) || !errors.Is(err, cause) {
		t.Fatalf("output=%q error=%v preserves cause=%t", output, err, errors.Is(err, cause))
	}
	wantEnvironment := "VISIBLE=yes\nJRELEASER_GITHUB_TOKEN=" + secret
	if strings.Join(process.environment, "\n") != wantEnvironment {
		t.Fatalf("process environment = %v", process.environment)
	}
}

type fixedV1Clock struct{ year int }

func (clock fixedV1Clock) Year() int { return clock.year }

type successfulV1BinaryLocator struct{}

func (successfulV1BinaryLocator) Require(string) error { return nil }

func TestV1JReleaserInitUsesInjectedClock(t *testing.T) {
	events := []string{}
	configs := &recordingV1Configs{events: &events}
	executor := &JReleaser{
		commands: recordingV1Commands{events: &events},
		files:    existingV1Files{exists: false},
		configs:  configs,
		clock:    fixedV1Clock{year: 2042},
		binaries: successfulV1BinaryLocator{},
	}
	cfg := &releaseconfig.V1ReleaseConfig{ProjectName: "example", ProjectOwner: "acme", Version: "1.2.3"}
	if err := executor.Init(cfg); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if configs.saved == nil || configs.saved.Project.InceptionYear != "2042" {
		t.Fatalf("saved inception year = %#v", configs.saved)
	}
}
