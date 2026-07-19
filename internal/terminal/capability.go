// Package terminal owns the focused terminal styling and capability primitives
// shared by terminal-facing Core packages.
package terminal

import (
	"io"
	"os"

	"golang.org/x/term"
)

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
