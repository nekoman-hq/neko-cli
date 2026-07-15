package goreleaser

import (
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
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
func (git recordingV1GitWriter) CreateGitTag(root string, version *semver.Version) error {
	*git.events = append(*git.events, "tag "+root+" "+version.String())
	return nil
}
func (git recordingV1GitWriter) PushCommits(root string) error {
	*git.events = append(*git.events, "push-commit "+root)
	return nil
}
func (git recordingV1GitWriter) PushGitTag(root string, version *semver.Version) error {
	*git.events = append(*git.events, "push-tag "+root+" "+version.String())
	return nil
}

type recordingV1Process struct{ events *[]string }

func (process recordingV1Process) Run(root string, args, environment []string) ([]byte, error) {
	*process.events = append(*process.events, "process "+root+" "+strings.Join(args, " ")+" env="+strings.Join(environment, ","))
	return nil, nil
}

type fixedV1Environment struct{}

func (fixedV1Environment) Environ() []string { return []string{"VISIBLE=yes"} }

type recordingV1Rollback struct {
	root  string
	state release2.GitReleaseState
}

func (rollback *recordingV1Rollback) Rollback(root string, state release2.GitReleaseState) error {
	rollback.root = root
	rollback.state = state
	return nil
}

func TestV1ExecutorUsesExplicitRepositoryGitProcessEnvironmentAndRollback(t *testing.T) {
	events := []string{}
	rollback := &recordingV1Rollback{}
	executor := &GoReleaser{
		git:         recordingV1GitWriter{events: &events},
		rollback:    rollback,
		process:     recordingV1Process{events: &events},
		environment: fixedV1Environment{},
	}
	request := release2.V1ExecutorRequest{Plan: release2.V1ReleasePlan{RepositoryRoot: "/repo", NextVersion: "1.2.4"}}
	if err := executor.Run(request); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []string{
		"head /repo",
		"commit /repo 1.2.4",
		"head /repo",
		"tag /repo 1.2.4",
		"push-commit /repo",
		"push-tag /repo 1.2.4",
		"process /repo release --snapshot --clean env=VISIBLE=yes",
		"process /repo release --clean env=VISIBLE=yes",
	}
	if strings.Join(events, "\n") != strings.Join(want, "\n") {
		t.Fatalf("events = %v, want %v", events, want)
	}
	if err := executor.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rollback.root != "/repo" || !rollback.state.PushedCommit || !rollback.state.PushedTag || !rollback.state.CreatedGitHubRelease {
		t.Fatalf("rollback evidence = root %q state %#v", rollback.root, rollback.state)
	}
}
