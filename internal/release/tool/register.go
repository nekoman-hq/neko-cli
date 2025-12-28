// Package tool imports all release systems so init gets called<D-s>
package tool

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

import (
	// Register all release tools
	_ "github.com/nekoman-hq/neko-cli/internal/release/tool/goreleaser"
	_ "github.com/nekoman-hq/neko-cli/internal/release/tool/jreleaser"
	_ "github.com/nekoman-hq/neko-cli/internal/release/tool/releaseit"
	// _ "git.com/nekoman-hq/neko-cli/internal/release/semantic-release"
	// More tools here
)
