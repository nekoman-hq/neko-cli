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

// PluginPrint writes a log entry to stderr (captured by dispatcher)
// This should be used inside plugins instead of Print
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

// PluginV writes a verbose log entry to stderr
func PluginV(cat Category, msg string, args ...any) {
	if !Verbose {
		return
	}

	verbosePrefix := ColorText(ColorPurple, "V$")
	enhancedMsg := fmt.Sprintf("%s %s", verbosePrefix, msg)

	PluginPrint(cat, enhancedMsg, args...)
}
