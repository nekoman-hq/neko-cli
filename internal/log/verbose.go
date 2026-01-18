package log

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import "fmt"

var Verbose = false

func V(cat Category, msg string, args ...any) {
	if !Verbose {
		return
	}

	verbosePrefix := ColorText(ColorPurple, "V$")
	enhancedMsg := fmt.Sprintf("%s %s", verbosePrefix, msg)

	Print(cat, enhancedMsg, args...)
}
