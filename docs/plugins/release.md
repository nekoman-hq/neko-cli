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

For V2 repositories, `patch`, `minor`, and `major` support dry-run planning with `--unit`. Non-dry-run V2 release execution is still blocked.

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

For V2 repositories, `--show` displays schema type, units, versions, working directories, tag prefixes, executor, delivery, and paths.

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

`neko release validate` can validate V2 now. `history`, `contributors`, and dry-run planning are unit-aware. V2 release execution, migration, state persistence for bumps, executor-context rewiring, and GitHub Actions dispatch are not active yet.

See:

- [Release overview](../release/overview.md)
- [Release configuration](../release/configuration.md)
- [Release state](../release/state.md)
- [Unit selection](../release/unit-selection.md)
- [Tag strategy](../release/tag-strategy.md)
- [CLI reference](../release/cli-reference.md)
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
3. Injects plugin version environment variables (if `.plugin.release.neko.json` exists)
4. Runs `goreleaser release` with injected environment variables
5. Handles rollback on failure

**Files managed:**
- `.goreleaser.yml`
- `.plugin.release.neko.json` (optional, for plugin-based projects)
- Git tags

**Plugin-Based Projects:**

For projects using a plugin architecture (like Neko CLI itself), the release plugin automatically injects plugin version information as environment variables during the GoReleaser build process. This allows you to embed plugin versions directly into your binary.

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

## Plugin Version Injection

When Neko CLI releases itself, it needs to embed the versions of bundled plugins into the binary. The release plugin handles this through automatic environment variable injection.

### How It Works

The release plugin reads `.plugin.release.neko.json` and converts plugin versions into environment variables that GoReleaser can access during the build process.

**Flow:**
```
.plugin.release.neko.json → Environment Variables → GoReleaser → Binary
```

### Configuration File

Create `.plugin.release.neko.json` in your project root:

```json
{
  "plugins": {
    "release": "2.3.1",
    "deploy": "1.0.5",
    "test": "0.9.2"
  }
}
```

### Environment Variable Mapping

Each plugin entry is converted to an environment variable:

| Plugin Entry | Environment Variable |
|--------------|---------------------|
| `"release": "2.3.1"` | `PLUGIN_RELEASE_VERSION=2.3.1` |
| `"deploy": "1.0.5"` | `PLUGIN_DEPLOY_VERSION=1.0.5` |
| `"test": "0.9.2"` | `PLUGIN_TEST_VERSION=0.9.2` |

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

- **File not found:** Plugin version injection is skipped (optional feature)
- **Parse error:** Logged but doesn't stop the release process
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

If a release fails, Neko attempts to automatically rollback:

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
| `GITHUB_TOKEN` | Required for GitHub releases |
| `PLUGIN_{NAME}_VERSION` | Auto-injected plugin versions (from `.plugin.release.neko.json`) |

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

# 2. Create plugin version configuration
cat > .plugin.release.neko.json << EOF
{
  "plugins": {
    "release": "2.3.1",
    "deploy": "1.0.5"
  }
}
EOF

# 3. Configure GoReleaser to use plugin versions
# Edit .goreleaser.yml and add to ldflags:
# - -X main.ReleasePluginVersion={{ .Env.PLUGIN_RELEASE_VERSION }}
# - -X main.DeployPluginVersion={{ .Env.PLUGIN_DEPLOY_VERSION }}

# 4. Release with embedded plugin versions
neko release patch
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

Ensure `GITHUB_TOKEN` is set:
```bash
export GITHUB_TOKEN=your_token_here
```

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
