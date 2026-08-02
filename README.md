<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=Neko%20CLI&amp;subtitle=Plugin-based%20commands%2C%20consistent%20output%2C%20and%20release%20automation.&amp;logo=go&amp;align=left&amp;theme=cyan&amp;font=geist-mono&amp;border=true&amp;mode=dark" />
    <img alt="Neko CLI technical header" src="https://shieldcn.dev/header/graph.svg?title=Neko%20CLI&amp;subtitle=Plugin-based%20commands%2C%20consistent%20output%2C%20and%20release%20automation.&amp;logo=go&amp;align=left&amp;theme=cyan&amp;font=geist-mono&amp;border=true&amp;mode=light" width="100%" />
  </picture>
</p>

<h1 align="center">Neko CLI</h1>

<p align="center"><strong>A plugin-based command-line framework for consistent command routing, structured responses, and release automation.</strong></p>

<!-- hero-badges:start -->
<p align="center">
  <a href="https://github.com/nekoman-hq/neko-cli/releases">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/github/dt/nekoman-hq/neko-cli.svg?variant=secondary&amp;font=geist-mono&amp;mode=dark" />
      <img alt="GitHub release downloads" src="https://shieldcn.dev/github/dt/nekoman-hq/neko-cli.svg?variant=secondary&amp;font=geist-mono&amp;mode=light" />
    </picture>
  </a>
  <a href="https://github.com/nekoman-hq/neko-cli/actions/workflows/go-lint.yml">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/github/ci/nekoman-hq/neko-cli.svg?workflow=go-lint.yml&amp;branch=main&amp;variant=secondary&amp;font=geist-mono&amp;mode=dark" />
      <img alt="golangci-lint workflow status" src="https://shieldcn.dev/github/ci/nekoman-hq/neko-cli.svg?workflow=go-lint.yml&amp;branch=main&amp;variant=secondary&amp;font=geist-mono&amp;mode=light" />
    </picture>
  </a>
  <a href="https://github.com/nekoman-hq/neko-cli/blob/main/LICENSE">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/badge/License-Personal--noncommercial-64748b.svg?variant=secondary&amp;font=geist-mono&amp;mode=dark" />
      <img alt="Personal noncommercial license" src="https://shieldcn.dev/badge/License-Personal--noncommercial-64748b.svg?variant=secondary&amp;font=geist-mono&amp;mode=light" />
    </picture>
  </a>
  <a href="https://github.com/nekoman-hq/neko-cli/blob/main/go.mod">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/badge/Go-1.24.4-00ADD8.svg?variant=branded&amp;logo=go&amp;font=geist-mono&amp;mode=dark" />
      <img alt="Go 1.24.4 requirement" src="https://shieldcn.dev/badge/Go-1.24.4-00ADD8.svg?variant=branded&amp;logo=go&amp;font=geist-mono&amp;mode=light" />
    </picture>
  </a>
</p>
<!-- hero-badges:end -->

<p align="center">
  <a href="#quick-start">Quick Start</a> ·
  <a href="docs/cli-reference.md">CLI Reference</a> ·
  <a href="docs/plugin-development.md">Build a Plugin</a>
</p>

Neko CLI turns standalone executables into one discoverable command tree. Core
loads installed plugin manifests, routes commands over a stable JSON process
protocol, renders consistent human and machine output, and applies explicit
plugin exit intent. Plugins stay independent and own their domain behavior.

## What Neko CLI provides

| Capability | Current contract |
| --- | --- |
| Manifest-driven commands | Installed manifests declare plugin commands, local flags, help, and output capabilities. |
| Standalone plugins | Each plugin is a separate executable with its own implementation and release lifecycle. |
| Consistent presentation | Core renders responsive tables, wide views, JSON responses, and declared GitHub command-file output. |
| Plugin management | Core lists installed plugins and discovers, installs, and updates published plugins through the Plugin Index. |
| Release automation | The first-party Release Plugin provides compatible V1 behavior and the canonical V2 unit-based workflow. |

The [complete CLI reference](docs/cli-reference.md) owns the public command,
flag, I/O, network, mutation, and process-exit inventory.

