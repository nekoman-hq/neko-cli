package release

import "github.com/nekoman-hq/neko-cli/internal/git"

func VersionGuard() {
	// Git fetch
	git.Fetch()
	// Git latest tag
	// Compare version
}
