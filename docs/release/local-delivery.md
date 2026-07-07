# Local Delivery

Local delivery is the delivery contract where Neko CLI runs the selected executor on the current machine.

## Delivery Values

| Delivery | Meaning | Local execution |
|----------|---------|-----------------|
| `local` | Local release delivery | dry-run only for public V2 commands in this milestone |
| `github-actions` | Remote workflow delivery with validated workflow path | recognized, not dispatched |

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

V1 local release behavior is unchanged. V2 `patch`, `minor`, and `major` dry-runs build plans, check requirements, and show state, materialization, Git ownership, release commit, unit tag, known files, and push order without writing files.

V2 non-dry-run local release commands are blocked until publish-only adapters exist. Internally, Neko CLI now has the `GitReleaseCoordinator` needed to stage known release files, create the release commit, create the unit tag, and push commit then tag. `release-it` remains blocked, and `github-actions` remains recognized but not dispatched.

GitHub Actions delivery requires `workflow` in canonical `.github/workflows/<file>.yml|yaml` form. Validation checks that the file exists in real repositories, but no workflow is contacted or queued. See [GitHub Actions delivery](github-actions-delivery.md).
