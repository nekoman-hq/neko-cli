# Plugin Development Guide

> **Audience:** Go developers building a Neko CLI plugin executable and manifest.
>
> **Purpose:** Define the supported process protocol, manifest contract, response presentation, exit ownership, testing, packaging, and installation layout.

A Neko plugin is a standalone executable. Core discovers commands from an
installed `manifest.json`, sends one JSON request on standard input, captures
standard error as logs, decodes one JSON response from standard output, renders
that response once, and applies its explicit exit request.

For the complete public command surface, see the
[CLI reference](cli-reference.md). For a compact repository-wide implementation
map, see [AI context](ai_context.md).

## Installed layout

Core looks for this layout below its plugin directory:

```text
<plugin-dir>/
└── example/
    ├── manifest.json
    └── plugin-example
```

The default plugin directory is `~/.neko/plugins`; `NEKO_PLUGIN_DIR` overrides
it. The executable name is `plugin-<manifest-name>`. Core skips missing or
invalid manifests when listing installed plugins and reports a missing
executable when dispatch is attempted.

## Manifest

The manifest owns plugin identity, discoverable commands, flags, and declared
output modes:

```json
{
  "name": "example",
  "version": "0.1.0",
  "description": "Example Neko plugin",
  "author": "example-org",
  "commands": [
    {
      "name": "inspect",
      "description": "Inspect the selected resource",
      "outputs": ["table", "json"],
      "flags": [
        {
          "name": "resource",
          "type": "string",
          "required": true,
          "description": "Resource identifier"
        },
        {
          "name": "details",
          "type": "bool",
          "required": false,
          "default": false,
          "description": "Include additional facts"
        }
      ]
    }
  ],
  "renderer_types": ["table", "json"]
}
```

Flag types are `string`, `bool`, and `int`. Core registers manifest flags with
Cobra, applies manifest defaults, validates required flags, and sends the
resolved values in `Request.Flags`. A command's positional values arrive in
`Request.Args`.

Do not declare Core global flags in a plugin manifest. Core owns
`--describe`, `--verbose`, `--output`, and `--github-output-file` for every
plugin response.

## Request protocol

The public request type is `pkg/plugin.Request`:

```go
type Request struct {
    Command string         `json:"command"`
    Args    []string       `json:"args"`
    Flags   map[string]any `json:"flags"`
    Context Context        `json:"context"`
}

type Context struct {
    WorkingDir string `json:"working_dir"`
    User       string `json:"user"`
    Verbose    bool   `json:"verbose"`
}
```

`WorkingDir` is the Core process working directory. Treat it as input and
resolve repository-owned paths deliberately. `Verbose` carries execution-log
intent. Presentation choices such as describe/output mode and credentials are
not transported in the request.

A plugin reads exactly one request from standard input. It should not prompt on
standard input or write non-JSON content to standard output.

## Response protocol

The public response envelope contains:

| Field | Responsibility |
| --- | --- |
| `status` | Domain status such as `success` or `error` |
| `metadata` | Timestamp, plugin, version, and command identity |
| `data` | Stable machine-readable command data |
| `error` | Machine code, message, and optional safe details for error status |
| `renderer_hint` | Default human rendering hint; `raw-json` is an explicit raw artifact exception |
| `logs` | Ordered structured log entries |
| `human_table` | Responsive table declaration on the wire |
| `human_properties` | Ordered property declaration on the wire |
| `human_text` | Preformatted text declaration on the wire |
| `github_output` | Ordered mapping from output names to stable data keys |
| `exit_code` | Explicit requested Core process exit from 0 through 125 |

Go plugins set the canonical `PresentationTable`, `PresentationProperties`, or
`PresentationText` fields. Their JSON encoding retains the established
`human_*` wire names. Deprecated `HumanTable`, `HumanProperties`, and
`HumanText` Go fields remain source-compatible; do not set conflicting old and
new fields.

An error response includes both `status: "error"` and a non-nil error envelope.
Core rejects a nil response, an error status without its envelope, conflicting
presentation declarations, and an explicit exit outside 0 through 125.

## Minimal Go executable

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "time"

    "github.com/nekoman-hq/neko-cli/pkg/plugin"
    "github.com/nekoman-hq/neko-cli/pkg/presentation"
)

func main() {
    var request plugin.Request
    if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
        writeResponse(errorResponse("PARSE_ERROR", err))
        return
    }

    response := route(request)
    writeResponse(response)
}

