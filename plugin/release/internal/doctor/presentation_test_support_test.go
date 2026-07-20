package doctor

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
)

type releasePlanOutputWidth struct {
	width     int
	available bool
}

func (width releasePlanOutputWidth) Width(io.Writer) (int, bool) {
	return width.width, width.available
}

func renderReleasePlanForTest(
	t *testing.T,
	response *plugin.Response,
	format renderer.OutputFormat,
	width renderer.OutputWidthProvider,
) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{Format: format, WidthProvider: width}, &output); err != nil {
		t.Fatalf("RenderWithOptionsTo: %v", err)
	}
	return output.String()
}

func assertReleasePlanLinesFit(t *testing.T, output string, width int) {
	t.Helper()
	for lineNumber, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if got := ansi.StringWidth(line); got > width {
			t.Fatalf("line %d visible width = %d, want at most %d: %q", lineNumber+1, got, width, line)
		}
	}
}
