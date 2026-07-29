# Release V2 Examples

This page contains copy-ready examples for Release V2 repositories and the
plugin registry. Reference docs stay concise; use this page when bootstrapping a
new repository or adding units to an existing one.

For the ownership boundary between Neko CLI, GitHub Actions workflows,
build-system adapters, and consumer-owned publication logic, see
[Release V2 Bootstrap Product Boundary](bootstrap-product-boundary.md).
For the complete installation, workflow, release, and recovery sequence, use
the [Release V2 GitHub Actions Golden Path](github-actions-golden-path.md).

## Mental Model

Release V2 has two committed repository-root files:

```text
.neko/release.config.json
.neko/release.state.json
```

`release.config.json` is static release architecture: units, path ownership,
working directories, tag prefixes, executor type, delivery mode, workflow path,
and optional plugin metadata.

`release.state.json` is authoritative version state. Tags are not stored in
state; each tag is derived from `tagPrefix + version`.

Core terms:

| Term | Meaning |
| --- | --- |
| `unit` | Independently releasable object such as `cli`, `api`, or `plugin-foo` |
| `tagPrefix` | Namespace used to derive tags for a unit, such as `v` or `api/v` |
| `executor` | Release tool: `goreleaser`, `jreleaser`, or `release-it` |
| `delivery` | Release handoff mode; V2 supports `github-actions` |
| `kind: plugin` | Marks a V2 unit as a public plugin registry entry |

Multi-unit repositories require `--unit` for unit-scoped commands.

## Normal Release Units Vs Neko CLI Plugin Units

Normal release units are the default for services, apps, CLIs, SDKs,
libraries, and backend modules. CLI commands use `--kind release` by default,
but V2 JSON omits `kind` for normal units. Normal units do not have a `plugin`
block, do not contribute to `plugin-index.json`, and do not use the
`plugin-registry` GitHub Release.

Neko CLI plugin units are only for plugins distributed through
`neko plugin install` and `neko plugin update`. They use `kind: "plugin"` in
V2 JSON, require the `plugin` metadata block, are included in
`plugin-index.json`, and that index is published as an asset on the mutable
`plugin-registry` GitHub Release.

Do I need plugin fields in a normal repository? No. A repository can contain
only normal release units and needs no plugin metadata or plugin registry.

When do I use `kind=plugin`? Only when publishing an actual Neko CLI plugin.
Plugin flags without `--kind plugin` are invalid.

Normal release unit JSON:

```json
{
  "id": "api",
  "displayName": "Onetake API",
  "paths": ["apps/api/**"],
  "workingDirectory": ".",
  "tagPrefix": "api/v",
  "executor": {
    "type": "jreleaser",
    "delivery": "github-actions",
    "workflow": ".github/workflows/release-api.yml"
  }
}
```

Neko CLI plugin unit JSON:

```json
{
  "id": "plugin-release",
  "displayName": "neko-cli release plugin",
  "paths": ["plugin/release/**", "docs/plugins/release.md"],
  "workingDirectory": ".",
  "tagPrefix": "plugin-release/v",
  "kind": "plugin",
  "plugin": {
    "name": "release",
    "manifest": "plugin/release/manifest.json",
    "assetPrefix": "plugin-release",
    "binaryName": "plugin-release"
  },
  "executor": {
    "type": "goreleaser",
    "delivery": "github-actions",
    "workflow": ".github/workflows/release-plugin-release.yml"
  }
}
```

## Stable Lifecycle

```bash
neko release init ...
neko release unit-add ...
neko release validate --show
neko release plan --change patch --unit <unit>
neko release patch --unit <unit>
neko release history --unit <unit>
neko release contributors --unit <unit>
neko release resume --unit <unit> --dry-run
```

Use `neko release migrate` for a legacy root `.release.neko.json` repository
before adding V2 units.

## Example A: Initialize A CLI Project

```bash
neko release init \
  --unit cli \
  --display-name my-cli \
  --version 0.1.0 \
  --executor goreleaser \
  --delivery github-actions \
  --workflow .github/workflows/release-cli.yml \
  --tag-prefix v \
  --working-directory . \
  --paths "**"
```

Generated files:

```text
.neko/release.config.json
.neko/release.state.json
```

Not generated yet:

- workflow files
- GoReleaser configs
- plugin manifests
- source files or directories

For `github-actions` delivery, the workflow file must already exist below
`.github/workflows/`.

Inspect the local release plan before running a release:

```bash
neko release plan --change patch --unit cli
```

