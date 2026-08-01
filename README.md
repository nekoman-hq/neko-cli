<div align="center">
<h1>Neko CLI</h1>
<img alt="Neko-Cli Logo" height="500" src="neko-cli-logo.png" width="500"/>

<br />
</div>

[![GitHub release](https://img.shields.io/github/v/release/nekoman-hq/neko-cli?style=flat-square)](https://github.com/nekoman-hq/neko-cli/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/nekoman-hq/neko-cli)](https://goreportcard.com/report/github.com/nekoman-hq/neko-cli)
[![Contributors](https://img.shields.io/github/contributors/nekoman-hq/neko-cli?style=flat-square)](https://github.com/nekoman-hq/neko-cli/graphs/contributors)

---

**Neko CLI** is a lightweight, plugin-based command-line framework. It acts as a dispatcher that loads and executes standalone plugin executables, providing a unified interface with consistent output formatting.

## ✨ Features

- 🔌 **Plugin Architecture** - Extensible via standalone plugin executables
- 📊 **kubectl-Style Output** - Core rendering in table, JSON, wide, and explicit GitHub command-file modes
- 🔄 **Unified Interface** - One CLI to rule all your plugins
- 📦 **Simple Plugin Management** - Drop-in plugin installation
- 🛠️ **Developer Friendly** - Easy to create and distribute custom plugins

---

## 📦 Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/nekoman-hq/neko-cli.git
cd neko-cli

# Build the CLI
make build

# Install globally
make install
```

### From Release

Download the latest CLI release from the [releases page](https://github.com/nekoman-hq/neko-cli/releases), or use the install script:

```bash
curl -fsSL https://raw.githubusercontent.com/nekoman-hq/neko-cli/main/install.sh | bash
```

The script installs the main CLI only. It reads GitHub Releases directly, ignores plugin releases such as `plugin-release/vX.Y.Z`, and selects the newest stable CLI tag matching `vX.Y.Z`. For an ordinary user the default destination is `$HOME/.local/bin`; when intentionally run as root it is `/usr/local/bin`. An explicit `NEKO_INSTALL_DIR` always wins, and the script never invokes `sudo`. If the selected directory is not on `PATH`, the script prints the required guidance.

The built-in `neko version` and `neko update` commands use the same CLI-aware release rule. `neko update` verifies the selected archive against the release checksum manifest before validating its contents and atomically replacing an unmanaged, writable installation. It refuses package-manager-owned or privileged installations before downloading the archive. None of these paths read local release config or state files.

```bash
# Install a specific CLI release
NEKO_VERSION=v3.0.4 ./install.sh

# Install to a custom directory
NEKO_INSTALL_DIR="$HOME/.local/bin" ./install.sh

# Install from a fork or mirror
NEKO_REPOSITORY=owner/repo ./install.sh
```

Plugin installation is separate and uses the published `plugin-index.json` registry:

```bash
neko plugin available
neko plugin install release
```

See [Installation](docs/installation.md) for install script details and the
[complete CLI reference](docs/cli-reference.md) for every public command and
flag.

### Updating the CLI

```bash
neko update
neko update --dry-run
neko update --force
```

`--force` only reinstalls the selected latest version when it is already installed. It does not permit downgrades and cannot bypass permissions, package-manager ownership, platform support, checksum verification, archive validation, or atomic replacement requirements. A dry-run selects and reports the action without downloading an archive or changing files.

### Requirements

- **Go 1.24+** (for building from source)
- **Git** (for repository operations)
- **curl, jq, tar** (for the release install script)

---

## 🔌 Plugin System

### How It Works

Neko CLI uses a **plugin-based architecture** where the core CLI acts as a dispatcher that communicates with standalone plugin executables via JSON over stdin/stdout.

```
┌─────────────────┐     JSON stdin      ┌─────────────────┐
│   Neko CLI      │ ──────────────────► │     Plugin      │
│   (Dispatcher)  │                     │   (Executable)  │
│                 │ ◄────────────────── │                 │
└─────────────────┘     JSON stdout     └─────────────────┘
                        Logs → stderr
```

### Plugin Directory

Plugins are installed in `~/.neko/plugins/{plugin-name}/` and include:
- The plugin executable (e.g., `plugin-release`)
- A `manifest.json` describing available commands and flags

---

## 📚 Available Plugins

| Plugin | Description | Documentation |
|--------|-------------|---------------|
| **release** | Release management with semantic versioning | [📖 Docs](docs/plugins/release.md) |
| **ui** | UI component helper plugin | [📖 Docs](docs/plugins/ui.md) |

> Each plugin has its own documentation with detailed command references. Use `neko plugin available` to see all plugins.

---

## 🛠️ Core Commands

### Managing Plugins

Use the built-in `plugin` command to manage plugins:

```bash
# List installed plugins
neko plugin list

# Show available plugins from registry
neko plugin available

# Install a plugin
neko plugin install <plugin-name>

# Update installed plugins
neko plugin update <plugin-name>
neko plugin update --all

# Uninstall a plugin
neko plugin uninstall <plugin-name>
```

Plugin available/install/update discovery uses the published `plugin-index.json` registry as its source of truth, not the repository's latest release. Local installed versions are read from each installed `manifest.json`; remote plugin versions, release tags, and asset names come from the `plugin-registry` release asset.

### Using Plugins

Once installed, plugin commands are available as subcommands:

```bash
# Format: neko <plugin> <command> [flags]
neko release patch
neko release history --output json

# Show installed plugin overview from its manifest
neko release
neko release --help

# Show command-specific manifest help, including flags
neko release patch --help
```

### Release V2 Dogfood

This repository releases itself with V2 multi-unit release state:

- `cli` uses global tags like `v3.0.0`.
- `plugin-release` uses tags like `plugin-release/v4.0.0`.
- `plugin-ui` uses tags like `plugin-ui/v1.0.0`.
- All releaseable versions live in `.neko/release.state.json`; the old `.plugin.release.neko.json` map has been removed.
- `neko release init` creates V2 `.neko/release.config.json` and `.neko/release.state.json` files for one unit. `neko release unit-add` appends one normal or plugin unit to existing V2 config/state. `--kind release` is the default for normal services, apps, CLIs, SDKs, libraries, and backend modules; normal units need no plugin metadata or plugin registry. Use `--kind plugin` and the plugin metadata flags only for Neko CLI plugins distributed through `neko plugin install` or `neko plugin update`. These commands do not generate workflow templates, GoReleaser configs, plugin manifests, or source directories, and `init` no longer creates `.release.neko.json`. Existing V1 projects should use `neko release migrate`.
- Plugin units declare `kind: "plugin"` metadata in `.neko/release.config.json`, including public Neko CLI plugin name, manifest path, asset prefix, and binary name. That metadata feeds `plugin-index.json`; normal release units are not included in the plugin index. See [Release V2 Examples](docs/release/examples.md#normal-release-units-vs-neko-cli-plugin-units) for the full boundary.
- Neko CLI owns version/materialization/state, release commit, tag, push, and workflow dispatch.
- GitHub Actions workflows own build, GitHub Release creation, and asset publishing.
- Dry-run release planning needs no token and writes nothing.
- CLI version, update, and install checks use only stable CLI tags matching `vX.Y.Z`; plugin releases and `plugin-registry` are ignored for CLI updates.
- Runtime plugin discovery, install, and update use `plugin-index.json` as the registry source of truth. The index is published as the `plugin-index.json` asset on the mutable `plugin-registry` GitHub Release after successful plugin releases; `/releases/latest` and release-prefix fallback discovery are not used for plugin discovery.
- Use global `--describe` for structured inspection details and global `--verbose` for execution/debug logs where the command owns useful phases. Static and complete read-only queries may intentionally treat either flag as a no-op. They are independent and may be combined.

The complete Release V1/V2 command, flag, output, source, I/O, and exit matrix
lives in the canonical [Release CLI Reference](docs/release/cli-reference.md).
Existing V1 repositories use the [V1-to-V2 migration
guide](docs/release/migration-v1-to-v2.md); new V2 integrations should follow
the [GitHub Actions golden path](docs/release/github-actions-golden-path.md).
Copy-ready Release V2 and plugin registry examples live in [Release V2
Examples](docs/release/examples.md). The product boundary for release-ready
GitHub Actions bootstrap lives in [Release V2 Bootstrap Product
Boundary](docs/release/bootstrap-product-boundary.md).
Current implementation architecture lives in the [Release Plugin architecture
reference](plugin/release/docs/architecture/current-state.md); completed
roadmaps and reviews are preserved in the [Release documentation
history](plugin/release/docs/history/README.md).

### Global Flags

Plugin command help separates manifest-owned `Command flags` from inherited
`Global plugin-response flags`. Core owns the inherited flags and does not add
them to a plugin command's local flag map:

| Flag | Shorthand | Description |
|------|-----------|-------------|
| `--help` | `-h` | Show help for any command |
| `--describe` | | Include structured details and response metadata |
| `--verbose` | `-v` | Include execution and debug logs in plugin output |
| `--output` | | Output format: `table`, `json`, `wide`, or `github` |
| `--github-output-file` | | Explicit GitHub Actions command-file destination for `--output github` |

`--verbose` is the only presentation-related global value sent to a plugin,
through `Request.Context.Verbose`, because it controls plugin-side log
production. `--describe`, `--output`, and `--github-output-file` stay in Core.
The Release Plugin uses the distinct manifest-local
`plugin-index --output-file <path>` option for file persistence. Core
`--output` therefore remains the response-format selector for this command as
well. The former `plugin-index --output <path>` spelling no longer writes a
file; non-format values are rejected by Core.

### Other Built-in Commands

```bash
# Show CLI version
neko version

# Show help
neko --help
neko plugin --help
neko <plugin>
neko <plugin> --help
neko <plugin> <command> --help
```

---

## 📖 Output Formats

Core supports `table`, `json`, `wide`, and explicit `github` response modes.
Successful GitHub command-file output is response-specific; it is not a
universal plugin format. See the [complete CLI
reference](docs/cli-reference.md#output-and-process-exit)
for the exact boundaries.

### Table (Default)
```bash
neko <plugin> <command>
```
```
COLUMN1   COLUMN2   COLUMN3
value1    value2    value3
```

### JSON
```bash
neko <plugin> <command> --output json
```
```json
{
  "status": "success",
  "metadata": {
    "plugin": "plugin-name",
    "command": "command-name",
    "timestamp": "2026-02-04T12:00:00Z"
  },
  "data": {
    "items": ["..."]
  }
}
```

### Structured Details and Logs
```bash
neko <plugin> <command> --describe
neko <plugin> <command> --verbose
neko <plugin> <command> --describe --verbose
```

`--describe` changes human presentation only. It does not enable verbose logs,
remote access, or a different JSON schema. `--verbose` adds captured execution
logs without revealing describe-only structured sections.

---

## 🏗️ Development



### Project Structure

```
neko-cli/
├── cmd/                    # CLI commands (root, plugin loading)
├── pkg/                    # Shared packages
│   ├── dispatcher/         # Plugin execution & communication
│   ├── plugin/             # Plugin types (Request, Response, Manifest)
│   ├── renderer/           # kubectl-style output rendering
│   ├── log/                # Logging utilities
│   └── errors/             # Error handling
├── plugin/                 # Official plugins
│   └── release/            # Release management plugin
└── docs/
    └── plugins/            # Plugin documentation
```

### Creating a Plugin

Plugins are standalone executables that:
1. Read a JSON `Request` from stdin
2. Execute the command logic
3. Write a JSON `Response` to stdout
4. Write logs to stderr

See [Plugin Development Guide](docs/plugin-development.md) for details.

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

---

## 📄 License

This project is licensed under our own License - see the [LICENSE](LICENSE) file for details.

---

## 👤 Author

**Benjamin Senekowitsch**
- Email: senekowitsch@nekoman.at
- GitHub: [@nekoman-hq](https://github.com/nekoman-hq)

---

<div align="center">
Made by Nekoman
</div>
