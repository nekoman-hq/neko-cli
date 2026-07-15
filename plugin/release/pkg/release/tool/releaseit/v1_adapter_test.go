package releaseit

import (
	"strings"
	"testing"

	release2 "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

type recordingV1GitReader struct {
	events *[]string
	heads  []string
}

func (git *recordingV1GitReader) Head(root string) (string, error) {
	*git.events = append(*git.events, "head "+root)
	head := git.heads[0]
	git.heads = git.heads[1:]
	return head, nil
}

type recordingV1Process struct{ events *[]string }

func (process recordingV1Process) Run(root, executable string, args ...string) ([]byte, error) {
	*process.events = append(*process.events, "process "+root+" "+executable+" "+strings.Join(args, " "))
	return nil, nil
}

type mappedV1Files struct{ existing map[string]bool }

func (files mappedV1Files) Exists(root, path string) (bool, error) {
	return files.existing[root+"/"+path], nil
}

func TestV1ExecutorUsesRepositoryPackageEvidenceAndReleaseItCommand(t *testing.T) {
	events := []string{}
	executor := &ReleaseIt{
		git: &recordingV1GitReader{
			events: &events,
			heads:  []string{"before", "release"},
		},
		process: recordingV1Process{events: &events},
		files: mappedV1Files{existing: map[string]bool{
			"/repo/bun.lock":          true,
			"/repo/package-lock.json": true,
		}},
	}
	request := release2.V1ExecutorRequest{Plan: release2.V1ReleasePlan{RepositoryRoot: "/repo", NextVersion: "1.2.4"}}
	if err := executor.Run(request); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []string{
		"head /repo",
		"process /repo bunx release-it 1.2.4 --ci --no-git.requireCleanWorkingDir",
		"head /repo",
	}
	if strings.Join(events, "\n") != strings.Join(want, "\n") {
		t.Fatalf("events = %v, want %v", events, want)
	}
	if executor.packageManager != "bun" {
		t.Fatalf("package manager = %q, want bun", executor.packageManager)
	}
	if executor.State.PreHead != "before" || executor.State.ReleaseCommitHash != "release" || !executor.State.PushedCommit || !executor.State.PushedTag || !executor.State.CreatedGitHubRelease {
		t.Fatalf("release evidence = %#v", executor.State)
	}
}
