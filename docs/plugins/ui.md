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

Neko CLI materializes only `plugin/ui/manifest.json` for a `plugin-ui` release. The workflow checks out the pushed tag, validates state/config/manifest, runs tests, builds only the UI plugin, and publishes a GitHub Release for the exact tag.

Plugin installation and updates resolve the newest UI plugin version from plugin-specific V2 unit releases with the tag prefix `plugin-ui/v`. The repository's latest release is not used for UI plugin discovery. The local installed version comes from `~/.neko/plugins/ui/manifest.json`; the remote version comes from the selected `plugin-ui/vX.Y.Z` GitHub Release tag.

The `plugin-ui` V2 unit declares `kind: "plugin"` metadata in `.neko/release.config.json` for future `plugin-index.json` generation. The index is not implemented yet, and install/update still uses the current registry behavior until the next plugin-registry milestone.

`.plugin.release.neko.json` is removed and is not used for UI plugin versioning.
