// Package log includes all helper functions to print correct output
// for the neko-cli core tool.
package log

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025

For logs in the core tool
*/

import (
	"fmt"
	"time"
)

// Print outputs a categorized log message to stdout with timestamp and colored prefix.
// This function is used by the core CLI tool for user-facing log output.
//
// The output format is: {timestamp} [{category}] {message}
// Example: "15:04:05 [exec] Starting command execution"
//
// Args:
//   - cat: The log category (determines the color of the prefix)
//   - msg: The message format string (supports printf-style formatting)
//   - args: Optional arguments for the format string
//
// Note: This function writes to stdout. Plugins should use PluginPrint instead,
// which writes to stderr so the dispatcher can capture logs separately from responses.
func Print(cat Category, msg any, args ...any) {
	color, ok := categoryColors[cat]
	if !ok {
		color = ColorReset
	}

	prefix := fmt.Sprintf("[%s]", cat)
	coloredPrefix := ColorText(color, prefix)
	timestamp := time.Now().Format("15:04:05")

	fullMsg := fmt.Sprint(append([]any{msg}, args...)...)
	fmt.Printf("%s %s %s\n", timestamp, coloredPrefix, fullMsg)
}

// Verbose controls whether verbose log messages are displayed.
// When set to true, V() function calls will output their messages.
// When false, V() calls are no-ops.
//
// This is typically set via a --verbose or -v command-line flag.
var Verbose = false

// V outputs a verbose log message when Verbose mode is enabled.
// Verbose messages are prefixed with "V$" in purple to distinguish them
// from regular log output.
//
// The output format is: {timestamp} [{category}] V$ {message}
// Example: "15:04:05 [config] V$ Parsing configuration file: /path/to/config.yml"
//
// Args:
//   - cat: The log category (determines the color of the prefix)
//   - msg: The message format string (supports printf-style formatting)
//   - args: Optional arguments for the format string
//
// Behavior:
//   - If Verbose is false, this function does nothing
//   - If Verbose is true, outputs the message with a "V$" prefix
//
// Use cases:
//   - Detailed debugging information
//   - Step-by-step operation traces
//   - Variable values and intermediate results
//   - Information useful for troubleshooting but too verbose for normal operation
func V(cat Category, msg string, args ...any) {
	if !Verbose {
		return
	}

	verbosePrefix := ColorText(ColorPurple, "V$")
	enhancedMsg := fmt.Sprintf("%s %s", verbosePrefix, msg)

	Print(cat, enhancedMsg, args...)
}
