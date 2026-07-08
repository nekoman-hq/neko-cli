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

`.plugin.release.neko.json` is removed and is not used for UI plugin versioning.
