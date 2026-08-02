# Go package architecture

> **Audience:** Contributors deciding where Core and plugin code belongs.
>
> **Purpose:** Define repository package visibility, dependency direction, and compatibility placement.

The repository uses directory placement as an API boundary:

- `pkg/` contains supported, intentionally importable contracts and reusable
  APIs. An established package with uncertain downstream use remains there as
  an explicit compatibility surface until a downstream audit and approved
  breaking change prove it private.
- `internal/` contains private Core implementation. Go's import rule prevents
  consumers outside this module tree from depending on it. The number of
  packages under `internal/` does not affect this boundary.
- `cmd/` contains Core CLI command composition and binary entry points.
- `plugin/` contains plugin executables and their plugin-specific application
  packages. Plugins consume public contracts from `pkg/` and may use private
  module implementation only when they are built as part of this repository.

## Root `pkg` classification

| Package | Classification | Intent and placement |
| --- | --- | --- |
| `config` | compatibility surface | Shared environment lookup used by the Release Plugin. Its exported path may have downstream users, so it remains importable pending a dedicated API migration. |
| `dispatcher` | public reusable API | Executes plugin processes and decodes the public plugin protocol for Core and potential embedders. |
| `errors` | public plugin/Core API | Creates plugin error responses and exposes established Core error helpers documented for plugin authors. |
| `git` | compatibility surface | Shared GitHub release metadata and lookup API used by Core and the Release Plugin. Unknown downstream use makes an internal move breaking. |
| `log` | public plugin API | The plugin development contract explicitly requires `PluginPrint` and `PluginV`; it therefore remains importable. |
| `plugin` | public stable contract | Owns requests, responses, manifests, plugin management, and protocol compatibility. |
| `presentation` | public stable contract | Owns transport-safe presentation declarations that plugins construct. It does not render output. |
| `renderer` | public reusable API | Exposes response rendering, output formats, width injection, and color-capability injection for Core and embedders. Existing importability is preserved. |
| `update` | compatibility surface | Implements Core and plugin update operations. It is Core-oriented, but its established exported API is retained because downstream use is unknown. |
| `version` | public build contract | Exposes build metadata and version reporting used by Core builds and commands. |

Compatibility-surface classification is intentional, not a promise to add new
API there. Moving one of these packages to `internal/` requires its own source-
compatibility decision and migration.

## Presentation and rendering boundaries

`pkg/presentation` declares tables, columns, properties, text, semantic style
roles, and the transport-only `Table.DescribeOnly` visibility marker. These
values are rendering instructions and metadata, not final rendered bytes.
`pkg/plugin.Response` carries them through the plugin protocol. Core's public
JSON renderer, raw JSON path, and GitHub output omit them.

`pkg/renderer` consumes `pkg/plugin` and `pkg/presentation`. The renderer owns
layout, responsive width behavior, ANSI-safe visible-width calculations,
describe-only section filtering, and the mapping from semantic roles to
terminal styles. Global `--describe` selects structured details and metadata;
global `--verbose` independently selects captured logs. Describe does not
change public or raw JSON; verbose leaves domain data unchanged while public
JSON logs may differ when the plugin produces additional verbose entries.
Structured error responses retain the normal error block and may explicitly
append the same neutral presentation sequence; errors without presentation
metadata render exactly as before.
Presentation declarations must never import the renderer, and the renderer
must not import the logger.

`cmd` owns the persistent `--describe`, `--verbose`, `--output`, and
`--github-output-file` flags plus their custom plugin-help composition. Only
verbose crosses the request boundary as `plugin.Context.Verbose`; local flag
extraction reads command-local, non-persistent flags. `pkg/dispatcher` owns
subprocess capture, ANSI sanitation before stderr parsing, and deterministic
log merging. Response-provided logs precede captured logs, with only exact
cross-channel duplicates collapsed. `pkg/plugin` remains a neutral transport
model and imports neither renderer nor terminal implementation.

`pkg/log` is independent of the renderer. Both packages may use only the
private `internal/terminal` primitives for ANSI application and terminal
color capability. This avoids a renderer-to-logger dependency while keeping
terminal policy consistent.

`internal/terminal` remains under `internal/` because its ANSI palette,
reset application, TTY check, and `NO_COLOR` policy are implementation details,
not an external theming or styling API. Its responsibilities remain limited to
styling primitives in `style.go` and terminal/color capability policy in
`capability.go`; it does not own layout, logging, or presentation semantics.

## Protocol compatibility

Canonical Go code uses `presentation.Table`, `presentation.Column`,
`presentation.Properties`, `presentation.Property`, `presentation.Text`, and
`presentation.StyleRole`, attached through `Response.PresentationTable`,
`Response.PresentationProperties`, and `Response.PresentationText`.

The process protocol retains `human_table`, `human_properties`, and `human_text`
for installed-plugin compatibility. `pkg/plugin/response_presentation.go`
isolates those tags. Deprecated Go aliases and constants live only in
`pkg/plugin/presentation_compatibility.go`; deprecated response fields remain on
`Response` because Go struct-literal compatibility cannot be provided by a type
alias in another file. Decoding mirrors one declaration into both field names,
while conflicting canonical and deprecated values are rejected.
