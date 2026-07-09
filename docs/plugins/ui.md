# UI Plugin

The `ui` plugin is released as the independent V2 unit `plugin-ui`.

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

Plugin installation and updates resolve the newest UI plugin version from the published `plugin-index.json` registry asset on the mutable `plugin-registry` GitHub Release. The repository's latest release is not used for UI plugin discovery. The local installed version comes from `~/.neko/plugins/ui/manifest.json`; the remote version, release tag, and asset names come from the index entry. Until M2E removes it, a temporary fallback can still discover the built-in UI plugin from `plugin-ui/vX.Y.Z` tags.

The `plugin-ui` V2 unit declares `kind: "plugin"` metadata in `.neko/release.config.json`. `neko release plugin-index` generates the public `plugin-index.json` from V2 plugin units, `.neko/release.state.json`, and plugin manifests. Runtime plugin discovery, install, and update use that index as the primary registry source. Plugin release workflows publish it as the `plugin-index.json` asset on the mutable `plugin-registry` GitHub Release after successful plugin releases; the index is not committed as source. Until M2E removes it, runtime install/update keeps an explicit temporary release-prefix fallback for existing built-in plugins.

`.plugin.release.neko.json` is removed and is not used for UI plugin versioning.
