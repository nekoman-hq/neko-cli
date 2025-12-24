package release

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

type Tool interface {
	Name() string
	Init(v *semver.Version) error
	Release(v *semver.Version) error
	Survey(v *semver.Version) (Type, error)
	SupportsSurvey() bool
}

type Type string

const (
	Major Type = "major"
	Minor Type = "minor"
	Patch Type = "patch"
)

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