## Quick Start

Install the latest stable CLI with the repository installer, then verify the
installation:

```bash
curl -fsSL https://raw.githubusercontent.com/nekoman-hq/neko-cli/main/install.sh | bash
neko version
```

Discover the published Plugin Index, install the Release Plugin, and inspect
its manifest-derived command surface:

```bash
neko plugin available
neko plugin install release
neko release --help
```

Inside a repository with Release V2 configuration and state, Doctor provides a
safe integration check:

```bash
neko release doctor
```

Doctor is read-only and never repairs repository configuration or workflows.
The [installation and update guide](docs/installation.md) covers custom install
directories, PATH setup, supported platforms, package-manager ownership,
self-update, integrity verification, and recovery.

`neko update --force` is a same-version reinstall only. It does not bypass
permissions, package-manager ownership, integrity checks, platform checks, or
downgrade protection.

## How plugins work

Core discovers `manifest.json` files below the configured plugin directory and
adds their declared commands to the CLI. It starts the corresponding plugin
executable only when one of those commands runs.

```text
User command
    │
    ▼
Neko Core ── JSON Request on stdin ──▶ Plugin executable
    ▲                                      │
    └── JSON Response on stdout ───────────┘
                 logs on stderr
    │
    └── responsive human / JSON / GitHub rendering + final process exit
```

Core owns discovery, command routing, subprocess transport, response
validation, terminal-aware rendering, and transport/render failures. Plugins
own command behavior, domain effects, structured response data, presentation
declarations, logs, and explicit response exits. A valid response is rendered
once; normal results and errors are not printed through competing paths.

See [package architecture](docs/package-architecture.md) for repository
responsibilities and the [plugin development guide](docs/plugin-development.md)
for the manifest, request, response, presentation, testing, and packaging
contracts.

## First-party plugins

Use `neko plugin available` to inspect the current published registry.

| Plugin | Purpose | Current status | Documentation |
| --- | --- | --- | --- |
| Release | Version planning, release units, local Git preparation, workflow handoff, evidence, and recovery | Available; V1 is supported compatibility and V2 is canonical for new setup | [Release Plugin](docs/plugins/release.md) |
| UI | Adds and manages Neko UI components in React Native projects | Available; the manifest advertises `hello`, but the current router does not implement that command | [UI Plugin](docs/plugins/ui.md) |

The UI discrepancy is a current product limitation: `hello` appears in
manifest-derived help, but dispatch fails because no handler is routed.

## Output and automation

Plugin responses use a consistent Core-owned presentation boundary:

* `table` is the default responsive human view;
* `wide` enables additional declared table columns;
* `json` emits the public response envelope;
* `github` writes declared fields to an explicit GitHub Actions command file;
* `--describe` adds safe structured human detail;
* `--verbose` requests and renders captured execution logs.

For example, a Release plan can be inspected as machine-readable JSON without
mutating the repository:

```bash
neko release plan --change patch --output json
```

Core `--output` selects a response format; it is not a file destination. Plugin
Index persistence uses `--output-file`, while its default mode emits the raw
schema-v1 registry artifact:

```bash
neko release plugin-index --output-file build/plugin-index.json
```

