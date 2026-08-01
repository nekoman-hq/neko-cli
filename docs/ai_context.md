# AI Context for Neko CLI

This document provides context for AI assistants working on the Neko CLI project.

## Project Overview

Neko CLI is a **plugin-based command-line tool** for managing software releases. The core CLI dispatches commands to plugins, which execute the logic and return structured JSON responses that get rendered in kubectl-style output.

## Architecture

### Plugin System

```
┌─────────────────┐     JSON stdin      ┌─────────────────┐
│   Neko CLI      │ ──────────────────► │     Plugin      │
│   (Dispatcher)  │                     │   (Executable)  │
│                 │ ◄────────────────── │                 │
└─────────────────┘     JSON stdout     └─────────────────┘
                        Logs → stderr
```

- **Plugins are standalone executables** located in `~/.neko/plugins/{plugin-name}/`
- Each plugin has a `manifest.json` describing commands, flags, and outputs
- Communication happens via **JSON over stdin/stdout**
- **Logs go to stderr** (captured by dispatcher), **JSON response goes to stdout**

### Key Directories

```
neko-cli/
├── cmd/                    # CLI commands (root, plugin loading)
├── pkg/                    # Supported importable contracts and APIs
│   ├── dispatcher/         # Plugin execution & communication
│   ├── plugin/             # Plugin types (Request, Response, Manifest)
│   ├── presentation/       # Plugin-to-Core presentation declarations
│   ├── renderer/           # kubectl-style output rendering
│   ├── log/                # Logging utilities
│   └── errors/             # Error handling
├── internal/               # Private Core implementation packages
│   └── terminal/           # Focused ANSI styling and terminal capability primitives
└── plugin/
    └── release/            # Release management plugin
        ├── main.go         # Plugin entry point
        ├── manifest.json   # Plugin metadata & command definitions
        └── pkg/            # Plugin-specific packages
            ├── init/       # Init command handler
            ├── config/     # V2 config/state and legacy V1 compatibility
            ├── release/    # Release logic & tool registry
            ├── git/        # Git operations
            └── history/    # Release history
```

## Critical Rules

### 1. Plugin Logging

**ALWAYS use `log.PluginPrint()` and `log.PluginV()` in plugin code, NEVER `log.Print()`**

```go
// ✅ CORRECT - writes to stderr
log.PluginPrint(log.Init, "Starting initialization")
log.PluginV(log.Config, "Verbose message: %s", value)

// ❌ WRONG - writes to stdout, corrupts JSON response
log.Print(log.Init, "This breaks the plugin!")
```

### 2. Plugin Response Format

All plugin handlers must return `*plugin.Response`:

```go
return &plugin.Response{
    Status: "success", // or "error"
    Metadata: plugin.ResponseMetadata{
        Plugin:    "release",
        Version:   "1.0.0",
        Command:   "init",
        Timestamp: time.Now(),
    },
    Data: map[string]any{
        "items": []map[string]any{...}, // For table rendering
        // or key-value pairs for text rendering
    },
    RendererHint: "table", // "table", "json", or "text"
}, nil
```

### 3. Table Rendering

For table output, data must have an `items` key with a slice of maps:

```go
Data: map[string]any{
    "items": []map[string]any{
        {"column1": "value1", "column2": "value2"},
        {"column1": "value3", "column2": "value4"},
    },
}
```

Responsive tables are explicitly opt-in through `plugin.Response.PresentationTable`.
Its ordered `presentation.Column` declarations carry a data key, human label, essential
marker, and optional presentation-row `RoleKey`. `presentation.Table` and
`presentation.Properties` may provide neutral titles; `presentation.Property` may declare a
closed semantic `presentation.StyleRole`, emphasis, or a record heading. Core owns
terminal-width detection, ANSI/Unicode display width, optional-column fitting,
vertical fallback, wrapping, bounded presentation-only truncation, and the semantic
style mapping. A plugin owns semantic meaning and priority and must retain the
complete typed result in `Data`. Presentation metadata is transported between
the plugin and Core but is excluded from public JSON and raw JSON. Do not add
domain fields, callbacks, layout modes, or policy to Core.

`presentation.Table.Rows` may provide a presentation-only projection when complete machine
data is not a slice of row maps. `presentation.Table.Details` may reuse one
`presentation.Properties` declaration after a response-level property summary and the
table. `presentation.Table.DescribeOnly` keeps a complete structured section out
of concise human output until global `--describe` is selected. Core then composes
the existing property/table/property renderers. These
fields are optional transport metadata; nil values preserve the established
table path, and neither field enters public JSON or raw JSON. This is a bounded
master/detail presentation capability, not a generic document or layout model.

Ordered property/value responses are responsive as well. Core recognizes the
established `items[property,value]` shape or an explicit
`plugin.Response.PresentationProperties` declaration. With a known writer width it
bounds the label column, preserves value space, uses ANSI- and Unicode-aware
visible-cell measurement, wraps at grapheme-safe word boundaries, aligns
continuation lines below the value column, and bounds the separator. Narrow and
width-unknown output uses deterministic vertical properties. A plugin owns
labels, order, grouping, and any presentation-only `presentation.Property.Value`; Core
must not interpret domain meaning. Presentation metadata remains absent from
public JSON and raw JSON.

