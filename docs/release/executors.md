# Release Executors

Release executors are resolved through a schema-neutral execution context. The context contains the absolute repository root, the selected unit, the absolute unit root, current and next version, tag, tag spec, release kind, dry-run mode, executor, delivery, and source format.

## Supported Executors

| Executor | Version files | Commit | Tag | Push | GitHub release | V2 local execution | Dry run |
|----------|---------------|--------|-----|------|----------------|--------------------|---------|
| `goreleaser` | no-op materializer | Neko CLI | Neko CLI | Neko CLI | GoReleaser | enabled | yes |
| `jreleaser` | Neko CLI materializes `jreleaser.yml` before state staging | Neko CLI | JReleaser | Neko CLI and JReleaser release flow | JReleaser | enabled | yes |
| `release-it` | release-it updates configured files | release-it | release-it | release-it | release-it | blocked for V2 local | no local dry-run contract yet |

The capability model is active in V2 local transactions. It decides whether state is written before executor start and whether the central V2 state can be guaranteed in the release commit.

## Requirement Files

Executor requirement checks are scoped to the selected unit root:

| Executor | Required file |
|----------|---------------|
| `goreleaser` | `.goreleaser.yml` or `.goreleaser.yaml` |
| `jreleaser` | `jreleaser.yml` |
| `release-it` | `.release-it.json` |

For V1, the unit root is the repository root. For V2, the unit root is the unit `workingDirectory` resolved under the Git root.

## V2 State Commit Guarantee

GoReleaser and JReleaser are enabled because Neko CLI owns the release commit in the current implementation. The transaction materializes required version files, writes state, and stages all release files before executor execution.

release-it is blocked for V2 local release because release-it owns commit, tag, push, and GitHub release creation. When a unit root is nested, the repository-root `.neko/release.state.json` cannot currently be proven to land in release-it's commit.

See [Version materialization](version-materialization.md).
