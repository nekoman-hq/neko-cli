package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestReleaseSharedCommandsPreserveV1V2DomainParityAcrossPresentationModes(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("GITHUB_TOKEN", "cross-command-parity-secret")
	t.Setenv("GH_TOKEN", "cross-command-parity-secret")
	manifest := installReleaseReadonlyHelperPlugin(t)

	v1Root := newReleaseLifecycleV1Repository(t)
	if err := os.WriteFile(filepath.Join(v1Root, ".goreleaser.yml"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write V1 release-tool fixture: %v", err)
	}
	runReleaseReadonlyGit(t, v1Root, "add", ".goreleaser.yml")
	runReleaseReadonlyGit(t, v1Root, "commit", "-m", "add V1 release tool fixture")

	repositories := []struct {
		name string
		root string
		unit string
	}{
		{name: "V1", root: v1Root, unit: "default"},
		{name: "V2", root: newReleaseLifecycleV2Repository(t), unit: "api"},
	}
	for _, repository := range repositories {
		t.Run(repository.name, func(t *testing.T) {
			filesBefore := snapshotReleaseSetupFiles(t, repository.root)
			statusBefore := runReleaseReadonlyGit(t, repository.root, "status", "--short")
			headBefore := runReleaseReadonlyGit(t, repository.root, "rev-parse", "HEAD")
			tagsBefore := runReleaseReadonlyGit(t, repository.root, "tag", "--list")
			journalsBefore := releaseLifecycleJournalSnapshot(t, repository.root)

			commands := []struct {
				name  string
				flags []string
			}{
				{name: "patch", flags: []string{"--unit", repository.unit, "--dry-run"}},
				{name: "minor", flags: []string{"--unit", repository.unit, "--dry-run"}},
				{name: "major", flags: []string{"--unit", repository.unit, "--dry-run"}},
				{name: "plan", flags: []string{"--change", "patch", "--unit", repository.unit}},
				{name: "history", flags: []string{"--unit", repository.unit}},
				{name: "contributors", flags: []string{"--unit", repository.unit}},
				{name: "validate", flags: []string{"--unit", repository.unit}},
				{name: "evidence"},
			}
			for _, command := range commands {
				t.Run(command.name, func(t *testing.T) {
					assertReleaseCommandJSONModesPreserveDomain(
						t,
						manifest,
						repository.root,
						command.name,
						command.flags,
					)
				})
			}

			if got := snapshotReleaseSetupFiles(t, repository.root); !reflect.DeepEqual(got, filesBefore) {
				t.Fatalf("%s presentation matrix changed repository files", repository.name)
			}
			if got := runReleaseReadonlyGit(t, repository.root, "status", "--short"); got != statusBefore {
				t.Fatalf("%s presentation matrix changed worktree/index: before %q after %q", repository.name, statusBefore, got)
			}
			if got := runReleaseReadonlyGit(t, repository.root, "rev-parse", "HEAD"); got != headBefore {
				t.Fatalf("%s presentation matrix moved HEAD: before %q after %q", repository.name, headBefore, got)
			}
			if got := runReleaseReadonlyGit(t, repository.root, "tag", "--list"); got != tagsBefore {
				t.Fatalf("%s presentation matrix changed tags: before %q after %q", repository.name, tagsBefore, got)
			}
			if got := releaseLifecycleJournalSnapshot(t, repository.root); !reflect.DeepEqual(got, journalsBefore) {
				t.Fatalf("%s presentation matrix changed release journals", repository.name)
			}
		})
	}
}

func assertReleaseCommandJSONModesPreserveDomain(
	t *testing.T,
	manifest plugin.Manifest,
	root string,
	command string,
	flags []string,
) {
	t.Helper()
	modes := []releaseReadonlyMode{
		{format: "json"},
		{format: "json", describe: true},
		{format: "json", verbose: true},
		{format: "json", describe: true, verbose: true},
	}
	var (
		baseline     releaseReadonlyPublicResponse
		baselineExit error
	)
	for index, mode := range modes {
		output, executeErr := executeReleaseReadonlyCommand(t, manifest, root, command, flags, mode)
		response := decodeReleaseReadonlyPublicResponse(t, output)
		if index == 0 {
			baseline = response
			baselineExit = executeErr
		} else {
			if !samePluginExit(baselineExit, executeErr) {
				t.Fatalf("%s mode %#v changed exit: baseline=%v got=%v", command, mode, baselineExit, executeErr)
			}
			if response.Status != baseline.Status || !reflect.DeepEqual(response.Data, baseline.Data) {
				t.Fatalf("%s mode %#v changed status or domain JSON", command, mode)
			}
		}
		for _, forbidden := range []string{
			"\x1b[",
			"cross-command-parity-secret",
			"human_table",
			"human_properties",
			"describe_only",
		} {
			if strings.Contains(output, forbidden) {
				t.Fatalf("%s mode %#v exposed %q:\n%s", command, mode, forbidden, output)
			}
		}
	}
}
