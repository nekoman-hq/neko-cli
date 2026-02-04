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
├── pkg/                    # Shared packages
│   ├── dispatcher/         # Plugin execution & communication
│   ├── plugin/             # Plugin types (Request, Response, Manifest)
│   ├── renderer/           # kubectl-style output rendering
│   ├── log/                # Logging utilities
│   └── errors/             # Error handling
└── plugin/
    └── release/            # Release management plugin
        ├── main.go         # Plugin entry point
        ├── manifest.json   # Plugin metadata & command definitions
        └── pkg/            # Plugin-specific packages
            ├── init/       # Init command handler
            ├── config/     # .release.neko.json management
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

### 4. Config File Naming

Plugin config files follow the pattern: `.{plugin-name}.neko.json`

- Release plugin: `.release.neko.json`
- Future deploy plugin: `.deploy.neko.json`

### 5. No Interactive Prompts in Plugins

**Plugins cannot use interactive prompts (survey, stdin reading)** because stdin is used for the JSON request. All user input must come via flags.

```go
// ❌ WRONG - survey doesn't work in plugins
survey.AskOne(&survey.Select{...}, &answer)

// ✅ CORRECT - use flags from request
projectType := getFlagString(req.Flags, "project-type")
```

### 6. Manifest Flags

Define flags in `manifest.json` for automatic CLI flag registration:

```json
{
  "name": "init",
  "flags": [
    {"name": "project-type", "type": "string", "required": true, "description": "..."},
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

### Tool Registration (Release Systems)

Release tools register themselves via `init()`:

```go
// In tool/goreleaser/goreleaser.go
func init() {
    release.Register(&GoReleaser{})
}

// Import in main.go to trigger init()
import _ "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release/tool"
```

## Building & Testing

```bash
# Build everything
make all

# Test a plugin directly
echo '{"command":"init-options","args":[],"flags":{},"context":{}}' | ./plugin/release/plugin-release

# Test via CLI
./neko release init --project-type=backend --release-system=goreleaser
./neko release init-options
./neko release history --describe
```

## Output Flags

- `--output table` (default) - kubectl-style table
- `--output json` - Raw JSON
- `--output wide` - Extended table
- `--describe` - Include logs and metadata
- `-v, --verbose` - Verbose logging

## Files to Ignore

When refactoring old code to plugin style, these patterns indicate deprecated code:
- Uses `survey.AskOne()` or similar interactive prompts
- Uses `log.Print()` instead of `log.PluginPrint()` in plugin code
- Cobra commands in `plugin/*/pkg/cmd/` (old style, should be handlers in `pkg/{command}/`)

## Current State

### Completed
- ✅ Core plugin dispatcher
- ✅ Release plugin: `init`, `init-options`, `history`, `contributors`
- ✅ Renderer with table/json/text output
- ✅ Dynamic flag loading from manifest

### In Progress / TODO
- Release plugin: `release` command (partially implemented)
- Release plugin: `validate` command
- Other plugins (deploy, etc.)

## Author

Benjamin Senekowitsch - senekowitsch@nekoman.at
