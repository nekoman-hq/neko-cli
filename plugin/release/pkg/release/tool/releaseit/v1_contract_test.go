package releaseit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
)

func TestV1ReleaseItCommandOrderAndOwnership(t *testing.T) {
	logPath := installFakeV1Commands(t, "git", "npx")
	releaser := &ReleaseIt{}

	if err := releaser.Release(semver.MustParse("1.2.4")); err != nil {
		t.Fatalf("Release: %v", err)
	}
	assertFakeV1Commands(t, logPath, []string{
		"git rev-parse --short HEAD",
		"npx release-it 1.2.4 --ci --no-git.requireCleanWorkingDir",
		"git rev-parse --short HEAD",
	})
	if releaser.packageManager != "npm" {
		t.Fatalf("default package manager = %q, want npm", releaser.packageManager)
	}
	if releaser.State.PreHead != "fake-head" || releaser.State.ReleaseCommitHash != "fake-head" || releaser.State.TagName != "v1.2.4" {
		t.Fatalf("unexpected release-it identity: %#v", releaser.State)
	}
	if !releaser.State.PushedCommit || !releaser.State.PushedTag || !releaser.State.CreatedGitHubRelease {
		t.Fatalf("release-it must retain commit/tag/push/publish ownership: %#v", releaser.State)
	}
}

func TestV1ReleaseItFailureDoesNotClaimOwnedEffects(t *testing.T) {
	logPath := installFakeV1Commands(t, "git", "npx")
	t.Setenv("NEKO_V1_COMMAND_FAIL", "npx release-it 1.2.4 --ci --no-git.requireCleanWorkingDir")
	releaser := &ReleaseIt{}

	if err := releaser.Release(semver.MustParse("1.2.4")); err == nil {
		t.Fatal("Release succeeded at injected release-it failure")
	}
	assertFakeV1Commands(t, logPath, []string{
		"git rev-parse --short HEAD",
		"npx release-it 1.2.4 --ci --no-git.requireCleanWorkingDir",
	})
	if releaser.State.ReleaseCommitHash != "" || releaser.State.TagName != "" || releaser.State.PushedCommit || releaser.State.PushedTag || releaser.State.CreatedGitHubRelease {
		t.Fatalf("failed release-it command claimed effects: %#v", releaser.State)
	}
}

func TestV1ReleaseItPackageManagerCompatibility(t *testing.T) {
	tests := []struct { //nolint:govet // Logical fixture order keeps the case name before filesystem evidence.
		name  string
		files []string
		want  string
	}{
		{name: "default npm", want: "npm"},
		{name: "package lock", files: []string{"package-lock.json"}, want: "npm"},
		{name: "bun lock", files: []string{"bun.lock"}, want: "bun"},
		{name: "bun takes precedence", files: []string{"package-lock.json", "bun.lock"}, want: "bun"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installFakeV1Commands(t, "git", "npx", "bunx")
			for _, file := range tt.files {
				if err := os.WriteFile(file, nil, 0644); err != nil {
					t.Fatalf("write %s: %v", file, err)
				}
			}
			releaser := &ReleaseIt{}
			if got := releaser.detectPackageManager(); got != tt.want {
				t.Fatalf("detectPackageManager = %q, want %q", got, tt.want)
			}
		})
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
