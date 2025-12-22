package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/log"
)

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

func LatestTag() string {
	log.V(log.Release, fmt.Sprintf("%s (Extract last tag)", log.ColorText(log.ColorGreen, "git describe --tags --abbrev=0")))
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		errors.Warning(
			"Failed to get latest tag",
			"No tags found or could not execute git describe.\nUsing default version 0.1.0.",
		)
		return "0.1.0"
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		errors.Warning(
			"No tags found",
			"No tags exist in this repository.\nUsing default version 0.1.0.",
		)
		return "0.1.0"
	}

	log.V(log.VersionGuard, fmt.Sprintf("Latest tag: %s", outputStr))
	return outputStr
}
