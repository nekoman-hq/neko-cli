package release

import (
	"fmt"
	"strings"
)

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

type Tool interface {
	Name() string
	Release(rt Type) error
	Survey() (Type, error)
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
