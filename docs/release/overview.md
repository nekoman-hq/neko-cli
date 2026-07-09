# Release System Overview

Neko CLI currently supports two release configuration formats.

## V1

V1 is the legacy format stored in `.release.neko.json`. It describes one global release stream:

```json
{
  "project-name": "neko-cli",
  "project-owner": "nekoman-hq",
  "project-type": "backend",
  "release-system": "goreleaser",
  "version": "2.3.1"
}
```

V1 remains supported. Existing `patch`, `minor`, `major`, `history`, `contributors`, `validate`, and `init` flows continue to use this file.

## V2

V2 is repository-root scoped and uses two files:

```text
.neko/release.config.json
.neko/release.state.json
```

`release.config.json` is committed architecture: units, paths, working directories, tag prefixes, executor type, delivery mode, and optional plugin metadata for units with `kind: "plugin"`.

`release.state.json` is the version source of truth for all units. Tags are not stored in state; a tag is derived later from `tagPrefix + version`.

V2 can be loaded, strictly parsed, validated, normalized, and used for unit workflows. `history` and `contributors` are unit-aware. `patch`, `minor`, and `major` support V2 dry-run planning with execution context, delivery resolution, version materialization planning, GitHub Actions workflow configuration, and Neko-owned Git release planning.

Nekocli itself now has `cli`, `plugin-release`, and `plugin-ui` V2 units. `.neko/release.state.json` is authoritative for every releaseable version. `.plugin.release.neko.json` has been removed and must not be reintroduced. Each plugin release plan materializes only that unit's `manifest.json`. Plugin units declare `name`, `manifest`, `assetPrefix`, and `binaryName` metadata in V2 config so `neko release plugin-index` can generate the future public registry contract without registry Go-code edits.

V2 GitHub Actions non-dry-run public release commands are active. Neko CLI owns materialization, state update, targeted staging, release commit, unit tag, commit push, tag push, execution journal, dispatch journal, and workflow dispatch. GitHub Actions owns build, GitHub Release creation, and asset publishing from the pushed tag. V2 local delivery remains blocked.

## Safety

Dry-run release planning is read-only. It calculates and displays the next version without writing `.release.neko.json`, updating executor files, starting executors, resolving `GITHUB_TOKEN`, running rollback, fetching remotes, committing, tagging, pushing, dispatching, or publishing.

Rollback is bounded to release runs that have recorded a mutating step. Guard and planning failures must not trigger destructive cleanup such as `git reset --hard` or `git clean -fd`.

## Current Scope

Implemented now:

- V1 compatibility.
- V2 models, strict JSON parsing, validation, and normalization.
- V2 support in `neko release validate`.
- Unit selection with `--unit`.
- Unit-specific V2 history and contributors.
- Non-mutating V2 dry-run version planning.
- Safe V1 root single-unit migration with `neko release migrate`.
- Atomic JSON write helpers for migration and state updates.
- Schema-neutral `ReleaseExecutionContext`.
- Local delivery resolver with `github-actions` recognized as non-local.
- GitHub Actions delivery schema with mandatory canonical `.github/workflows/<file>.yml|yaml` workflow validation.
- Immutable GitHub Actions dispatch request contract and SHA-256 identity.
- Durable V2 release execution journal and read-only recovery assessment under the Git common directory.
- Durable dispatch journal model under the Git common directory.
- Internal GitHub.com-only Actions workflow-dispatch client with `GITHUB_TOKEN` token resolution, redirect blocking, and accepted/rejected/unknown classification.
- Public V2 GitHub Actions release execution.
- Production GitHub Actions publishing workflows for `cli`, `plugin-release`, and `plugin-ui`.
- Dedicated GoReleaser configs for `cli`, `plugin-release`, and `plugin-ui`.
- V2 plugin unit metadata validation for future plugin index generation.
- Deterministic `plugin-index.json` generation from V2 plugin units, state, and manifests.
- Public `neko release resume --unit <unit>` and read-only `--dry-run` recovery assessment.
- Executor capability contracts for `goreleaser`, `jreleaser`, and `release-it`.
- Executor requirement checks scoped to unit roots.
- Version materialization before V2 local release commits.
- V2 local release transaction internals with atomic materialization and state persistence.
- Internal V2 `GitReleaseCoordinator` for targeted staging, deterministic release commits, unit tags, and explicit commit/tag pushes.
- Non-destructive V2 recovery boundary.
- Public V2 dry-run Git planning.

Not implemented yet:

- Automatic multi-unit migration.
- V2 local `release-it` execution.
- Publishing `plugin-index.json` and making install/update consume it; runtime install/update still uses the current registry behavior until that follow-up milestone.
- V2 local non-dry-run release execution.
- Public standalone dispatch and retry commands.
