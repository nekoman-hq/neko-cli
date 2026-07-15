// Package tool retains the legacy side-effect registration surface.
package tool

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

import (
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release/tool/goreleaser"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release/tool/jreleaser"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release/tool/releaseit"
)

// Importing this package is the explicitly bounded compatibility opt-in for
// callers that still use release.Register/Get. Production command composition
// constructs immutable V1 executor catalogs instead.
func init() {
	release.Register(goreleaser.NewV1Executor())
	release.Register(jreleaser.NewV1Executor())
	release.Register(releaseit.NewV1Executor())
}
