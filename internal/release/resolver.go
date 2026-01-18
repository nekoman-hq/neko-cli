package release

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Masterminds/semver/v3"

	"github.com/nekoman-hq/neko-cli/internal/log"
)

type Type string

const (
	Major Type = "major"
	Minor Type = "minor"
	Patch Type = "patch"
)

func ResolveReleaseType(version *semver.Version, args []string, t Tool) (Type, error) {
	if len(args) > 0 {
		rt, err := ParseReleaseType(args[0])
		if err != nil {
			var zero Type
			return zero, fmt.Errorf(
				"not a valid increment: %w", err,
			)
		}

		newVer := NextVersion(version, rt)

		log.Print(log.Release,
			"Applying %s (%s \uF178 %s)",
			log.ColorText(log.ColorPurple, string(rt)),
			version.String(),
			log.ColorText(log.ColorCyan, newVer.String()),
		)

		return rt, nil
	}

	if !t.SupportsSurvey() {
		var zero Type
		return zero, fmt.Errorf(
			"interactive mode not supported: %s requires an explicit release type", t.Name(),
		)
	}

	return t.Survey(version)
}

func NekoSurvey(version *semver.Version) (Type, error) {
	options := []string{
		fmt.Sprintf("Patch \uF178 %s", NextVersion(version, Patch)),
		fmt.Sprintf("Minor \uF178 %s", NextVersion(version, Minor)),
		fmt.Sprintf("Major \uF178 %s", NextVersion(version, Major)),
	}

	var choice string
	prompt := &survey.Select{
		Message: "Which type of release do you want to create?",
		Options: options,
		Default: options[0], // Patch
	}

	if err := survey.AskOne(prompt, &choice); err != nil {
		return Patch, err
	}

	return ParseReleaseType(choice[:5])
}

func NextVersion(current *semver.Version, t Type) semver.Version {
	switch t {
	case Major:
		return current.IncMajor()
	case Minor:
		return current.IncMinor()
	case Patch:
		return current.IncPatch()
	default:
		return *current
	}
}

func ParseReleaseType(input string) (Type, error) {
	switch strings.ToLower(input) {
	case "major":
		return Major, nil
	case "minor":
		return Minor, nil
	case "patch":
		return Patch, nil
	default:
		// TODO - Handle Fatal Error
		return Patch, fmt.Errorf("valid options: major, minor, patch")
	}
}
