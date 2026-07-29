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

The complete consumer setup is the [Release V2 GitHub Actions Golden Path](../release/github-actions-golden-path.md). Additional Release V2 and plugin registry examples live in [Release V2 Examples](../release/examples.md). Bootstrap ownership across Neko CLI, GitHub Actions, adapters, and consumer workflows is defined in [Release V2 Bootstrap Product Boundary](../release/bootstrap-product-boundary.md).

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

`release init` no longer creates `.release.neko.json`. Existing V1 repositories should use `neko release migrate`. `--kind release` is the CLI default for normal release units; V2 JSON omits `kind` for those units, and they do not use plugin metadata or the plugin registry. `--kind plugin` creates one Neko CLI plugin unit with V2 plugin metadata. Plugin flags without `--kind plugin` are invalid. Plugin unit ids must start with `plugin-`, plugin tag prefixes must be `<unit-id>/v`, and plugin asset prefixes must match the unit id. Use `neko release unit-add` to append units to an existing V2 configuration. `init` and `unit-add` do not generate workflow or executor files; use the separate opt-in `neko release github-workflow-init` command for a configured GitHub Actions workflow. See [Normal release units vs Neko CLI plugin units](../release/examples.md#normal-release-units-vs-neko-cli-plugin-units).

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

#### Setup and migration output modes

`init`, `unit-add`, `migrate`, and `github-workflow-init` support the Core
output formats `table` and `json`. The global `--describe` and `--verbose`
flags are inherited from Core; they are not command-local flags and they can
be combined.

- Default table output keeps the primary result, affected repository-relative
  artifacts, and the next useful action concise. Conflicts and refusals always
  retain their reason and remediation.
- `--describe` adds structured configuration, source/target, comparison,
  validation, artifact, write-plan, outcome, and limitation facts. It does not
  change the operation or create a second machine-data shape.
- `--verbose` adds chronological orchestration logs from the command that owns
  each phase. Logs do not repeat the full describe projection.
- `--output json` remains the complete established machine contract.
  `--describe --output json` has the same domain data, and verbose logs stay
  outside that data.

Init describe output contains `Resolved Configuration`, `Artifact Write Plan`,
`Validation Facts`, and `Limitations`, including the complete resolved unit,
plugin metadata when applicable, and force policy. Its verbose path covers
input validation, repository-state inspection, default resolution, generated
pair validation, artifact preparation, pair persistence, and completion.

Unit Add describe output contains `Resolved Unit`, `Existing Unit Comparison`,
`Artifact Write Plan`, `Validation Facts`, and `Limitations`. Its verbose path
covers existing-pair inspection and reading, input/default resolution,
duplicate detection, updated-pair validation, persistence, and completion.
Duplicate identities are refused and never replaced.

Migrate describe output contains `Source Facts`, `Resolved V2 Configuration`,
`Generated Artifacts`, `Ordered Migration Plan`, `Archive and Journal`,
`Validation Facts`, `Write Plan`, and `Limitations`. Dry-run logs stop after
planning and explicitly report that no migration files were written. Actual
logs report V2 pair writes, verification, V1 archive handling, and journal
completion only after those phases succeed. Post-write failures may retain
recovery evidence; planning failures write nothing.

Workflow Init describe output contains `Workflow Identity`, `Target
Comparison`, `Validation Facts`, `Required Workflow Inputs`, `Write Plan`, and
`Limitations`. The existing YAML preview remains the principal dry-run output
and is not duplicated in describe tables. Verbose output records target
resolution, path and generated-content validation, comparison,
create/unchanged/conflict classification, dry-run or write handling, and
completion. Differing workflow content is always preserved.

Human projections and logs use safe repository-relative paths, emit no token
or credential values, and are ANSI-free when redirected or when `NO_COLOR` is
set. These modes do not alter dry-run, write, overwrite, exit, Git, or network
behavior.

---

### `neko release github-workflow-init`

Create or preview the canonical GitHub Actions Release V2 workflow selected
from an existing config/state pair.

```bash
neko release github-workflow-init [--unit <unit-id>] [--path <configured-path>] [--dry-run]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--unit` | string | | Select one unit and its configured workflow path |
| `--path` | string | | Select an exact configured repository-root-relative workflow path |
| `--dry-run` | bool | `false` | Return the exact generated YAML without writing |

Without a selector, all V2 units must resolve to one unique workflow path.
Multiple units may share that path and are reported as one workflow scope.
Multiple distinct paths require `--unit` or `--path`; the explicit path must
match config, and a combined unit/path selection must agree. Only direct
`.github/workflows/*.yml|yaml` targets are accepted. Absolute, traversal,
nested, protected, unsupported, or symlink-escaping targets fail closed.

This is a create-only ownership contract:

1. A missing target is written once with mode `0644` through target-local
   atomic no-clobber publication.
2. Byte-identical content is already current and is not rewritten.
3. Different content is preserved and returns `WORKFLOW_TARGET_CONFLICT` with
   exit code `1`.
4. `--dry-run` writes nothing and includes summary plus complete generated
   YAML for create, unchanged, and conflict classifications.

Contract version `1` is marked by:

```text
# Generated by Neko Release workflow scaffolding.
# Workflow contract version: 1
# Create-only scaffold: customize consumer-owned steps after generation.
```

The workflow contains only the four required dispatch inputs, minimal
`contents: read`, non-cancelling unit/tag concurrency, exact release-SHA
checkout with full history and tags, pinned Neko CLI and Release Plugin
installation, `ci-validate-context`, and a deliberately failing consumer-owned
extension point. It does not generate a build system, credentials, publication
permissions, GitHub Release, commit, tag, push, or dispatch. Consumers replace
the final step and own builds, tests, signing, artifacts, publication, secrets,
OIDC, deployments, release notes, and any GitHub Release creation.

Generation is token-free, network-free, Git-mutation-free, rooted at the
resolved repository, and independent of process cwd. Existing manually
maintained workflows remain supported and are never silently overwritten.
Default human output uses a concise result plus readable preview text where
preview is the command's purpose. Describe adds identity, comparison,
validation, input, and write-plan facts without repeating that YAML. Stable
JSON keeps machine booleans and includes generated content only for preview.
There is no provider, force, update, or arbitrary consumer-command flag.

---

### `neko release doctor`

Inspect all configured Release V2 units and their GitHub Actions workflows, or
select one unit while retaining checks for every unit sharing its workflow:

```bash
neko release doctor [--unit <unit-id>]
neko release doctor --output json
neko release doctor --verify-remote
neko release doctor --unit <unit-id> --verify-remote
neko release doctor --output json --verify-remote
```

Without `--verify-remote`, the command reads only local V2 config/state and
configured workflow files. The default remains offline, token-free,
deterministic, and mutation-free. Explicit remote verification adds only
bounded GitHub `GET` requests; it never dispatches, uploads, publishes, writes
repository settings, or mutates Git or local files. Public repository,
workflow, tag, and release facts are read anonymously first. A token is
resolved only for a private-repository retry or protected variable, secret-name,
or Actions-policy metadata.

It reports `ready`, `ready_with_warnings`, or `not_ready`, uses exit code `1` only
for `not_ready`, and never runs Git, reads journals, constructs Evidence
stores, or mutates files. JSON contains stable `readiness`, `summary`, `units`,
`workflows`, `diagnostics`, and additive deterministic `verifications` plus
`remote_verification`. The remote summary records `requested`, `status`
(`not_requested`, `complete`, `partial`, or `unavailable`), and verified,
unresolved, and failed counts. Verification states additionally include
`not_attempted`, `unavailable`, `unauthorized`, and `rate_limited` so an access
failure is not misreported as a missing integration. Collections remain empty
arrays rather than `null`; tokens, authorization headers, secret values,
arbitrary variables, raw private response bodies, and absolute paths never
enter output.

Human output starts with the `Release Integration Doctor` title, readiness,
severity counts, and inspected unit and workflow counts. A `Diagnostics`
section contains a compact index that keeps `Severity` and `Code` essential and
admits optional `Target` and `Scope` columns in that priority order. Complete
ordered records follow with a severity/code heading, scope, optional unit, the
full repository-relative workflow path, message, and remediation.
Core fits or removes optional index columns, switches to vertical records, and
wraps detail values from the actual output width. Width-unknown output uses the
deterministic vertical fallback. Terminal width affects only this human view;
JSON remains the stable automation contract.

On an interactive terminal, readiness and non-zero counts use semantic color:
ready/success is green, warnings are yellow, errors are red, recommendations
are cyan, and not-verifiable facts use muted secondary text. Diagnostic
severity and code carry the same severity role; targets, scopes, messages, and
remediation remain neutral. Zero counts remain neutral. A non-empty `NO_COLOR`
disables color, and redirected, piped, JSON, raw JSON, and GitHub output is
ANSI-free. This hierarchy is presentation-only and does not change diagnostic
meaning, order, readiness, or exit codes.

Each diagnostic has one closed severity (`error`, `warning`,
`recommendation`, or `not_verifiable`), source/unit/workflow scope, optional
unit and workflow identity, a stable code, message, and remediation. Errors
produce `not_ready` and exit code `1`; warnings without errors produce
`ready_with_warnings` and exit code `0`; recommendation and not-verifiable
findings alone produce `ready` and exit code `0`.

Checks cover strict V2 source presence, JSON/schema/alignment/recovery safety,
canonical unit executor/delivery/version/tag/workflow facts, workflow path and
YAML safety, dispatch triggers and inputs, permissions, concurrency, checkout,
pinned installation order, context-validator flags and GitHub output wiring,
and the consumer extension point. Supported workflows receive five locally
verified categories: consumer validation/test/build/publication structure,
focused GoReleaser configuration, installation/artifact identity, credential
wiring, and publication/registry identity. Three additional facts retain the
remote-workflow, repository-variable, and exact-dispatch boundaries.

GoReleaser inspection reads only the supported action arguments and focused
version/project/build/archive/checksum/release fields. Installer inspection
cross-checks generic local origin and installer identities, CLI platform/archive/
binary/install-directory rules, and Release Plugin unit/manifest/version/binary/
asset-prefix rules. Credential inspection classifies each `secrets.*` reference
as built-in `GITHUB_TOKEN` or custom, records its job/step, requires
publication-only scope and compatible permissions, and rejects visible echo or
workflow-output exposure without resolving a value. Publication inspection
recognizes real GoReleaser and explicit `gh release` forms, validated tag/SHA
flow, artifact/checksum identity, and release → plugin-index generation → index
publication order. Unsupported shapes remain limited; this is not a general
shell or GoReleaser parser.

Offline mode retains the seven stable limitation codes. Successful explicit
remote checks replace the remote-workflow and repository-variable limitations,
narrow installation uncertainty to download/extraction/execution/loading, and
narrow publication uncertainty to future acceptance. Secret-name metadata
never proves issuance, value validity, expiry, authorization, or service
acceptance. Exact dispatch authorization remains mutation-required, and future
runner/build and publication outcomes remain runtime- or mutation-dependent.
Definite missing workflows, byte mismatches, disabled Actions/workflows,
missing or invalid recognized variables, missing custom-secret metadata, and
missing exact installation releases/assets become focused errors. Unauthorized,
rate-limited, unavailable, unsupported, and ambiguous private-resource results
remain honest partial evidence and do not create false structural errors.

Remote workflow content is compared byte-for-byte on the repository default
branch. Only locally recognized `NEKO_VERSION` and
`NEKO_RELEASE_PLUGIN_VERSION` pins and locally referenced custom secret names
are queried; built-in `GITHUB_TOKEN` is never queried as a repository secret.
Release and asset checks use exact locally derived tags and names—never
`/releases/latest`, tag-prefix discovery, newest-run selection, or fuzzy asset
matching. Workflow runs are not queried because Doctor currently owns no exact
durable run ID. See [Remote Doctor verification](../release/integration-doctor-remote-verification.md)
for endpoints, authentication, classifications, and remaining boundaries.

Permission inspection follows GitHub Actions scope replacement directly. An
omitted job declaration inherits the workflow declaration; an explicit job
declaration replaces it for that job. Supported scopes use their GitHub-valid
`read`, `write`, and `none` values (`id-token` has `write|none`; `models` and
`vulnerability-alerts` have `read|none`), plus the scalar `read-all` and
`write-all` forms and an empty mapping.
`PERMISSIONS_IMPLICIT` remains a warning when neither the workflow nor every
job declares permissions. Any workflow-level write, `write-all`, unsupported
scope/value/shape, or job write without matching same-job publication evidence
produces the warning `PERMISSIONS_BROAD`.

The local evidence is intentionally narrow. `contents: write` is justified
only by a same-job GoReleaser `release` action that is not snapshot/skip-publish,
or a direct `gh release create`/`gh release upload` command. `packages: write`
is justified only by a direct `docker push ghcr.io/...` command or a
`docker/build-push-action` step with `push: true` and a literal `ghcr.io/...`
tag. Job or step names, paths, secret presence, and a job named `publish` are
not evidence; credential echo/output paths are inspected only to reject
exposure. No OIDC publication form is currently recognized, so
`id-token: write` remains conservative. The Doctor reads only the two known
repository-confined plugin-index scripts and local installer/registry contracts;
it does not execute them or prove remote success.

---

### `neko release units`

List every configured Release V2 unit and its current local state:

```bash
neko release units
neko release units --output json
```

The flat command has no unit selector or verification flags. Units are ordered
by canonical unit ID. Human output uses the responsive table contract with
essential `Unit`, `Version`, and `Status` columns. `Name`, `Tag prefix`,
`Executor`, `Delivery`, `Workflow`, `Working directory`, and concise issue
codes are optional columns admitted in that priority order. Narrow terminals
retain the essential facts or use bounded vertical records when they cannot
fit; width-unknown output is deterministic vertical output. Invalid and
incomplete units remain visible.

Each JSON unit can contain `id`, optional `display_name`, canonical `version`,
raw `configured_version`, `tag_prefix`, `tag_shape`, `configured_tag`,
`executor`, `delivery`, `workflow_path`, `working_directory`, `alignment`,
`issues`, and `issue_codes`. The current version comes only from
`.neko/release.state.json`. An invalid raw version remains in
`configured_version` while `version` and `configured_tag` are absent.
`tag_shape` and `configured_tag` are derived through canonical `TagSpec`;
`configured_tag` is a configured value for the validated current state
version, not evidence that a local or remote Git tag exists. No next version is
calculated.

Alignment is one of `aligned`, `config_only`, `state_only`, or `invalid`.
Unit issues use `error` or `warning` severity and the stable codes
`UNIT_STATE_MISSING`, `UNIT_CONFIG_MISSING`, `UNIT_VERSION_INVALID`,
`UNIT_TAG_PREFIX_INVALID`, `UNIT_TAG_PREFIX_CONFLICT`,
`UNIT_EXECUTOR_INVALID`, `UNIT_DELIVERY_INVALID`,
`UNIT_WORKFLOW_PATH_INVALID`, and `UNIT_CONFIG_INVALID`. The repository status
is `valid`, `has_issues`, or `source_invalid`. Summary fields are `total`,
`aligned`, `incomplete`, `invalid`, `workflow_paths`, and `source_usable`;
distinct workflow paths are also returned lexically under `workflow_paths`.
Unit issues are ordered by severity, unit ID, code, and message.

Expected source problems are structured output, not command crashes. Source
issue codes are `V2_SOURCE_INSPECTION_FAILED`, `MIXED_RELEASE_SOURCES`,
`V1_SOURCE_UNSUPPORTED`, `V2_SOURCE_MISSING`, `V2_CONFIG_INVALID`,
`V2_STATE_INVALID`, `V2_SCHEMA_UNSUPPORTED`, `V2_RECOVERY_BLOCKED`,
`V2_CONFIG_MISSING`, `V2_STATE_MISSING`, and `V2_SOURCE_EMPTY`. A missing half
of the V2 pair can still yield explicit config-only or state-only rows; unsafe,
malformed, mixed, V1-only, unsupported, or recovery-blocked sources do not
produce trusted rows. `valid` exits `0`; `has_issues` and `source_invalid` exit
`1`.

The overview reuses strict V2 loading, canonical version/tag/unit policy, and
explicit repository-root handling. It is offline and strictly read-only: it
does not open or parse workflow YAML, call the integration doctor, inspect Git
or tags, read tokens, contact the network, inspect build systems, plan a future
release, read journals or Evidence, execute releases, or mutate config, state,
workflows, Git, cwd, environment, file modes, or mtimes. Use `release doctor`
for GitHub Actions workflow readiness and `release pipeline` for one unit's
configured execution path.

---

### `neko release pipeline`

Inspect the configured Release V2 pipeline for one unit without executing it:

```bash
neko release pipeline --unit cli
neko release pipeline --unit plugin-release
neko release pipeline --unit plugin-ui
neko release pipeline --unit cli --describe
neko release pipeline --unit cli --output json
neko release pipeline --unit cli --verify-remote
neko release pipeline --unit cli --verify-remote --describe
neko release pipeline --unit cli --verify-remote --output json
```

The command is V2-only. Multi-unit repositories require `--unit`; omission is
accepted only for a single-unit repository. Default inspection is local,
offline, token-free, and read-only. It always shows the concise Summary plus
actionable Findings; global `--describe` adds complete structured evidence;
global `--verbose` is a deterministic no-op. Explicit `--verify-remote` is the
sole bounded GitHub GET-only path and is independent from both global
presentation flags.

JSON remains the complete invariant schema-version-1 machine response in every
presentation mode. Valid lifecycle observations, including blocked, uncertain,
and rejected, exit `0`; typed invalid requests/configuration and structurally
invalid local evidence exit `1`. `completed` means accepted handoff evidence,
not publication completion. Pipeline never plans a future version, resumes,
retries, repairs, cleans, dispatches, uploads, or publishes.

See the canonical [Pipeline inspection contract](../release/cli-reference.md#pipeline-inspection)
for the complete JSON field inventory, frozen machine vocabularies, ordering,
nullability, lifecycle/runtime/verification/recovery/exit semantics, support
fixtures, release-asset target matrix, presentation behavior, and read-only/
remote safety boundaries.

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

Dry-run does not require `GITHUB_TOKEN`. Patch, Minor, and Major share one
Release-owned presentation vocabulary. Default dry-run keeps release identity,
previous/planned version and tag, executor/delivery, ordered principal
operations, primary materialized files, the no-mutation boundary, blockers,
and the preview result. Ordinary completion is concise: change, unit,
previous/resulting version, tag, executor/delivery, lifecycle/handoff result,
and next action.

Global `--describe` adds `Source and Configuration`, declared and materialized
release files, complete operations, `Execution Evidence`, `Git and Handoff`,
ownership, safe journal/handoff facts, and `Limitations`. Global `--verbose`
independently adds chronological phases from the authoritative V1/V2 path. It
does not duplicate policy or structured detail. Human paths are
repository-relative or safe labels; captured logs omit repository roots, raw
Git output, tokens, authorization values, config/journal payloads, and raw
provider bodies.

The existing machine rows and V1/V2 outcome variants remain unchanged across
default, describe, verbose, combined, redirected, `NO_COLOR`, and JSON modes.
Unknown dispatch or ambiguous push outcomes remain actionable by default and
must not be retried blindly; inspect with
`neko release resume --unit <unit> --dry-run`.

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

`plan` reports the selected release source and unit, current version, requested change, next version, tag, planned materialized files, known release files, local readiness, local blockers, and explicit limitations. Human-readable output presents each limitation with its own semantic label, including local-only inspection, omitted Evidence inspection, omitted remote checks, and token-free operation; it no longer combines them into one pipe-delimited terminal value. The underlying typed limitations and the established JSON `data.items` projection are unchanged.

Property/value output uses the actual terminal width. Normal and wide output bound the label column and wrap long paths, status text, blockers, and limitation descriptions with continuation lines aligned below the value column. Very narrow, piped, redirected, or otherwise width-unknown output uses deterministic vertical properties. `--output json` remains the machine-readable response and excludes presentation metadata.

The command is strictly read-only and token-free: it does not read `GITHUB_TOKEN`, inspect remotes, inspect execution or dispatch journals, write config/state/manifests, mutate Git, dispatch workflows, publish releases, or run executors.

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

Validate the complete release configuration.

**Usage:**
```bash
neko release validate [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--show` | bool | `false` | Display structured release configuration details and unit summaries |
| `--unit` | string | | Focus displayed V2 unit details; the complete repository is still validated. |

**Examples:**
```bash
# Validate configuration
neko release validate

# Show configuration details
neko release validate --show
```

Without `--show`, human output stays concise as a real responsive summary table
and does not include a unit-detail table:

```
Release Configuration Validation

PROPERTY          VALUE
────────────────────────────────────────────
Status            ✓ Valid
Source            V2 config and state
Schema            v2
Configuration     .neko/release.config.json
State             .neko/release.state.json
Configured units  3
```

For V2 repositories, `--show` adds one responsive row per displayed unit.
`Unit`, `Version`, and `Kind` are essential columns; `Executor`, `Delivery`,
and `Workflow` are admitted in that priority order when terminal width permits.
Complete details follow the table in this order: optional display name,
version, kind, working directory, tag prefix, executor, delivery, workflow, and
paths. Paths are one entry per line. Plugin name, manifest, asset prefix, and
binary follow only for units with `kind: plugin`.

Interactive semantic color uses the existing Core roles: valid status is
success-colored, unit IDs are emphasized, and versions, plugin kinds, and unit
detail headings use the information role. Workflows, materialized paths, and
ordinary metadata remain neutral. Redirected and machine-readable output remain
ANSI-free.

V1 `--show` uses one virtual `default` unit with essential `Unit`, `Version`,
and `Project type` columns plus optional `Release system`. Its details contain
the legacy project name, owner, type, release system, and version; no V2 state
path is displayed.

`--unit` never narrows validation. V2 config and state are decoded and validated
as a complete pair before the selected unit is used to focus displayed details.
Public `--output json` retains the established `data.items` schema, values, and
order and does not include presentation metadata.

---

### `neko release ci-validate-context`

Validate the four canonical dispatched Release V2 values against the checked-out
repository before build or publication.

**Usage:**
```bash
neko release ci-validate-context \
  --unit <unit> \
  --version <version> \
  --tag <tag> \
  --release-sha <full-lowercase-commit-object-id>
```

All four flags are required strings. Default output is an ordered human
property/value view. `--output json` emits canonical typed data. GitHub Actions
uses `--output github --github-output-file "$GITHUB_OUTPUT"`; the ten stable
outputs are `unit`, `display_name`, `version`, `tag_prefix`, `tag`,
`release_sha`, `working_directory`, `executor`, `delivery`, and `workflow`.

The command requires an unambiguous valid V2 source and complete local tag
history. It accepts matching detached HEAD and both annotated and lightweight
tags. It performs no fetch, network request, token lookup, filesystem mutation,
Git mutation, dispatch, build, publication, or journal operation. Validation or
output failures return a nonzero CLI exit. The complete JSON key and error-code
contract is documented in the [Release CLI reference](../release/cli-reference.md#ci-release-context-validation).

---

### `neko release plugin-index`

Generate the public `plugin-index.json` registry artifact from V2 plugin units, `.neko/release.state.json`, and each plugin manifest. The command itself does not publish the index and does not commit it as source. Plugin release workflows publish or replace the `plugin-index.json` asset on the mutable `plugin-registry` GitHub Release after successful plugin releases. Runtime plugin discovery, install, and update read that asset as the source of truth; release-prefix fallback discovery has been removed.

**Usage:**
```bash
neko release plugin-index [flags]
```

**Command flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--output-file` | string | | Persist `plugin-index.json` to an explicit file; omit for raw stdout |
| `--check` | bool | `false` | Validate that the index can be generated without writing a file |
| `--pretty` | bool | `true` | Pretty-print raw schema-v1 JSON on stdout or in the persisted file |
| `--repository` | string | `nekoman-hq/neko-cli` | Repository identifier to include in the generated index |

**Global plugin-response flags:**

| Flag | Ownership | Plugin Index behavior |
|------|-----------|-----------------------|
| `--describe` | Core | No-op for undecorated raw table output; complete structured facts for check and persist |
| `--verbose` | Core transport; plugin log production through context | No-op for raw render; safe validation or persistence phases for check/persist |
| `--output` | Core | Selects `table`, `json`, `wide`, or `github`; never a file path |
| `--github-output-file` | Core | Destination only for Core GitHub response output; it is not the Plugin Index artifact path |

`--output-file` accepts either a clean repository-root-relative path or an
explicit absolute artifact path. Relative paths are resolved from the
repository root, not from the shell's current directory. Absolute paths are
retained for CI temporary artifacts such as `/tmp/plugin-index.json`.
Repository-contained output cannot overwrite release config/state, release
recovery evidence, Git internals, or plugin manifests used as index inputs.
Existing target directories and target symlinks are rejected. Missing parents
use mode `0755`, new files use `0644`, an existing file keeps its mode, and
replacement is target-local and atomic.

The former local `--output <path>` spelling is intentionally not retained.
Core interprets `--output json` and `--output table` only as response formats;
unsupported path-like values fail Core output validation before the plugin
runs, and no file fallback occurs.

**Mode and JSON contracts:**

| Mode | Default | Describe | Verbose | JSON |
|------|---------|----------|---------|------|
| Raw render | Exact schema-v1 artifact; pretty by default and compact with `--pretty=false` | Intentional no-op for raw table output | Intentional no-op | `--output json` is the Core public response envelope containing `data.raw`, not a second index schema |
| Check (`--check`) | Read-only validation summary with repository/plugin counts and next action | Source resolution, repository/plugin inventories, ordering/duplicate facts, validation checks, and limitations | Source derivation and validation phases only | Established public response with `Status=ok`, `Plugins`, and `Repository` rows |
| Persist (`--output-file`) | Successful write summary with safe target label, formatting, counts, validation, and next action | Source/inventory facts, validated target and atomic write plan, outcome, and limitations | Construction, validation, target resolution, write preparation/completion, result confirmation | Established public response with `Status=written`, `Output`, `Plugins`, and `Repository` rows |

The raw schema-v1 artifact keeps its exact top-level and plugin field order,
plugin-name ordering, JSON escaping, empty-array behavior, pretty/compact
formatting, and single trailing newline. Default raw stdout is not wrapped in a
Core response and contains no logs, presentation metadata, timestamp, or ANSI.

**Examples:**
```bash
# Print the generated index JSON
neko release plugin-index

# Print compact schema-v1 JSON
neko release plugin-index --pretty=false

# Validate generation without writing
neko release plugin-index --check

# Render the structured check response as Core public JSON
neko release plugin-index --check --output json

# Write to a temporary artifact path
neko release plugin-index --output-file /tmp/plugin-index.json

# Render the structured write response as Core public JSON
neko release plugin-index --output-file /tmp/plugin-index.json --output json
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

Default human output summarizes the source and destination contracts, dry-run
state, planned action count, V2 targets, archive decision, and next action.
Describe adds the normalized source, resolved V2 pair, artifact summaries,
ordered actions, archive/journal policy, validation results, write outcomes,
and limitations. Generated config/state documents remain available in JSON
instead of being dumped into normal terminal output. Migration blockers state
whether no write began or recovery evidence may remain.

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
| `--identity` | string | | Inspect one record using an unambiguous 8-64 character lowercase hexadecimal identity prefix after family and unit filters |

The command is read-only, offline, and token-free. Default human output contains an `Evidence Summary` and concise inventory. `Family`, exact `Identity`, `State`, `Classification`, and `Action` are essential; `Unit`, `Version`, `Tag`, and `Linked execution` are optional. Every actionable classification and diagnostic remains visible, including corrupt, unsupported, conflicting, ambiguous, unlinked, active, resumable, uncertain, terminal, and manual-recovery conditions. Narrow output keeps essential fields, while redirected or width-unknown output uses deterministic vertical records.

Global `--describe` adds focused `Execution Evidence`, `Dispatch Evidence`, `Linkage`, `Local Git Evidence`, `Classification`, `Recovery Relevance`, and `Limitations` sections. These contain only facts retained by the evidence model, safe repository-relative path labels, and digests. Evidence does not perform a second local Git inspection; missing or mismatched commit/tag validation remains owned by Resume and Pipeline. Global `--verbose` is an intentional no-op, so it does not narrate deterministic reads.

Use JSON to obtain the complete identity, then inspect all safe fields for one record:

```bash
neko release evidence --family release-execution --unit api --output json
neko release evidence --family release-execution --unit api --identity 0123abcd
```

Identity inspection is read-only. The prefix is trimmed, must contain 8 through 64 lowercase hexadecimal characters, and is not case-normalized. Family and unit filters are applied first; zero matches and multiple matches are errors, and a full 64-character identity is accepted.

The legacy JSON contract is frozen exactly: summary and identity-filtered responses retain complete `data.items`, typed `data.evidence`, and `data.diagnostics`, including established duplicated representations, field casing, value types, ordering, and nullability. `--describe`, `--verbose`, and their combination do not alter domain data or status; human presentation metadata and logs remain excluded. Human output does not print tokens, authorization headers, raw response bodies, raw Git output, absolute developer roots, or full evidence files.

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
| `--identity` | string | | Exact 64-character Evidence identity from `neko release evidence --output json`; prefixes are not accepted for archival |
| `--digest-sha256` | string | | Current evidence digest from inspection output |
| `--confirm-archive` | bool | `false` | Required explicit confirmation |

Only completed `release-execution`, completed `v1-compensation`, and completed `v2-pair-recovery` evidence can be archived. Default success output reports the selected family and identity, confirmation state, digest match, archive result, safe source and target labels, and next action. Missing or invalid family, missing or invalid identity/digest, absent confirmation, unknown identity, stale digest, ineligible or missing source, existing target, and filesystem errors retain specific actionable failures.

Global `--describe` adds existing validation facts, source classification, confirmation contract, guarded write plan, final outcome, and limitations. Global `--verbose` reports the chronological guarded phases: request validation, family resolution, evidence read/classification, exact identity resolution, digest verification, target check, private write preparation/completion, byte verification, selected-source removal, completion, or the refusal phase. Logs abbreviate identities/digests and use safe repository-relative labels.

The command re-observes the evidence, rejects stale digests, writes an exact `0600` archive copy in a private `0700` archive directory, verifies the copy, and only then removes the completed source evidence. No mutation occurs before confirmation, on identity/digest refusal, on a missing source, or on target conflict. It changes no repository worktree/index, commit, tag, remote, or unrelated evidence. Existing JSON, error envelopes, exits, idempotency, conflict handling, and rollback limits are unchanged. It does not support force, repair, retry, arbitrary paths, dispatch archival, or migration archival.

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

The `plugin-release` unit uses `plugin-release/vX.Y.Z` tags and `.github/workflows/release-plugin-release.yml`. Neko CLI owns state, materialized files, release commit, tag, push, and workflow dispatch. The workflow's read-only `validate` job checks out the dispatched release SHA with full history and tags, installs repository-variable-pinned Neko CLI and Release Plugin versions, runs the canonical `ci-validate-context` command, exports its four validated values as job outputs, validates the materialized plugin manifest, runs tests, checks `.goreleaser.plugin-release.yaml`, and performs a plugin-release-only snapshot build. The dependent `publish` job alone grants `contents: write`, checks out the validated SHA again without persisted credentials, packages plugin-release archives with that dedicated GoReleaser config, and creates the GitHub Release for the exact prefixed tag with GitHub CLI using `secrets.GITHUB_TOKEN`. After that publish succeeds, it generates and validates `plugin-index.json`, then uploads/replaces that single asset on the mutable `plugin-registry` release. The dedicated config must not build or publish the main CLI or `plugin-ui`; it embeds `PLUGIN_RELEASE_VERSION` from the validated version output into the release plugin binary and archives the committed `plugin/release/manifest.json`.

The `plugin-ui` unit follows the same two-job, validated-output production pattern with `plugin-ui/vX.Y.Z`, `.github/workflows/release-plugin-ui.yml`, `.goreleaser.plugin-ui.yaml`, `PLUGIN_UI_VERSION`, and `plugin/ui/manifest.json`.

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
| `plugin-release` | `PLUGIN_RELEASE_VERSION=${{ steps.release-context.outputs.version }}` for validation builds; `${{ needs.validate.outputs.version }}` for publication |
| `plugin-ui` | `PLUGIN_UI_VERSION=${{ steps.release-context.outputs.version }}` for validation builds; `${{ needs.validate.outputs.version }}` for publication |

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
- [Release V2 GitHub Actions Golden Path](../release/github-actions-golden-path.md)
- [GoReleaser Documentation](https://goreleaser.com/intro/)
- [JReleaser Documentation](https://jreleaser.org/guide/latest/)
- [release-it Documentation](https://github.com/release-it/release-it)
