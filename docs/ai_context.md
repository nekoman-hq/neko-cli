# AI Context for Neko CLI

## Purpose and authority

This document is the compact bootstrap for an agent beginning work in this
repository. It summarizes current product and architecture contracts; it is not
a command reference, design specification, roadmap, or replacement for code.
When a statement here conflicts with a current owner, fix this summary rather
than treating it as a second authority.

Use evidence in this order:

1. command registration, plugin manifests, routing, and flag parsers;
2. wire types plus current config and state schemas;
3. current architecture and package-ownership documents;
4. canonical user-facing references;
5. compatibility and deprecation policy;
6. focused contract tests and guards;
7. numbered history for rationale only.

The [repository-wide CLI reference](cli-reference.md) and
[Release CLI reference](release/cli-reference.md) own public behavior. Current
architecture documents own internal boundaries. The numbered history records
completed or superseded work and never overrides either. Do not revive stale
roadmap text as current implementation guidance.

## Repository and Core CLI

Neko CLI is an extensible Go command-line application. Core owns the root
process, direct commands such as version, self-update, and plugin management,
and the plugin command tree. Installed plugin manifests declare their command,
local-flag, help, and selectable-output surface. Core discovers those manifests
under the configured plugin directory, constructs the visible routes, and
starts a plugin executable only when a routed plugin command runs. The full
Core, Release, and UI inventory is maintained in the
[CLI reference](cli-reference.md); do not duplicate its command matrix here.

For plugin responses Core owns the global `--describe`, `--verbose`, `--output`,
and `--github-output-file` flags. Describe requests safe additional human
structure where a command declares it. Verbose transports a diagnostic intent
and may add chronological logs; it is not structured domain detail. Output
selects `table`, `wide`, `json`, or `github`. GitHub output requires an explicit
command-file destination and only commands that declare that output may use it.

Human table and wide presentation is responsive to terminal capabilities and
width. JSON is the public `plugin.Response` envelope, with presentation-only
metadata excluded. Plugin Index is a deliberate raw-output exception: its
schema-v1 JSON can be rendered directly. `--output-file` persists Plugin Index
schema-v1 bytes. `--output` selects the Core response format; it is not a
persistence flag.

Production Release routing resolves one repository root and passes that typed
root to explicit-root handlers; it does not change process cwd. Cwd-based public
facades remain compatibility surfaces, not the production composition model.
Core dispatches once, validates and renders one result once, then maps the
validated result or Core failure to the final process status.

## Plugin transport and exit ownership

A plugin is a non-interactive subprocess. Core sends one JSON `plugin.Request`
on stdin; the plugin returns one JSON `plugin.Response` on stdout. Diagnostic
logs go to stderr so they cannot corrupt transport. Domain packages declare
neutral presentation data rather than printing tables or interpreting terminal
state.

A valid decoded `plugin.Response` owns the final process exit; explicit exits
from `0` through `125` propagate exactly. In-repository Release commands assign
only `0` and `1`; omitted exits are temporary legacy compatibility. Invalid
requests, failed checks, refusals, and execution failures use `1`. A negative
domain observation can still be successful when the command completed its
inspection. Important examples are:

- Pipeline blocked → `0`; Pipeline invalid evidence → `1`.
- Doctor warning → `0`; Doctor not ready → `1`.
- Resume unsafe dry-run → `0`; Resume no journal → `1`.

The boundary is explicit: transport and rendering failures are Core-owned and
exit `1`, including malformed transport, an invalid response envelope or exit,
and subprocess startup. A valid response is not discarded merely because the
subprocess itself returned nonzero. Each normal
result or error renders exactly once; handlers do not pre-render or call another
handler to reuse policy.

## Release V1 compatibility

V1 is supported compatibility, not the canonical architecture. The root
`.release.neko.json` is the V1 authority and maps to one virtual default unit.
Its configured GoReleaser, JReleaser, and release-it behavior remains supported.
Shared `patch`, `minor`, `major`, `plan`, `history`, `contributors`, `validate`,
`evidence`, and eligible `evidence-archive` surfaces remain available as the
[Release CLI reference](release/cli-reference.md) specifies.

V1 is not the preferred setup for a new repository. V2-only setup, integration
inspection, context validation, workflow scaffolding, recovery, and registry
commands are unavailable to a pure V1 repository. V1 and V2 cannot remain
active as competing authorities: a mixed active source is rejected.
`neko release migrate` is the supported transition. See
[compatibility](release/compatibility.md) and the
[migration guide](release/migration-v1-to-v2.md) before changing legacy
behavior.

Direct Go integrations that retain V1 executor composition use concrete,
caller-owned executors; the mutable legacy registry is not production
composition:

```go
v1Executors := []release.V1Executor{
    goreleaser.NewV1Executor(),
    jreleaser.NewV1Executor(),
    releaseit.NewV1Executor(),
}
resp, err := release.HandleReleaseWithV1Executors(req, release.Patch, v1Executors...)
```

