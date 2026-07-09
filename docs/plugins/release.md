# Release Plugin

The **release** plugin is the core plugin for Neko CLI, providing comprehensive release management with semantic versioning support across multiple release systems.

## Overview

- **Plugin Name:** `release`
- **Last Change:** v2.4.0
- **Author:** nekoman-hq
- **Config Files:** `.release.neko.json` (V1 legacy), `.neko/release.config.json` and `.neko/release.state.json` (V2)

## Installation

The release plugin is bundled with Neko CLI. After building Neko CLI, install the plugin:

```bash
neko plugin install release
```

This installs the plugin to `~/.neko/plugins/release/`.

Plugin installation and updates resolve the newest release plugin version from plugin-specific V2 unit releases with the tag prefix `plugin-release/v`. The repository's latest release is not used for release plugin discovery. The local installed version comes from `~/.neko/plugins/release/manifest.json`; the remote version comes from the selected `plugin-release/vX.Y.Z` GitHub Release tag.

The `plugin-release` V2 unit declares `kind: "plugin"` metadata in `.neko/release.config.json`: public name `release`, manifest `plugin/release/manifest.json`, asset prefix `plugin-release`, and binary name `plugin-release`. `neko release plugin-index` generates the future public `plugin-index.json` from this metadata, `.neko/release.state.json`, and plugin manifests. The generated index is not committed as source and install/update still uses the current registry fallback until the next plugin-registry milestone.

---

## Commands

### `neko release init`

Initialize a new release configuration for your project.

**Usage:**
```bash
neko release init --project-type=<type> --release-system=<system> [flags]
```

**Required Flags:**

| Flag | Type | Description |
|------|------|-------------|
| `--project-type` | string | Project type: `frontend`, `backend`, or `other` |
| `--release-system` | string | Release system: `goreleaser`, `jreleaser`, or `release-it` |

**Optional Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--version` | string | `0.1.0` | Initial semantic version |
| `--force` | bool | `false` | Overwrite existing configuration |

`metadata` is accepted only as a deprecated compatibility fallback if old callers still send it. `--version` is the canonical flag.

**Examples:**
```bash
# Initialize a Go backend project with GoReleaser
neko release init --project-type=backend --release-system=goreleaser

# Initialize a Node.js frontend project
neko release init --project-type=frontend --release-system=release-it

# Reinitialize with force
neko release init --project-type=backend --release-system=goreleaser --force

