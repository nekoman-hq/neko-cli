package log

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

const (
	ColorReset = "\033[0m"
	ColorBold  = "\033[1m"

	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"

	ColorBrightBlack  = "\033[90m"
	ColorBrightRed    = "\033[91m"
	ColorBrightGreen  = "\033[92m"
	ColorBrightYellow = "\033[93m"
	ColorBrightBlue   = "\033[94m"
	ColorBrightPurple = "\033[95m"
	ColorBrightCyan   = "\033[96m"
	ColorBrightWhite  = "\033[97m"
)

func ColorText(color, text string) string {
	return color + text + ColorReset
}
