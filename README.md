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
- 📊 **kubectl-Style Output** - Consistent table, JSON, and text output formats across all plugins
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

Download the latest release from the [releases page](https://github.com/nekoman-hq/neko-cli/releases).

```bash
# Using the install script (requires GITHUB_TOKEN for private repos)
./install.sh
```

### Requirements

- **Go 1.24+** (for building from source)
- **Git** (for repository operations)

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

# Uninstall a plugin
neko plugin uninstall <plugin-name>
```

### Using Plugins

Once installed, plugin commands are available as subcommands:

```bash
# Format: neko <plugin> <command> [flags]
neko release patch
neko release history --output json
```

### Global Flags

All plugins inherit these global flags:

| Flag | Shorthand | Description                                          |
|------|-----------|------------------------------------------------------|
| `--help` | `-h` | Show help for any command                            |
| `--verbose` | `-v` | Enable verbose output                                |
| `--output` | `-o` | Output format: `table`, `json`, `wide (in progress)` |
| `--describe` | | Include logs and metadata in output                  |

### Other Built-in Commands

```bash
# Show CLI version
neko version

# Show help
neko --help
neko plugin --help
neko <plugin> <command> --help
```

---

## 📖 Output Formats

Neko CLI provides consistent output formatting across all plugins:

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

### Verbose with Logs
```bash
neko <plugin> <command> --describe -v
```

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
