package renderer

import (
	"fmt"
	"io"
	"strings"

	"github.com/nekoman-hq/neko-cli/internal/terminalstyle"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

// ColorProvider is the injection boundary for interactive human-output color.
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

type humanStyler struct {
	enabled bool
}

func newHumanStyler(provider ColorProvider, writer io.Writer) humanStyler {
	return humanStyler{enabled: colorProvider(provider).ColorEnabled(writer)}
}

func plainHumanStyler() humanStyler {
	return humanStyler{}
}

func (styler humanStyler) semantic(role plugin.HumanStyleRole, emphasized bool, text string) string {
	if !styler.enabled || text == "" {
		return text
	}
	style := semanticANSIStyle(role)
	if role == plugin.HumanStyleEmphasis {
		emphasized = true
	}
	if emphasized {
		style += terminalstyle.Bold
	}
	return terminalstyle.Apply(style, text)
}

func (styler humanStyler) ansi(style, text string) string {
	if !styler.enabled {
		return text
	}
	return terminalstyle.Apply(style, text)
}

func semanticANSIStyle(role plugin.HumanStyleRole) string {
	switch role {
	case "", plugin.HumanStyleDefault, plugin.HumanStyleEmphasis:
		return ""
	case plugin.HumanStyleSuccess:
		return terminalstyle.Green
	case plugin.HumanStyleWarning:
		return terminalstyle.Yellow
	case plugin.HumanStyleError:
		return terminalstyle.Red
	case plugin.HumanStyleInfo:
		return terminalstyle.Cyan
	case plugin.HumanStyleMuted:
		return terminalstyle.BrightBlack
	default:
		return ""
	}
}

func validHumanStyleRole(role plugin.HumanStyleRole) bool {
	switch role {
	case "", plugin.HumanStyleDefault, plugin.HumanStyleEmphasis, plugin.HumanStyleSuccess,
		plugin.HumanStyleWarning, plugin.HumanStyleError, plugin.HumanStyleInfo, plugin.HumanStyleMuted:
		return true
	default:
		return false
	}
}

func printTableSeparator(writer io.Writer, width int, styler humanStyler) {
	_, _ = fmt.Fprintln(writer, styler.semantic(plugin.HumanStyleMuted, false, strings.Repeat("─", width)))
}

func printSectionHeader(writer io.Writer, title string, role plugin.HumanStyleRole, styler humanStyler) {
	text := fmt.Sprintf("━━━ %s ━━━", title)
	_, _ = fmt.Fprintf(writer, "\n%s\n", styler.semantic(role, true, text))
}

func printHumanTitle(writer io.Writer, title string, styler humanStyler) {
	_, _ = fmt.Fprintln(writer, styler.semantic(plugin.HumanStyleDefault, true, title))
}
