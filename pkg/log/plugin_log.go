package log

import (
	"fmt"
	"os"
	"time"
)

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      03.02.2026

For logs in plugins
*/

// PluginPrint writes a categorized log entry to stderr for capture by the dispatcher.
// This function should be used inside plugins instead of Print.
//
// Plugins must write logs to stderr (not stdout) because:
//   - stdout is reserved for the JSON response that the dispatcher reads
//   - stderr is captured separately by the dispatcher and parsed into log entries
//   - This separation allows structured responses and logging to coexist
//
// The output format is: {timestamp} [{category}] {message}
// Example: "15:04:05 [exec] Processing release configuration"
//
// Args:
//   - cat: The log category (determines the color of the prefix)
//   - msg: The message format string (supports printf-style formatting)
//   - args: Optional arguments for the format string
//
// Note: The dispatcher parses stderr output and converts it into plugin.LogEntry
// structures that are included in the final response.
func PluginPrint(cat Category, msg string, args ...any) {
	color, ok := categoryColors[cat]
	if !ok {
		color = ColorReset
	}

	prefix := fmt.Sprintf("[%s]", cat)
	coloredPrefix := ColorText(color, prefix)
	timestamp := time.Now().Format("15:04:05")
	fullMsg := fmt.Sprintf(msg, args...)

	// Write to stderr so dispatcher can capture it
	_, _ = fmt.Fprintf(os.Stderr, "%s %s %s\n", timestamp, coloredPrefix, fullMsg)
}

// PluginV writes a verbose log entry to stderr when Verbose mode is enabled.
// This function should be used inside plugins instead of V.
//
// Verbose messages are prefixed with "V$" in purple to distinguish them
// from regular log output. The dispatcher recognizes the "V$" prefix and
// assigns these entries a "verbose" log level.
//
// The output format is: {timestamp} [{category}] V$ {message}
// Example: "15:04:05 [config] V$ Reading manifest from /path/to/manifest.json"
//
// Args:
//   - cat: The log category (determines the color of the prefix)
//   - msg: The message format string (supports printf-style formatting)
//   - args: Optional arguments for the format string
//
// Behavior:
//   - If Verbose is false, this function does nothing
//   - If Verbose is true, outputs the message with a "V$" prefix to stderr
//
// Use cases in plugins:
//   - Detailed operation traces
//   - File paths and configuration values being processed
//   - Intermediate computation results
//   - Debugging information for plugin development
func PluginV(cat Category, msg string, args ...any) {
	if !Verbose {
		return
	}

	verbosePrefix := ColorText(ColorPurple, "V$")
	enhancedMsg := fmt.Sprintf("%s %s", verbosePrefix, msg)

	PluginPrint(cat, enhancedMsg, args...)
}
