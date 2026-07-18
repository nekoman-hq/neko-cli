# Plugin Development Guide

> ⚠️ **Work in Progress** - This documentation is under active development. Some sections may be incomplete or subject to change.

This guide explains how to create plugins for Neko CLI. Plugins are standalone executables that communicate with the CLI via JSON over stdin/stdout.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Plugin Structure](#plugin-structure)
- [Getting Started](#getting-started)
- [The Manifest File](#the-manifest-file)
- [Request & Response Types](#request--response-types)
- [Logging](#logging)
- [Error Handling](#error-handling)
- [Best Practices](#best-practices)
- [Building & Installing](#building--installing)
- [Testing Your Plugin](#testing-your-plugin)
- [Example: Complete Plugin](#example-complete-plugin)

---

## Architecture Overview

Neko CLI uses a **dispatcher architecture** where the core CLI acts as a router that executes standalone plugin executables and renders their output.

```
┌─────────────────┐     JSON stdin      ┌─────────────────┐
│   Neko CLI      │ ──────────────────► │     Plugin      │
│   (Dispatcher)  │                     │   (Executable)  │
│                 │ ◄────────────────── │                 │
└─────────────────┘     JSON stdout     └─────────────────┘
                        Logs → stderr
```

### Communication Flow

1. User runs `neko <plugin> <command> [flags]`
2. CLI reads the plugin's `manifest.json` to validate flags
3. CLI sends a `Request` JSON to the plugin via stdin
4. Plugin executes the command logic
5. Plugin writes logs to stderr (captured by CLI)
6. Plugin writes a `Response` JSON to stdout
7. CLI renders the response in the requested format (table, json, text)

---

## Plugin Structure

Plugins are installed in `~/.neko/plugins/{plugin-name}/` and must contain:

```
~/.neko/plugins/my-plugin/
├── plugin-my-plugin    # Executable (must be named plugin-{name})
└── manifest.json       # Plugin metadata & command definitions
```

### Development Structure

When developing a plugin within the neko-cli repository:

```
plugin/
└── my-plugin/
    ├── main.go           # Plugin entry point
    ├── Makefile          # Build & install scripts
    ├── manifest.json     # Plugin metadata
    └── pkg/              # Plugin-specific packages
        ├── command1/
        │   └── handler.go
        └── command2/
            └── handler.go
```

---

## Getting Started

### 1. Create Your Plugin Directory

```bash
mkdir -p plugin/my-plugin/pkg
cd plugin/my-plugin
```

### 2. Create the Manifest

Create `manifest.json`:

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "My awesome plugin",
  "author": "your-name",
  "commands": [
    {
      "name": "hello",
      "description": "Say hello",
      "outputs": ["text", "json"],
      "flags": [
        {
          "name": "name",
          "type": "string",
          "required": false,
          "default": "World",
          "description": "Name to greet"
        }
      ]
    }
  ],
  "renderer_types": ["table", "json", "text"]
}
```

### 3. Create the Main Entry Point

Create `main.go`:

```sh
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "time"

    "github.com/nekoman-hq/neko-cli/pkg/errors"
    "github.com/nekoman-hq/neko-cli/pkg/log"
    "github.com/nekoman-hq/neko-cli/pkg/plugin"
)

const (
    pluginName    = "my-plugin"
    pluginVersion = "1.0.0"
)

func main() {
    // Set plugin info for error responses
    errors.PluginName = pluginName
    errors.PluginVersion = pluginVersion

    // Read request from stdin
    var req plugin.Request
    if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
        errors.WriteError("PARSE_ERROR", fmt.Sprintf("failed to parse request: %v", err))
    }

    // Set verbose mode from request context
    log.Verbose = req.Context.Verbose

    var resp *plugin.Response
    var err error

    // Route to command handlers
    switch req.Command {
    case "hello":
        resp, err = handleHello(req)
    default:
        resp, err = nil, fmt.Errorf("unknown command: %s", req.Command)
    }

    if err != nil {
        errors.WriteError("EXECUTION_ERROR", err.Error())
    }

    // Write response to stdout
    if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
        errors.WriteError("RESPONSE_ERROR", fmt.Sprintf("failed to encode response: %v", err))
    }
}

func handleHello(req plugin.Request) (*plugin.Response, error) {
    log.PluginPrint(log.Exec, "Starting hello command")

    // Extract flag value
    name := "World"
    if n, ok := req.Flags["name"].(string); ok && n != "" {
        name = n
    }

    log.PluginV(log.Exec, "Greeting: %s", name)

    return &plugin.Response{
        Status: "success",
        Metadata: plugin.ResponseMetadata{
            Plugin:    pluginName,
            Version:   pluginVersion,
            Command:   "hello",
            Timestamp: time.Now(),
        },
        Data: map[string]any{
            "message": fmt.Sprintf("Hello, %s!", name),
        },
        RendererHint: "text",
    }, nil
}
```

---

## Plugin Versioning

When Neko CLI releases itself, it needs to embed the versions of bundled plugins into the binary. The release plugin handles this through automatic environment variable injection.

### How It Works

Nekocli keeps releaseable versions in `.neko/release.state.json`. The release workflows pass the selected unit version to GoReleaser as environment variables so the CLI binary can embed the bundled plugin versions.

**Flow:**
```
.neko/release.state.json → release workflow → Environment Variables → GoReleaser → Binary
```

### V2 State

Plugin units are stored beside the CLI unit:

```json
{
  "schemaVersion": 2,
  "units": {
    "cli": {
      "version": "2.2.4"
    },
    "plugin-release": {
      "version": "3.0.0"
    },
    "plugin-ui": {
      "version": "1.0.0"
    }
  }
}
```

### Environment Variable Mapping

Each plugin unit can be passed to GoReleaser as an environment variable:

| Unit | Environment Variable |
|--------------|---------------------|
| `plugin-release` | `PLUGIN_RELEASE_VERSION=3.0.0` |
| `plugin-ui` | `PLUGIN_UI_VERSION=1.0.0` |

**Pattern:** `PLUGIN_{UPPERCASE_UNIT_NAME}_VERSION={version}`, with dashes converted to underscores and the `plugin-` prefix omitted.

### Using in GoReleaser

Access these variables in your `.goreleaser.yml`:

```yaml
builds:
  - ldflags:
      - -X main.ReleasePluginVersion={{ .Env.PLUGIN_RELEASE_VERSION }}
      - -X main.UIPluginVersion={{ .Env.PLUGIN_UI_VERSION }}
```

### Behavior

- **State update:** Version bumps are committed to `.neko/release.state.json`
- **Manifest materialization:** Plugin releases update only their own `manifest.json`
- **Workflow input:** GitHub Actions passes the released version to GoReleaser
- **Registry index:** `neko release plugin-index` generates the public `plugin-index.json` from V2 plugin units, state, and manifests. Runtime plugin discovery, install, and update use that index as their source of truth. Plugin release workflows publish it as the `plugin-index.json` asset on the mutable `plugin-registry` GitHub Release after successful plugin releases; the index is not committed as source. Release-prefix fallback discovery has been removed, so new plugins become available through the published index rather than registry Go-code mappings.

To make a new plugin eligible for the generated index, create the first V2 plugin unit with `neko release init --kind plugin` or append another plugin unit with `neko release unit-add --kind plugin`. Plugin units require a `plugin-` unit id, `tagPrefix` set to `<unit-id>/v`, `plugin.assetPrefix` matching the unit id, and plugin metadata for `plugin.name`, `plugin.manifest`, `plugin.assetPrefix`, and `plugin.binaryName`. Keep the manifest name/version synchronized with that state. `unit-add` updates only V2 config/state; workflow template generation, plugin manifest generation, source directory generation, and executor scaffolding are not implemented yet. See [Release V2 Examples](release/examples.md) for copy-ready plugin unit and plugin registry examples.

### Self-Bootstrapping

This creates an interesting architectural pattern:

1. Neko CLI uses the **release plugin** to release itself
2. Bundled plugins need their versions embedded in Neko CLI
3. V2 release state and workflow-provided environment variables bridge that gap

It's metadata injection that allows plugins to declare their versions in the host binary.

---

## The Manifest File

The `manifest.json` file describes your plugin and its commands. The CLI uses this to:
- Register subcommands automatically
- Parse and validate flags
- Render `neko <plugin>`, `neko <plugin> --help`, and `neko <plugin> <command> --help` without starting the plugin binary
- Determine available output formats

### Manifest Schema

```json
{
  "name": "plugin-name",
  "version": "1.0.0",
  "description": "What the plugin does",
  "author": "author-name",
  "commands": ["..."],
  "renderer_types": ["table", "json", "text"]
}
```

### Command Definition

```json
{
  "name": "command-name",
  "description": "What the command does",
  "outputs": ["table", "json"],
  "flags": ["..."]
}
```

### Flag Definition

```sh
{
  "name": "flag-name",
  "type": "string",       // "string", "bool", or "int"
  "required": true,
  "default": "value",     // Optional default value
  "description": "What the flag does"
}
```

**Supported Flag Types:**
- `string` - Text values
- `bool` - Boolean flags (true/false)
- `int` - Integer values

---

## Request & Response Types

### Request

The plugin receives a `Request` via stdin:

```sh
type Request struct {
    Command string         `json:"command"`  // Command name to execute
    Args    []string       `json:"args"`     // Positional arguments
    Flags   map[string]any `json:"flags"`    // Flag values
    Context Context        `json:"context"`  // Execution context
}

type Context struct {
    WorkingDir string `json:"working_dir"` // Current working directory
    User       string `json:"user"`        // Current user
    Verbose    bool   `json:"verbose"`     // Verbose mode enabled
}
```

### Response

The plugin returns a `Response` via stdout:

```sh
type Response struct {
    Status       string           `json:"status"`        // "success" or "error"
    Metadata     ResponseMetadata `json:"metadata"`
    Data         map[string]any   `json:"data,omitempty"`
    Error        *ResponseError   `json:"error,omitempty"`
    RendererHint string           `json:"renderer_hint,omitempty"` // "table", "json", "text"
    Logs         []LogEntry       `json:"logs,omitempty"`          // Populated by dispatcher
    HumanTable   *HumanTable      `json:"human_table,omitempty"`   // Optional human presentation metadata
}

type HumanTable struct {
    Columns []HumanColumn `json:"columns"`
}

type HumanColumn struct {
    Key       string `json:"key"`
    Label     string `json:"label"`
    Essential bool   `json:"essential,omitempty"`
}

type ResponseMetadata struct {
    Timestamp time.Time `json:"timestamp"`
    Plugin    string    `json:"plugin"`
    Version   string    `json:"version"`
    Command   string    `json:"command"`
}

type ResponseError struct {
    Code    string         `json:"code"`
    Message string         `json:"message"`
    Details map[string]any `json:"details,omitempty"`
}
```

### Renderer Hints

The `RendererHint` tells the CLI how to render the response:

| Hint    | Description         | Data Format                               |
|---------|---------------------|-------------------------------------------|
| `table` | kubectl-style table | `{"items": [{"col1": "val1", ...}, ...]}` |
| `json`  | Raw JSON output     | Any structure                             |
| `text`  | Key-value text      | `{"key": "value", ...}`                   |

#### Table Rendering

For table output, `Data` must contain an `items` key with a slice of maps:

```sh
Data: map[string]any{
    "items": []map[string]any{
        {"version": "1.0.0", "date": "2026-02-04", "commits": 10},
        {"version": "0.9.0", "date": "2026-01-15", "commits": 25},
    },
}
```

Commands may opt in to responsive human output by attaching `HumanTable` to
the response. Column order is declaration order; non-essential columns use the
same order as their admission priority. Core measures the actual output
writer, ANSI-free Unicode display cells, and the declared values. It renders a
table when all essential columns fit, adds optional columns while they fit,
and otherwise uses vertical records. An unavailable width, including a pipe or
redirected file, deterministically uses vertical records. `--output wide`
permits every declared summary column for opted-in responses, falling back to
vertical records when the complete declaration does not fit.

This capability is presentation-only. Keep complete command data in `Data`.
The `human_table` declaration crosses the plugin transport so Core can render
it, but Core excludes it from public `--output json` and raw JSON. Commands
without `HumanTable` retain the legacy inferred table and existing `wide`
behavior. Plugins own field meaning, labels, order, and essential/optional
classification; Core owns only layout mechanics.

---

## Logging

### Critical Rule

**ALWAYS use `log.PluginPrint()` and `log.PluginV()` in plugin code, NEVER `log.Print()`**

```sh
// ✅ CORRECT - writes to stderr (captured by dispatcher)
log.PluginPrint(log.Init, "Starting initialization")
log.PluginV(log.Config, "Verbose message: %s", value)

// ❌ WRONG - writes to stdout, CORRUPTS JSON response
log.Print(log.Init, "This breaks the plugin!")
```

### Log Categories

Available log categories with their colors:

```sh
log.Init      // Yellow   - Initialization messages
log.Config    // Cyan     - Configuration messages
log.Preflight // Yellow   - Pre-flight check messages
log.Guard     // Blue     - Guard/validation messages
log.Exec      // Green    - Execution messages
```

### Verbose Logging

Use `log.PluginV()` for verbose-only messages. These only appear when the user runs with `-v` or `--verbose`.

```sh
log.PluginV(log.Exec, "Detailed info: %v", someValue)
```

---

## Error Handling

### Returning Errors in Response

For recoverable errors (validation failures, missing config, etc.), return an error response:

```sh
return &plugin.Response{
    Status: "error",
    Metadata: plugin.ResponseMetadata{
        Plugin:    pluginName,
        Version:   pluginVersion,
        Command:   "my-command",
        Timestamp: time.Now(),
    },
    Error: &plugin.ResponseError{
        Code:    "CONFIG_NOT_FOUND",
        Message: "Configuration file not found",
        Details: map[string]any{
            "expected_file": ".my-plugin.neko.json",
            "hint":          "Run 'neko my-plugin init' first",
        },
    },
}, nil
```

### Fatal Errors

For unrecoverable errors (parse failures, etc.), use the errors helper:

```sh
import "github.com/nekoman-hq/neko-cli/pkg/errors"

// Simple error
errors.WriteError("FATAL_ERROR", "Something went terribly wrong")

// Error with details
errors.WriteErrorWithDetails("PARSE_ERROR", "Invalid input", map[string]any{
    "input": rawInput,
    "expected": "valid JSON",
})
```

These functions write an error response and exit immediately.

---

## Best Practices

### 1. No Interactive Prompts

**Plugins cannot use interactive prompts** because stdin is used for the JSON request.

```sh
// ❌ WRONG - survey/stdin doesn't work in plugins
survey.AskOne(&survey.Select{...}, &answer)

// ✅ CORRECT - use flags from request
unitID := req.Flags["unit"].(string)
```

### 2. Config File Naming

Plugin config files should follow the pattern: `.{plugin-name}.neko.json`

```
.deploy.neko.json     # Deploy plugin config
.ui.neko.json         # UI plugin config
.my-plugin.neko.json  # Your plugin config
```

The release plugin is the exception: new `neko release init` writes V2 repository files at `.neko/release.config.json` and `.neko/release.state.json`. It can initialize one normal unit or one plugin unit with `--kind plugin` and the plugin metadata flags. `neko release unit-add` appends more units to existing V2 config/state. It no longer creates `.release.neko.json`; existing V1 release projects should use `neko release migrate`.

### 3. Extract Flags Safely

Always handle type assertions safely:

```sh
func getFlagString(flags map[string]any, name string) string {
    if v, ok := flags[name].(string); ok {
        return v
    }
    return ""
}

func getFlagBool(flags map[string]any, name string) bool {
    if v, ok := flags[name].(bool); ok {
        return v
    }
    return false
}

func getFlagInt(flags map[string]any, name string) int {
    if v, ok := flags[name].(float64); ok { // JSON numbers are float64
        return int(v)
    }
    return 0
}
```

### 4. Use Metadata Constants

Define plugin metadata in a dedicated package:

```sh
// pkg/metadata/metadata.go
package metadata

var (
    PluginName     = "my-plugin"
    Version        = "1.0.0"        // Overridden by ldflags
    GitCommit      = "unknown"      // Overridden by ldflags
    BuildDate      = "unknown"      // Overridden by ldflags
    ConfigFileName = ".my-plugin.neko.json"
)
```

---

## Building & Installing

### Makefile Template

Create a `Makefile` for your plugin:

```makefile
PLUGIN_NAME := my-plugin
BINARY := plugin-$(PLUGIN_NAME)
INSTALL_DIR := $(HOME)/.neko/plugins/$(PLUGIN_NAME)

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X github.com/nekoman-hq/neko-cli/plugin/$(PLUGIN_NAME)/pkg/metadata.Version=$(VERSION) \
           -X github.com/nekoman-hq/neko-cli/plugin/$(PLUGIN_NAME)/pkg/metadata.GitCommit=$(COMMIT) \
           -X github.com/nekoman-hq/neko-cli/plugin/$(PLUGIN_NAME)/pkg/metadata.BuildDate=$(DATE)

.PHONY: build install clean update-manifest

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) main.go

update-manifest:
	@jq '.version = "$(VERSION)"' manifest.json > manifest.json.tmp && mv manifest.json.tmp manifest.json

install: update-manifest build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/
	cp manifest.json $(INSTALL_DIR)/

clean:
	rm -f $(BINARY)
	rm -rf $(INSTALL_DIR)
```

### Build Commands

```bash
# Build the plugin
make build

# Install to ~/.neko/plugins/
make install

# Clean build artifacts
make clean
```

---

## Testing Your Plugin

### Direct Testing

Test your plugin directly by piping JSON to it:

```bash
# Test a command
echo '{"command":"hello","args":[],"flags":{"name":"Developer"},"context":{"verbose":true}}' | ./plugin-my-plugin

# Pretty print the output
echo '{"command":"hello","args":[],"flags":{},"context":{}}' | ./plugin-my-plugin | jq .
```

### Testing via CLI

After installing, test through the CLI:

```bash
# Run the command
neko my-plugin hello --name Developer

# With verbose output
neko my-plugin hello -v --describe

# JSON output
neko my-plugin hello --output json
```

### Debug Tips

1. **Check stderr for logs**: Plugin logs go to stderr
2. **Use verbose mode**: `-v` enables detailed logging
3. **Test manifest**: Ensure your `manifest.json` is valid JSON
4. **Check permissions**: The plugin binary must be executable

---

## Example: Complete Plugin

See the [release plugin](../plugin/release/) for a complete example:

- [`main.go`](../plugin/release/main.go) - Entry point with command routing
- [`manifest.json`](../plugin/release/manifest.json) - Full manifest example
- [`pkg/init/handler.go`](../plugin/release/pkg/init/handler.go) - Command handler example
- [`pkg/history/history.go`](../plugin/release/pkg/history/history.go) - Simple table output example

---

## Questions?

- Check the [main documentation](README.md)
- Look at the [AI Context](ai_context.md) for additional development notes
- Open an issue on [GitHub](https://github.com/nekoman-hq/neko-cli/issues)

---

## License

Neko CLI is licensed under our own License. See [LICENSE](../LICENSE) for details.
