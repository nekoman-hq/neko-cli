package cmd

import (
	"bytes"
	"io"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/spf13/cobra"
)

var releaseHelpFlagPattern = regexp.MustCompile(`(?m)^  --([a-z0-9-]+)\s`)

func TestReleaseCommandHelpMatchesEveryManifestCommand(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	if len(manifest.Commands) != 20 {
		t.Fatalf("Release help command count = %d, want 20", len(manifest.Commands))
	}

	for _, command := range manifest.Commands {
		t.Run(command.Name, func(t *testing.T) {
			output := executeReleaseCommandHelp(t, manifest, command.Name)
			if !strings.Contains(output, "Plugin: release\n") ||
				!strings.Contains(output, "Command: "+command.Name+"\n") ||
				!strings.Contains(output, "Usage: neko release "+command.Name+" [flags]\n") {
				t.Fatalf("%s help identity is incomplete:\n%s", command.Name, output)
			}
			if len(command.Outputs) > 0 {
				want := "Outputs: " + strings.Join(command.Outputs, ", ")
				if !strings.Contains(output, want) {
					t.Fatalf("%s help omitted %q:\n%s", command.Name, want, output)
				}
			}
			if strings.Contains(output, "Outputs: text") {
				t.Fatalf("%s help exposes unsupported selectable text format:\n%s", command.Name, output)
			}

			general, pluginUnit := splitPluginUnitFlags(command.Flags)
			wantFlags := make([]string, 0, len(command.Flags)+len(inheritedPluginResponseFlagOrder))
			for _, flag := range append(general, pluginUnit...) {
				wantFlags = append(wantFlags, flag.Name)
			}
			wantFlags = append(wantFlags, inheritedPluginResponseFlagOrder...)

			matches := releaseHelpFlagPattern.FindAllStringSubmatch(output, -1)
			gotFlags := make([]string, 0, len(matches))
			seen := map[string]bool{}
			for _, match := range matches {
				name := match[1]
				if seen[name] {
					t.Fatalf("%s help duplicates --%s:\n%s", command.Name, name, output)
				}
				seen[name] = true
				gotFlags = append(gotFlags, name)
			}
			if !reflect.DeepEqual(gotFlags, wantFlags) {
				t.Fatalf("%s help flags = %v, want %v:\n%s", command.Name, gotFlags, wantFlags, output)
			}
			if strings.Count(output, "Global plugin-response flags:") != 1 {
				t.Fatalf("%s global help section count drifted:\n%s", command.Name, output)
			}

			for _, flag := range command.Flags {
				if !flag.Required {
					continue
				}
				line := releaseHelpFlagLine(t, output, flag.Name)
				if !strings.Contains(line, " required") {
					t.Fatalf("%s required flag --%s is not marked required: %q", command.Name, flag.Name, line)
				}
			}
		})
	}
}

func TestReleaseHelpKeepsPlanAndPluginIndexOwnershipExplicit(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	planHelp := executeReleaseCommandHelp(t, manifest, "plan")
	if !strings.Contains(planHelp, "--change") || !strings.Contains(releaseHelpFlagLine(t, planHelp, "change"), "required") {
		t.Fatalf("plan help does not require --change:\n%s", planHelp)
	}

	pluginIndexHelp := executeReleaseCommandHelp(t, manifest, "plugin-index")
	if !strings.Contains(pluginIndexHelp, "--output-file") {
		t.Fatalf("plugin-index help omitted --output-file:\n%s", pluginIndexHelp)
	}
	localFlags := pluginIndexHelp[:strings.Index(pluginIndexHelp, "Global plugin-response flags:")]
	for _, match := range releaseHelpFlagPattern.FindAllStringSubmatch(localFlags, -1) {
		if match[1] == "output" {
			t.Fatalf("plugin-index local help shadows Core --output:\n%s", pluginIndexHelp)
		}
	}
	if strings.Count(pluginIndexHelp, "--output") != 2 {
		t.Fatalf("plugin-index help should mention --output-file and inherited --output exactly once each:\n%s", pluginIndexHelp)
	}
}

func executeReleaseCommandHelp(t *testing.T, manifest plugin.Manifest, command string) string {
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
	root.SetArgs([]string{"release", command, "--help"})

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(io.Discard)
	if err := root.Execute(); err != nil {
		t.Fatalf("execute release %s help: %v", command, err)
	}
	return output.String()
}

func releaseHelpFlagLine(t *testing.T, output, flag string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--"+flag+" ") {
			return line
		}
	}
	t.Fatalf("help omitted --%s:\n%s", flag, output)
	return ""
}
