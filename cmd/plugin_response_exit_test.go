package cmd

import (
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestPluginResponseExitIsOptInAndRetainsStructuredFailure(t *testing.T) {
	if err := pluginResponseExitError(&plugin.Response{Status: "error"}); err != nil {
		t.Fatalf("legacy response without exit request changed behavior: %v", err)
	}
	response := &plugin.Response{
		Status:   "error",
		ExitCode: 1,
		Error:    &plugin.ResponseError{Code: "HEAD_MISMATCH", Message: "checked-out HEAD does not match release_sha"},
	}
	err := pluginResponseExitError(response)
	if err == nil || !strings.Contains(err.Error(), "HEAD_MISMATCH") || !strings.Contains(err.Error(), "checked-out HEAD") {
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
