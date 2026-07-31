/*
@Title      neko-cli
@Description Command Line Interface for Neko projects
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      16.12.2025
*/
package main

import (
	"os"

	"github.com/nekoman-hq/neko-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(cmd.ProcessExitCode(err))
	}
}
