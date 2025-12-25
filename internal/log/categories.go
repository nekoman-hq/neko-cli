package log

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

type Category string

const (
	Init         Category = "init"
	Config       Category = "config"
	Preflight    Category = "pre-flight"
	VersionGuard Category = "version-guard"
	Release      Category = "release"
)

var categoryColors = map[Category]string{
	Init:         ColorBrightYellow,
	Config:       ColorBrightCyan,
	Preflight:    ColorBrightYellow,
	VersionGuard: ColorBrightBlue,
	Release:      ColorBrightGreen,
}
