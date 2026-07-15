package goreleaser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
)

func TestV1GoReleaserCommandOrderAndOwnership(t *testing.T) {
	logPath := installFakeV1Commands(t, "git", "goreleaser")
	releaser := &GoReleaser{}

	if err := releaser.Release(semver.MustParse("1.2.4")); err != nil {
		t.Fatalf("Release: %v", err)
	}
	assertFakeV1Commands(t, logPath, []string{
		"git rev-parse --short HEAD",
		"git commit --allow-empty -a -m chore(neko-release): 1.2.4",
		"git rev-parse --short HEAD",
		"git tag v1.2.4",
		"git push origin HEAD",
		"git push origin v1.2.4",
		"goreleaser release --snapshot --clean",
		"goreleaser release --clean",
	})
	if releaser.State.PreHead != "fake-head" || releaser.State.ReleaseCommitHash != "fake-head" || releaser.State.TagName != "v1.2.4" {
		t.Fatalf("unexpected GoReleaser release identity: %#v", releaser.State)
	}
	if !releaser.State.PushedCommit || !releaser.State.PushedTag || !releaser.State.RanGoRelease {
		t.Fatalf("unexpected GoReleaser ownership flags: %#v", releaser.State)
	}
}

func TestV1GoReleaserDryRunFailureWarnsAndContinues(t *testing.T) {
	logPath := installFakeV1Commands(t, "git", "goreleaser")
	t.Setenv("NEKO_V1_COMMAND_FAIL", "goreleaser release --snapshot --clean")
	releaser := &GoReleaser{}

	if err := releaser.Release(semver.MustParse("1.2.4")); err != nil {
		t.Fatalf("snapshot failure must remain warning-only: %v", err)
	}
	commands := readFakeV1Commands(t, logPath)
	if commands[len(commands)-1] != "goreleaser release --clean" || !releaser.State.RanGoRelease {
		t.Fatalf("full release did not follow warning-only snapshot failure: %#v, state=%#v", commands, releaser.State)
	}
}

func TestV1GoReleaserFailureBoundaries(t *testing.T) {
	tests := []struct {
		name           string
		fail           string
		wantCommands   int
		wantTag        bool
		wantCommitPush bool
		wantTagPush    bool
	}{
		{name: "commit", fail: "git commit --allow-empty -a -m chore(neko-release): 1.2.4", wantCommands: 2},
		{name: "tag", fail: "git tag v1.2.4", wantCommands: 4},
		{name: "commit push", fail: "git push origin HEAD", wantCommands: 5, wantTag: true},
		{name: "tag push", fail: "git push origin v1.2.4", wantCommands: 6, wantTag: true, wantCommitPush: true},
		{name: "publish", fail: "goreleaser release --clean", wantCommands: 8, wantTag: true, wantCommitPush: true, wantTagPush: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := installFakeV1Commands(t, "git", "goreleaser")
			t.Setenv("NEKO_V1_COMMAND_FAIL", tt.fail)
			releaser := &GoReleaser{}
			if err := releaser.Release(semver.MustParse("1.2.4")); err == nil {
				t.Fatalf("Release succeeded at injected %s failure", tt.name)
			}
			if got := len(readFakeV1Commands(t, logPath)); got != tt.wantCommands {
				t.Fatalf("command count = %d, want %d", got, tt.wantCommands)
			}
			if (releaser.State.TagName != "") != tt.wantTag || releaser.State.PushedCommit != tt.wantCommitPush || releaser.State.PushedTag != tt.wantTagPush {
				t.Fatalf("failure ownership flags = %#v", releaser.State)
			}
			if releaser.State.RanGoRelease {
				t.Fatal("failed release must not claim successful publication")
			}
		})
	}
}

func installFakeV1Commands(t *testing.T, names ...string) string {
	t.Helper()
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "commands.log")
	script := `#!/bin/sh
name=${0##*/}
line="$name $*"
printf '%s\n' "$line" >> "$NEKO_V1_COMMAND_LOG"
if [ -n "$NEKO_V1_COMMAND_FAIL" ] && [ "$line" = "$NEKO_V1_COMMAND_FAIL" ]; then
  printf 'injected command failure\n' >&2
  exit 23
fi
if [ "$line" = "git rev-parse --short HEAD" ]; then
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
		t.Fatalf("read fake command log: %v", err)
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
