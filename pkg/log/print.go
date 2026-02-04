package log

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// DefaultSeparatorWidth Default separator width
const DefaultSeparatorWidth = 80

// PrintTableSeparator prints a horizontal separator line to stdout
func PrintTableSeparator() {
	PrintTableSeparatorTo(os.Stdout, DefaultSeparatorWidth)
}

// PrintTableSeparatorWidth prints a separator with custom width to stdout
func PrintTableSeparatorWidth(width int) {
	PrintTableSeparatorTo(os.Stdout, width)
}

// PrintTableSeparatorTo prints a horizontal separator line to the given writer
func PrintTableSeparatorTo(w io.Writer, width int) {
	_, _ = fmt.Fprintf(w, "%s%s%s\n", ColorBrightBlack, strings.Repeat("─", width), ColorReset)
}

// PrintHeader prints a styled header with cyan bold text
func PrintHeader(format string, args ...any) {
	PrintHeaderTo(os.Stdout, format, args...)
}

// PrintHeaderTo prints a styled header to the given writer
func PrintHeaderTo(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, "%s%s"+format+"%s\n", ColorCyan, ColorBold, fmt.Sprintf(format, args...), ColorReset)
}

// PrintSuccess prints a success message with green checkmark
func PrintSuccess(format string, args ...any) {
	PrintSuccessTo(os.Stdout, format, args...)
}

// PrintSuccessTo prints a success message to the given writer
func PrintSuccessTo(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, "%s%s✓%s %s\n", ColorGreen, ColorBold, ColorReset, fmt.Sprintf(format, args...))
}

// PrintWarning prints a warning message in yellow
func PrintWarning(format string, args ...any) {
	PrintWarningTo(os.Stdout, format, args...)
}

// PrintWarningTo prints a warning message to the given writer
func PrintWarningTo(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, "%s%s%s\n", ColorBrightYellow, fmt.Sprintf(format, args...), ColorReset)
}

// PrintDim prints dimmed text (for hints, counts, etc.)
func PrintDim(format string, args ...any) {
	PrintDimTo(os.Stdout, format, args...)
}

// PrintDimTo prints dimmed text to the given writer
func PrintDimTo(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, "%s%s%s\n", ColorBrightBlack, fmt.Sprintf(format, args...), ColorReset)
}

// PrintSectionHeader prints a section header with separator characters (━━━)
func PrintSectionHeader(title string, color string) {
	PrintSectionHeaderTo(os.Stdout, title, color)
}

// PrintSectionHeaderTo prints a section header to the given writer
func PrintSectionHeaderTo(w io.Writer, title string, color string) {
	_, _ = fmt.Fprintf(w, "\n%s%s━━━ %s ━━━%s\n", color, ColorBold, title, ColorReset)
}