The executable uses the corresponding explicit-root entry point. Preserve the
characterized V1 facades until compatibility policy and consumer evidence
authorize a separate removal.

## Release V2 architecture

V2 is the canonical architecture for new repositories. At repository root,
`.neko/release.config.json` owns declared configuration and
`.neko/release.state.json` owns current unit versions. Each release unit owns an
ID, paths and working directory, version, tag prefix, executor, delivery mode,
and optional plugin metadata. Normal units release a product; plugin units also
declare the manifest identity and artifact naming needed by the registry.

Strict loading validates config/state agreement, unit and path safety, tag
namespace separation, workflow confinement, and plugin metadata. Planning and
materialization operate only on the selected unit and its declared known
release files. State is the version authority; tags are derived from the unit's
prefix and planned version.

Neko owns source and unit resolution, preflight, version planning,
materialization, state update, targeted release commit, lightweight unit tag,
commit and tag pushes, execution and dispatch journals, evidence, recovery, and
workflow dispatch. The configured consumer-owned GitHub Actions workflow owns
builds, GitHub Release creation, and artifact publication. Therefore dispatch
is a handoff, not publication completion.

The 21 public Release paths are covered by these capability groups:

| Group | Current paths |
| --- | --- |
| Overview and setup | `neko release`, `neko release init`, `neko release unit-add`, `neko release init-options` |
| Migration | `neko release migrate` |
| Lifecycle | `neko release patch`, `neko release minor`, `neko release major` |
| Planning and queries | `neko release plan`, `neko release validate`, `neko release history`, `neko release contributors` |
| Inspection | `neko release doctor`, `neko release units`, `neko release pipeline`, `neko release evidence` |
| CI, scaffolding, and recovery | `neko release ci-validate-context`, `neko release github-workflow-init`, `neko release resume`, `neko release evidence-archive` |
| Registry artifact | `neko release plugin-index` |

Support, flags, output, I/O, and exit detail belongs in the canonical
[Release CLI reference](release/cli-reference.md), not in this bootstrap.

## Inspection and safety boundaries

Doctor is strictly read-only. It is offline and token-free by default. Remote
facts are requested only with explicit `--verify-remote`, through bounded
GET-only verification that cannot dispatch, upload, publish, or mutate. Doctor
never repairs configuration, files, or workflows. Units is a read-only unit
inventory and readiness inspection.

Pipeline is a read-only local projection of configured lifecycle steps, exactly
correlated journals, local Git facts, recovery and retry safety. Optional remote
verification uses Doctor's bounded read boundary. Pipeline has no transition,
retry, Resume, or mutation capability; it is not a lifecycle engine or state
machine. Blocked, uncertain, or rejected states may be successful observations.

Plan is read-only release planning; a blocked plan can still be a successful
observation. Evidence is read-only inspection across the supported journal and
recovery families. It may report malformed evidence diagnostically with
success, while invalid filters fail. Evidence Archive is a separate, explicit
guarded local mutation for eligible completed evidence only.

Workflow Init is create-only. It can create one missing starter workflow and
accept byte-identical existing content without rewriting it. It never
overwrites differing customized content; workflows remain consumer-owned after
scaffolding. Resume continues one existing unresolved execution under exact
journal selection and established recovery policy; it does not create a new
release.

Global presentation flags do not change these boundaries: `--describe` and
`--verbose` never add network, token, or mutation reachability.

## Release lifecycle and recovery

Keep the conceptual flow readable without inventing a second engine:

```text
command routing
→ source and unit resolution
→ preflight
→ planning and materialization
→ local Git mutation
→ release-tool preparation
→ push and provider workflow dispatch
→ consumer build and publication
→ journals, Evidence, and recovery
```

Command dispatch, release-tool invocation, Git push, provider workflow
dispatch, and artifact publication are distinct operations with different
owners and failure evidence. Dry-run stops before mutation, token lookup,
network, journals, executor invocation, push, or dispatch. Unsafe or uncertain
effects fail closed instead of being guessed or destructively rolled back.

`plugin/release/pkg/release` is the authoritative lifecycle owner: release
planning/orchestration, Git coordination, execution and dispatch journals,
Resume, compensation, and recovery live there. Pipeline and other projections
consume typed facts and cannot advance that lifecycle.

## Architecture constraints

| Area | Stable responsibility |
| --- | --- |
| `cmd` | CLI composition, global flags, plugin routing, rendering call, final process status |
| `pkg/plugin` | neutral request, response, log, and explicit-exit transport contracts |
| `pkg/dispatcher` | subprocess execution and response decoding |
| `pkg/presentation` | domain-neutral presentation declarations |
| `pkg/renderer` | responsive human, JSON, and GitHub rendering |
| `internal/terminal` | terminal width, TTY, color, and display capabilities |
| `plugin/release/internal/*` | focused internal capability projections and shared leaf facts |
| `plugin/release/pkg/*` | command-owned parsing, operations, and presentation mapping |
| `plugin/release/pkg/release` | authoritative lifecycle, Git, journals, Resume, and recovery |

