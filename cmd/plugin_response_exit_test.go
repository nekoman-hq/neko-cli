package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	"github.com/spf13/cobra"
)

func TestPluginResponseExitIsOptInAndRetainsStructuredFailure(t *testing.T) {
	if err := pluginResponseExitError(&plugin.Response{
		Status: "error",
		Error:  &plugin.ResponseError{Code: "LEGACY", Message: "legacy structured failure"},
	}); err != nil {
		t.Fatalf("legacy response without exit request changed behavior: %v", err)
	}
	response := &plugin.Response{
		Status:   "error",
		ExitCode: 1,
		Error:    &plugin.ResponseError{Code: "HEAD_MISMATCH", Message: "checked-out HEAD does not match release_sha"},
	}
	err := pluginResponseExitError(response)
	if err == nil || ProcessExitCode(err) != 1 || strings.Contains(err.Error(), "HEAD_MISMATCH") || strings.Contains(err.Error(), "checked-out HEAD") {
		t.Fatalf("opt-in response exit error = %v", err)
	}
}

func TestGitHubOutputFlagsAreRegisteredAsExplicitCoreOptions(t *testing.T) {
	if flag := rootCmd.PersistentFlags().Lookup("output"); flag == nil || !strings.Contains(flag.Usage, "github") {
		t.Fatalf("output flag does not advertise GitHub mode: %#v", flag)
	}
	if flag := rootCmd.PersistentFlags().Lookup("github-output-file"); flag == nil || flag.DefValue != "" {
		t.Fatalf("explicit GitHub output destination flag = %#v", flag)
	}
}

func TestStructuredPluginErrorIsRenderedOnceForHumanAndJSONOutput(t *testing.T) {
	response := &plugin.Response{
		Status: "error",
		Error: &plugin.ResponseError{
			Code:    "PIPELINE_UNIT_INVALID",
			Message: "Release unit is required. Available units: cli, plugin-release, plugin-ui",
		},
		ExitCode: 1,
	}

	for _, format := range []renderer.OutputFormat{renderer.FormatTable, renderer.FormatJSON} {
		t.Run(string(format), func(t *testing.T) {
			var rendered bytes.Buffer
			var fallback bytes.Buffer
			root := &cobra.Command{Use: "neko", SilenceUsage: true}
			root.SetErr(&fallback)
			command := &cobra.Command{
				Use: "pipeline",
				RunE: func(cmd *cobra.Command, _ []string) error {
					if err := renderer.RenderTo(response, format, &rendered); err != nil {
						return err
					}
					return renderedPluginResponseExitError(cmd, response)
				},
			}
			root.AddCommand(command)
			root.SetArgs([]string{"pipeline"})

			err := root.Execute()
			if err == nil {
				t.Fatal("structured plugin exit unexpectedly succeeded")
			}
			if got := strings.Count(rendered.String()+fallback.String(), response.Error.Code); got != 1 {
				t.Fatalf("error code render paths = %d, want 1\nrendered:\n%s\nfallback:\n%s", got, rendered.String(), fallback.String())
			}
			if got := strings.Count(rendered.String()+fallback.String(), response.Error.Message); got != 1 {
				t.Fatalf("error message render paths = %d, want 1\nrendered:\n%s\nfallback:\n%s", got, rendered.String(), fallback.String())
			}
			if fallback.Len() != 0 {
				t.Fatalf("Cobra duplicated an already-rendered plugin failure: %q", fallback.String())
			}
			if !command.SilenceErrors {
				t.Fatal("rendered plugin failure did not silence only its executed command")
			}
		})
	}
}

func TestUnrenderedCommandErrorCharacterizationUsesCobraFallback(t *testing.T) {
	var output bytes.Buffer
	root := &cobra.Command{Use: "neko", SilenceUsage: true}
	root.SetErr(&output)
	root.AddCommand(&cobra.Command{
		Use:  "pipeline",
		RunE: func(*cobra.Command, []string) error { return assertiveCommandError{} },
	})
	root.SetArgs([]string{"pipeline"})

	if err := root.Execute(); err == nil {
		t.Fatal("unrendered command error unexpectedly succeeded")
	}
	if got := strings.Count(output.String(), "unrendered command failure"); got != 1 {
		t.Fatalf("Cobra fallback count = %d, want 1: %q", got, output.String())
	}
}

type assertiveCommandError struct{}

func (assertiveCommandError) Error() string { return "unrendered command failure" }
