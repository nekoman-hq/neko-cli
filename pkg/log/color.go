package log

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

const (
	// ColorReset resets all text attributes (color, bold, etc.) to terminal defaults.
	// This should be used after any colored text to prevent color bleeding.
	ColorReset = "\033[0m"

	// ColorBold applies bold/bright text formatting.
	// Can be combined with color codes for emphasized output.
	ColorBold = "\033[1m"

	// Standard ANSI color codes (normal intensity)

	// ColorRed applies red text color. Typically used for errors and failures.
	ColorRed = "\033[31m"

	// ColorGreen applies green text color. Typically used for success and completion.
	ColorGreen = "\033[32m"

	// ColorYellow applies yellow text color. Typically used for warnings and caution.
	ColorYellow = "\033[33m"

	// ColorBlue applies blue text color. Typically used for informational messages.
	ColorBlue = "\033[34m"

	// ColorPurple applies purple/magenta text color. Typically used for versions and special values.
	ColorPurple = "\033[35m"

	// ColorCyan applies cyan text color. Typically used for headers and labels.
	ColorCyan = "\033[36m"

	// Bright/intense ANSI color codes (high intensity variants)

	// ColorBrightBlack applies bright black (gray) text color.
	ColorBrightBlack = "\033[90m"

	// ColorBrightRed applies bright red text color.
	// Typically used for critical errors and fatal messages.
	ColorBrightRed = "\033[91m"

	// ColorBrightGreen applies bright green text color.
	// Typically used for successful operations and positive states.
	ColorBrightGreen = "\033[92m"

	// ColorBrightYellow applies bright yellow text color.
	// Typically used for important warnings and highlights.
	ColorBrightYellow = "\033[93m"

	// ColorBrightBlue applies bright blue text color.
	ColorBrightBlue = "\033[94m"

	// ColorBrightPurple applies bright purple/magenta text color.
	ColorBrightPurple = "\033[95m"

	// ColorBrightCyan applies bright cyan text color.
	ColorBrightCyan = "\033[96m"

	// ColorBrightWhite applies bright white text color.
	ColorBrightWhite = "\033[97m"
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
	return color + text + ColorReset
}
