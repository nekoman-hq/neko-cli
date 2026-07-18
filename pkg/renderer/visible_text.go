package renderer

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func visibleWidth(value string) int {
	return ansi.StringWidth(value)
}

func truncateVisible(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if visibleWidth(value) <= width {
		return value
	}
	return ansi.Truncate(value, width, "…")
}

func wrapVisibleLines(value string, width int) []string {
	if width <= 0 {
		return strings.Split(value, "\n")
	}
	return strings.Split(ansi.Wrap(value, width, ""), "\n")
}
