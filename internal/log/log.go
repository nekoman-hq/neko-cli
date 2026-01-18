// Package log includes all helper functions to print corrext output
package log

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"fmt"
	"time"
)

func Print(cat Category, msg string, args ...any) {
	color, ok := categoryColors[cat]
	if !ok {
		color = ColorReset
	}

	prefix := fmt.Sprintf("[%s]", cat)
	coloredPrefix := ColorText(color, prefix)
	timestamp := time.Now().Format("15:04:05")
	fullMsg := fmt.Sprintf(msg, args...)
	fmt.Printf("%s %s %s\n", timestamp, coloredPrefix, fullMsg)
}
