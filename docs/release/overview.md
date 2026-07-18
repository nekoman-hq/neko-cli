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

V1 remains supported for existing release repositories. Existing `patch`, `minor`, `major`, `history`, `contributors`, and `validate` flows continue to use this file when no V2 config exists. New `neko release init` is V2-only; use `neko release migrate` to convert a root V1 repository.

## V2

V2 is repository-root scoped and uses two files:

```text
.neko/release.config.json
.neko/release.state.json
```

`release.config.json` is committed architecture: units, paths, working directories, tag prefixes, executor type, delivery mode, and optional plugin metadata for units with `kind: "plugin"`.

`release.state.json` is the version source of truth for all units. Tags are not stored in state; a tag is derived later from `tagPrefix + version`.

Core V2 terms:

| Term | Meaning |
| --- | --- |
| `unit` | Independently releasable object such as `cli`, `api`, or `plugin-release` |
| `tagPrefix` | Namespace used to derive tags for a unit, such as `v`, `api/v`, or `plugin-release/v` |
| `executor` | Release tool: `goreleaser`, `jreleaser`, or `release-it` |
| `delivery` | Release handoff mode; V2 supports `github-actions` |

V2 can be loaded, strictly parsed, validated, normalized, and used for unit workflows. `history` and `contributors` are unit-aware. `patch`, `minor`, and `major` support V2 dry-run planning with execution context, delivery resolution, version materialization planning, GitHub Actions workflow configuration, and Neko-owned Git release planning.

Nekocli itself now has `cli`, `plugin-release`, and `plugin-ui` V2 units. `.neko/release.state.json` is authoritative for every releaseable version. `.plugin.release.neko.json` has been removed and must not be reintroduced. Each plugin release plan materializes only that unit's `manifest.json`. Plugin units declare `name`, `manifest`, `assetPrefix`, and `binaryName` metadata in V2 config so `neko release plugin-index` can generate the public registry contract without registry Go-code edits. CLI install, version, and update checks use only stable CLI tags matching `vX.Y.Z`; plugin releases and `plugin-registry` are ignored for CLI updates. Runtime plugin discovery, install, and update use `plugin-index.json` as the source of truth from the `plugin-registry` GitHub Release asset; `/releases/latest` and release-prefix fallback discovery are not used for plugin discovery. Plugin release workflows publish or replace that asset after successful plugin releases.

V2 GitHub Actions non-dry-run public release commands are active. Neko CLI owns materialization, state update, targeted staging, release commit, unit tag, commit push, tag push, execution journal, dispatch journal, and workflow dispatch. GitHub Actions owns build, GitHub Release creation, and asset publishing from the pushed tag. V2 local delivery is deliberately unsupported and rejected by validation.

The stable V2 lifecycle is:

```bash
neko release init ...
neko release unit-add ...
neko release validate --show
neko release patch --unit <unit>
neko release history --unit <unit>
neko release contributors --unit <unit>
neko release resume --unit <unit> --dry-run
```

Multi-unit repositories require `--unit` for unit-scoped commands. See [Release V2 Examples](examples.md) for copy-ready CLI, service, plugin, release, registry, and install/update examples. The product boundary for turning a valid V2 config into a release-ready GitHub Actions integration is defined in [Release V2 Bootstrap Product Boundary](bootstrap-product-boundary.md).

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
- Delivery resolver with `github-actions` recognized as the supported V2 handoff mode and `local` retained only for legacy compatibility/error reporting.
- GitHub Actions delivery schema with mandatory canonical `.github/workflows/<file>.yml|yaml` workflow validation.
- Immutable GitHub Actions dispatch request contract and SHA-256 identity.
- Durable V2 release execution journal and read-only recovery assessment under the Git common directory.
- Durable dispatch journal model under the Git common directory.
- Internal GitHub.com-only Actions workflow-dispatch client with `GITHUB_TOKEN` token resolution, redirect blocking, and accepted/rejected/unknown classification.
- Public V2 GitHub Actions release execution.
- Production GitHub Actions publishing workflows for `cli`, `plugin-release`, and `plugin-ui`.
- Dedicated GoReleaser configs for `cli`, `plugin-release`, and `plugin-ui`.
- V2-only `neko release init` for one release unit, writing `.neko/release.config.json` and `.neko/release.state.json`.
- Plugin unit metadata support in `neko release init` with `--kind plugin`.
- `neko release unit-add` for appending normal and plugin units to existing V2 config/state.
- V2 plugin unit metadata validation for plugin index generation.
- Deterministic `plugin-index.json` generation from V2 plugin units, state, and manifests.
- Runtime plugin registry reads `plugin-index.json` as its source of truth; release-prefix fallback discovery has been removed.
- Plugin release workflows publish/update the mutable `plugin-registry` release asset after successful plugin releases.
- Public `neko release resume --unit <unit>` and read-only `--dry-run` recovery assessment.
- Executor capability contracts for `goreleaser`, `jreleaser`, and `release-it`.
- Executor requirement checks scoped to unit roots.
- Version materialization before V2 GitHub Actions release commits.
- Deprecated V2 local release transaction wrapper that rejects local execution directly.
- Internal V2 `GitReleaseCoordinator` for targeted staging, deterministic release commits, unit tags, and explicit commit/tag pushes.
- Non-destructive V2 recovery boundary.
- Public V2 dry-run Git planning.

Not implemented yet:

- Automatic multi-unit migration.
- Workflow template generation from `neko release init`.
- Executor scaffolding from `neko release init`.
- Stable CI release-context validation.
- Read-only integration doctor.
- Release unit overview.
- Release pipeline inspection.
- Build-system adapters such as Gradle.
- V2 local executor execution.
- Automated cross-platform install/update smoke testing against the published plugin registry.
- V2 local non-dry-run release execution.
- Public standalone dispatch and retry commands.
