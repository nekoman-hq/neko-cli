// Package terminalstyle owns the small ANSI palette and interactive-color
// policy shared by terminal-facing Core packages.
package terminalstyle

import (
	"io"
	"os"

	"golang.org/x/term"
)

const (
	Reset = "\x1b[0m"
	Bold  = "\x1b[1m"

	Red    = "\x1b[31m"
	Green  = "\x1b[32m"
	Yellow = "\x1b[33m"
	Blue   = "\x1b[34m"
	Purple = "\x1b[35m"
	Cyan   = "\x1b[36m"

	BrightBlack  = "\x1b[90m"
	BrightRed    = "\x1b[91m"
	BrightGreen  = "\x1b[92m"
	BrightYellow = "\x1b[93m"
	BrightBlue   = "\x1b[94m"
	BrightPurple = "\x1b[95m"
	BrightCyan   = "\x1b[96m"
	BrightWhite  = "\x1b[97m"
)

// Apply wraps text in one or more ANSI style sequences and resets the terminal
// immediately afterward. Empty styles leave the text byte-for-byte unchanged.
func Apply(style, text string) string {
	if style == "" {
		return text
	}
	return style + text + Reset
}

// ColorEnabled reports whether writer is an interactive terminal and the
// standard NO_COLOR environment convention does not disable styling.
func ColorEnabled(writer io.Writer) bool {
	return systemColorPolicy().enabled(writer)
}

type colorPolicy struct {
	lookupEnv  func(string) (string, bool)
	isTerminal func(int) bool
}

func systemColorPolicy() colorPolicy {
	return colorPolicy{lookupEnv: os.LookupEnv, isTerminal: term.IsTerminal}
}

func (policy colorPolicy) enabled(writer io.Writer) bool {
	if value, present := policy.lookupEnv("NO_COLOR"); present && value != "" {
		return false
	}
	file, ok := writer.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	return policy.isTerminal(int(file.Fd()))
}
