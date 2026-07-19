package renderer

import (
	"fmt"
	"io"
	"strings"

	"github.com/nekoman-hq/neko-cli/internal/terminalstyle"
	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

// ColorProvider is the injection boundary for interactive presentation color.
// Core's default requires a TTY and honors NO_COLOR.
type ColorProvider interface {
	ColorEnabled(io.Writer) bool
}

type terminalColorProvider struct{}

func (terminalColorProvider) ColorEnabled(writer io.Writer) bool {
	return terminalstyle.ColorEnabled(writer)
}

func colorProvider(provider ColorProvider) ColorProvider {
	if provider != nil {
		return provider
	}
	return terminalColorProvider{}
}

type presentationStyler struct {
	enabled bool
}

func newPresentationStyler(provider ColorProvider, writer io.Writer) presentationStyler {
	return presentationStyler{enabled: colorProvider(provider).ColorEnabled(writer)}
}

func plainPresentationStyler() presentationStyler {
	return presentationStyler{}
}

func (styler presentationStyler) semantic(role presentation.StyleRole, emphasized bool, text string) string {
	if !styler.enabled || text == "" {
		return text
	}
	style := semanticANSIStyle(role)
	if role == presentation.StyleEmphasis {
		emphasized = true
	}
	if emphasized {
		style += terminalstyle.Bold
	}
	return terminalstyle.Apply(style, text)
}

func (styler presentationStyler) ansi(style, text string) string {
	if !styler.enabled {
		return text
	}
	return terminalstyle.Apply(style, text)
}

func semanticANSIStyle(role presentation.StyleRole) string {
	switch role {
	case "", presentation.StyleDefault, presentation.StyleEmphasis:
		return ""
	case presentation.StyleSuccess:
		return terminalstyle.Green
	case presentation.StyleWarning:
		return terminalstyle.Yellow
	case presentation.StyleError:
		return terminalstyle.Red
	case presentation.StyleInfo:
		return terminalstyle.Cyan
	case presentation.StyleMuted:
		return terminalstyle.BrightBlack
	default:
		return ""
	}
}

func validStyleRole(role presentation.StyleRole) bool {
	switch role {
	case "", presentation.StyleDefault, presentation.StyleEmphasis, presentation.StyleSuccess,
		presentation.StyleWarning, presentation.StyleError, presentation.StyleInfo, presentation.StyleMuted:
		return true
	default:
		return false
	}
}

func printTableSeparator(writer io.Writer, width int, styler presentationStyler) {
	_, _ = fmt.Fprintln(writer, styler.semantic(presentation.StyleMuted, false, strings.Repeat("─", width)))
}

func printSectionHeader(writer io.Writer, title string, role presentation.StyleRole, styler presentationStyler) {
	text := fmt.Sprintf("━━━ %s ━━━", title)
	_, _ = fmt.Fprintf(writer, "\n%s\n", styler.semantic(role, true, text))
}

func printPresentationTitle(writer io.Writer, title string, styler presentationStyler) {
	_, _ = fmt.Fprintln(writer, styler.semantic(presentation.StyleDefault, true, title))
}