# Start with a specific version
neko release init --project-type=backend --release-system=goreleaser --version=1.0.0
```

**What it does:**
1. Creates `.release.neko.json` configuration file
2. Detects repository owner/name from git remote
3. Initializes the underlying release system (e.g., creates `.goreleaser.yml`)
4. Validates the configuration

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
1. Loads configuration from `.release.neko.json`
2. Runs preflight checks (git state, version validation)
3. Calculates the next patch version
4. Creates a release commit: `chore(neko-release): x.y.z`
5. Creates and pushes a git tag
6. Runs the underlying release system (e.g., `goreleaser release`)
7. Updates the version in `.release.neko.json`

With `--dry-run`, Neko only calculates and displays the next version. It does not write config, update executor files, run executors, fetch remotes, commit, tag, push, publish, or rollback.

For V2 repositories, `patch`, `minor`, and `major` support dry-run planning with `--unit`. Non-dry-run V2 releases are active for `delivery: github-actions`; V2 local delivery remains blocked. The GitHub Actions path writes execution and dispatch journals, commits and tags the release, pushes commit and tag, and dispatches the configured workflow. Neko CLI owns commit/tag/push/dispatch; GitHub Actions owns build, GitHub Release creation, and asset publishing from the pushed tag.

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

**Sample Output (with --show):**
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

Generate the future public `plugin-index.json` registry artifact from V2 plugin units, `.neko/release.state.json`, and each plugin manifest. The command does not publish the index, does not commit it as source, and runtime install/update continues to use the current registry fallback until the next plugin-registry milestone.

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
DESCRIPTION                        OPTION          REQUIRED  VALUES
────────────────────────────────────────────────────────────────────────────────────────────────
Type of project being released     project-type    true      frontend, backend, other
Release tool to use                release-system  true      release-it, jreleaser, goreleaser
Initial version (default: 0.1.0)   version         false     semver (e.g. 0.1.0)
Deprecated fallback for --version  metadata        false     semver (e.g. 0.1.0)
Overwrite existing config          force           false     true, false
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

`neko release validate` can validate V2 now. `history`, `contributors`, dry-run planning, and root V1-to-V2 migration are unit-aware. GitHub Actions delivery is valid V2 configuration when `workflow` points to an existing `.github/workflows/<file>.yml|yaml` file. Dry-run planning builds the execution context, materialization plan, local delivery/executor capabilities, planned release commit, unit tag, known release files, push order, workflow reference, dispatch input contract, dispatch status, and V2 Git ownership. V2 GitHub Actions non-dry-run release commands are active and journaled; `neko release resume --unit <unit>` resumes only existing unresolved execution journals. V2 local `release-it` and standalone public dispatch/retry commands are not active.

In Nekocli itself, `plugin-release` and `plugin-ui` are V2 units. `.neko/release.state.json` is authoritative for both plugin versions; `plugin/release/manifest.json` and `plugin/ui/manifest.json` are materialized release files for their selected units. Both plugin units declare plugin metadata in `.neko/release.config.json`; `neko release plugin-index` uses that metadata so adding a releaseable plugin is a V2 unit-config change, not a registry Go-code edit. The generated `plugin-index.json` is not committed as source and install/update still uses the current registry fallback until the next milestone. `make update-manifests` remains a manual compatibility helper and reads V2 state. V2 dry-run planning does not require or resolve `GITHUB_TOKEN`; real GitHub Actions release execution still requires it.

The `plugin-release` unit uses `plugin-release/vX.Y.Z` tags and `.github/workflows/release-plugin-release.yml`. Neko CLI owns state, materialized files, release commit, tag, push, and workflow dispatch. The workflow checks out the dispatched tag, validates `release_sha`, validates the materialized version files and unit config, runs tests, checks `.goreleaser.plugin-release.yaml`, performs a plugin-release-only snapshot build, packages plugin-release archives with that dedicated GoReleaser config, and creates the GitHub Release for the exact prefixed tag with GitHub CLI. The dedicated config must not build or publish the main CLI or `plugin-ui`; it embeds `PLUGIN_RELEASE_VERSION` from the dispatch version into the release plugin binary and archives the committed `plugin/release/manifest.json`.

The `plugin-ui` unit follows the same production pattern with `plugin-ui/vX.Y.Z`, `.github/workflows/release-plugin-ui.yml`, `.goreleaser.plugin-ui.yaml`, `PLUGIN_UI_VERSION`, and `plugin/ui/manifest.json`.

`neko release migrate` can convert a root V1 single-unit repository to V2. It archives `.release.neko.json` as `.release.neko.json.v1.bak`, writes V2 config and state atomically, and uses a temporary recovery journal.

See:

- [Release overview](../release/overview.md)
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
- `.goreleaser.yml` configuration (created by `neko release init`)

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
No .release.neko.json configuration found
Hint: Run 'neko release init' first to initialize the release configuration
```

**CONFIG_EXISTS**
```
.release.neko.json already exists
Hint: Use --force to overwrite
```

**VALIDATION_FAILED**
```
Invalid project type: invalid
Hint: Must be one of: frontend, backend, other
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

### Complete Workflow

```bash
# 1. Initialize a new project
neko release init --project-type=backend --release-system=goreleaser

# 2. Validate the setup
neko release validate --show

# 3. Check current history
neko release history

# 4. Create your first release
neko release patch --dry-run  # Preview first
neko release patch            # Execute

# 5. Create a minor release with new features
neko release minor

# 6. Create a major release for breaking changes
neko release major
```

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
# 1. Initialize release configuration
neko release init --project-type=backend --release-system=goreleaser

# 2. Add plugin units to .neko/release.config.json and .neko/release.state.json

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
