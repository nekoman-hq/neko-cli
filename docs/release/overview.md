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

In this milestone, V2 can be loaded, strictly parsed, validated, normalized, and used for read-only unit workflows. `history` and `contributors` are unit-aware. `patch`, `minor`, and `major` support V2 dry-run planning only.

V2 non-dry-run release execution is intentionally not active yet and returns:

```text
release schema v2 supports planning and read-only history, but release execution is not available yet
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
- Atomic JSON write helpers for future migration and state updates.

Not implemented yet:

- V2 release execution.
- V2 state persistence for version bumps.
- Automatic multi-unit migration.
- ExecutorContext and executor rewiring.
- GitHub Actions delivery execution.
