// Package tool retains the legacy side-effect registration surface.
//
// Deprecated: import concrete executor packages and pass NewV1Executor values
// to release.HandleReleaseWithV1Executors instead.
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
	release.Register(goreleaser.NewV1Executor()) //nolint:staticcheck // Legacy shim intentionally wires deprecated V1 registry APIs.
	release.Register(jreleaser.NewV1Executor())  //nolint:staticcheck // Legacy shim intentionally wires deprecated V1 registry APIs.
	release.Register(releaseit.NewV1Executor())  //nolint:staticcheck // Legacy shim intentionally wires deprecated V1 registry APIs.
}
