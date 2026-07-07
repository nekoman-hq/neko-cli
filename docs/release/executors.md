# Release Executors

Release executors are resolved through a schema-neutral execution context. The context contains the absolute repository root, the selected unit, the absolute unit root, current and next version, tag, tag spec, release kind, dry-run mode, executor, delivery, and source format.

## Supported Executors

| Executor | Legacy version files | Legacy commit | Legacy tag | Legacy push | Legacy GitHub release | V2 public execution | Dry run |
|----------|---------------|--------|-----|------|----------------|--------------------|---------|
| `goreleaser` | no-op materializer | legacy adapter | legacy adapter | legacy adapter | GoReleaser | blocked until publish-only adapter | yes |
| `jreleaser` | Neko CLI materializes `jreleaser.yml` before state staging | legacy adapter | JReleaser | legacy adapter and JReleaser release flow | JReleaser | blocked until publish-only adapter | yes |
| `release-it` | release-it updates configured files | release-it | release-it | release-it | release-it | blocked | no local dry-run contract yet |

The capability model describes current tool or legacy-adapter behavior. V2 Git ownership is separate: Neko CLI owns the V2 release commit, unit tag, and push of commit then tag. Executors must become publish-only before public V2 non-dry-run release execution is enabled.

## Requirement Files

Executor requirement checks are scoped to the selected unit root:

| Executor | Required file |
|----------|---------------|
| `goreleaser` | `.goreleaser.yml` or `.goreleaser.yaml` |
| `jreleaser` | `jreleaser.yml` |
| `release-it` | `.release-it.json` |

For V1, the unit root is the repository root. For V2, the unit root is the unit `workingDirectory` resolved under the Git root.

## V2 Git Ownership

For V2, Neko CLI owns:

```text
version authority
version materialization
central release state
release commit
unit tag
push of commit and tag
```

The `GitReleaseCoordinator` creates commits as `chore(release): <unit-id> <tag>`, creates lightweight unit tags from `TagSpec`, and pushes commit before tag. No executor owns V2 commit, tag, or push.

release-it is blocked for V2 local release because release-it owns commit, tag, push, and GitHub release creation. When a unit root is nested, the repository-root `.neko/release.state.json` cannot currently be proven to land in release-it's commit.

See [Version materialization](version-materialization.md) and [Git release coordination](git-coordination.md).
