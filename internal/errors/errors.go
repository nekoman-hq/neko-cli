package errors

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since     17.12.2025
*/

import (
	"fmt"
	"os"
)

type ErrorLevel int

const (
	ErrorLevelWarning ErrorLevel = iota
	ErrorLevelError
	ErrorLevelFatal
)

type CLIError struct {
	Level   ErrorLevel
	Title   string
	Message string
	Code    string
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBold   = "\033[1m"
)

func PrintError(err CLIError) {
	if err.Message == "" {
		return
	}

	var prefix, color string
	switch err.Level {
	case ErrorLevelWarning:
		prefix = "⚠ WARNING"
		color = colorYellow
	case ErrorLevelError:
		prefix = "✗ ERROR"
		color = colorRed
	case ErrorLevelFatal:
		prefix = "✗ FATAL"
		color = colorRed
	}

	fmt.Fprintf(os.Stderr, "%s%s%s", color, colorBold, prefix)
	if err.Title != "" {
		fmt.Fprintf(os.Stderr, ": %s", err.Title)
	}
	fmt.Fprintf(os.Stderr, "%s\n", colorReset)

	fmt.Fprintf(os.Stderr, "%s%s%s\n", color, err.Message, colorReset)

	if err.Code != "" {
		fmt.Fprintf(os.Stderr, "%sError Code: %d%s\n", color, err.Code, colorReset)
	}

	fmt.Fprintln(os.Stderr)

	if err.Level == ErrorLevelFatal {
		os.Exit(1)
	}
}

// Convenience functions
func Warning(title, message string) {
	PrintError(CLIError{
		Level:   ErrorLevelWarning,
		Title:   title,
		Message: message,
	})
}

func Error(title, message string, code string) {
	PrintError(CLIError{
		Level:   ErrorLevelError,
		Title:   title,
		Message: message,
		Code:    code,
	})
}

func Fatal(title, message string, code string) {
	PrintError(CLIError{
		Level:   ErrorLevelFatal,
		Title:   title,
		Message: message,
		Code:    code,
	})
}
