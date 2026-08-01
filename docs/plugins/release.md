# Release Plugin

The `release` plugin provides Release V1 compatibility and the canonical
Release V2 lifecycle. Its current manifest version is `v4.1.0`.

The [Release CLI Reference](../release/cli-reference.md) is authoritative for
every command and flag, V1/V2 support, required source, I/O, network and token
boundary, output mode, and exit status. This page is a workflow summary and
does not duplicate that matrix.

## Installation

Install the published plugin through the Core plugin registry:

```bash
neko plugin available
neko plugin install release
```

Core installs it below `~/.neko/plugins/release/` by default. Discovery,
installation, and update use the `plugin-index.json` asset on the mutable
`plugin-registry` GitHub Release; they do not infer the plugin version from the
repository's latest release.

## Source generations

| Generation | Authority | Role |
| --- | --- | --- |
| Release V1 | Root `.release.neko.json` | Supported compatibility surface for existing single-stream repositories |
| Release V2 | `.neko/release.config.json` and `.neko/release.state.json` | Canonical active architecture for new setup and multi-unit lifecycle |

V1 is not removed, but new setup is V2-only. A root V1 source and V2 source
must not compete; source selection rejects coexistence, incomplete pairs, and
malformed or mismatched V2 state instead of merging authorities. Use the
[migration guide](../release/migration-v1-to-v2.md) to move a supported root
V1 repository to V2.

## Command groups

- Shared V1/V2 surfaces: `patch`, `minor`, `major`, `plan`, `history`,
  `contributors`, `validate`, `evidence`, and `evidence-archive` where the
  selected evidence family supports the source generation.
- V2-only surfaces: `init`, `unit-add`, `init-options`, `doctor`, `units`,
  `pipeline`, `ci-validate-context`, `github-workflow-init`, `resume`, and
  `plugin-index`.
- Migration-only surface: `migrate`, from one supported root V1 source to one
  V2 `default` unit.
- Core-owned surface: `neko release` renders the installed manifest overview
  without starting the plugin executable.

There are no public Release command aliases, compatibility flag aliases, or
deprecated public commands/flags. The full 20-command and 66-local-flag
inventories are maintained in the canonical reference.

## Typical V2 setup

```bash
neko release init \
  --executor goreleaser \
  --delivery github-actions \
  --workflow .github/workflows/release-cli.yml

neko release github-workflow-init --dry-run
neko release validate --show
neko release doctor
neko release pipeline --unit cli
neko release patch --unit cli --dry-run
```

`init` writes only the validated V2 config/state pair and its bounded recovery
evidence. It does not create a workflow or executor configuration. Workflow
creation is the separate create-only `github-workflow-init` capability; it
accepts identical bytes and refuses to overwrite different content.

For multiple units, use `unit-add`. Normal units use the default `release`
kind. Only Neko CLI plugins use `--kind plugin` and the four plugin metadata
flags.

## Migration

```bash
neko release migrate --dry-run
neko release migrate
neko release validate --show
```

Dry-run reads and validates the root `.release.neko.json` and returns the
planned V2 artifacts without writing. A real migration writes the V2 pair,
uses its recovery journal, and archives the V1 source as
`.release.neko.json.v1.bak`. It refuses nested V1, mixed authorities, unsafe
existing destinations, and conflicting recovery evidence.

## Output and exit

Core accepts `table`, `json`, `wide`, and `github` for structured plugin
responses. Release manifests declare `table` and `json` for ordinary commands;
only `ci-validate-context` declares successful GitHub command-file fields.
Plugin Index raw mode is undecorated schema-v1 JSON, distinct from Core
`--output json`.

`--describe` adds complete safe human facts where available and is a
deterministic no-op otherwise. `--verbose` adds already-owned safe execution
phases where available and is likewise a no-op on static commands. Neither
flag independently enables a read, token lookup, network request, or mutation.

Release responses explicitly use exit `0` or `1`. Successful mutations,
dry-runs, and successful negative observations use `0`; invalid requests,
failed checks, actionable refusals, and execution failures use `1`. Core owns
transport, response-validation, render, and command-file failures as exit `1`.

## Network and mutation boundaries

- Help, default Doctor/Pipeline, Units, Evidence, Plugin Index, validation,
  planning, and dry-run lifecycle paths are offline.
- Doctor and Pipeline use bounded GitHub GET requests only with explicit
  `--verify-remote`; `GITHUB_TOKEN` is optional for that path.
- CI context validation inspects local Git and never dispatches or publishes.
- Actual V2 lifecycle execution may materialize files, update state, commit,
  tag, push, journal, and dispatch the configured workflow. Dispatch is a
  remote handoff, not publication; the consumer workflow owns build and
  publication.
- Resume continues only an existing unresolved journal and never plans a new
  release. Its dry-run is local and non-mutating.
- Evidence inspection is read-only. Evidence Archive is the separate guarded
  local mutation for eligible completed evidence.
- Plugin Index persists bytes only with `--output-file`; `--check` is
  read-only and mutually exclusive with persistence.

## Plugin Index example

```bash
neko release plugin-index
neko release plugin-index --check
neko release plugin-index --output-file /private/tmp/plugin-index.json
```

Core `--output` selects response rendering and is never a file path. The old
`plugin-index --output <path>` spelling is not an alias.

## Release of this plugin

This repository models the plugin as V2 unit `plugin-release`, with version in
`.neko/release.state.json`, manifest at `plugin/release/manifest.json`, tag
prefix `plugin-release/v`, and workflow
`.github/workflows/release-plugin-release.yml`. Neko owns release identity,
state, commit, tag, push, and dispatch. The workflow owns the plugin-only
archive, GitHub Release, assets, and subsequent registry-index publication.

## Related documentation

- [Canonical Release CLI Reference](../release/cli-reference.md)
- [Release system overview](../release/overview.md)
- [Release V1 compatibility](../release/compatibility.md)
- [V1-to-V2 migration](../release/migration-v1-to-v2.md)
- [Release V2 examples](../release/examples.md)
- [GitHub Actions golden path](../release/github-actions-golden-path.md)
- [Bootstrap ownership boundary](../release/bootstrap-product-boundary.md)
- [Plugin development](../plugin-development.md)