The plan inspection reports local source/unit selection, current and next
version, tag, materialized files, known release files, local blockers, and
limitations. It does not read tokens, inspect remotes or journals, write files,
mutate Git, dispatch workflows, publish, or run executors.

## Example B: Add A Backend Or Service Unit

```bash
neko release unit-add \
  --unit api \
  --display-name api \
  --version 0.1.0 \
  --executor goreleaser \
  --delivery github-actions \
  --workflow .github/workflows/release-api.yml \
  --tag-prefix api/v \
  --working-directory . \
  --paths "apps/api/**,docs/api/**"
```

`unit-add` appends to existing V2 config/state, preserves existing units in
order, and never overwrites an existing unit.

## Example C: Add A Plugin Unit

```bash
neko release unit-add \
  --unit plugin-foo \
  --display-name "foo plugin" \
  --version 0.1.0 \
  --executor goreleaser \
  --delivery github-actions \
  --workflow .github/workflows/release-plugin-foo.yml \
  --tag-prefix plugin-foo/v \
  --working-directory . \
  --paths "plugin/foo/**,docs/plugins/foo.md" \
  --kind plugin \
  --plugin-name foo \
  --plugin-manifest plugin/foo/manifest.json \
  --plugin-asset-prefix plugin-foo \
  --plugin-binary-name plugin-foo
```

Prerequisites:

- workflow file already exists
- plugin manifest already exists
- GoReleaser config already exists or is created manually
- plugin unit id starts with `plugin-`
- plugin tag prefix is `<unit-id>/v`
- plugin asset prefix equals the unit id

Plugin metadata lives on the V2 unit. `neko release plugin-index` reads unit
metadata, `.neko/release.state.json`, and plugin manifests to generate the
public registry index.

## Example D: Release A Unit

```bash
neko release patch --unit plugin-foo --verbose --describe
```

Expected output fields for a real GitHub Actions handoff:

```text
Unit
Version
Tag
Release Commit
Workflow
Execution Journal
Dispatch Journal
Execution State
Dispatch State
Dispatch Run
Status
```

Dry-run is read-only and does not require `GITHUB_TOKEN`:

```bash
neko release patch --unit plugin-foo --dry-run --verbose --describe
```

## Example E: Generate The Plugin Registry Index

```bash
neko release plugin-index --check
neko release plugin-index --check --output json
neko release plugin-index --output-file /tmp/plugin-index.json
```

The generated `plugin-index.json` is not committed to the repository and this
command does not publish it. Plugin release workflows generate and publish the
index automatically after successful plugin releases by uploading or replacing
the `plugin-index.json` asset on the mutable `plugin-registry` GitHub Release.
Relative `--output-file` paths are resolved from the repository root. Absolute
artifact paths such as `/tmp/plugin-index.json` remain supported for CI and
temporary files, while repository-contained output is blocked from overwriting
release state, evidence, Git internals, or plugin manifest inputs.

Core `--output` selects only the response format. `--output json` renders the
public plugin-response JSON for check or persist results; it does not name a
file. Raw mode prints the exact schema-v1 artifact by default, and
`--pretty=false` selects its compact byte form. The former
`plugin-index --output <path>` spelling is rejected as an invalid Core format
and never falls back to persistence.

## Example F: Discover, Install, And Update Plugins

```bash
neko plugin available
neko plugin install release
neko plugin update release
```

Temp-safe smoke usage:

```bash
NEKO_PLUGIN_DIR=/private/tmp/neko-plugin-smoke neko plugin available
```

Runtime plugin discovery, install, and update read the published
`plugin-index.json` asset from the `plugin-registry` GitHub Release. They do not
use `/releases/latest` or release-prefix fallback discovery.

## GitHub Release Categories

| Category | Tag | Assets | Notes |
| --- | --- | --- | --- |
| CLI | `vX.Y.Z` | `neko-cli_...` | Used by `install.sh`, `neko version`, and `neko update` |
| Release plugin | `plugin-release/vX.Y.Z` | `plugin-release_...` | Plugin unit with `kind: plugin` |
| UI plugin | `plugin-ui/vX.Y.Z` | `plugin-ui_...` | Plugin unit with `kind: plugin` |
| Plugin registry | `plugin-registry` | `plugin-index.json` | Mutable registry release, not a product release unit |

Important boundaries:

- `plugin-index.json` is not committed to the repository.
- `plugin-index.json` is not attached to CLI releases.
- `plugin-index.json` lives as an asset on `plugin-registry`.
- `/releases/latest` is not used for plugin discovery.
- CLI install/update/version resolves only stable CLI tags matching `vX.Y.Z`.
