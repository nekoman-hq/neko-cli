# Local Delivery

Local delivery is the delivery contract where Neko CLI runs the selected executor on the current machine.

## Delivery Values

| Delivery | Meaning | Local execution |
|----------|---------|-----------------|
| `local` | Execute the release locally through the configured executor | supported |
| `github-actions` | Remote workflow delivery | recognized, not dispatched |

Unknown delivery values are rejected.

## Context Roots

The local delivery context uses absolute paths:

```text
RepositoryRoot = absolute Git root
UnitRoot       = RepositoryRoot for V1
UnitRoot       = RepositoryRoot + workingDirectory for V2
```

V2 `workingDirectory` must be relative, stay inside the repository, and exist. Requirement files are checked under `UnitRoot`, not the process working directory.

## Current Boundary

V1 local release behavior is unchanged. V2 `patch`, `minor`, and `major` dry-runs build plans, check requirements, and show state/ownership information without writing files.

V2 non-dry-run local release is enabled for `goreleaser` and `jreleaser` through the local release transaction. Required version files are materialized before state write and before the commit/tag boundary. `release-it` is blocked until the root state commit guarantee can be implemented and tested. `github-actions` remains recognized but not dispatched.
