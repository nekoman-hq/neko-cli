# Git Release Coordination

The internal V2 `GitReleaseCoordinator` implements repository-owned Git coordination:

```text
Neko CLI owns:
  - version authority
  - version materialization
  - central release state
  - release commit
  - unit tag
  - push of commit and tag

GitHub Actions later owns:
  - checkout of an already pushed tag
  - validation of config, state, unit, version, and tag
  - build, publish, and GitHub Release creation
```

The coordinator does not load V1 files, start executors, call GitHub APIs, calculate versions, materialize files, write state, or dispatch workflows. It only coordinates known release files that were already prepared by the materialization and state transactions.

For GitHub Actions dispatch, V2 units validate a canonical workflow path under `.github/workflows/`. The execution journal records the local V2 transaction before materialization, state write, staging, commit, tag, push, and dispatch handoff. The dispatch journal records the later HTTP dispatch attempt.

## Known Release Files

Known files are exactly:

```text
.neko/release.state.json
materialization files with RequiredForReleaseCommit == true
```

Every known file must resolve to an absolute path inside `RepositoryRoot` and to an auditable repository-relative path. Paths outside the repository are rejected before staging.

For Nekocli plugin units, `.neko/release.state.json` remains the authoritative version source. `plugin-release` release commits contain `.neko/release.state.json` and `plugin/release/manifest.json`. `plugin-ui` release commits contain `.neko/release.state.json` and `plugin/ui/manifest.json`.

## Preflight

Real V2 Git coordination requires:

- `RepositoryRoot` is the Git worktree root.
- The current branch is resolvable.
- The branch has one configured upstream remote and branch.
- Worktree and index are clean before release file preparation starts.
- The expected unit tag does not already exist before commit creation.
- Known release files are valid repository paths.

The cleanliness error is intentionally explicit:

```text
V2 releases require a clean worktree and index because Nekocli commits only the generated release state and declared materialized files.
```

Dry-run planning does not perform network operations and does not stage, commit, tag, push, publish, or dispatch.

## Staging

The coordinator stages only known release files with safe Git argument passing:

```text
git add -- .neko/release.state.json <materialized files>
```

After staging, `git diff --cached --name-only` must exactly match the known release file set. Additional or missing staged files are errors. If targeted staging needs cleanup before the commit boundary, only the known release files are unstaged. No global `git reset` is used and foreign index entries are not removed.

## Commit

The V2 release commit is deterministic:

```text
chore(release): <unit-id> <tag>
```

Examples:

```text
chore(release): api api/v0.2.1
chore(release): web web/v1.4.0
chore(release): default v2.2.5
```

After commit, the coordinator verifies that `HEAD` is the created commit, the commit contains exactly the known release files, and `.neko/release.state.json` contains the selected unit's `NextVersion`.

## Unit Tag

The coordinator creates a lightweight Git tag, matching the existing V1 tag style, but it uses `ReleaseExecutionContext.Tag` and validates it through the selected unit `TagSpec`. It never builds tags from a global `v%s` format.

If the tag already points to the same release commit, the operation is idempotently accepted. If it points to another commit, coordination stops with a conflict and no tag is moved.

## Push

Push order is explicit:

```text
1. git push <remote> HEAD:<upstream-branch>
2. git push <remote> refs/tags/<unit-tag>:refs/tags/<unit-tag>
```

The coordinator does not use `--follow-tags`, does not push all tags, does not push foreign branches, and does not delete remote refs. If commit push fails, tag push is not attempted. If tag push fails after the commit was pushed, no remote rollback is attempted.

## Recovery Boundary

Before the commit boundary, state and materialization transactions may restore their own snapshots and unstage only their known files. After commit, tag, or push starts, V2 does not run `git reset --hard`, `git clean -fd`, remote tag deletion, or GitHub release deletion.

The result model records unit, version, tag, commit SHA, selected remote, created/pushed booleans, reached phase, known files, and recovery guidance. Public V2 GitHub Actions releases now use the execution journal before mutation and dispatch journal before workflow dispatch.

Execution journals are stored outside the worktree under:

```text
<git-common-dir>/neko/release/executions/<sha256>.json
```

They do not affect Git cleanliness and do not enter release commits.

## Public Boundary

Public V2 GitHub Actions non-dry-run release commands are active for Nekocli `cli`, `plugin-release`, and `plugin-ui`. Every successful handoff dispatches a workflow that publishes a GitHub Release for the exact pushed tag. V2 local non-dry-run release commands remain blocked until publish-only adapter boundaries exist.

V1 behavior is unchanged.