A valid structured plugin response owns its explicit process exit from `0`
through `125`. Core owns malformed transport, invalid response, rendering, and
GitHub command-file failures. Each result or error is rendered once. Detailed
output and exit behavior lives in the [CLI reference](docs/cli-reference.md#output-and-process-exit).

## Release Plugin

Release V1 remains a supported compatibility surface backed by the root
`.release.neko.json`. Release V2 is canonical for new setup: configuration in
`.neko/release.config.json` declares release units and policy, while
`.neko/release.state.json` owns mutable unit versions. Mixed active V1 and V2
authority is rejected.

Useful read-only entry points include:

```bash
neko release init-options
neko release units
neko release pipeline
neko release plan --change patch
```

Pipeline is a read-only projection; it does not execute, retry, or resume
lifecycle stages. Workflow Init is create-only and never overwrites a differing
customized workflow. Neko owns local planning, selected-unit materialization,
state, the release commit and tag, pushes, journals, and workflow handoff.
Consumer-owned workflows own builds, GitHub Release creation, and artifact
publication.

Start with the [Release overview](docs/release/overview.md), then use the
[Release command reference](docs/release/cli-reference.md) for exact commands,
flags, outputs, exits, and safety boundaries.

## How Neko CLI releases itself

Neko CLI dogfoods Release V2 through three independently versioned release units.
Each is configured with GoReleaser and GitHub Actions delivery. Their current
versions are owned exclusively by `.neko/release.state.json`;
`.neko/release.config.json` owns the stable unit structure, tag namespaces, and
delivery configuration.

| Unit | Tag namespace | Consumer workflow |
| --- | --- | --- |
| `cli` | `vX.Y.Z` | `.github/workflows/release-neko-cli.yml` |
| `plugin-release` | `plugin-release/vX.Y.Z` | `.github/workflows/release-plugin-release.yml` |
| `plugin-ui` | `plugin-ui/vX.Y.Z` | `.github/workflows/release-plugin-ui.yml` |

1. Release config and state identify the selected unit and authoritative version; its tag is calculated from the configured prefix and next version.
2. `neko release patch|minor|major` plans and materializes the selected unit, then creates the release commit and lightweight unit tag.
3. Neko pushes the owned Git state—commit before tag—and dispatches the configured workflow with validated release identity.
4. The consumer-owned GitHub Actions workflow builds, creates the GitHub Release, and publishes the unit's artifacts. Dispatch is the handoff to that workflow, not completed publication.

`neko release doctor` is read-only and validates the local V2 integration. It
is offline and token-free by default; remote verification performs bounded
GitHub GETs only when `--verify-remote` is explicitly requested. Doctor never
repairs configuration or files.

`neko release github-workflow-init` is create-only. It creates one missing
starter workflow and accepts an identical existing workflow without rewriting
it; differing customized content is never overwritten. Repository workflows
remain intentionally consumer-owned after scaffolding.

For exact behavior, use the [Release CLI Reference](docs/release/cli-reference.md),
[V1-to-V2 migration guide](docs/release/migration-v1-to-v2.md),
[GitHub Actions golden path](docs/release/github-actions-golden-path.md),
[release-unit examples](docs/release/examples.md#normal-release-units-vs-neko-cli-plugin-units),
and [current implementation architecture](plugin/release/docs/architecture/current-state.md#historical-context).

## Documentation

| Topic | Authoritative document |
| --- | --- |
| Complete CLI reference | [Commands, flags, I/O, and exits](docs/cli-reference.md) |
| Installation and update | [Installer and self-update](docs/installation.md) |
| Package architecture | [Repository package boundaries](docs/package-architecture.md) |
| Plugin development | [Manifest and process protocol](docs/plugin-development.md) |
| Release overview | [Release concepts and navigation](docs/release/overview.md) |
| Release command reference | [Release commands and safety contracts](docs/release/cli-reference.md) |
| Release workflow model | [GitHub Actions golden path](docs/release/github-actions-golden-path.md) |
| Release architecture | [Current implementation architecture](plugin/release/docs/architecture/current-state.md) |
| Historical rationale | [Numbered, non-authoritative history](plugin/release/docs/history/README.md) |

## Contributing

Begin with the [package architecture](docs/package-architecture.md) and the
tests around the behavior you intend to change. Work under `plugin/release`
also follows its repository-local [contributor instructions](plugin/release/AGENTS.md)
and [engineering rules](plugin/release/RULES.md). Use
[GitHub issues](https://github.com/nekoman-hq/neko-cli/issues) to discuss a
problem and [pull requests](https://github.com/nekoman-hq/neko-cli/pulls) to
propose a reviewed change.

## License

Neko CLI uses the repository's **Internal Use & Personal License**. It permits
personal or non-commercial use, copying, and sharing; commercial or business
use is prohibited. Modifications and redistribution must retain the license
and copyright notice. See [LICENSE](LICENSE) for the complete terms.

Maintained by [Nekoman](https://github.com/nekoman-hq).
