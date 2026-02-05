// Package errors includes helper functions to display CLI errors or warnings.
// This package is used by the core neko-cli tool to display user-facing errors
// to stderr with appropriate formatting and colors.
//
// Note: This package is for the core CLI only. Plugins should use the plugin
// errors package instead.
package errors

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since     17.12.2025
*/

import (
	"fmt"
	"os"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

// ErrorLevel represents the severity level of a CLI error.
type ErrorLevel int

const (
	// ErrorLevelWarning indicates a non-fatal issue that should be brought to the user's attention.
	// Warnings do not cause the program to exit.
	ErrorLevelWarning ErrorLevel = iota

	// ErrorLevelError indicates a recoverable error.
	// The program will exit with code 1 after displaying the error.
	ErrorLevelError

	// ErrorLevelFatal indicates a critical, unrecoverable error.
	// The program will exit with code 1 after displaying the error.
	ErrorLevelFatal
)

// CLIError represents a structured error message to be displayed to the user.
// It includes a title, detailed message, optional error code, and severity level.
type CLIError struct {
	Title   string     // Optional short title describing the error context
	Message string     // The main error message (required)
	Code    string     // Optional error code for reference (e.g., "E001", "PLUGIN_NOT_FOUND")
	Level   ErrorLevel // Severity level of the error
}

// PrintError displays a formatted error message to stderr with appropriate
// colors and formatting based on the error level. For errors and fatal errors,
// the program exits with code 1 after printing.
//
// The output format is:
//
//	{LEVEL_ICON} {LEVEL}: {Title}
//	{Message}
//	Error Code: {Code}
//
// Args:
//   - err: The CLIError to display
//
// Behavior:
//   - If Message is empty, nothing is printed
//   - Warnings are printed but do not cause program exit
//   - Errors and Fatal errors cause os.Exit(1) after printing
func PrintError(err CLIError) {
	if err.Message == "" {
		return
	}

	var prefix, color string
	switch err.Level {
	case ErrorLevelWarning:
		prefix = "⚠ WARNING"
		color = log.ColorYellow
	case ErrorLevelError:
		prefix = "✗ ERROR"
		color = log.ColorRed
	case ErrorLevelFatal:
		prefix = "✗ FATAL"
		color = log.ColorRed
	}

	_, _ = fmt.Fprintf(os.Stderr, "%s%s%s", color, log.ColorBold, prefix)
	if err.Title != "" {
		_, _ = fmt.Fprintf(os.Stderr, ": %s", err.Title)
	}
	_, _ = fmt.Fprintf(os.Stderr, "%s\n", log.ColorReset)

	_, _ = fmt.Fprintf(os.Stderr, "%s%s%s\n", color, err.Message, log.ColorReset)

	if err.Code != "" {
		_, _ = fmt.Fprintf(os.Stderr, "%sError Code: %s%s\n", color, err.Code, log.ColorReset)
	}

	_, _ = fmt.Fprintln(os.Stderr)

	if err.Level == ErrorLevelFatal || err.Level == ErrorLevelError {
		os.Exit(1)
	}
}

// Warning displays a warning message to stderr and continues execution.
// Warnings are shown in yellow with a warning icon.
//
// Args:
//   - title: Optional short title for the warning context
//   - message: The warning message to display
func Warning(title, message string) {
	PrintError(CLIError{
		Level:   ErrorLevelWarning,
		Title:   title,
		Message: message,
	})
}

// Error displays an error message to stderr and exits the program with code 1.
// Errors are shown in red with an error icon.
//
// Args:
//   - title: Optional short title for the error context
//   - message: The error message to display
//   - code: Optional error code for reference and debugging
//
// Note: This function does not return; it exits the program.
func Error(title, message string, code string) {
	PrintError(CLIError{
		Level:   ErrorLevelError,
		Title:   title,
		Message: message,
		Code:    code,
	})
}

// Fatal displays a fatal error message to stderr and exits the program with code 1.
// Fatal errors are shown in red with a fatal icon, indicating an unrecoverable error.
//
// Args:
//   - title: Optional short title for the error context
//   - message: The error message to display
//   - code: Optional error code for reference and debugging
//
// Note: This function does not return; it exits the program.
func Fatal(title, message string, code string) {
	PrintError(CLIError{
		Level:   ErrorLevelFatal,
		Title:   title,
		Message: message,
		Code:    code,
	})
}
