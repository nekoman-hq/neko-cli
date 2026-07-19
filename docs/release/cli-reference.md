# Release CLI Reference
## General

```bash
neko release init --executor goreleaser --delivery github-actions --workflow .github/workflows/release-cli.yml
neko release init --unit plugin-release --kind plugin --plugin-name release --plugin-manifest plugin/release/manifest.json --plugin-asset-prefix plugin-release --plugin-binary-name plugin-release --executor goreleaser --delivery github-actions --workflow .github/workflows/release-plugin-release.yml --tag-prefix plugin-release/v
neko release unit-add --unit api --executor goreleaser --delivery github-actions --workflow .github/workflows/release-api.yml --tag-prefix api/v --paths "apps/api/**"
neko release github-workflow-init --dry-run
neko release doctor
neko release units
neko release init-options
```

`init` creates V2 `.neko/release.config.json` and `.neko/release.state.json` files for one release unit. Use `--kind release` or omit `--kind` for a normal service, app, CLI, SDK, library, or backend module; V2 JSON omits `kind` for normal units. Use `--kind plugin` plus `--plugin-name`, `--plugin-manifest`, `--plugin-asset-prefix`, and `--plugin-binary-name` only for a Neko CLI plugin unit. Plugin flags without `--kind plugin` are invalid. Plugin unit ids must start with `plugin-`, plugin tag prefixes must be `<unit-id>/v`, and plugin asset prefixes must match the unit id. `init` no longer creates `.release.neko.json` or initializes executor-specific tool files. Existing V1 repositories should use `neko release migrate`. `github-actions` delivery requires `--workflow .github/workflows/<file>.yml|yaml`; `init` and `unit-add` do not generate workflows or executor configuration. Use the separate `github-workflow-init` command for opt-in workflow scaffolding from an existing V2 pair.

