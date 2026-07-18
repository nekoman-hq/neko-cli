package renderer

import (
	"io"

	"golang.org/x/term"
)

// OutputWidthProvider reports the visible width of the actual output writer.
// The boolean is false when width is unavailable or cannot be trusted.
type OutputWidthProvider interface {
	Width(io.Writer) (int, bool)
}

type terminalOutputWidth struct{}

func (terminalOutputWidth) Width(writer io.Writer) (int, bool) {
	file, ok := writer.(interface{ Fd() uintptr })
	if !ok {
		return 0, false
	}
	fd := int(file.Fd())
	if !term.IsTerminal(fd) {
		return 0, false
	}
	width, _, err := term.GetSize(fd)
	if err != nil || width <= 0 {
		return 0, false
	}
	return width, true
}

func outputWidthProvider(provider OutputWidthProvider) OutputWidthProvider {
	if provider != nil {
		return provider
	}
	return terminalOutputWidth{}
}
