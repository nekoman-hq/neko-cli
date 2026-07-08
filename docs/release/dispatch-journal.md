# Dispatch Journal

Dispatch journals record the GitHub Actions workflow-dispatch attempt used by public V2 GitHub Actions releases. They are distinct from release execution journals.

`ReleaseExecutionJournal` covers local V2 release execution through materialization, state write, staging, commit, tag, push, and dispatch handoff. `DispatchJournal` starts only after a verified release commit and unit tag exist and covers one GitHub Actions HTTP dispatch attempt.

## Location

Dispatch journals are stored outside the repository worktree:

```text
<git-common-dir>/neko/release/dispatches/<sha256>.json
```

The Git common directory is resolved through Git so normal repositories and Git worktrees are supported. Journals are not written under `.neko/`, are not added to `.gitignore`, and cannot enter release commits or disturb the V2 clean-worktree rule.

The filename is the SHA-256 dispatch identity. It does not contain tags, unit names, workflow paths, or user-controlled path fragments.

## Content

The journal stores release facts and state:

```text
identity
repositoryRemoteName
repositoryRemote
unit
version
tag
releaseCommitSHA
workflowPath
workflowFileName
executor
delivery
inputs
state
createdAt
updatedAt
lastError
dispatchMetadata
recoveryGuidance
```

It must not contain tokens, authorization headers, secrets, environment values, or GitHub credentials.

## States

```text
prepared
request-started
accepted
rejected
unknown
```

`prepared` means the request is fully validated locally and no HTTP request has been attempted.

`request-started` is reserved for the exact moment immediately before a future HTTP request. If the process stops after this state, the outcome is uncertain and must not be retried automatically.

`accepted` means GitHub returned a `2xx` response.

`rejected` means GitHub returned a definitive rejection: `400`, `401`, `403`, `404`, `422`, or `429`.

`unknown` means the outcome cannot be determined safely: timeout, transport interruption, context cancellation after request start, redirect, `5xx`, or another unexpected response.

The valid transitions are `prepared -> request-started`, then `request-started -> accepted`, `request-started -> rejected`, or `request-started -> unknown`.

## Idempotency

If no journal exists, the V2 GitHub Actions release path may create a `prepared` journal after the local release commit and unit tag exist.

If a `prepared` journal with identical identity and content exists, it is reused.

If the journal state is `request-started`, `accepted`, `rejected`, or `unknown`, the journal is preserved and the caller receives recovery guidance. The journal is not overwritten.

If the journal file is corrupt or does not match the expected request identity, the operation fails clearly. Neko CLI does not delete or rewrite it automatically.

`resume` may inspect an existing dispatch journal. If the dispatch journal is `request-started`, `rejected`, or `unknown`, resume does not issue another HTTP request automatically. Unknown outcomes require manual inspection of GitHub Actions and this journal before any future explicit retry design.
