package renderer

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestInvalidErrorEnvelopeFailsBeforeAnyRendererWrites(t *testing.T) {
	t.Parallel()

	for _, format := range []OutputFormat{FormatTable, FormatJSON, FormatGitHub} {
		format := format
		t.Run(string(format), func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			destination := filepath.Join(t.TempDir(), "github-output")
			if err := os.WriteFile(destination, []byte("existing=yes\n"), 0o600); err != nil {
				t.Fatalf("seed GitHub output: %v", err)
			}
			err := RenderWithOptionsTo(
				&plugin.Response{Status: "error"},
				RenderOptions{Format: format, GitHubOutputFile: destination},
				&output,
			)
			if err == nil || !strings.Contains(err.Error(), "missing its error envelope") {
				t.Fatalf("invalid envelope error = %v", err)
			}
			if output.Len() != 0 {
				t.Fatalf("invalid envelope produced partial output: %q", output.String())
			}
			written, readErr := os.ReadFile(destination)
			if readErr != nil {
				t.Fatalf("read GitHub output: %v", readErr)
			}
			if string(written) != "existing=yes\n" {
				t.Fatalf("invalid envelope changed GitHub output: %q", written)
			}
		})
	}
}

func TestInvalidExitRangeFailsBeforePublicJSON(t *testing.T) {
	t.Parallel()

	for _, code := range []int{-1, 126} {
		response := &plugin.Response{Status: "success", Data: map[string]any{"marker": "unchanged"}}
		response.SetExitCode(code)
		var output bytes.Buffer
		err := RenderWithOptionsTo(response, RenderOptions{Format: FormatJSON}, &output)
		if err == nil || !strings.Contains(err.Error(), "supported range 0 through 125") {
			t.Fatalf("exit %d validation error = %v", code, err)
		}
		if output.Len() != 0 {
			t.Fatalf("exit %d produced partial public JSON: %q", code, output.String())
		}
	}
}

func TestJSONWriterFailureRemainsCoreOwned(t *testing.T) {
	t.Parallel()

	want := errors.New("writer unavailable")
	err := RenderWithOptionsTo(
		&plugin.Response{Status: "success", Data: map[string]any{"marker": "unchanged"}},
		RenderOptions{Format: FormatJSON},
		writerThatFails{err: want},
	)
	if !errors.Is(err, want) {
		t.Fatalf("JSON writer error = %v, want %v", err, want)
	}
}

type writerThatFails struct {
	err error
}

func (writer writerThatFails) Write([]byte) (int, error) {
	return 0, writer.err
}

var _ io.Writer = writerThatFails{}
