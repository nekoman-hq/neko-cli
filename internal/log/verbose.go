package log

import "fmt"

var Verbose = false

func V(msg string, args ...interface{}) {
	if !Verbose {
		return
	}

	prefix := ColorText(ColorYellow, "V$")
	fullMsg := fmt.Sprintf("%s %s", prefix, fmt.Sprintf(msg, args...))

	fmt.Println(fullMsg)
}
