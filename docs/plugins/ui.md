# UI Plugin

The `ui` plugin is released as the independent V2 unit `plugin-ui`.

## Release Model

| Property | Value |
| --- | --- |
| Unit | `plugin-ui` |
| Version source | `.neko/release.state.json` |
| Manifest | `plugin/ui/manifest.json` |
| Tag prefix | `plugin-ui/v` |
| Workflow | `.github/workflows/release-plugin-ui.yml` |
| GoReleaser config | `.goreleaser.plugin-ui.yaml` |

Neko CLI materializes only `plugin/ui/manifest.json` for a `plugin-ui` release. The workflow checks out the pushed tag, validates state/config/manifest, runs tests, builds only the UI plugin, and publishes a GitHub Release for the exact tag.

Plugin installation and updates resolve the newest UI plugin version from plugin-specific V2 unit releases with the tag prefix `plugin-ui/v`. The repository's latest release is not used for UI plugin discovery. The local installed version comes from `~/.neko/plugins/ui/manifest.json`; the remote version comes from the selected `plugin-ui/vX.Y.Z` GitHub Release tag.

`.plugin.release.neko.json` is removed and is not used for UI plugin versioning.