func route(request plugin.Request) *plugin.Response {
    if request.Command != "inspect" {
        return errorResponse("UNKNOWN_COMMAND", fmt.Errorf("unknown command %q", request.Command))
    }

    response := &plugin.Response{
        Status: "success",
        Metadata: plugin.ResponseMetadata{
            Timestamp: time.Now().UTC(),
            Plugin: "example",
            Version: "dev",
            Command: request.Command,
        },
        Data: map[string]any{"resource": request.Flags["resource"]},
        RendererHint: "table",
        PresentationProperties: &presentation.Properties{
            Properties: []presentation.Property{
                {Label: "Resource", Value: fmt.Sprint(request.Flags["resource"])},
            },
        },
    }
    response.SetExitCode(0)
    return response
}

func errorResponse(code string, err error) *plugin.Response {
    response := &plugin.Response{
        Status: "error",
        Metadata: plugin.ResponseMetadata{
            Timestamp: time.Now().UTC(),
            Plugin: "example",
            Version: "dev",
            Command: "unknown",
        },
        Error: &plugin.ResponseError{Code: code, Message: err.Error()},
    }
    response.SetExitCode(1)
    return response
}

func writeResponse(response *plugin.Response) {
    _ = json.NewEncoder(os.Stdout).Encode(response)
}
```

Use command-specific packages behind the router as the plugin grows. Keep
request parsing and response encoding at the executable boundary; keep domain
logic independent of terminal width, color, and Core rendering.

## Presentation and output

Core owns the public output mode:

- `table` selects responsive human rendering;
- `wide` allows additional declared columns;
- `json` encodes the public response envelope;
- `github` writes only declared GitHub output fields to the explicitly supplied command file.

`--describe` changes visibility of declared presentation details and metadata;
it does not alter the plugin request. `--verbose` enables request context and
Core log rendering. JSON remains machine data and excludes presentation-only
declarations. A plugin should place stable values in `Data` and declare human
views from those values rather than formatting terminal tables itself.

Use `renderer_hint: "raw-json"` only when the command deliberately emits an
artifact as exact JSON bytes. Ordinary `--output json` is not raw command data;
it is the public response envelope.

GitHub output requires a `GitHubOutput` declaration and Core's explicit
`--github-output-file`. The plugin names data keys; it does not select the file.

## Exit ownership

Every new response calls `SetExitCode` explicitly. Exit `0` represents a
successful command or an intentional successful negative observation. Exit
`1` represents command failure. Values through 125 are available when a plugin
has a documented portable contract.

Core treats a valid decoded response as authoritative even if the plugin
process itself exited nonzero. Core then renders once and applies the response
exit. An omitted exit is legacy implicit-success compatibility, not the
recommended contract. Transport, decode, response validation, rendering, and
GitHub command-file failures remain Core-owned failures.

## Logging

Plugins can return `LogEntry` values and can write logs to standard error.
Captured standard-error lines are ANSI-stripped, parsed, appended after response
logs, and deduplicated when the exact same entry appeared in both channels.
Structured standard-error lines use:

```text
15:04:05 [category] message
```

Core infers `error`, `warn`, `verbose`, or `info` for captured lines. Never put
tokens, authorization headers, credentials, or sensitive environment values in
logs or response data.

## Testing

At minimum, test:

- manifest JSON parsing and command/flag registration;
- exact manifest/router parity;
- request decoding and typed flag validation;
- success and structured error responses;
- explicit exit presence and range;
- JSON wire tags and stable machine fields;
- presentation at narrow and wide output widths;
- default, describe, verbose, JSON, and GitHub output behavior;
- no credentials or local absolute paths in response/log output;
- filesystem and network boundaries with temporary directories and injected clients;
- unknown command handling.

Repository tests use self-owned loopback servers for HTTP behavior; they do not
require real external services or credentials.

## Packaging and distribution

Build the executable with the installed name and package it with its manifest:

```bash
go build -o build/plugin-example ./cmd/plugin-example
mkdir -p build/example
cp build/plugin-example build/example/plugin-example
cp manifest.json build/example/manifest.json
```

First-party plugin releases are represented as V2 `kind: "plugin"` units.
Their unit metadata supplies public name, manifest, asset prefix, and binary
name. The release workflow publishes platform archives and refreshes the
`plugin-index.json` asset on the `plugin-registry` release. Core discovery,
installation, and update resolve exact plugin tags and assets from that index.

For the first-party implementations, see the [Release Plugin](plugins/release.md)
and [UI Plugin](plugins/ui.md) guides.