`unit-add` appends one unit to existing V2 config/state. It uses the same unit flags as `init`; the plugin metadata flags are only for `--kind plugin` Neko CLI plugin units. Normal repositories can contain only normal release units and need no plugin metadata or plugin registry. It requires both `.neko/release.config.json` and `.neko/release.state.json`, preserves existing units, and never overwrites an existing unit. It does not generate workflow files, GoReleaser config files, plugin manifests, source directories, or any release artifacts. See [Normal release units vs Neko CLI plugin units](examples.md#normal-release-units-vs-neko-cli-plugin-units).

## V1

V1 uses `.release.neko.json` and supports the existing release commands.

```bash
neko release patch --dry-run
neko release plan --change patch
neko release patch
neko release minor
neko release major
neko release history
neko release contributors
neko release validate --show
```

`--unit default` is accepted for V1. Any other unit is rejected.

## V2

V2 uses:

```text
.neko/release.config.json
.neko/release.state.json
```

Supported now:

```bash
neko release migrate
neko release migrate --dry-run
neko release validate
neko release validate --unit api --show
neko release history --unit api
neko release contributors --unit web
neko release patch --unit api --dry-run
neko release minor --unit api --dry-run
neko release major --unit api --dry-run
neko release plan --change patch --unit api
neko release plan --change minor --unit api
neko release plan --change major --unit api
neko release doctor
neko release doctor --unit api
neko release doctor --output json
neko release units
neko release units --output json
neko release github-workflow-init --dry-run
neko release github-workflow-init --unit api
neko release github-workflow-init --path .github/workflows/release-api.yml
neko release ci-validate-context --unit api --version 2.4.0 --tag api/v2.4.0 --release-sha <full-commit-object-id>
neko release patch --unit api
neko release minor --unit api
neko release major --unit api
neko release resume --unit api
neko release resume --unit api --dry-run
neko release evidence
neko release evidence --family release-execution
neko release evidence --family release-execution --unit api --identity 0123abcd
neko release evidence-archive --family release-execution --identity <sha256> --digest-sha256 <sha256> --confirm-archive
neko release plugin-index
neko release plugin-index --check
neko release plugin-index --output /tmp/plugin-index.json
```

For `plugin-index --output`, relative paths are resolved from the repository root. Explicit absolute paths remain supported for CI or temporary artifacts. Repository-contained output is blocked from overwriting release config/state, recovery evidence, Git internals, or plugin manifest inputs.

### Validate presentation

`neko release validate` validates the complete repository release source. In V2 this
means strict decoding and repository-wide validation of both
`.neko/release.config.json` and `.neko/release.state.json`, even when `--unit`
is supplied. `--unit` only focuses displayed V2 details.

Default human output is a concise `Release Configuration Validation` summary
with status, source, schema, config path, V2 state path, an explicit selected
unit when supplied, and the complete configured-unit count for V2. It does not
include a unit table.

`--show` adds a responsive V2 unit table. Its essential columns are `Unit`,
`Version`, and `Kind`; optional columns are considered in `Executor`,
`Delivery`, `Workflow` order. Complete per-unit details follow the table in
version, kind, working directory, tag prefix, executor, delivery, workflow, and
paths order. Paths render one per line, and plugin name, manifest, asset prefix,
and binary are appended only for plugin units. V1 uses one virtual `default`
row with essential `Unit`, `Version`, and `Project type`, optional `Release
system`, and legacy-only details.

The flags are:

```text
--show  Display structured release configuration details and unit summaries
--unit  Focus displayed V2 unit details; the complete repository is still validated.
```

Human presentation metadata does not alter the established public JSON
`data.items` schema, values, or ordering.

`neko release github-workflow-init [--unit <unit-id>] [--path
<configured-path>] [--dry-run]` creates the canonical GitHub Actions Release V2
workflow without overwriting consumer content. All flags are optional. With no
selector, the V2 pair must contain one unique configured workflow path; shared
paths produce one central workflow. Multiple distinct paths require an exact
unit or path selection. `--path` must exactly match at least one configured
unit, and when combined with `--unit` it must match that unit's path.

The only accepted targets are configured, repository-root-relative
`.github/workflows/*.yml` or `.yaml` files directly below that directory.
Absolute paths, traversal, nested paths, protected release files, unsupported
names/extensions, target symlinks, and parent symlink escapes are rejected.

The command is create-only:

- missing target: atomically create exact canonical bytes with mode `0644`;
- byte-identical target: return success without rewriting;
- different target: return `WORKFLOW_TARGET_CONFLICT` with exit code `1` and
  preserve the file;
- `--dry-run`: make no write and return classification plus the complete
  generated YAML, including for a conflict.

Human-readable output uses ordered target/status/action/scope/version/write/guidance
fields; preview uses readable preformatted YAML. JSON returns typed `target`,
`classification`, `action`, `written`, `unchanged`, `dry_run`,
`contract_version`, `selected_unit`, `units_using_workflow`, and `guidance`;
`generated_content` appears only for preview. The command supports `table` and
`json`, not GitHub output mode. It reads no token, contacts no network, runs no
Git operation, and never commits, pushes, dispatches, publishes, or creates a
GitHub Release.

Generated contract version `1` installs pinned Neko CLI and Release Plugin
versions from repository variables, validates the exact four dispatch inputs
through `ci-validate-context`, and ends in a deliberately failing
consumer-owned extension point. Consumers own build systems, secrets,
permissions, publication, signing, deployment, release notes, and GitHub
Release creation. There is no provider, force, managed-update, or arbitrary
consumer-command flag.

Follow the [Release V2 GitHub Actions Golden Path](github-actions-golden-path.md) for the complete installation-through-publication workflow. See [Release V2 Examples](examples.md) for additional copy-ready init, unit-add, release, plugin-index, and plugin install/update flows.

V2 non-dry-run release commands are active only for `delivery: github-actions`. `local` is not a supported executable V2 delivery mode; existing V2 configs using it are rejected during validation before planning or execution. Dry-run output includes:

```text
unit
currentVersion
nextVersion
tag
executor
delivery
workflow
dispatch
dispatchRef
dispatchInputs
journalIdentity
journalLocation
workingDirectory
unitRoot
stateChange
materializedFiles
knownReleaseFiles
plannedReleaseCommit
plannedTag
plannedPushOrder
toolOwnership
v2GitOwnership
stateCommitGuarantee
executorStart
```

`neko release plan --change <patch|minor|major> [--unit <unit>]` is a dedicated local plan inspection command. It reports the selected release source and unit, current version, requested change, next version, tag, planned materialized files, known release files, local readiness, local blockers, and explicit limitations. Human-readable output gives every limitation its own semantic label instead of one pipe-delimited value. At a known width, property labels are bounded and long values wrap with aligned continuation lines; very narrow or width-unknown output uses vertical properties. The typed plan facts and established JSON `data.items` projection are unchanged. The command does not read tokens, inspect remotes, inspect execution or dispatch journals, write files, mutate Git, dispatch workflows, publish releases, or start executors. It is separate from `--dry-run`: dry-run keeps the existing release preview contract, while `plan` is for stable local planning facts.

`neko release doctor [--unit <unit-id>]` is a strictly local Release V2
integration inspection. By default it checks all configured units and unique
workflow paths; selecting a unit still retains every unit sharing that
workflow. It returns `ready`, `ready_with_warnings`, or `not_ready`, with exit
code `1` for `not_ready`. JSON exposes `readiness`, severity counts, ordered
unit/workflow facts, and ordered diagnostics. The doctor does not use tokens,
network clients, Git commands, journal stores, Evidence writers, or filesystem
writers. Source validation only reads the local V2 pair-recovery readiness
marker already owned by the strict config/state contract.

Human-readable output presents readiness and counts first, followed by a compact
diagnostic index and complete responsive details. `Severity` and `Code` are
essential index columns; optional `Target` then `Scope` are admitted while the
actual width permits. Details retain the full workflow path, message, and
remediation and wrap at known widths. Narrow output can use vertical records,
and width-unknown or non-terminal output uses the deterministic vertical
fallback. These layout choices do not alter JSON or raw JSON.

Diagnostics use the closed severities `error`, `warning`, `recommendation`,
and `not_verifiable`. Their stable fields are `severity`, `scope`, optional
`unit`, optional `workflow`, `code`, `message`, and `remediation`. Ordering is
deterministic by severity, scope, unit, workflow, code, and message. A generated
canonical workflow is recognized byte-for-byte but remains `not_ready` while
its deliberately failing consumer placeholder is present. A structurally
equivalent manual workflow is supported; custom build/publication correctness
remains explicitly not verifiable.

`neko release units` is the flat, Release V2-only inventory command. It has no
command-specific flags and lists every config or state unit in canonical unit
ID order. Current versions come only from state. Canonical versions are exposed
as `version`; a safe raw invalid value remains `configured_version`. Tag facts
come from canonical `TagSpec`: `tag_shape` is version-independent and
`configured_tag` uses only the validated current state version. Neither field
claims that a Git tag exists, and the command never calculates a future
version.

The unit alignment classification is closed: `aligned` means config and state
exist and canonical fields validate; `config_only` and `state_only` identify a
missing pair member; `invalid` identifies malformed or conflicting unit facts.
Unit issues contain `severity`, `unit`, `code`, `message`, and `remediation`.
Stable unit codes are `UNIT_STATE_MISSING`, `UNIT_CONFIG_MISSING`,
`UNIT_VERSION_INVALID`, `UNIT_TAG_PREFIX_INVALID`,
`UNIT_TAG_PREFIX_CONFLICT`, `UNIT_EXECUTOR_INVALID`,
`UNIT_DELIVERY_INVALID`, `UNIT_WORKFLOW_PATH_INVALID`, and
`UNIT_CONFIG_INVALID`.

JSON `data` contains `status`, `summary`, `units`, `workflow_paths`, and an
optional `source_issue`. `status` is `valid`, `has_issues`, or
`source_invalid`. Summary contains numeric `total`, `aligned`, `incomplete`,
`invalid`, and `workflow_paths` counts plus boolean `source_usable`. Unit rows
contain only stable inventory facts: `id`, optional `display_name`, state
version fields, tag fields, executor/delivery/workflow/working-directory
fields, `alignment`, `issues`, and `issue_codes`. Empty issue lists remain JSON
arrays; absent optional facts are omitted. Units sort by ID, issues by severity,
unit, code, and message, and distinct workflow paths lexically.

Human-readable output uses a responsive `presentation.Table`. `Unit`, `Version`,
and `Status` are essential; name, tag prefix, executor, delivery, workflow, working directory,
and concise issue codes are optional. Unknown width uses deterministic vertical
records, and invalid units remain visible. `valid` exits `0`; `has_issues` and
`source_invalid` exit `1`. Missing, malformed, unsupported, V1-only, mixed, and
recovery-blocked source states are nil-Go-error structured responses with a
stable source issue. Source codes are `V2_SOURCE_INSPECTION_FAILED`,
`MIXED_RELEASE_SOURCES`, `V1_SOURCE_UNSUPPORTED`, `V2_SOURCE_MISSING`,
`V2_CONFIG_INVALID`, `V2_STATE_INVALID`, `V2_SCHEMA_UNSUPPORTED`,
`V2_RECOVERY_BLOCKED`, `V2_CONFIG_MISSING`, `V2_STATE_MISSING`, and
`V2_SOURCE_EMPTY`.

The command reads only strict local V2 config/state and the existing pair
recovery readiness marker. It does not parse workflow YAML, invoke Doctor
workflow inspection, inspect Git or tags, read tokens, contact a network, read
journals or Evidence, inspect build files, plan releases, execute releases, or
write anything. `release doctor` remains the workflow-readiness command;
release pipeline inspection remains unsupported.

Unsupported or read-only boundaries:

- Existing V2 configs with `delivery: local` are rejected with a clear unsupported-delivery validation error.
- Execution journals and dispatch journals are not written by dry-run.
- Public V2 local executor execution is not configured because no supported executor exposes a safe publish-only boundary.

For `delivery: github-actions`, V2 config must include `workflow: ".github/workflows/<file>.yml"` or `workflow: ".github/workflows/<file>.yaml"`. `neko release validate --show` displays the workflow only after repository-aware validation confirms that the file exists and stays inside `.github/workflows/`.

The execution journal records V2 release phases and recovery evidence under the Git common directory. The dispatch contract targets GitHub.com remotes only, uses `GITHUB_TOKEN` with repository Actions write permission, sends the existing unit tag as `ref`, and sends exactly four inputs: `unit`, `version`, `tag`, and `release_sha`. No public standalone dispatch or retry command exists.

`neko release evidence` is read-only. It inspects release-execution journals, dispatch journals, migration journals, V1 compensation evidence, and V2 pair-recovery evidence with redacted summaries and diagnostics for corrupt, unsupported, conflicting, terminal, unresolved, and completed evidence. Human table output prioritizes family, state, classification, resume eligibility, and manual recovery; optional unit/version/tag/pending/automatic/lifecycle columns are admitted by available terminal width. Narrow, piped, redirected, or otherwise width-unknown human output uses vertical records. `--output wide` permits every declared summary column but excludes forensic detail fields.

`neko release evidence --identity <prefix>` selects one complete redacted record after `--family` and `--unit` filters. Prefixes must be 8-64 lowercase hexadecimal characters; uppercase input is rejected rather than normalized, full identities are accepted, and zero or ambiguous matches fail. Human-readable output uses property/value detail. JSON retains `data.items`, `data.evidence`, and `data.diagnostics`, including the complete typed record, while existing summary JSON remains byte-for-byte schema compatible and excludes presentation metadata.

`neko release evidence-archive` supports only `archive-completed` for completed `release-execution`, `v1-compensation`, and completed `v2-pair-recovery` evidence. It still requires `--family`, the exact 64-character `--identity`, the current `--digest-sha256` from inspection output, and `--confirm-archive`; identity prefixes are inspection-only. It writes an exact private archive before removing the completed source evidence. Neither Evidence command repairs, retries, infers remote state, or archives dispatch/migration evidence.

`neko release plugin-index` generates the public plugin registry index from V2 plugin units, `.neko/release.state.json`, and plugin manifests. The generated `plugin-index.json` is not committed as source and is not published by this command. Plugin release workflows publish or replace it as the `plugin-index.json` asset on the mutable `plugin-registry` GitHub Release after successful plugin releases. Runtime `neko plugin available`, `install`, and `update` use that asset as the source of truth and do not use `/releases/latest` or release-prefix fallback discovery for plugin discovery.

### CI release-context validation

`neko release ci-validate-context` validates one dispatched Release V2 context
before a consumer builds or publishes:

```bash
neko release ci-validate-context \
  --unit api \
  --version 2.4.0 \
  --tag api/v2.4.0 \
  --release-sha <full-lowercase-commit-object-id>
```

All four flags are required; there are no command-specific optional flags. The
command requires a complete valid V2 config/state pair with no V1 conflict or
unresolved pair-recovery evidence. It resolves the explicit unit, compares the
exact authoritative state version and canonical tag, verifies that
`release_sha` is a full local commit object ID for the repository's object
format, and requires checked-out `HEAD` and the peeled local tag target to equal
it. Lightweight and annotated tags are accepted. Detached HEAD is accepted when
it matches. Missing or incomplete local tag history fails with guidance; the
command never fetches.

Default table output is an ordered responsive property/value view. Long values
wrap within the actual output width, and narrow or width-unknown output uses
vertical properties. `--output json` returns the normal response envelope with
deterministic `data` keys:

```text
valid
unit
display_name
version
tag_prefix
tag
release_sha
working_directory
executor
delivery
workflow
git_object_format
head_matches
tag_target_matches
```

`valid`, `head_matches`, and `tag_target_matches` are JSON booleans. All other
data values are canonical strings. Presentation and integration transport
metadata are excluded from public JSON.

GitHub Actions uses an explicit command-file destination:

```bash
neko release ci-validate-context \
  --unit "$RELEASE_UNIT" \
  --version "$RELEASE_VERSION" \
  --tag "$RELEASE_TAG" \
  --release-sha "$RELEASE_SHA" \
  --output github \
  --github-output-file "$GITHUB_OUTPUT"
```

The stable step outputs, in order, are `unit`, `display_name`, `version`,
`tag_prefix`, `tag`, `release_sha`, `working_directory`, `executor`, `delivery`,
and `workflow`. Empty values, Unicode, spaces, newlines, carriage returns, and
delimiter-like lines are encoded without shell evaluation. The destination is
never inferred from ambient environment, so outside GitHub Actions callers can
use human/JSON output or explicitly supply a pre-created command file.

Validation mismatches return a structured error response and nonzero CLI exit.
Stable command codes are:

```text
INVALID_CONTEXT_INPUT
INVALID_RELEASE_SHA
UNSUPPORTED_RELEASE_SOURCE
V2_CONTEXT_SOURCE_MISSING
V2_CONTEXT_SOURCE_CONFLICT
V2_CONTEXT_RECOVERY_BLOCKED
V2_CONFIGURATION_INVALID
V2_STATE_INVALID
V2_CONFIG_STATE_MISMATCH
V2_CONTEXT_SOURCE_INVALID
RELEASE_UNIT_NOT_FOUND
RELEASE_VERSION_MISMATCH
RELEASE_TAG_MISMATCH
GIT_REPOSITORY_UNAVAILABLE
GIT_OBJECT_FORMAT_UNSUPPORTED
RELEASE_SHA_NOT_COMMIT
HEAD_UNAVAILABLE
HEAD_MISMATCH
TAG_HISTORY_UNAVAILABLE
RELEASE_TAG_MISSING
TAG_TARGET_INVALID
TAG_TARGET_MISMATCH
```

Core output failures use `GITHUB_OUTPUT_DESTINATION_UNAVAILABLE` or
`GITHUB_OUTPUT_ENCODING_FAILED`. Safe messages omit raw Git output, filesystem
internals, and secrets. The command reads no token, contacts no network, writes
no release files or journals, and mutates no Git worktree, index, or ref.

## Resume

`resume` continues only an existing unresolved V2 GitHub Actions execution journal. It never calculates a new version, chooses a new tag, or blindly retries uncertain push or dispatch outcomes.

```bash
neko release resume --unit api
neko release resume --unit api --dry-run
```

## Unit Flag

`--unit <unit-id>` is available on:

```text
patch
minor
major
plan
history
contributors
validate
resume
evidence
github-workflow-init
```

It is required for unit-bound commands when a V2 repository defines multiple units.
`validate` is not unit-bound: it validates the complete repository without a
unit and treats `--unit` only as a presentation focus.

## Migrate

`migrate` has one flag:

```text
--dry-run
```

It migrates only `.release.neko.json` in the Git root to a V2 `default` unit. It does not migrate nested V1 files, infer multiple units, change tag prefixes, or run a release.
