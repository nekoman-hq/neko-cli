package cmd

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/spf13/cobra"
)

func TestReleaseOverviewIsCoreOwnedAndPresentationFlagsAreNoOps(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	restorePluginDir(t, t.TempDir())

	baseline := executeReleaseOverview(t, manifest)
	for _, args := range [][]string{
		{"--describe"},
		{"--verbose"},
		{"--describe", "--verbose"},
		{"--output", "json"},
	} {
		if got := executeReleaseOverview(t, manifest, args...); got != baseline {
			t.Fatalf("release overview changed for %v:\nwant:\n%s\ngot:\n%s", args, baseline, got)
		}
	}
	for _, want := range []string{
		"Plugin: release",
		"Available Commands:",
		"plugin-index",
		`Use "neko release <command> --help" for command details.`,
	} {
		if !strings.Contains(baseline, want) {
			t.Fatalf("release overview omitted %q:\n%s", want, baseline)
		}
	}
	for _, forbidden := range []string{"Command Metadata", "Execution Logs", "Global plugin-response flags"} {
		if strings.Contains(baseline, forbidden) {
			t.Fatalf("release overview contains plugin-response section %q:\n%s", forbidden, baseline)
		}
	}
}

func TestReleaseInitOptionsCharacterizesIntentionalNoOpModes(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	root := newReleaseLifecycleV2Repository(t)

	defaultOutput, defaultErr := executeReleaseReadonlyCommand(
		t, manifest, root, "init-options", nil, releaseReadonlyMode{},
	)
	verboseOutput, verboseErr := executeReleaseReadonlyCommand(
		t, manifest, root, "init-options", nil, releaseReadonlyMode{verbose: true},
	)
	describeOutput, describeErr := executeReleaseReadonlyCommand(
		t, manifest, root, "init-options", nil, releaseReadonlyMode{describe: true},
	)
	combinedOutput, combinedErr := executeReleaseReadonlyCommand(
		t, manifest, root, "init-options", nil, releaseReadonlyMode{describe: true, verbose: true},
	)
	if defaultErr != nil || verboseErr != nil || describeErr != nil || combinedErr != nil {
		t.Fatalf(
			"init-options exits: default=%v verbose=%v describe=%v combined=%v",
			defaultErr,
			verboseErr,
			describeErr,
			combinedErr,
		)
	}
	if verboseOutput != defaultOutput {
		t.Fatalf("init-options verbose changed complete default output")
	}
	if combinedOutput != describeOutput {
		t.Fatalf("init-options combined mode added content beyond ordinary describe metadata")
	}
	if strings.Contains(describeOutput, "Execution Logs") {
		t.Fatalf("init-options describe emitted execution logs:\n%s", describeOutput)
	}

	assertReleaseNoOpJSONInvariance(t, manifest, root, "init-options", nil)
}

func TestReleaseHistoryAndContributorsCharacterizeGenericVerboseLogDrift(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	root := newReleaseLifecycleV2Repository(t)
	flags := []string{"--unit", "api"}

	statusBefore := runReleaseReadonlyGit(t, root, "status", "--short")
	headBefore := runReleaseReadonlyGit(t, root, "rev-parse", "HEAD")
	tagsBefore := runReleaseReadonlyGit(t, root, "tag", "--list")

	tests := []struct {
		command string
		logs    []string
	}{
		{command: "history", logs: []string{"Starting release history"}},
		{command: "contributors", logs: []string{"Collecting contributors", "Successfully collected contributors"}},
	}
	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			defaultOutput, defaultErr := executeReleaseReadonlyCommand(
				t, manifest, root, test.command, flags, releaseReadonlyMode{},
			)
			verboseOutput, verboseErr := executeReleaseReadonlyCommand(
				t, manifest, root, test.command, flags, releaseReadonlyMode{verbose: true},
			)
			if defaultErr != nil || verboseErr != nil {
				t.Fatalf("%s exits: default=%v verbose=%v", test.command, defaultErr, verboseErr)
			}
			for _, logLine := range test.logs {
				if strings.Contains(defaultOutput, logLine) || !strings.Contains(verboseOutput, logLine) {
					t.Fatalf(
						"%s generic log characterization for %q changed:\ndefault:\n%s\nverbose:\n%s",
						test.command,
						logLine,
						defaultOutput,
						verboseOutput,
					)
				}
			}
			assertReleaseNoOpJSONInvariance(t, manifest, root, test.command, flags)
		})
	}

	if got := runReleaseReadonlyGit(t, root, "status", "--short"); got != statusBefore {
		t.Fatalf("no-op characterization mutated worktree/index: before %q after %q", statusBefore, got)
	}
	if got := runReleaseReadonlyGit(t, root, "rev-parse", "HEAD"); got != headBefore {
		t.Fatalf("no-op characterization moved HEAD: before %q after %q", headBefore, got)
	}
	if got := runReleaseReadonlyGit(t, root, "tag", "--list"); got != tagsBefore {
		t.Fatalf("no-op characterization changed tags: before %q after %q", tagsBefore, got)
	}
}

func assertReleaseNoOpJSONInvariance(
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
	var baseline releaseReadonlyPublicResponse
	for index, mode := range modes {
		output, err := executeReleaseReadonlyCommand(t, manifest, root, command, flags, mode)
		if err != nil {
			t.Fatalf("%s JSON mode %#v: %v", command, mode, err)
		}
		response := decodeReleaseReadonlyPublicResponse(t, output)
		if index == 0 {
			baseline = response
		} else if response.Status != baseline.Status || !reflect.DeepEqual(response.Data, baseline.Data) {
			t.Fatalf("%s JSON status or domain data changed in mode %#v", command, mode)
		}
		for _, forbidden := range []string{
			"human_table",
			"human_properties",
			"describe_only",
			"\x1b[",
			"final-output-secret",
		} {
			if strings.Contains(output, forbidden) {
				t.Fatalf("%s JSON contains %q:\n%s", command, forbidden, output)
			}
		}
	}
}

func executeReleaseOverview(t *testing.T, manifest plugin.Manifest, args ...string) string {
	t.Helper()
	describe = false
	verbose = false
	outputFormat = "table"
	githubOutputFile = ""

	root := &cobra.Command{Use: "neko", SilenceUsage: true}
	root.PersistentFlags().BoolVar(&describe, "describe", false, "structured details")
	root.PersistentFlags().BoolVar(&verbose, "verbose", false, "execution logs")
	root.PersistentFlags().StringVar(&outputFormat, "output", "table", "output format")
	root.PersistentFlags().StringVar(&githubOutputFile, "github-output-file", "", "GitHub output file")
	root.AddCommand(CreatePluginCommand(manifest))
	commandArgs := []string{"release"}
	commandArgs = append(commandArgs, args...)
	root.SetArgs(commandArgs)

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(io.Discard)
	if err := root.Execute(); err != nil {
		t.Fatalf("execute release overview %v: %v", args, err)
	}
	return output.String()
}
