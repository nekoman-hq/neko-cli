# Installation

## Release Install Script

`install.sh` installs the main `neko` CLI from GitHub Releases. It does not install plugins and does not read repository-local release files such as legacy V1 config or V2 state/config.

By default, the script queries the configured repository's releases, filters for stable CLI tags matching `vX.Y.Z`, ignores plugin releases and the `plugin-registry` release, sorts the CLI tags as semantic versions, and installs the newest one. The Go implementation used by `neko version` and `neko update` follows the same multi-unit rule for CLI release checks.

For an ordinary user, the default destination is `$HOME/.local/bin`. The script creates that directory when necessary and prints PATH guidance when it is not already reachable. When the script is intentionally run as root, the default remains `/usr/local/bin`. `NEKO_INSTALL_DIR` always preserves an explicitly selected destination, including paths containing spaces. The script rejects an empty or non-writable destination before any release request and never invokes `sudo` or changes ownership of a system directory.

```bash
./install.sh
```

The script selects CLI archive assets by platform from GoReleaser names such as:

```text
neko-cli_Darwin_arm64.tar.gz
neko-cli_Linux_x86_64.tar.gz
neko-cli_Windows_x86_64.zip
```

The supported matrix is Darwin `amd64`/`arm64`, Linux
`386`/`amd64`/`arm64`, and Windows `386`/`amd64`/`arm64`. Darwin/i386 is not a
supported Go target and the installer rejects it instead of requesting the
nonexistent `neko-cli_Darwin_i386.tar.gz` asset. `amd64` is encoded as
`x86_64`; supported `386` targets are encoded as `i386`.

Supported environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `NEKO_VERSION` | newest stable CLI release | Optional CLI version, either `3.0.4` or `v3.0.4` |
| `NEKO_INSTALL_DIR` | `$HOME/.local/bin` for ordinary users; `/usr/local/bin` for root | Destination directory for the installed `neko` binary |
| `NEKO_REPOSITORY` | `nekoman-hq/neko-cli` | GitHub repository to query |
| `GITHUB_TOKEN` | unset | Optional token for private forks or higher API limits |

Examples:

```bash
NEKO_VERSION=v3.0.4 ./install.sh
NEKO_INSTALL_DIR="$HOME/.local/bin" ./install.sh
NEKO_REPOSITORY=owner/repo ./install.sh
```

If `$HOME/.local/bin` is not already on `PATH`, add it using the mechanism for your shell, for example:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

## Self-update policy

Once installed, check or apply a CLI update with:

```bash
neko update
neko update --dry-run
neko update --force
```

The updater first selects the newest stable `vX.Y.Z` CLI release and classifies the running installation. The action rules are:

| Installed versus selected | Without `--force` | With `--force` |
| --- | --- | --- |
| older | upgrade | the same upgrade |
| equal | successful no-op | reinstall the selected version |
| newer | successful newer-installed no-op | refuse the downgrade |

`--force` is only a same-version reinstall switch. It does not bypass installation permissions, package-manager ownership, platform compatibility, asset selection, checksum verification, archive validation, or atomic replacement.

`--dry-run` performs release selection and static installation inspection, then reports the planned action and any statically detectable capability issue. It does not reserve a sibling file, download an archive, or change the filesystem.

### Installation ownership

- A user-owned unmanaged installation is updated without `sudo` when its canonical target directory permits sibling creation and atomic replacement.
- A direct unmanaged privileged installation is refused before archive download. The diagnostic reports the canonical target, owner when available, parent directory, missing create/rename/remove capability, and recommends reinstalling into `$HOME/.local/bin`. It may also show an exactly escaped privileged rerun as a secondary deliberate choice; the updater never executes it.
- A positively identified Homebrew Cellar installation is refused before archive download and directs the user to `brew upgrade` or `brew reinstall`. A generic path substring alone is not treated as manager ownership.
- For an unmanaged symlink, the updater preserves the symlink and applies all checks and replacement to its canonical target. Missing targets, loops, non-regular targets, manager-owned targets, and non-writable target parents are refused before archive download.

Migrating a system installation to the recommended user-owned location is explicit:

```bash
NEKO_INSTALL_DIR="$HOME/.local/bin" ./install.sh
export PATH="$HOME/.local/bin:$PATH"
```

The updater and installer never invoke `sudo`, prompt for a password, `chown` `/usr/local/bin`, or make a system directory user-owned.

### Integrity and replacement

Self-update is supported for macOS and Linux on `amd64` and `arm64`. Other combinations, including Windows, are rejected before asset download. This is intentionally narrower than the release install script matrix documented above.

For a supported platform, `neko update` requires exactly one compatible `.tar.gz` archive and exactly one authoritative GoReleaser checksum asset named `neko-cli_<version>_checksums.txt` (with legacy `checksums.txt` accepted when it is the sole match). The checksum manifest must contain exactly one valid SHA-256 entry for the archive. Missing, malformed, duplicate, or mismatched data stops the update before executable bytes are written.

The archive validator rejects absolute or traversing paths, symlinks, hardlinks, devices, special entries, malformed tar/gzip data, oversized binaries, and archives with zero or multiple supported binaries. It extracts no arbitrary archive paths.

After verification, the updater writes a unique hidden sibling beside the canonical target, flushes and closes it, preserves the target's ordinary permission bits, strips special mode bits, fsyncs the sibling, atomically renames it over the target, and fsyncs the parent directory where supported. Every pre-commit failure leaves the original target byte-identical. A post-rename directory-sync failure is reported as committed but not durability-confirmed; the updater does not copy back or pretend the old binary was restored.

Replacement naturally gives the new file the updater process identity. Ownership is not preserved through `chown`, and quarantine/provenance attributes, arbitrary ACLs, and custom extended attributes are not copied. Use the package manager for managed installations if those properties are part of its contract.

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
