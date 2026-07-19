package log

import "github.com/nekoman-hq/neko-cli/internal/terminal"

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

const (
	// ColorReset resets all text attributes (color, bold, etc.) to terminal defaults.
	// This should be used after any colored text to prevent color bleeding.
	ColorReset = terminal.Reset

	// ColorBold applies bold/bright text formatting.
	// Can be combined with color codes for emphasized output.
	ColorBold = terminal.Bold

	// Standard ANSI color codes (normal intensity)

	// ColorRed applies red text color. Typically used for errors and failures.
	ColorRed = terminal.Red

	// ColorGreen applies green text color. Typically used for success and completion.
	ColorGreen = terminal.Green

	// ColorYellow applies yellow text color. Typically used for warnings and caution.
	ColorYellow = terminal.Yellow

	// ColorBlue applies blue text color. Typically used for informational messages.
	ColorBlue = terminal.Blue

	// ColorPurple applies purple/magenta text color. Typically used for versions and special values.
	ColorPurple = terminal.Purple

	// ColorCyan applies cyan text color. Typically used for headers and labels.
	ColorCyan = terminal.Cyan

	// Bright/intense ANSI color codes (high intensity variants)

	// ColorBrightBlack applies bright black (gray) text color.
	ColorBrightBlack = terminal.BrightBlack

	// ColorBrightRed applies bright red text color.
	// Typically used for critical errors and fatal messages.
	ColorBrightRed = terminal.BrightRed

	// ColorBrightGreen applies bright green text color.
	// Typically used for successful operations and positive states.
	ColorBrightGreen = terminal.BrightGreen

	// ColorBrightYellow applies bright yellow text color.
	// Typically used for important warnings and highlights.
	ColorBrightYellow = terminal.BrightYellow

	// ColorBrightBlue applies bright blue text color.
	ColorBrightBlue = terminal.BrightBlue

	// ColorBrightPurple applies bright purple/magenta text color.
	ColorBrightPurple = terminal.BrightPurple

	// ColorBrightCyan applies bright cyan text color.
	ColorBrightCyan = terminal.BrightCyan

	// ColorBrightWhite applies bright white text color.
	ColorBrightWhite = terminal.BrightWhite
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
	return terminal.Apply(color, text)
}
