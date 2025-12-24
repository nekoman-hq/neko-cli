package tools

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

import (
	// Register all release tools
	_ "github.com/nekoman-hq/neko-cli/internal/release/tools/goreleaser"
	_ "github.com/nekoman-hq/neko-cli/internal/release/tools/jreleaser"
	// _ "git.com/nekoman-hq/neko-cli/internal/release/semantic-release"
	// More tools here
)
