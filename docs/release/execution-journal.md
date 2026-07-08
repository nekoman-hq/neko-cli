# Release Execution Journal

Milestone 5C3B uses this durable journal for public V2 GitHub Actions release execution. It is separate from the GitHub Actions dispatch journal.

## Purpose

`ReleaseExecutionJournal` covers interruption before a dispatch request can exist:

```text
materialization
state write
staging
release commit
unit tag
commit push
tag push
dispatch-journal handoff
```

`DispatchJournal` starts later and covers only one GitHub Actions HTTP dispatch attempt.

## Location

Execution journals are stored outside the worktree:

```text
<git-common-dir>/neko/release/executions/<sha256>.json
```

The Git common directory is resolved through Git, so normal repositories and linked worktrees use the correct shared metadata location. Journals are not written under `.neko/`, are not added to `.gitignore`, and do not affect V2 clean-worktree checks.

## Identity

The execution identity is computed before the release commit exists. It includes:

```text
repository remote identity
base commit SHA
unit ID
current version
next version
target tag
executor
delivery
workflow path when configured
```

The canonical identity is hashed with SHA-256. Raw unit IDs, tags, workflow paths, and other user-controlled fragments are never used as journal filenames.

## Content

The journal records immutable release intent, latest confirmed phase, pending action, known release file metadata, commit/tag/push metadata, and recovery metadata.

Known release files store safe metadata only:

```text
repository-relative path
expected existence before mutation
expected existence after mutation
preimage SHA-256 when present
postimage SHA-256 when present
required-for-release-commit flag
reason
```

Arbitrary file bytes, tokens, authorization headers, secrets, environment values, and GitHub credentials are not stored.

## Phases

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

There is no generic `failed` state. Errors are recorded separately so the last confirmed safe boundary remains visible.

## Pending Actions

Before a mutation, the future integration must persist `PendingAction`. After the mutation succeeds, it must confirm the phase and clear the marker.

Supported pending actions:

```text
none
apply-materialization
write-state
stage-release-files
create-release-commit
create-unit-tag
create-dispatch-journal
push-release-commit
push-unit-tag
```

A pending action cannot be replaced by another pending action. The public `resume` command can continue only exact, unambiguous V2 GitHub Actions execution journals. It does not blindly retry uncertain push or dispatch outcomes, and no automatic rollback, restore, reset, clean, publish, standalone retry, or standalone dispatch command exists.

## Public Boundary

Public V2 GitHub Actions non-dry-run releases create and update this journal. V2 local non-dry-run releases remain blocked. `resume` uses this journal to continue only an exact existing release intent.
