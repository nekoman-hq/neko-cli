# Version Materialization

V2 state is the version source of truth, but state alone is not enough for every executor. Some release tools also need local version files to match the planned version before the release commit is created.

## Model

```text
ReleasePlan
  -> VersionMaterializer
  -> MaterializationPlan
  -> MaterializationTransaction
  -> StateTransaction
  -> targeted staging
  -> commit/tag/push/delivery
```

`MaterializationPlan` lists structured file changes with absolute paths, repository-relative paths, before bytes or hashes, after bytes, file mode, reason, and whether the file is required in the release commit. `.neko/release.state.json` is not part of this plan; it remains owned by the state transaction.

Dry runs only plan and validate materialization. They do not write files, stage files, start executors, commit, tag, push, publish, or rollback.

## Executor Decisions

| Executor | Materializer | V2 local status |
|----------|--------------|-----------------|
| `goreleaser` | no-op | enabled |
| `jreleaser` | updates `jreleaser.yml` project version before state staging | enabled |
| `release-it` | no real materialization | blocked |

GoReleaser does not currently need a local version file from Neko CLI. Its release version is anchored by the Neko-created release context and tag. Existing optional `.plugin.release.neko.json` data is read as environment injection and is not materialized by V2 release.

JReleaser needs `jreleaser.yml` to contain the planned project version. The materializer updates only `project.version`, snapshots the original bytes and mode, writes before state staging, and stages `jreleaser.yml` with state for the release commit.

release-it remains blocked because it owns commit, tag, push, and GitHub release behavior in the current adapter. No package or `.release-it.json` mutation is performed for real V2 execution.

## Recovery

Before commit or tag work starts, the materialization transaction can restore changed files byte-for-byte or remove files it created. It does not run global cleanup, `git reset --hard`, or `git clean -fd`.

After commit/tag/remote work starts, V2 does not automatically restore materialized files or state. The error message reports known changed files for manual inspection.
