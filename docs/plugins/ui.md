# UI Plugin

> **Audience:** Users of the first-party UI Plugin and contributors comparing its manifest with its router.
>
> **Purpose:** State the UI Plugin's actual command surface, file and network effects, and known manifest/router discrepancy.

The `ui` plugin is released as the independent V2 unit `plugin-ui`.

## Public CLI surface

The authoritative repository-wide command/flag and I/O matrix is the
[CLI reference](../cli-reference.md). The UI manifest advertises `hello`,
`init`, `list`, `add [component-name]`, and
`remove [component-name]`. `hello --name` is manifest-declared but the current
UI router does not route a `hello` handler, so execution fails; it is documented as an
existing discrepancy, not as a working command.

`init --components-path` and `init --force` write `.ui.neko.json` and the
component directory locally. `list` is read-only locally but performs GitHub
GET requests. `add [component-name]` or `add --all` performs GitHub reads and
writes component files. `remove [component-name]` or `remove --all` is offline
and recursively deletes selected component directories. UI responses retain
legacy implicit-success exit compatibility when their valid transport omits an
explicit exit; dispatch, transport, or malformed-response failures exit `1`.
Core response flags select presentation and do not change these I/O boundaries.

## Release Model

| Property | Value |
| --- | --- |
| Unit | `plugin-ui` |
| Version source | `.neko/release.state.json` |
| Manifest | `plugin/ui/manifest.json` |
| Tag prefix | `plugin-ui/v` |
| Plugin metadata | `name=ui`, `assetPrefix=plugin-ui`, `binaryName=plugin-ui` |
| Workflow | `.github/workflows/release-plugin-ui.yml` |
| GoReleaser config | `.goreleaser.plugin-ui.yaml` |

Neko CLI materializes only `plugin/ui/manifest.json` for a `plugin-ui` release. The workflow checks out the pushed tag, validates state/config/manifest, runs tests, builds only the UI plugin, and publishes a GitHub Release for the exact tag. After that publish succeeds, it generates and validates `plugin-index.json`, then uploads or replaces that single asset on the mutable `plugin-registry` release.

Plugin installation and updates resolve the newest UI plugin version from the published `plugin-index.json` registry asset on the mutable `plugin-registry` GitHub Release. The repository's latest release and release-prefix fallback discovery are not used for UI plugin discovery. The local installed version comes from `~/.neko/plugins/ui/manifest.json`; the remote version, release tag, and asset names come from the index entry.

The `plugin-ui` V2 unit declares `kind: "plugin"` metadata in `.neko/release.config.json`. `neko release plugin-index` generates the public `plugin-index.json` from V2 plugin units, `.neko/release.state.json`, and plugin manifests. Runtime plugin discovery, install, and update use that index as the registry source of truth. Plugin release workflows publish it as the `plugin-index.json` asset on the mutable `plugin-registry` GitHub Release after successful plugin releases; the index is not committed as source.

`.plugin.release.neko.json` is removed and is not used for UI plugin versioning.
