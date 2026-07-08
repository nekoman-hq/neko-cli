# Version Materialization

V2 state is the version source of truth, but state alone is not enough for every executor. Some release tools also need local version files to match the planned version before the release commit is created.

## Model

```text
ReleasePlan
  -> VersionMaterializer
  -> MaterializationPlan
  -> MaterializationTransaction
  -> StateTransaction
  -> GitReleaseCoordinator
  -> later dispatch or publish-only adapter
```

`MaterializationPlan` lists structured file changes with absolute paths, repository-relative paths, before bytes or hashes, after bytes, file mode, reason, and whether the file is required in the release commit. `.neko/release.state.json` is not part of this plan; it remains owned by the state transaction.

Dry runs only plan and validate materialization. They do not write files, stage files, start executors, commit, tag, push, publish, or rollback.

## Executor Decisions

| Executor | Materializer | V2 public status |
|----------|--------------|-----------------|
| `goreleaser` | tag/context based by default; Nekocli plugin units materialize their own `manifest.json` files | active through GitHub Actions delivery when configured |
| `jreleaser` | updates `jreleaser.yml` project version before state staging | blocked until publish-only adapter exists |
| `release-it` | no real materialization | blocked |

GoReleaser units normally do not need a local version file from Neko CLI. Their release version is anchored by the Neko-created release context and tag. Nekocli's plugin units are the current exception: `.neko/release.state.json` is authoritative, and V2 planning materializes the selected plugin manifest to the planned next version. `plugin-release` updates only `plugin/release/manifest.json`; `plugin-ui` updates only `plugin/ui/manifest.json`; `cli` materializes no plugin manifest. `make update-manifests` remains a manual developer helper and reads V2 state. `.plugin.release.neko.json` has been removed and is not part of any active materialization path.

JReleaser needs `jreleaser.yml` to contain the planned project version. The materializer updates only `project.version`, snapshots the original bytes and mode, writes before state preparation, and marks `jreleaser.yml` as required for the Neko-owned release commit.

release-it remains blocked because it owns commit, tag, push, and GitHub release behavior in the current adapter. No package or `.release-it.json` mutation is performed for V2 Git coordination.

## Recovery

Before commit or tag work starts, the materialization transaction can restore changed files byte-for-byte or remove files it created. It does not run global cleanup, `git reset --hard`, or `git clean -fd`.

After commit/tag/remote work starts, V2 does not automatically restore materialized files or state. The error message reports known changed files for manual inspection.
