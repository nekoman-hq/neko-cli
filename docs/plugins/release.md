# Release Plugin

The **release** plugin is the core plugin for Neko CLI, providing comprehensive release management with semantic versioning support across multiple release systems.

## Overview

- **Plugin Name:** `release`
- **Current Version:** v4.1.0
- **Author:** nekoman-hq
- **Config Files:** `.release.neko.json` (V1 legacy), `.neko/release.config.json` and `.neko/release.state.json` (V2)

## Installation

The release plugin is bundled with Neko CLI. After building Neko CLI, install the plugin:

```bash
neko plugin install release
```

This installs the plugin to `~/.neko/plugins/release/`.

Plugin installation and updates resolve the newest release plugin version from the published `plugin-index.json` registry asset on the mutable `plugin-registry` GitHub Release. The repository's latest release and release-prefix fallback discovery are not used for release plugin discovery. The local installed version comes from `~/.neko/plugins/release/manifest.json`; the remote version, release tag, and asset names come from the index entry.

The `plugin-release` V2 unit declares `kind: "plugin"` metadata in `.neko/release.config.json`: public name `release`, manifest `plugin/release/manifest.json`, asset prefix `plugin-release`, and binary name `plugin-release`. `neko release plugin-index` generates the public `plugin-index.json` from this metadata, `.neko/release.state.json`, and plugin manifests. Runtime plugin discovery, install, and update use that index as the registry source of truth. Plugin release workflows publish it as the `plugin-index.json` asset on the mutable `plugin-registry` GitHub Release after successful plugin releases; the index is not committed as source.

Copy-ready Release V2 and plugin registry examples live in [Release V2 Examples](../release/examples.md). Bootstrap ownership across Neko CLI, GitHub Actions, adapters, and consumer workflows is defined in [Release V2 Bootstrap Product Boundary](../release/bootstrap-product-boundary.md).

---

## Commands

### `neko release init`

Initialize a new V2 release configuration for a normal release unit by default. Use `--kind plugin` only for Neko CLI plugins.

**Usage:**
```bash
neko release init --executor=<executor> --delivery=github-actions --workflow=<workflow> [flags]
```

**Required Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--executor` | string | Release executor: `goreleaser`, `jreleaser`, or `release-it` |
| `--delivery` | string | Delivery mode: `github-actions` |

**Optional Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--unit` | string | `cli` | Release unit id |
| `--display-name` | string | unit id | Human-readable unit name |
| `--version` | string | `0.1.0` | Initial semantic version |
| `--workflow` | string | | Required for V2 `github-actions`; must point to `.github/workflows/*.yml` or `.yaml` |
| `--tag-prefix` | string | `v` | Release tag prefix |
| `--working-directory` | string | `.` | Unit working directory |
| `--paths` | string | `**` | Comma-separated unit path globs |
| `--kind` | string | `release` | `release` is the default for normal services, apps, CLIs, SDKs, libraries, and backend modules; `plugin` is only for Neko CLI plugins |
| `--plugin-name` | string | | Public Neko CLI plugin name; only for `--kind plugin` and required there |
| `--plugin-manifest` | string | | Repository-root-relative Neko CLI plugin manifest path; only for `--kind plugin` and required there |
| `--plugin-asset-prefix` | string | | Neko CLI plugin release asset prefix; only for `--kind plugin`, required there, and must match unit id |
| `--plugin-binary-name` | string | | Neko CLI plugin executable name in release archives; only for `--kind plugin` and required there |
| `--force` | bool | `false` | Recreate existing V2 config/state |

