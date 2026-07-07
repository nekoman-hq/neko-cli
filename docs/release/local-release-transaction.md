# Local Release Transaction

V2 local release internals run through one transaction boundary. The transaction owns the order of preflight, materialization, state persistence, handoff to Git coordination, and recovery.

Public V2 non-dry-run commands are blocked in Milestone 5A before this transaction mutates files. The transaction exists so later publish-only adapters can reuse the same preparation boundary.

## Phases

```text
planned
preflight-validated
materialization-prepared
materialization-applied
state-prepared
release-files-staged
commit-or-tag-started
remote-side-effect-started
completed
failed
```

Before `commit-or-tag-started`, materialized version files and the canonical V2 state file may be restored from their snapshots. After commit, tag, push, or GitHub release work has begun, Neko CLI does not run destructive rollback for V2.

## State Update

For a V2 local release, the transaction:

1. loads and validates V2 config and state;
2. resolves the selected unit and execution context;
3. validates local delivery and executor capabilities;
4. checks executor requirements under the unit root;
5. checks repository cleanliness when required by the executor;
6. plans and validates version materialization;
7. captures exact bytes and mode for materialized files;
8. writes materialized version files;
9. captures exact bytes and mode of `.neko/release.state.json`;
10. writes the selected unit's next version through the atomic JSON writer;
11. validates the written state through the real V2 loader;
12. hands the known release files to `GitReleaseCoordinator` for targeted staging, commit, tag, and push.

Only the selected unit version is changed. Other units remain unchanged in the state model.

## Recovery

If a local error occurs after materialization or state write but before commit/tag work starts, the transaction restores `.neko/release.state.json` and materialized files from snapshots, then unstages only the files staged by this transaction.

If the release reaches commit/tag/remote phases, no `git reset --hard`, `git clean -fd`, remote tag deletion, or GitHub release deletion is attempted by the V2 transaction. The error reports the reached phase, unit, tag, and known changed files for manual inspection.

## Executor Status

| Executor | V2 local public status | Internal preparation |
|----------|-----------------|------------------------|
| `goreleaser` | blocked until publish-only adapter exists | No version file materialization; state is prepared as a known release file |
| `jreleaser` | blocked until publish-only adapter exists | Neko CLI materializes `jreleaser.yml` and prepares it with state as known release files |
| `release-it` | blocked | release-it owns commit/tag/push/release in the legacy adapter and has no V2 publish-only boundary |

GitHub Actions delivery remains recognized but not implemented.

See [Git release coordination](git-coordination.md).