Core applies semantic ANSI styles only to interactive terminal human-readable output.
A non-empty `NO_COLOR`, a pipe, redirect, or file disables color; public JSON,
raw JSON, and GitHub output are always ANSI-free. Plugins declare meaning but
must not inspect terminals or emit ANSI themselves. Styling is presentation
only and cannot alter typed data or exit behavior.

### 4. Config File Naming

Plugin config files usually follow the pattern: `.{plugin-name}.neko.json`

- Release plugin: `.neko/release.config.json` and `.neko/release.state.json`
- Future deploy plugin: `.deploy.neko.json`

Release V2 uses `.neko/release.config.json` for static unit architecture and `.neko/release.state.json` for authoritative unit versions. `neko release init` creates the first V2 unit, `neko release unit-add` appends additional normal or plugin units, and `neko release migrate` converts a root legacy `.release.neko.json` repository. Plugin registry discovery uses `plugin-index.json` from the mutable `plugin-registry` GitHub Release, not `/releases/latest`. See [Release V2 Examples](release/examples.md) for copy-ready command flows.

### 5. No Interactive Prompts in Plugins

**Plugins cannot use interactive prompts (survey, stdin reading)** because stdin is used for the JSON request. All user input must come via flags.

```go
// ❌ WRONG - survey doesn't work in plugins
survey.AskOne(&survey.Select{...}, &answer)

// ✅ CORRECT - use flags from request
unitID := getFlagString(req.Flags, "unit")
```

### 6. Manifest Flags

Define flags in `manifest.json` for automatic CLI flag registration:

```json
{
  "name": "init",
  "flags": [
    {"name": "executor", "type": "string", "required": true, "description": "..."},
    {"name": "delivery", "type": "string", "required": true, "description": "..."},
    {"name": "force", "type": "bool", "required": false, "default": false, "description": "..."}
  ]
}
```

Supported types: `string`, `bool`, `int`

## Common Patterns

### Handler Function Pattern

```go
func HandleCommand(req plugin.Request) (*plugin.Response, error) {
    log.PluginPrint(log.Exec, "Starting command")
    
    // Extract flags
    myFlag := getFlagString(req.Flags, "my-flag")
    
    // Do work...
    
    // Return response
    return &plugin.Response{
        Status: "success",
        Metadata: plugin.ResponseMetadata{...},
        Data: map[string]any{...},
    }, nil
}
```

### Error Response Pattern

```go
return &plugin.Response{
    Status: "error",
    Metadata: plugin.ResponseMetadata{...},
    Error: &plugin.ResponseError{
        Code:    "ERROR_CODE",
        Message: "Human readable message",
        Details: map[string]any{"hint": "helpful info"},
    },
}, nil
```

### Release Tool Composition

```go
v1Executors := []release.V1Executor{
    goreleaser.NewV1Executor(),
    jreleaser.NewV1Executor(),
    releaseit.NewV1Executor(),
}

resp, err := release.HandleReleaseWithV1Executors(req, release.Patch, v1Executors...)
```

The legacy `release.Register` / `release.Get` registry remains available only as
a compatibility surface for old embedders. New code should compose explicit
`V1Executor` values and pass them to `HandleReleaseWithV1Executors`.

## Building & Testing

```bash
# Build everything
make all

# Test a plugin directly
echo '{"command":"init-options","args":[],"flags":{},"context":{}}' | ./plugin/release/plugin-release

# Test via CLI
./neko release init --executor=goreleaser --delivery=github-actions --workflow=.github/workflows/release-cli.yml
./neko release init-options
./neko release history --describe
```

## Output Flags

- `--output table` (default) - kubectl-style table
- `--output json` - Raw JSON
- `--output wide` - All declared summary columns for opted-in responsive tables; legacy behavior otherwise
- `--describe` - Include structured details and response metadata in human output
- `-v, --verbose` - Include captured execution and debug logs independently

## Files to Ignore

When refactoring old code to plugin style, these patterns indicate deprecated code:
- Uses `survey.AskOne()` or similar interactive prompts
- Uses `log.Print()` instead of `log.PluginPrint()` in plugin code
- Cobra commands in `plugin/*/pkg/cmd/` (old style, should be handlers in `pkg/{command}/`)

## Current State

The repository-wide public inventory is maintained in the
[CLI command and flag reference](cli-reference.md). The Release Plugin's
20-command manifest, V1/V2 classifications, 66 local flags, output behavior,
and exits are maintained in the
[canonical Release CLI reference](release/cli-reference.md). Core dynamically
loads plugin commands and local flags from manifests and supports `table`,
`json`, `wide`, and explicit GitHub command-file rendering for compatible
structured responses. Do not infer current implementation status from the
older architecture sketches elsewhere in this context file. Use the
[Release Plugin current architecture](../plugin/release/docs/architecture/current-state.md)
for implementation facts and the [Release documentation
history](../plugin/release/docs/history/README.md) only for completed or
superseded planning context.

## Author

Benjamin Senekowitsch - senekowitsch@nekoman.at
