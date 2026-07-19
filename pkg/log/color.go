package log

import "github.com/nekoman-hq/neko-cli/internal/terminalstyle"

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

const (
	// ColorReset resets all text attributes (color, bold, etc.) to terminal defaults.
	// This should be used after any colored text to prevent color bleeding.
	ColorReset = terminalstyle.Reset

	// ColorBold applies bold/bright text formatting.
	// Can be combined with color codes for emphasized output.
	ColorBold = terminalstyle.Bold

	// Standard ANSI color codes (normal intensity)

	// ColorRed applies red text color. Typically used for errors and failures.
	ColorRed = terminalstyle.Red

	// ColorGreen applies green text color. Typically used for success and completion.
	ColorGreen = terminalstyle.Green

	// ColorYellow applies yellow text color. Typically used for warnings and caution.
	ColorYellow = terminalstyle.Yellow

	// ColorBlue applies blue text color. Typically used for informational messages.
	ColorBlue = terminalstyle.Blue

	// ColorPurple applies purple/magenta text color. Typically used for versions and special values.
	ColorPurple = terminalstyle.Purple

	// ColorCyan applies cyan text color. Typically used for headers and labels.
	ColorCyan = terminalstyle.Cyan

	// Bright/intense ANSI color codes (high intensity variants)

	// ColorBrightBlack applies bright black (gray) text color.
	ColorBrightBlack = terminalstyle.BrightBlack

	// ColorBrightRed applies bright red text color.
	// Typically used for critical errors and fatal messages.
	ColorBrightRed = terminalstyle.BrightRed

	// ColorBrightGreen applies bright green text color.
	// Typically used for successful operations and positive states.
	ColorBrightGreen = terminalstyle.BrightGreen

	// ColorBrightYellow applies bright yellow text color.
	// Typically used for important warnings and highlights.
	ColorBrightYellow = terminalstyle.BrightYellow

	// ColorBrightBlue applies bright blue text color.
	ColorBrightBlue = terminalstyle.BrightBlue

	// ColorBrightPurple applies bright purple/magenta text color.
	ColorBrightPurple = terminalstyle.BrightPurple

	// ColorBrightCyan applies bright cyan text color.
	ColorBrightCyan = terminalstyle.BrightCyan

	// ColorBrightWhite applies bright white text color.
	ColorBrightWhite = terminalstyle.BrightWhite
)

// ColorText wraps the given text with ANSI color codes.
// It automatically appends ColorReset to prevent color bleeding into
// subsequent output.
//
// Args:
//   - color: The ANSI color code to apply (e.g., ColorRed, ColorBrightGreen)
//   - text: The text to colorize
//
// Returns the text wrapped with the color code and reset sequence.
//
// Example:
//
//	colored := ColorText(ColorGreen, "Success!")
//	fmt.Println(colored) // Prints "Success!" in green
func ColorText(color, text string) string {
	return terminalstyle.Apply(color, text)
}
