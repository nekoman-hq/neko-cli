# Release Lifecycle

> **Audience:** Release operators and contributors who need the executable V2 transaction and its ownership boundaries.
>
> **Purpose:** Define planning, materialization, Git coordination, workflow handoff, and the unsupported local-delivery boundary.

This document owns the lifecycle concepts behind `patch`, `minor`, and `major`.
The exact commands, flags, output fields, and exit behavior are owned by the
[Release command reference](cli-reference.md).

## Lifecycle summary

V2 GitHub Actions delivery separates repository mutation from publication:

```text
resolve repository and unit
  -> calculate next version and tag
  -> validate executor requirements
  -> plan materialization
  -> preflight Git and credentials
  -> create execution journal
  -> apply materialization and state
  -> stage known release files
  -> create release commit and unit tag
  -> create dispatch journal
  -> push commit, then tag
  -> dispatch consumer workflow
  -> record handoff-ready
```

Neko CLI owns policy, version authority, state, the release commit, tag, push
order, journals, and the dispatch request. The consumer workflow owns build,
artifact production, GitHub Release creation, and publication from the pushed
tag. An accepted dispatch proves handoff, not publication completion.

## Planning and dry-run

Planning resolves the repository source, selected unit, current and next
version, tag, executor, delivery, workflow, known files, materialization, and
dispatch inputs. `plan` and release `--dry-run` are read-only.

A dry-run does not:

- write configuration, state, manifests, or executor files;
- stage, commit, tag, push, or clean Git state;
- create execution or dispatch journals;
- resolve `GITHUB_TOKEN`;
- contact GitHub;
- start a release executor;
- publish or roll back anything.

## Executor and requirement boundary

The selected executor's requirement file is checked below the unit
`workingDirectory`:

| Executor | Requirement file | V2 local execution | V2 GitHub Actions delivery |
| --- | --- | --- | --- |
| `goreleaser` | `.goreleaser.yml` or `.goreleaser.yaml` | unsupported | workflow-owned |
| `jreleaser` | `jreleaser.yml` | unsupported | workflow-owned |
| `release-it` | `.release-it.json` | unsupported | workflow-owned |

Neko CLI does not start these executors on the local V2 GitHub Actions path.
The workflow reads executor and delivery from the checked-out V2
configuration. V1 continues to use its compatibility adapters and their
executor-specific behavior.

## Version materialization

State is authoritative, but a release commit can also require version-bearing
files. `MaterializationPlan` records repository-relative target, before/after
evidence, file mode, reason, and whether the file belongs in the release commit.
State itself is owned by the separate state transaction.

Current materializers are:

- GoReleaser units are tag/context based by default.
- A Neko CLI plugin unit materializes only its configured plugin manifest to the selected next version.
- JReleaser materializes only `project.version` in `jreleaser.yml`.
- release-it has no V2 materialization because its local ownership of commit, tag, push, and publication conflicts with Neko-owned Git coordination.

Targets must resolve inside the repository. Symlinked materialization targets
are rejected. Before the Git boundary, the transaction can restore changed
files byte-for-byte or remove files it created; it never runs global Git
cleanup.

## Known release files

The release commit contains exactly:

```text
.neko/release.state.json
materialization files marked RequiredForReleaseCommit
```

For this repository, `plugin-release` includes
`plugin/release/manifest.json`, `plugin-ui` includes
`plugin/ui/manifest.json`, and `cli` has no plugin manifest materialization.
Plugin Index output is not a known release file and is not committed.

Every known file is inside the repository and has an auditable relative path.
The coordinator stages only that set and verifies the staged set exactly before
commit.

## Git preflight and commit

Executable V2 coordination requires:

- the resolved repository root is the Git worktree root;
- the current branch and its single upstream are resolvable;
- worktree and index are clean before release preparation;
- the unit tag does not already identify another release;
- every known release path is valid.

The commit subject is deterministic:

```text
chore(release): <unit-id> <tag>
```

After commit, Neko CLI verifies that `HEAD` is the created commit, the commit
contains exactly the known files, and state contains the selected next version.

## Tag and push

Neko CLI creates a lightweight tag using the selected unit's exact `TagSpec`.
An existing tag at the release commit is accepted idempotently; a tag at another
commit is a conflict and is never moved.

Push order is fixed:

```text
git push <remote> HEAD:<upstream-branch>
git push <remote> refs/tags/<unit-tag>:refs/tags/<unit-tag>
```

The coordinator does not use `--follow-tags`, push unrelated refs, or delete
remote refs. A failed commit push prevents tag push. A failed tag push does not
trigger remote rollback.

## Dispatch handoff

After the exact release commit and tag are verified and pushed, Neko CLI sends
one GitHub Actions workflow-dispatch request. The request is journaled before
the HTTP attempt and contains only:

```text
ref = <unit-tag>
inputs.unit
inputs.version
inputs.tag
inputs.release_sha
```

The target repository comes from the verified GitHub.com remote. The workflow
identifier comes from the validated configuration path. Tokens, paths,
executor values, arbitrary inputs, and credentials are not sent as workflow
inputs. See [GitHub Actions Delivery](github-actions-delivery.md) for the HTTP,
credential, and workflow contract.

## Recovery boundary

Before commit/tag coordination, state and materialization transactions can
restore only their known files and unstage only those paths. Once commit, tag,
push, or dispatch uncertainty exists, Neko CLI does not run destructive or
remote compensation:

- no `git reset --hard` or `git clean -fd`;
- no local or remote tag deletion;
- no remote branch rewrite;
- no GitHub Release deletion;
- no automatic retry of uncertain push or dispatch outcomes.

The execution and dispatch journals retain the last confirmed boundary.
`resume` continues only an exact unresolved release intent. Start with
`neko release resume --unit <unit> --dry-run`; see
[Journals and Recovery](journals-and-recovery.md).

## Local delivery boundary

V1 local release behavior remains supported compatibility. Executable V2
configuration accepts only `github-actions`; V2 local non-dry-run execution is
unsupported. Running a local executor does not prove a safe publish-only
boundary, and the supported executors can own overlapping commit, tag, push, or
publication behavior.

The unsupported boundary is enforced before state write, materialization,
staging, commit, tag, push, executor start, or publication. V2 dry-run remains
available because it performs planning without those effects.

## Related documentation

- [Release configuration and state](configuration.md)
- [GitHub Actions delivery](github-actions-delivery.md)
- [Journals and recovery](journals-and-recovery.md)
- [GitHub Actions golden path](github-actions-golden-path.md)
- [Compatibility](compatibility.md)