Release root handlers decode, resolve a root, compose fresh executors, and
route. Command handlers parse and map; read-only capabilities do not receive
mutation ports. Presentation mapping does not read Git, files, credentials,
journals, or provider state. Terminal dependencies stay outside domain and
application logic.

Do not introduce a generic lifecycle framework, second state machine, stage
registry, command-handler chaining, provider hierarchy, dependency bag or
service locator, workflow DSL, second renderer, command-name switches in Core,
or domain-status interpretation in Core. Existing release-tool support is
concrete and bounded to the current supported tools, not a speculative generic
framework. Review package changes against
[package ownership](../plugin/release/docs/architecture/package-ownership.md)
and the [maintainability policy](../plugin/release/docs/architecture/maintainability-policy.md).

## Self-update and installation

The ordinary-user installer default is `$HOME/.local/bin`; an explicit
`NEKO_INSTALL_DIR` wins, and deliberate root execution may default to
`/usr/local/bin`. The installer never invokes `sudo` automatically, and the
updater likewise does not; neither changes directory ownership.

`neko update --force` means same-version reinstall only. It does not bypass
permissions, integrity checks, package-manager ownership, or downgrade
protection. Positively identified Homebrew-owned installations are refused in
favor of the package manager. Self-update supports only the documented macOS
and Linux architectures, which are narrower than the install script matrix.

For every update, checksum verification and archive validation are mandatory.
After validation,
replacement uses a unique same-directory sibling followed by atomic rename;
ordinary mode bits are preserved while special bits are stripped. Dry-run does
not download the archive or replace the executable. See
[Installation](installation.md) for platform, ownership, integrity, symlink,
and failure details.

## Self-release of this repository

This repository dogfoods V2 with independently versioned `cli`,
`plugin-release`, and `plugin-ui` units. Current versions exist only in
`.neko/release.state.json`; do not copy them into documentation. Configured tag
namespaces are `vX.Y.Z`, `plugin-release/vX.Y.Z`, and `plugin-ui/vX.Y.Z`.

The consumer-owned workflows are
`.github/workflows/release-neko-cli.yml`,
`.github/workflows/release-plugin-release.yml`, and
`.github/workflows/release-plugin-ui.yml`. Neko prepares the selected unit's
state, materialization, commit, tag, pushes, evidence, and validated dispatch.
The selected workflow builds and publishes. Doctor remains read-only, and
Workflow Init remains create-only. The concise repository entry point is
[How Neko CLI releases itself](../README.md#how-neko-cli-releases-itself).

## Documentation navigation

Current product contracts:

- [repository-wide CLI reference](cli-reference.md),
  [Release CLI reference](release/cli-reference.md), and
  [Installation](installation.md);
- [Release overview](release/overview.md),
  [GitHub Actions Golden Path](release/github-actions-golden-path.md), and
  [bootstrap product boundary](release/bootstrap-product-boundary.md);
- [migration](release/migration-v1-to-v2.md),
  [compatibility](release/compatibility.md),
  [Release plugin guide](plugins/release.md), and [UI plugin guide](plugins/ui.md).

Current architecture:

- [current state](../plugin/release/docs/architecture/current-state.md),
  [package ownership](../plugin/release/docs/architecture/package-ownership.md),
  and [architecture decisions](../plugin/release/docs/architecture/architecture-decisions.md);
- [maintainability policy](../plugin/release/docs/architecture/maintainability-policy.md),
  [compatibility notes](../plugin/release/docs/architecture/compatibility-notes.md),
  and [V1 compatibility policy](../plugin/release/docs/architecture/v1-compatibility-policy.md).

Historical rationale lives only in the
[numbered history index](../plugin/release/docs/history/README.md). README is an
entry point, the CLI references own command contracts, architecture documents
own current internal boundaries, and history is not implementation guidance.

## Active versus completed work

Release V2 product contracts described above are implemented and guarded; the
completed refactor and finalization sequences belong to numbered history, not
an active checklist. Current bounded limitations are maintained in
[current state](../plugin/release/docs/architecture/current-state.md#current-bounded-limitations-prioritized)
and unresolved boundaries in
[architecture decisions](../plugin/release/docs/architecture/architecture-decisions.md#pending-architecture-decisions).

Not implemented means exactly that, not an implied roadmap: V2 local execution,
public standalone dispatch or retry commands, and durable workflow-run or
publication-completion state are absent. Future changes must begin from current
code and current owners rather than historical plans.

## Do not assume

- V1 is supported compatibility; it is neither removed nor canonical.
- Doctor does not repair, and Workflow Init does not update customized workflows.
- Pipeline does not execute, retry, or resume lifecycle steps.
- Dispatch is not publication completion.
- `--describe` adds no I/O; `--verbose` is not structured product detail.
- `--output` is a response format, not Plugin Index persistence; Plugin Index file persistence uses `--output-file`.
- Update force is not a permission, integrity, manager-ownership, or downgrade bypass.
- Historical roadmaps are not current architecture.
