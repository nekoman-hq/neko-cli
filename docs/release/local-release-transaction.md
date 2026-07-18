# Local Release Transaction

V2 local release execution is deliberately unsupported. The former exported local transaction wrapper remains only as a deprecated compatibility shape and rejects execution directly. It no longer owns private materialization, state persistence, Git coordination, rollback, or executor invocation logic.

V2 GitHub Actions releases use the journaled flow described in [GitHub Actions release flow](github-actions-release-flow.md).

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

## Retired State Update Path

The former local transaction preparation path used to model this order:

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

That path is not active production behavior. Public V2 releases use GitHub Actions delivery, where Neko CLI owns materialization, state, targeted staging, commit, tag, push, journals, and dispatch before the workflow owns build and publication.

## Recovery

Because V2 local execution is rejected before mutation, there is no active local transaction crash window. No materialized files, V2 state writes, commits, tags, pushes, executor processes, or remote publications are started by the deprecated wrapper.

If a future local delivery feature is designed, it must define fresh executor-specific evidence, crash windows, retry refusal, and compensation limits before any local executor process can run.

## Executor Status

| Executor | V2 local public status | Internal preparation |
|----------|-----------------|------------------------|
| `goreleaser` | unsupported | No active V2 local preparation |
| `jreleaser` | unsupported | No active V2 local preparation |
| `release-it` | unsupported | release-it owns commit/tag/push/release in the legacy adapter and has no V2 publish-only boundary |

GitHub Actions delivery is active through the journaled remote workflow dispatch path; local executors do not publish V2 GitHub Actions releases.

See [Git release coordination](git-coordination.md).
