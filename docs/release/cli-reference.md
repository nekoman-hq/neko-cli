# Release CLI Reference
## General

```bash
neko release init --executor goreleaser --delivery github-actions --workflow .github/workflows/release-cli.yml
neko release init --unit plugin-release --kind plugin --plugin-name release --plugin-manifest plugin/release/manifest.json --plugin-asset-prefix plugin-release --plugin-binary-name plugin-release --executor goreleaser --delivery github-actions --workflow .github/workflows/release-plugin-release.yml --tag-prefix plugin-release/v
neko release unit-add --unit api --executor goreleaser --delivery github-actions --workflow .github/workflows/release-api.yml --tag-prefix api/v --paths "apps/api/**"
neko release init-options
```

`init` creates V2 `.neko/release.config.json` and `.neko/release.state.json` files for one release unit. Use `--kind release` or omit `--kind` for a normal service, app, CLI, SDK, library, or backend module; V2 JSON omits `kind` for normal units. Use `--kind plugin` plus `--plugin-name`, `--plugin-manifest`, `--plugin-asset-prefix`, and `--plugin-binary-name` only for a Neko CLI plugin unit. Plugin flags without `--kind plugin` are invalid. Plugin unit ids must start with `plugin-`, plugin tag prefixes must be `<unit-id>/v`, and plugin asset prefixes must match the unit id. `init` no longer creates `.release.neko.json` or initializes executor-specific tool files. Existing V1 repositories should use `neko release migrate`. `github-actions` delivery requires `--workflow .github/workflows/<file>.yml|yaml`; workflow template generation and executor scaffolding are not implemented yet.

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

`neko release plan --change <patch|minor|major> [--unit <unit>]` is a dedicated local plan inspection command. It reports the selected release source and unit, current version, requested change, next version, tag, planned materialized files, known release files, local readiness, local blockers, and explicit limitations. It does not read tokens, inspect remotes, inspect execution or dispatch journals, write files, mutate Git, dispatch workflows, publish releases, or start executors. It is separate from `--dry-run`: dry-run keeps the existing release preview contract, while `plan` is for stable local planning facts.

Unsupported or read-only boundaries:

- Existing V2 configs with `delivery: local` are rejected with a clear unsupported-delivery validation error.
- Execution journals and dispatch journals are not written by dry-run.
- Public V2 local executor execution is not configured because no supported executor exposes a safe publish-only boundary.

For `delivery: github-actions`, V2 config must include `workflow: ".github/workflows/<file>.yml"` or `workflow: ".github/workflows/<file>.yaml"`. `neko release validate --show` displays the workflow only after repository-aware validation confirms that the file exists and stays inside `.github/workflows/`.

The execution journal records V2 release phases and recovery evidence under the Git common directory. The dispatch contract targets GitHub.com remotes only, uses `GITHUB_TOKEN` with repository Actions write permission, sends the existing unit tag as `ref`, and sends exactly four inputs: `unit`, `version`, `tag`, and `release_sha`. No public standalone dispatch or retry command exists.

`neko release evidence` is read-only. It inspects release-execution journals, dispatch journals, migration journals, V1 compensation evidence, and V2 pair-recovery evidence with redacted summaries and diagnostics for corrupt, unsupported, conflicting, terminal, unresolved, and completed evidence. Human table output prioritizes family, state, classification, resume eligibility, and manual recovery; optional unit/version/tag/pending/automatic/lifecycle columns are admitted by available terminal width. Narrow, piped, redirected, or otherwise width-unknown human output uses vertical records. `--output wide` permits every declared summary column but excludes forensic detail fields.

`neko release evidence --identity <prefix>` selects one complete redacted record after `--family` and `--unit` filters. Prefixes must be 8-64 lowercase hexadecimal characters; uppercase input is rejected rather than normalized, full identities are accepted, and zero or ambiguous matches fail. Human output uses property/value detail. JSON retains `data.items`, `data.evidence`, and `data.diagnostics`, including the complete typed record, while existing summary JSON remains byte-for-byte schema compatible and excludes presentation metadata.

`neko release evidence-archive` supports only `archive-completed` for completed `release-execution`, `v1-compensation`, and completed `v2-pair-recovery` evidence. It still requires `--family`, the exact 64-character `--identity`, the current `--digest-sha256` from inspection output, and `--confirm-archive`; identity prefixes are inspection-only. It writes an exact private archive before removing the completed source evidence. Neither Evidence command repairs, retries, infers remote state, or archives dispatch/migration evidence.

`neko release plugin-index` generates the public plugin registry index from V2 plugin units, `.neko/release.state.json`, and plugin manifests. The generated `plugin-index.json` is not committed as source and is not published by this command. Plugin release workflows publish or replace it as the `plugin-index.json` asset on the mutable `plugin-registry` GitHub Release after successful plugin releases. Runtime `neko plugin available`, `install`, and `update` use that asset as the source of truth and do not use `/releases/latest` or release-prefix fallback discovery for plugin discovery.

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
```

It is required for unit-bound commands when a V2 repository defines multiple units.

## Migrate

`migrate` has one flag:

```text
--dry-run
```

It migrates only `.release.neko.json` in the Git root to a V2 `default` unit. It does not migrate nested V1 files, infer multiple units, change tag prefixes, or run a release.
