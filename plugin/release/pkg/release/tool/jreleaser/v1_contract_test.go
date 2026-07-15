package jreleaser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
)

func TestV1JReleaserCommandOrderConfigAndOwnership(t *testing.T) {
	logPath := installFakeV1Commands(t, "git", "jreleaser")
	t.Setenv("GITHUB_TOKEN", "test-token")
	writeV1JReleaserConfig(t, "1.2.3")
	releaser := &JReleaser{}

	if err := releaser.Release(semver.MustParse("1.2.4")); err != nil {
		t.Fatalf("Release: %v", err)
	}
	assertFakeV1Commands(t, logPath, []string{
		"git rev-parse --short HEAD",
		"git commit --allow-empty -a -m chore(neko-release): 1.2.4",
		"git rev-parse --short HEAD",
		"git push origin HEAD",
		"jreleaser full-release --dry-run token=set",
		"jreleaser full-release token=set",
	})
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if config.Project.Version != "1.2.4" {
		t.Fatalf("jreleaser project version = %q, want 1.2.4", config.Project.Version)
	}
	if releaser.State.PreHead != "fake-head" || releaser.State.ReleaseCommitHash != "fake-head" || releaser.State.TagName != "v1.2.4" {
		t.Fatalf("unexpected JReleaser identity: %#v", releaser.State)
	}
	if !releaser.State.PushedCommit || !releaser.State.RanJRelease {
		t.Fatalf("unexpected JReleaser ownership flags: %#v", releaser.State)
	}
}

func TestV1JReleaserDryRunFailureWarnsAndContinues(t *testing.T) {
	logPath := installFakeV1Commands(t, "git", "jreleaser")
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("NEKO_V1_COMMAND_FAIL", "jreleaser full-release --dry-run")
	writeV1JReleaserConfig(t, "1.2.3")
	releaser := &JReleaser{}

	if err := releaser.Release(semver.MustParse("1.2.4")); err != nil {
		t.Fatalf("dry-run failure must remain warning-only: %v", err)
	}
	commands := readFakeV1Commands(t, logPath)
	if commands[len(commands)-1] != "jreleaser full-release token=set" || !releaser.State.RanJRelease {
		t.Fatalf("full release did not follow warning-only dry-run: %#v, state=%#v", commands, releaser.State)
	}
}

func TestV1JReleaserFailureBoundaries(t *testing.T) {
	tests := []struct {
		name           string
		fail           string
		wantCommands   int
		wantCommitPush bool
	}{
		{name: "commit", fail: "git commit --allow-empty -a -m chore(neko-release): 1.2.4", wantCommands: 2},
		{name: "commit push", fail: "git push origin HEAD", wantCommands: 4},
		{name: "publish", fail: "jreleaser full-release", wantCommands: 6, wantCommitPush: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := installFakeV1Commands(t, "git", "jreleaser")
			t.Setenv("GITHUB_TOKEN", "test-token")
			t.Setenv("NEKO_V1_COMMAND_FAIL", tt.fail)
			writeV1JReleaserConfig(t, "1.2.3")
			releaser := &JReleaser{}
			if err := releaser.Release(semver.MustParse("1.2.4")); err == nil {
				t.Fatalf("Release succeeded at injected %s failure", tt.name)
			}
			if got := len(readFakeV1Commands(t, logPath)); got != tt.wantCommands {
				t.Fatalf("command count = %d, want %d", got, tt.wantCommands)
			}
			if releaser.State.PushedCommit != tt.wantCommitPush || releaser.State.RanJRelease || releaser.State.TagName != "" {
				t.Fatalf("failure ownership flags = %#v", releaser.State)
			}
		})
	}
}

func writeV1JReleaserConfig(t *testing.T, version string) {
	t.Helper()
	config := &Config{Project: Project{Name: "example", Version: version}}
	if err := SaveConfig(config); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
}

func installFakeV1Commands(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("enter temp directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "commands.log")
	script := `#!/bin/sh
name=${0##*/}
raw="$name $*"
line="$raw"
if [ "$name" = "jreleaser" ] && [ -n "$JRELEASER_GITHUB_TOKEN" ]; then
  line="$line token=set"
fi
printf '%s\n' "$line" >> "$NEKO_V1_COMMAND_LOG"
if [ -n "$NEKO_V1_COMMAND_FAIL" ] && [ "$raw" = "$NEKO_V1_COMMAND_FAIL" ]; then
  printf 'injected command failure\n' >&2
  exit 23
fi
if [ "$raw" = "git rev-parse --short HEAD" ]; then
  printf 'fake-head\n'
fi
`
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(script), 0755); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}
	t.Setenv("NEKO_V1_COMMAND_LOG", logPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func readFakeV1Commands(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read command log: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func assertFakeV1Commands(t *testing.T, path string, want []string) {
	t.Helper()
	got := readFakeV1Commands(t, path)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("commands:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
