// Package errors includes helper functions to display cli errors or warnings
package errors

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since     17.12.2025
*/

import (
	"fmt"
	"os"

	"github.com/nekoman-hq/neko-cli/internal/log"
)

type ErrorLevel int

const (
	ErrorLevelWarning ErrorLevel = iota
	ErrorLevelError
	ErrorLevelFatal
)

type CLIError struct {
	Title   string
	Message string
	Code    string
	Level   ErrorLevel
}

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
