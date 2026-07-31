package errors_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	pluginerrors "github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestFatalPluginErrorWritersSerializeExplicitFailure(t *testing.T) {
	for _, helper := range []string{"plain", "details"} {
		t.Run(helper, func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=TestFatalPluginErrorWriterHelper")
			command.Env = append(os.Environ(), "NEKO_FATAL_ERROR_HELPER="+helper)
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			err := command.Run()
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
				t.Fatalf("fatal helper exit = %v", err)
			}
			if stderr.Len() != 0 {
				t.Fatalf("fatal helper wrote stderr: %q", stderr.String())
			}
			if strings.Count(stdout.String(), `"code":"FATAL_FAILURE"`) != 1 {
				t.Fatalf("fatal response count changed: %q", stdout.String())
			}

			var response plugin.Response
			if decodeErr := json.Unmarshal(stdout.Bytes(), &response); decodeErr != nil {
				t.Fatalf("decode fatal response: %v", decodeErr)
			}
			code, present := response.ExplicitExitCode()
			if !present || code != 1 {
				t.Fatalf("fatal response exit = (%d, %t), want (1, true): %s", code, present, stdout.String())
			}
			if response.Status != "error" || response.Error == nil || response.Error.Code != "FATAL_FAILURE" || response.Metadata.Plugin != "probe" || response.Metadata.Version != "1.2.3" {
				t.Fatalf("fatal response contract changed: %#v", response)
			}
			if helper == "details" && response.Error.Details["safe"] != "detail" {
				t.Fatalf("fatal details changed: %#v", response.Error.Details)
			}
		})
	}
}

func TestFatalPluginErrorWriterHelper(t *testing.T) {
	helper := os.Getenv("NEKO_FATAL_ERROR_HELPER")
	if helper == "" {
		return
	}
	pluginerrors.PluginName = "probe"
	pluginerrors.PluginVersion = "1.2.3"
	if helper == "details" {
		pluginerrors.WriteErrorWithDetails("FATAL_FAILURE", "fatal failure", map[string]any{"safe": "detail"})
	}
	pluginerrors.WriteError("FATAL_FAILURE", "fatal failure")
}

func TestPluginErrorResponseConstructorsAssignSemanticExitIntent(t *testing.T) {
	t.Parallel()

	for _, response := range []*plugin.Response{
		pluginerrors.NewErrorResponse("FAILURE", "failed"),
		pluginerrors.NewErrorResponseWithDetails("FAILURE", "failed", map[string]any{"safe": true}),
	} {
		if code, present := response.ExplicitExitCode(); !present || code != 1 {
			t.Fatalf("error constructor exit = (%d, %t), want (1, true)", code, present)
		}
	}
	if code, present := pluginerrors.WriteWarning("WARNING", "warning").ExplicitExitCode(); !present || code != 0 {
		t.Fatalf("warning constructor exit = (%d, %t), want (0, true)", code, present)
	}
}
