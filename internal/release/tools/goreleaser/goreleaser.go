package goreleaser

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/nekoman-hq/neko-cli/internal/release"
)

type GoReleaser struct{}

func (g *GoReleaser) Name() string {
	return "goreleaser"
}

func (g *GoReleaser) SupportsSurvey() bool {
	return true
}

func (g *GoReleaser) Release(rt release.Type) error {
	fmt.Println("Goreleaser release:", rt)

	// Detect Version - if no version - default 0.1.0 or from config
	// Select or execute increment
	// Commit chore(release): version
	// Tag - version
	// Create release

	return nil
}

func (g *GoReleaser) Survey() (release.Type, error) {
	var choice string

	prompt := &survey.Select{
		Message: "Which type of release do you want to create?",
		Options: []string{"Patch", "Minor", "Major"},
		Default: "Patch",
	}

	if err := survey.AskOne(prompt, &choice); err != nil {
		return release.Patch, err
	}

	return release.ParseReleaseType(choice)
}

func init() {
	release.Register(&GoReleaser{})
}