`release init` no longer creates `.release.neko.json`. Existing V1 repositories should use `neko release migrate`. `--kind release` is the CLI default for normal release units; V2 JSON omits `kind` for those units, and they do not use plugin metadata or the plugin registry. `--kind plugin` creates one Neko CLI plugin unit with V2 plugin metadata. Plugin flags without `--kind plugin` are invalid. Plugin unit ids must start with `plugin-`, plugin tag prefixes must be `<unit-id>/v`, and plugin asset prefixes must match the unit id. Use `neko release unit-add` to append units to an existing V2 configuration. Workflow template generation and executor scaffolding are not implemented by `init` or `unit-add` yet. See [Normal release units vs Neko CLI plugin units](../release/examples.md#normal-release-units-vs-neko-cli-plugin-units).

**Examples:**
```bash
# Initialize a GitHub Actions-delivered GoReleaser CLI unit
neko release init --executor=goreleaser --delivery=github-actions --workflow=.github/workflows/release-cli.yml

# Initialize a GitHub Actions-delivered CLI unit
neko release init \
  --unit=cli \
  --display-name=neko-cli \
  --executor=goreleaser \
  --delivery=github-actions \
  --workflow=.github/workflows/release.yml

# Reinitialize existing V2 config/state
neko release init --executor=goreleaser --delivery=github-actions --workflow=.github/workflows/release-cli.yml --force

# Start with a specific version
neko release init --executor=goreleaser --delivery=github-actions --workflow=.github/workflows/release-cli.yml --version=1.0.0

# Initialize one GitHub Actions-delivered plugin unit
neko release init \
  --unit=plugin-release \
  --display-name="neko-cli release plugin" \
  --version=4.0.0 \
  --executor=goreleaser \
  --delivery=github-actions \
  --workflow=.github/workflows/release-plugin-release.yml \
  --tag-prefix=plugin-release/v \
  --paths="plugin/release/**,docs/plugins/release.md" \
  --kind=plugin \
  --plugin-name=release \
  --plugin-manifest=plugin/release/manifest.json \
  --plugin-asset-prefix=plugin-release \
  --plugin-binary-name=plugin-release
```

**What it does:**
1. Creates `.neko/release.config.json`
2. Creates `.neko/release.state.json`
3. Validates the generated V2 repository configuration
4. Leaves executor-specific tool configuration to be added separately

---

### `neko release unit-add`

Append one release unit to an existing V2 `.neko/release.config.json` and `.neko/release.state.json`.

```bash
neko release unit-add \
  --unit=api \
  --display-name=api \
  --version=0.1.0 \
  --executor=goreleaser \
  --delivery=github-actions \
  --workflow=.github/workflows/release-api.yml \
  --tag-prefix=api/v \
  --paths="apps/api/**"
```

`unit-add` uses the same unit flags as `release init`. The plugin metadata flags are only for `--kind plugin` Neko CLI plugin units; normal repositories can contain only normal release units and need no plugin metadata or plugin registry. It requires existing V2 config/state, preserves existing units in order, appends the new unit at the end, and fails for duplicate unit ids, duplicate plugin names, overlapping tag prefixes, missing workflows, or missing plugin manifests.

It does not generate workflow files, GoReleaser config files, plugin manifests, source directories, tags, releases, or release assets. V1 repositories should use `neko release migrate` first.

---

### `neko release patch`

Create a patch release, incrementing the Z in X.Y.Z (e.g., 1.2.3 → 1.2.4).

**Usage:**
```bash
neko release patch [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dry-run` | bool | `false` | Preview the release without making changes |
| `--unit` | string | | Select the release unit. Required when a V2 repository defines multiple units. |

**Examples:**
```bash
# Create a patch release
neko release patch

# Preview what would happen
neko release patch --dry-run
```

**What it does:**
1. Loads V2 config/state when `.neko/release.config.json` exists, otherwise falls back to legacy V1 compatibility.
2. Runs preflight checks (git state, version validation, executor requirements where applicable).
3. Calculates the next patch version.
4. For V2 GitHub Actions units, updates state/materialized files, creates the Neko-owned release commit and tag, pushes them, and dispatches the configured workflow.
5. For legacy V1 repositories, keeps the existing `.release.neko.json` release path.

With `--dry-run`, Neko only calculates and displays the next version. It does not write config, update executor files, run executors, fetch remotes, commit, tag, push, publish, or rollback.

For V2 repositories, `patch`, `minor`, and `major` support dry-run planning with `--unit`. Non-dry-run V2 releases are active for `delivery: github-actions`; V2 local delivery is unsupported and rejected during validation. The GitHub Actions path writes execution and dispatch journals, commits and tags the release, pushes commit and tag, and dispatches the configured workflow. Neko CLI owns commit/tag/push/dispatch; GitHub Actions owns build, GitHub Release creation, and asset publishing from the pushed tag.

Nekocli dogfoods three independent V2 units: `cli`, `plugin-release`, and `plugin-ui`. Their versions live in `.neko/release.state.json`; `.plugin.release.neko.json` has been removed. Plugin releases materialize only their own manifest before the release commit.

Production publishing uses dedicated workflows and GoReleaser configs:

| Unit | Workflow | GoReleaser config |
| --- | --- | --- |
| `cli` | `.github/workflows/release-neko-cli.yml` | `.goreleaser.cli.yaml` |
| `plugin-release` | `.github/workflows/release-plugin-release.yml` | `.goreleaser.plugin-release.yaml` |
| `plugin-ui` | `.github/workflows/release-plugin-ui.yml` | `.goreleaser.plugin-ui.yaml` |

Dry-run does not require `GITHUB_TOKEN`. Verbose and `--describe` output shows the planned materialized files, known release files, workflow, dispatch inputs, and, for real handoffs, execution/dispatch journal paths, release commit SHA, dispatch state, run URL when resolvable, and recovery guidance. Unknown dispatch or ambiguous push outcomes must not be retried blindly; inspect with `neko release resume --unit <unit> --dry-run`.

---

### `neko release minor`

Create a minor release, incrementing the Y in X.Y.Z and resetting Z (e.g., 1.2.3 → 1.3.0).

**Usage:**
```bash
neko release minor [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dry-run` | bool | `false` | Preview the release without making changes |
| `--unit` | string | | Select the release unit. Required when a V2 repository defines multiple units. |

**Examples:**
```bash
# Create a minor release
neko release minor

# Preview what would happen
neko release minor --dry-run
```

---

### `neko release major`

Create a major release, incrementing the X in X.Y.Z and resetting Y and Z (e.g., 1.2.3 → 2.0.0).

**Usage:**
```bash
neko release major [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dry-run` | bool | `false` | Preview the release without making changes |
| `--unit` | string | | Select the release unit. Required when a V2 repository defines multiple units. |

**Examples:**
```bash
# Create a major release
neko release major

# Preview what would happen
neko release major --dry-run
```

---

### `neko release plan`

Inspect the local release plan for a requested version change without starting release execution.

**Usage:**
```bash
neko release plan --change=<patch|minor|major> [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--change` | string | | Requested version change to inspect: `patch`, `minor`, or `major` |
| `--unit` | string | | Select the release unit. Required when a V2 repository defines multiple units. |

**Examples:**
```bash
# Inspect the local patch plan for a selected unit
neko release plan --change patch --unit api

# Inspect a plugin unit before deciding whether to release it
neko release plan --change patch --unit plugin-release --output json
```

`plan` reports the selected release source and unit, current version, requested change, next version, tag, planned materialized files, known release files, local readiness, local blockers, and explicit limitations. It is strictly read-only and token-free: it does not read `GITHUB_TOKEN`, inspect remotes, inspect execution or dispatch journals, write config/state/manifests, mutate Git, dispatch workflows, publish releases, or run executors.

Use `plan` when tooling or a human needs stable local planning facts. Use `patch`, `minor`, or `major --dry-run` when you want the existing release preview contract. Use `resume --dry-run` or `evidence` for already-started release execution and recovery state.

---

### `neko release history`

Show the release history with commit counts between versions.

**Usage:**
```bash
neko release history [flags]
```

For V2 repositories with multiple units, pass `--unit <unit-id>`. History then includes only tags owned by that unit and counts commits through the unit paths.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--unit` | string | | Select the release unit. Required when a V2 repository defines multiple units. |

**Output Formats:**
- `table` (default) - Clean tabular view
- `json` - Full JSON response

**Examples:**
```bash
# View release history as table
neko release history

# Get JSON output
neko release history --output json

# Verbose output with logs
neko release history --describe -v
```

**Sample Output:**
```
COMMITS  FROM    VERSION
──────────────────────────
4        <none>  v0.1.0
2        v0.1.0  v0.1.1
37       v0.1.1  v0.2.0
3        v0.2.0  v0.2.1
2        v0.2.1  v0.2.2
```

---

### `neko release contributors`

List all contributors to the repository with their commit counts.

**Usage:**
```bash
neko release contributors [flags]
```

For V2 repositories with multiple units, pass `--unit <unit-id>`. Contributors are calculated through the selected unit paths.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--unit` | string | | Select the release unit. Required when a V2 repository defines multiple units. |

**Examples:**
```bash
# View contributors as table
neko release contributors

# Get JSON output
neko release contributors --output json
```

**Sample Output:**
```
AUTHOR                                                              COMMITS
─────────────────────────────────────────────────────────────────────────────
Benjamin Senekowisch <122978402+senbeb21@users.noreply.github.com>  140
Flokkq <webcla21@htl-kaindorf.at>                                   1
```

---

### `neko release validate`

Validate the release configuration.

**Usage:**
```bash
neko release validate [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--show` | bool | `false` | Display current configuration details |
| `--unit` | string | | Focus displayed V2 unit details. Required only for unit-bound release commands in multi-unit repositories. |

**Examples:**
```bash
# Validate configuration
neko release validate

# Show configuration details
neko release validate --show
```

**Sample Output for legacy V1 repositories (with --show):**
```
PROPERTY        VALUE
────────────────────────────
Project Name    neko-cli
Project Owner   nekoman-hq
Project Type    other
Release System  goreleaser
Version         2.1.7
Status          ✓ Valid
```

For V2 repositories, `--show` displays schema type, units, versions, working directories, tag prefixes, executor, delivery, workflow when configured, and paths.

---

### `neko release plugin-index`

Generate the public `plugin-index.json` registry artifact from V2 plugin units, `.neko/release.state.json`, and each plugin manifest. The command itself does not publish the index and does not commit it as source. Plugin release workflows publish or replace the `plugin-index.json` asset on the mutable `plugin-registry` GitHub Release after successful plugin releases. Runtime plugin discovery, install, and update read that asset as the source of truth; release-prefix fallback discovery has been removed.

**Usage:**
```bash
neko release plugin-index [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--output` | string | | Optional file path to write `plugin-index.json` |
| `--check` | bool | `false` | Validate that the index can be generated without writing a file |
| `--pretty` | bool | `true` | Pretty-print generated JSON |
| `--repository` | string | `nekoman-hq/neko-cli` | Repository identifier to include in the generated index |

`--output` accepts either a clean repository-root-relative path or an explicit absolute artifact path. Relative paths are resolved from the repository root, not from the shell's current directory. Absolute paths are retained for CI temporary artifacts such as `/tmp/plugin-index.json`. Repository-contained output cannot overwrite release config/state, release recovery evidence, Git internals, or plugin manifests used as index inputs. Existing target directories and target symlinks are rejected.

**Examples:**
```bash
# Print the generated index JSON
neko release plugin-index

# Validate generation without writing
neko release plugin-index --check

# Write to a temporary artifact path
neko release plugin-index --output /tmp/plugin-index.json
```

New plugins appear in the generated index after adding a V2 unit with `kind: "plugin"`, matching plugin metadata, a matching `.neko/release.state.json` entry, and a manifest whose name and version match that metadata and state.

---

### `neko release init-options`

Get available options for the init command. Useful for scripting or discovering available choices.

**Usage:**
```bash
neko release init-options
```

**Examples:**
```bash
# Get available options as table
neko release init-options

# Get as JSON for scripting
neko release init-options --output json
```

**Sample Output:**
```
DESCRIPTION                                                                                                 OPTION                REQUIRED          VALUES
────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
Release unit id                                                                                             unit                  false             cli, api, plugin-release, ...
Release unit display name                                                                                   display-name          false             string
Initial version                                                                                             version               false             semver, default 0.1.0
Release executor                                                                                            executor              true              goreleaser, jreleaser, release-it
Release delivery mode                                                                                       delivery              true              github-actions
GitHub Actions workflow path                                                                                workflow              conditional       .github/workflows/*.yml
Release tag prefix                                                                                          tag-prefix            false             v
Unit working directory                                                                                      working-directory     false             .
Unit path scope                                                                                             paths                 false             comma-separated globs
release is the default for normal release units; plugin is only for Neko CLI plugins. Plugin fields are invalid unless kind=plugin.  kind                  false             release, plugin
Only with kind=plugin; public Neko CLI plugin name. Normal repositories do not use plugin fields.            plugin-name           when kind=plugin  release, ui, ...
Only with kind=plugin; repository-root-relative Neko CLI plugin manifest path.                               plugin-manifest       when kind=plugin  plugin/<name>/manifest.json
Only with kind=plugin; Neko CLI plugin asset prefix, required there and must match unit id.                  plugin-asset-prefix   when kind=plugin  plugin-<name>
Only with kind=plugin; Neko CLI plugin executable name in release archives.                                  plugin-binary-name    when kind=plugin  plugin-<name>
Overwrite existing V2 config/state                                                                          force                 false             true, false
```

---

### `neko release migrate`

Safely migrate a root V1 release configuration to V2.

**Usage:**
```bash
neko release migrate [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dry-run` | bool | `false` | Preview the migration without writing files |

`migrate` only converts `.release.neko.json` in the Git root to a single V2 `default` unit. It does not infer multiple units, convert nested V1 files, or run a release.

---

### `neko release resume`

Resume a previously journaled V2 GitHub Actions release.

**Usage:**
```bash
neko release resume --unit <unit-id> [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--unit` | string | | Select the unit whose existing V2 GitHub Actions journal should be resumed |
| `--dry-run` | bool | `false` | Assess the existing release journal without writing files, refs, journals, remotes, or dispatching |

`resume` never calculates a fresh version. It requires exactly one unresolved execution journal for the selected unit and blocks ambiguous push or dispatch outcomes. Resume before `commit-created` is intentionally conservative and requires manual inspection after `--dry-run`.

---

### `neko release evidence`

Inspect release evidence without mutating recovery state.

**Usage:**
```bash
neko release evidence [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--family` | string | | Filter by `release-execution`, `dispatch`, `migration`, `v1-compensation`, or `v2-pair-recovery` |
| `--unit` | string | | Filter records by release unit when the evidence records one |

The command reports redacted summaries plus diagnostics for corrupt, unsupported, conflicting, unresolved, terminal, and completed evidence. It does not print tokens, request headers, raw response bodies, process output, environment values, or full evidence files.

### `neko release evidence-archive`

Archive one completed evidence file through a guarded lifecycle operation.

**Usage:**
```bash
neko release evidence-archive --family <family> --identity <sha256> --digest-sha256 <sha256> --confirm-archive
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--family` | string | | Required evidence family |
| `--identity` | string | | Evidence identity from `neko release evidence --output json` |
| `--digest-sha256` | string | | Current evidence digest from inspection output |
| `--confirm-archive` | bool | `false` | Required explicit confirmation |

Only completed `release-execution`, completed `v1-compensation`, and completed `v2-pair-recovery` evidence can be archived. The command re-inspects the evidence, rejects stale digests, writes an exact `0600` archive copy in a private `0700` archive directory, verifies the copy, and only then removes the completed source evidence. It does not support force, repair, retry, arbitrary paths, dispatch archival, or migration archival.

---

## Configuration

### V1 Configuration File

The V1 release plugin uses `.release.neko.json` in your project root:

```json
{
  "project-name": "neko-cli",
  "project-owner": "nekoman-hq",
  "project-type": "backend",
  "release-system": "goreleaser",
  "version": "2.3.1"
}
```

### Configuration Properties

| Property | Type | Description |
|----------|------|-------------|
| `project-name` | string | Repository/project name (auto-detected from git) |
| `project-owner` | string | GitHub organization or user (auto-detected from git) |
| `project-type` | string | One of: `frontend`, `backend`, `other` |
| `release-system` | string | One of: `goreleaser`, `jreleaser`, `release-it` |
| `version` | string | Current semantic version (without `v` prefix) |

### V2 Configuration Files

V2 uses repository-root files:

```text
.neko/release.config.json
.neko/release.state.json
```

`release.config.json` stores committed repository architecture: units, paths, working directories, tag prefixes, executor type, and delivery. `release.state.json` stores unit versions. Tags are derived from `tagPrefix + version` and are not stored in state.

`neko release validate` can validate V2 now. `history`, `contributors`, dry-run planning, and root V1-to-V2 migration are unit-aware. GitHub Actions delivery is valid V2 configuration when `workflow` points to an existing `.github/workflows/<file>.yml|yaml` file. Dry-run planning builds the execution context, materialization plan, delivery/executor facts, planned release commit, unit tag, known release files, push order, workflow reference, dispatch input contract, dispatch status, and V2 Git ownership. V2 GitHub Actions non-dry-run release commands are active and journaled; `neko release resume --unit <unit>` resumes only existing unresolved execution journals. V2 local delivery and standalone public dispatch/retry commands are not active.

In Nekocli itself, `plugin-release` and `plugin-ui` are V2 units. `.neko/release.state.json` is authoritative for both plugin versions; `plugin/release/manifest.json` and `plugin/ui/manifest.json` are materialized release files for their selected units. Both plugin units declare plugin metadata in `.neko/release.config.json`; `neko release plugin-index` uses that metadata so adding a releaseable plugin is a V2 unit-config change, not a registry Go-code edit. `neko release init --kind plugin` can create one new plugin unit with that metadata when no V2 config exists yet; `neko release unit-add --kind plugin` appends another plugin unit to existing V2 config/state. Runtime plugin discovery uses the published `plugin-index.json` as its source of truth and does not use `/releases/latest`; the generated index is not committed as source. Plugin workflows publish or replace the `plugin-index.json` asset on the mutable `plugin-registry` GitHub Release only after the plugin GitHub Release succeeds. Release-prefix fallback discovery has been removed. `make update-manifests` remains a manual compatibility helper and reads V2 state. V2 dry-run planning does not require or resolve `GITHUB_TOKEN`; real GitHub Actions release execution still requires it.

The `plugin-release` unit uses `plugin-release/vX.Y.Z` tags and `.github/workflows/release-plugin-release.yml`. Neko CLI owns state, materialized files, release commit, tag, push, and workflow dispatch. The workflow checks out the dispatched tag, validates `release_sha`, validates the materialized version files and unit config, runs tests, checks `.goreleaser.plugin-release.yaml`, performs a plugin-release-only snapshot build, packages plugin-release archives with that dedicated GoReleaser config, and creates the GitHub Release for the exact prefixed tag with GitHub CLI. After that publish succeeds, it generates and validates `plugin-index.json`, then uploads/replaces that single asset on the mutable `plugin-registry` release. The dedicated config must not build or publish the main CLI or `plugin-ui`; it embeds `PLUGIN_RELEASE_VERSION` from the dispatch version into the release plugin binary and archives the committed `plugin/release/manifest.json`.

The `plugin-ui` unit follows the same production pattern with `plugin-ui/vX.Y.Z`, `.github/workflows/release-plugin-ui.yml`, `.goreleaser.plugin-ui.yaml`, `PLUGIN_UI_VERSION`, and `plugin/ui/manifest.json`.

`neko release migrate` can convert a root V1 single-unit repository to V2. It archives `.release.neko.json` as `.release.neko.json.v1.bak`, writes V2 config and state atomically, and uses a temporary recovery journal.

See:

- [Release overview](../release/overview.md)
- [Release V2 bootstrap product boundary](../release/bootstrap-product-boundary.md)
- [Release configuration](../release/configuration.md)
- [Release state](../release/state.md)
- [Unit selection](../release/unit-selection.md)
- [Tag strategy](../release/tag-strategy.md)
- [CLI reference](../release/cli-reference.md)
- [V1 to V2 migration](../release/migration-v1-to-v2.md)
- [Release executors](../release/executors.md)
- [Version materialization](../release/version-materialization.md)
- [Local delivery](../release/local-delivery.md)
- [GitHub Actions delivery](../release/github-actions-delivery.md)
- [GitHub Actions release flow](../release/github-actions-release-flow.md)
- [Execution journal](../release/execution-journal.md)
- [Recovery model](../release/recovery-model.md)
- [GitHub Actions dispatch](../release/github-actions-dispatch.md)
- [Dispatch contract](../release/dispatch-contract.md)
- [Dispatch journal](../release/dispatch-journal.md)
- [Local release transaction](../release/local-release-transaction.md)
- [Compatibility](../release/compatibility.md)

---

## Release Systems

### GoReleaser

Best for: **Go projects**

**Prerequisites:**
- [GoReleaser](https://goreleaser.com/install/) installed
- `.goreleaser.yml` or a dedicated GoReleaser configuration. `neko release init` creates V2 release config/state only; tool-specific executor config is added separately.

**What Neko does:**
1. Creates release commit with version
2. Creates and pushes git tag
3. Materializes configured version files from release state when required
4. Runs `goreleaser release`
5. Handles rollback on failure

**Files managed:**
- `.goreleaser.yml`
- Git tags

**Plugin-Based Projects:**

For projects using a plugin architecture, model each releaseable plugin as its own V2 unit. `.neko/release.state.json` stores the authoritative version, and V2 materialization updates the selected plugin manifest before the Neko-owned release commit.

See the [Plugin Version Injection](#plugin-version-injection) section for details on how to configure this feature.

---

### JReleaser

Best for: **Java/JVM projects**

**Prerequisites:**
- [JReleaser](https://jreleaser.org/guide/latest/install/) installed
- `jreleaser.yml` configuration

**What Neko does:**
1. Updates version in `jreleaser.yml`
2. Creates release commit
3. Creates and pushes git tag
4. Runs `jreleaser release`

**Files managed:**
- `jreleaser.yml`
- `pom.xml` or `build.gradle` (version updates)
- Git tags

---

### release-it

Best for: **Node.js/Frontend projects**

**Prerequisites:**
- [release-it](https://github.com/release-it/release-it) installed (`npm install -g release-it`)
- `.release-it.json` configuration

**What Neko does:**
1. Updates version in `package.json`
2. Updates `.release-it.json` configuration
3. Runs `release-it` with appropriate flags

**Files managed:**
- `package.json`
- `.release-it.json`
- Git tags

---

## Plugin Versioning

### How It Works

Release V2 stores plugin versions in `.neko/release.state.json`. During a plugin release, Neko materializes the selected plugin's `manifest.json` to the planned next version before creating the release commit. GitHub Actions workflows may pass the dispatched `version` input to GoReleaser as a non-secret environment variable such as `PLUGIN_RELEASE_VERSION`.

**Flow:**
```
.neko/release.state.json → plugin manifest materialization → release commit → workflow input version → GoReleaser → Binary
```

### V2 State

Plugin units are configured in `.neko/release.config.json`, and their versions are stored in `.neko/release.state.json`:

```json
{
  "schemaVersion": 2,
  "units": {
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

Workflows can map the dispatch `version` input into the environment variable expected by their dedicated GoReleaser config:

| Unit | Environment Variable |
|------|----------------------|
| `plugin-release` | `PLUGIN_RELEASE_VERSION=${{ inputs.version }}` |
| `plugin-ui` | `PLUGIN_UI_VERSION=${{ inputs.version }}` |

**Pattern:** `PLUGIN_{UPPERCASE_NAME}_VERSION={version}`

### Using in GoReleaser

Access these variables in your `.goreleaser.yml`:

```yaml
builds:
  - ldflags:
      - -X main.ReleasePluginVersion={{ .Env.PLUGIN_RELEASE_VERSION }}
      - -X main.DeployPluginVersion={{ .Env.PLUGIN_DEPLOY_VERSION }}
      - -X main.TestPluginVersion={{ .Env.PLUGIN_TEST_VERSION }}
```

### Behavior

- Plugin manifests are committed release-owned files.
- `update-manifests`, when used manually, reads `.neko/release.state.json`.
- Missing or malformed materialized manifests fail release planning clearly.
- **No impact on release:** Works silently in the background

### Self-Bootstrapping

This creates an interesting architectural pattern:

1. Neko CLI uses the **release plugin** to release itself
2. The **release plugin** needs its version embedded in Neko CLI
3. This **injection system** bridges that gap automatically

It's metadata injection that allows plugins to declare their versions in the host binary.

---

## Error Handling

The release plugin provides detailed error responses with hints:

### Common Errors

**CONFIG_NOT_FOUND**
```
No release configuration found
Hint: Run 'neko release init' for a new V2 config or 'neko release migrate' for an existing V1 config
```

**CONFIG_EXISTS**
```
.neko/release.config.json or .neko/release.state.json already exists
Hint: Use --force to recreate both V2 files
```

**VALIDATION_FAILED**
```
Invalid executor: custom
Hint: Must be one of: goreleaser, jreleaser, release-it
```

**VERSION_ERROR**
```
No version tags found in repository
Hint: Make sure you have at least one semantic version tag (e.g., v1.0.0)
```

---

## Rollback Behavior

For V1 legacy release execution, if a release fails after a mutating step, Neko attempts to automatically rollback:

1. **Commit Rollback** - Reverts to the pre-release HEAD
2. **Tag Rollback** - Deletes local and remote tags if pushed
3. **Remote Rollback** - Reverts pushed commits if possible

```
[GUARD] Encountered error while releasing. Trying to undo changes...
[GUARD] Successfully undid changes.
```

Rollback only runs after a mutating release step has been recorded. Dry-run planning and guard failures do not trigger destructive rollback operations such as hard reset or untracked-file cleanup.

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` | Required for V1 GitHub releases and V2 GitHub Actions dispatch attempts with repository Actions write permission |
| `PLUGIN_{NAME}_VERSION` | Optional workflow-provided plugin version for dedicated GoReleaser configs |

Custom token naming options are not currently supported but may be added in the future.

---

## Examples

### Release V2 Workflow

```bash
# 1. Initialize a first V2 unit
neko release init --unit cli --executor=goreleaser --delivery=github-actions --workflow .github/workflows/release-cli.yml

# 2. Append additional units when needed
neko release unit-add --unit api --executor=goreleaser --delivery=github-actions --workflow .github/workflows/release-api.yml --tag-prefix api/v --paths "apps/api/**"

# 3. Validate the setup
neko release validate --show

# 4. Preview and release a selected unit
neko release patch --unit api --dry-run --verbose --describe
neko release patch --unit api --verbose --describe
```

See [Release V2 Examples](../release/examples.md) for full CLI, service, plugin unit, plugin registry, and temp plugin smoke examples.

### Scripting with JSON Output

```bash
# Get current version from validate output
VERSION=$(neko release validate --show --output json | jq -r '.data.items[] | select(.property == "Version") | .value')
echo "Current version: $VERSION"

# Get release history as JSON
neko release history --output json | jq '.data.items'
```

### Plugin-Based Project Setup

```bash
# 1. Initialize release configuration for the first unit
neko release init --unit cli --executor=goreleaser --delivery=github-actions --workflow .github/workflows/release-cli.yml

# 2. Append plugin units to .neko/release.config.json and .neko/release.state.json
neko release unit-add --unit plugin-release --kind plugin --plugin-name release --plugin-manifest plugin/release/manifest.json --plugin-asset-prefix plugin-release --plugin-binary-name plugin-release --executor goreleaser --delivery github-actions --workflow .github/workflows/release-plugin-release.yml --tag-prefix plugin-release/v

# 3. Configure GoReleaser to use plugin versions
# Edit .goreleaser.yml and add to ldflags:
# - -X main.ReleasePluginVersion={{ .Env.PLUGIN_RELEASE_VERSION }}
# - -X main.DeployPluginVersion={{ .Env.PLUGIN_DEPLOY_VERSION }}

# 4. Release the selected plugin unit
neko release patch --unit plugin-release
```

---

## Troubleshooting

### Plugin not found

Ensure the plugin is installed:
```bash
ls ~/.neko/plugins/release/
# Should show: plugin-release manifest.json
```

If missing, reinstall:
```bash
neko plugin uninstall release
neko plugin install release
```

### Git authentication errors

Ensure `GITHUB_TOKEN` is set for real release execution:
```bash
export GITHUB_TOKEN=your_token_here
```

Dry-run commands do not require `GITHUB_TOKEN`.

### Release system not found

Ensure the underlying tool is installed:
```bash
# For GoReleaser
goreleaser --version

# For JReleaser
jreleaser --version

# For release-it
release-it --version
```

---

## See Also

- [Neko CLI README](../../README.md)
- [GoReleaser Documentation](https://goreleaser.com/intro/)
- [JReleaser Documentation](https://jreleaser.org/guide/latest/)
- [release-it Documentation](https://github.com/release-it/release-it)
