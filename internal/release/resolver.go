package release

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"fmt"

	"github.com/nekoman-hq/neko-cli/internal/errors"
)

func ResolveReleaseType(args []string, t Tool) (Type, error) {
	if len(args) > 0 {
		return ParseReleaseType(args[0])
	}

	if !t.SupportsSurvey() {
		errors.Fatal(
			"Interactive mode not supported",
			fmt.Sprintf("%s requires an explicit release type", t.Name()),
			errors.ErrSurveyFailed,
		)
	}

	return t.Survey()
}
