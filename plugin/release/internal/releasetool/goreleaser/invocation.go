package goreleaser

import "strings"

// Invocation is the neutral classification of one GoReleaser argument string.
type Invocation struct {
	Command         string
	ConfigReference string
	Snapshot        bool
	SkipPublication bool
	RealPublication bool
}

// ClassifyArguments classifies the focused GoReleaser command and flags used
// by Release Plugin workflows. It does not resolve workflow environment values.
func ClassifyArguments(arguments string) Invocation {
	realPublication := classifiesAsRealPublication(arguments)
	normalized := strings.NewReplacer(
		"${{ ", "${{",
		" }}", "}}",
		"'", "",
		"\"", "",
	).Replace(arguments)
	fields := strings.Fields(normalized)
	invocation := Invocation{}
	if len(fields) > 0 {
		invocation.Command = fields[0]
	}
	for index, field := range fields {
		switch {
		case field == "--snapshot" || field == "--snapshot=true":
			invocation.Snapshot = true
		case strings.HasPrefix(field, "--skip=") && commaListContains(strings.TrimPrefix(field, "--skip="), "publish"):
			invocation.SkipPublication = true
		case field == "--skip" && index+1 < len(fields) && commaListContains(fields[index+1], "publish"):
			invocation.SkipPublication = true
		case strings.HasPrefix(field, "--config="):
			invocation.ConfigReference = strings.TrimPrefix(field, "--config=")
		case field == "--config" && index+1 < len(fields):
			invocation.ConfigReference = fields[index+1]
		}
	}
	invocation.RealPublication = realPublication
	return invocation
}

// classifiesAsRealPublication intentionally preserves the existing Doctor
// classifier's treatment of raw workflow arguments. In particular, a quoted
// command token is not considered a publication even though the normalized
// command fact is still useful to workflow inspection.
func classifiesAsRealPublication(arguments string) bool {
	fields := strings.Fields(arguments)
	if len(fields) == 0 || fields[0] != "release" {
		return false
	}
	for index, field := range fields {
		field = strings.Trim(field, "'\"")
		if field == "--snapshot" || field == "--snapshot=true" {
			return false
		}
		if strings.HasPrefix(field, "--skip=") && commaListContains(strings.TrimPrefix(field, "--skip="), "publish") {
			return false
		}
		if field == "--skip" && index+1 < len(fields) && commaListContains(fields[index+1], "publish") {
			return false
		}
	}
	return true
}

func commaListContains(value, wanted string) bool {
	for _, item := range strings.Split(strings.Trim(value, "'\""), ",") {
		if item == wanted {
			return true
		}
	}
	return false
}
