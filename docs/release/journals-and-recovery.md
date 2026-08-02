# Release Journals and Recovery

> **Audience:** Release operators diagnosing an interrupted V2 GitHub Actions release and contributors maintaining recovery semantics.
>
> **Purpose:** Define durable execution evidence, dispatch outcomes, recovery classification, Resume behavior, and safety limits.

V2 uses two journals outside the worktree. The execution journal covers local
release preparation through handoff. The dispatch journal covers one GitHub
Actions HTTP attempt. They are evidence, not configuration or version state.

## Storage and identity

Journals live below the Git common directory:

```text
<git-common-dir>/neko/release/executions/<sha256>.json
<git-common-dir>/neko/release/dispatches/<sha256>.json
```

This placement supports normal repositories and linked worktrees without
affecting clean-worktree checks or release commits. A canonical identity is
hashed with SHA-256; raw tags, units, workflows, and other user-controlled text
never become filename fragments.

Execution identity is known before the release commit and includes remote
identity, base commit, unit, current and next versions, tag, executor, delivery,
and workflow. Dispatch identity additionally binds the verified release commit
and exact dispatch request. Moving the same remote checkout does not create a
different remote identity.

Journals never store tokens, authorization headers, credentials, environment
values, or arbitrary file bytes.

## Execution journal

`ReleaseExecutionJournal` records immutable release intent, known-file hashes,
the last confirmed phase, a pending action, Git metadata, and recovery guidance.
Known-file evidence contains relative paths, expected before/after existence,
preimage and postimage hashes, release-commit inclusion, and reason.

Confirmed phases are monotonic:

```text
prepared
preflight-validated
materialization-applied
state-written
release-files-staged
commit-created
tag-created
dispatch-journal-prepared
commit-pushed
tag-pushed
handoff-ready
```

There is no generic failed phase. An error is recorded separately so the last
confirmed boundary remains usable.

Before each mutation, the runner persists one pending action. After success it
confirms the phase and clears the marker:

```text
apply-materialization
write-state
stage-release-files
create-release-commit
create-unit-tag
create-dispatch-journal
push-release-commit
push-unit-tag
```

A pending action cannot be replaced by another pending action.

## Dispatch journal

`DispatchJournal` is created only after a verified release commit and unit tag
exist. It records the repository target, unit, version, tag, release SHA,
workflow, executor, delivery, canonical inputs, timestamps, safe error fields,
and recovery guidance.

States are:

| State | Meaning |
| --- | --- |
| `prepared` | Request is locally validated; no HTTP request has started. |
| `request-started` | The state was persisted immediately before the request; interruption is uncertain. |
| `accepted` | GitHub returned a `2xx` response. |
| `rejected` | GitHub returned `400`, `401`, `403`, `404`, `422`, or `429`. |
| `unknown` | Timeout, interruption, redirect, `5xx`, cancellation after start, or another ambiguous response. |

Valid transitions are `prepared -> request-started`, followed by exactly one of
`accepted`, `rejected`, or `unknown`. An identical `prepared` journal can be
reused. A journal in any later state is preserved and not overwritten.

Corrupt content or identity mismatch fails closed. Neko CLI does not delete or
repair the journal automatically. A run URL is shown only when GitHub provides
resolvable metadata; otherwise output states that it is not resolved.

## Recovery assessment

Recovery uses local evidence only. It inspects the execution journal, current
`HEAD`, local tag, known files, state metadata, index state where knowable, and
push markers already recorded in the journal. It does not fetch or infer remote
success.

Assessment statuses are:

| Status | Local interpretation |
| --- | --- |
| `not-started` | Prepared journal with no confirmed mutation. |
| `interrupted-before-commit` | Known-file mutation may exist; no commit boundary is confirmed. |
| `interrupted-after-commit` | Expected local release commit exists; tag is not confirmed. |
| `interrupted-after-tag` | Expected local tag points to the release commit. |
| `interrupted-before-push` | Local staging is confirmed; push is not. |
| `interrupted-after-commit-push` | Commit push marker is recorded; tag push is not. |
| `interrupted-after-tag-push` | Tag push marker is recorded; dispatch handoff is not. |
| `ready-for-dispatch` | Dispatch journal preparation is confirmed. |
| `already-handed-off` | Execution handoff is complete; dispatch journal is authoritative. |
| `conflicted` | Journal facts and repository state disagree. |
| `corrupted` | Journal structure or evidence cannot be trusted. |

The assessor reports `conflicted` or `corrupted` instead of guessing.

## Resume

```bash
neko release resume --unit api --dry-run
neko release resume --unit api
```

Resume continues one existing unresolved V2 GitHub Actions execution. It does
not calculate a new version, choose another tag, create a second release intent,
or act as a general retry command.

Dry-run performs read-only assessment and does not require `GITHUB_TOKEN`.
Non-dry-run resumes only exact, unambiguous local operations. It blocks when a
push or dispatch result may already have occurred remotely. Dispatch states
`request-started`, `rejected`, and `unknown` do not cause another automatic HTTP
request.

Failure and structured output expose the last confirmed phase, pending action,
unit, version, tag, release commit when known, push markers, journal paths,
dispatch state, optional run URL, and recovery guidance.

## Operator procedure

1. Keep the release tag and both journals intact.
2. Run `neko release resume --unit <unit> --dry-run`.
3. Compare local evidence with GitHub Actions and GitHub Releases when the assessor reports remote uncertainty.
4. Run non-dry-run Resume only when the assessment reports an exact resumable operation.

No standalone dispatch or retry command exists. Neko CLI does not automatically
reset, clean, delete refs, delete releases, restore after the Git boundary, or
retry an uncertain remote operation.

## Related documentation

- [Release lifecycle](lifecycle.md)
- [GitHub Actions delivery](github-actions-delivery.md)
- [Release command reference](cli-reference.md#resume)
