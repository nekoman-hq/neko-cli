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

`release.config.json` is committed architecture: units, paths, working directories, tag prefixes, executor type, and delivery mode.

`release.state.json` is the version source of truth for all units. Tags are not stored in state; a tag is derived later from `tagPrefix + version`.

V2 can be loaded, strictly parsed, validated, normalized, and used for unit workflows. `history` and `contributors` are unit-aware. `patch`, `minor`, and `major` support V2 dry-run planning with execution context, delivery resolution, version materialization planning, GitHub Actions workflow configuration, and Neko-owned Git release planning.

V2 non-dry-run public release commands are intentionally blocked after Milestone 5A. The internal `GitReleaseCoordinator` can stage known release files, create the deterministic release commit, create the unit tag, and push commit then tag, but publish-only adapters and GitHub Actions dispatch are not available yet. Non-dry-run V2 commands return:

```text
V2 Git release coordination is prepared, but V2 publication adapters are not available yet. No release state, commit, tag, push, or publish operation was performed.
```

## Safety

Dry-run release planning is read-only. It calculates and displays the next version without writing `.release.neko.json`, updating executor files, starting executors, running rollback, fetching remotes, committing, tagging, pushing, or publishing.

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
- Executor capability contracts for `goreleaser`, `jreleaser`, and `release-it`.
- Executor requirement checks scoped to unit roots.
- Version materialization before V2 local release commits.
- V2 local release transaction internals with atomic materialization and state persistence.
- Internal V2 `GitReleaseCoordinator` for targeted staging, deterministic release commits, unit tags, and explicit commit/tag pushes.
- Non-destructive V2 recovery boundary.
- Public V2 dry-run Git planning.

Not implemented yet:

- Automatic multi-unit migration.
- GitHub Actions delivery execution.
- V2 local `release-it` execution.
- Publish-only adapters for GoReleaser and JReleaser.
- Public V2 non-dry-run release execution.
- Dispatch journal and resume command.
