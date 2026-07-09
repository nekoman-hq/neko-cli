# Installation

## Release Install Script

`install.sh` installs the main `neko` CLI from GitHub Releases. It does not install plugins and does not read repository-local release files such as legacy V1 config or V2 state/config.

By default, the script queries the configured repository's releases, filters for stable CLI tags matching `vX.Y.Z`, ignores plugin releases and the `plugin-registry` release, sorts the CLI tags as semantic versions, and installs the newest one. The Go implementation used by `neko version` and `neko update` follows the same multi-unit rule for CLI release checks.

```bash
./install.sh
```

The script selects CLI archive assets by platform from GoReleaser names such as:

```text
neko-cli_Darwin_arm64.tar.gz
neko-cli_Linux_x86_64.tar.gz
neko-cli_Windows_x86_64.zip
```

Supported environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `NEKO_VERSION` | newest stable CLI release | Optional CLI version, either `3.0.4` or `v3.0.4` |
| `NEKO_INSTALL_DIR` | `/usr/local/bin` | Destination directory for the installed `neko` binary |
| `NEKO_REPOSITORY` | `nekoman-hq/neko-cli` | GitHub repository to query |
| `GITHUB_TOKEN` | unset | Optional token for private forks or higher API limits |

Examples:

```bash
NEKO_VERSION=v3.0.4 ./install.sh
NEKO_INSTALL_DIR="$HOME/.local/bin" ./install.sh
NEKO_REPOSITORY=owner/repo ./install.sh
```

Plugin installation is separate:

```bash
neko plugin available
neko plugin install release
```

Plugin discovery, install, and update use the published `plugin-index.json` registry asset from the `plugin-registry` GitHub Release. CLI install, version, and update checks intentionally do not use plugin release tags or plugin registry metadata when selecting the CLI release or archive.

For temp-safe plugin smoke checks, override the plugin directory:

```bash
NEKO_PLUGIN_DIR=/private/tmp/neko-plugin-smoke neko plugin available
```
