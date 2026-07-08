# Local Delivery

Local delivery is the delivery contract where Neko CLI runs the selected executor on the current machine.

## Delivery Values

| Delivery | Meaning | Local execution |
|----------|---------|-----------------|
| `local` | Local release delivery | dry-run only for public V2 commands |
| `github-actions` | Remote workflow delivery with validated workflow path | not local; non-dry-run uses the GitHub Actions release flow |

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

V2 non-dry-run local release commands are blocked until publish-only adapters exist. Internally, Neko CLI has the `GitReleaseCoordinator` needed to stage known release files, create the release commit, create the unit tag, and push commit then tag. `release-it` remains blocked for V2 local delivery because it cannot yet be constrained to publish-only behavior.

GitHub Actions delivery requires `workflow` in canonical `.github/workflows/<file>.yml|yaml` form. V2 GitHub Actions non-dry-run releases are active, journaled, and dispatch the configured workflow after Neko CLI has committed, tagged, and pushed the unit release. See [GitHub Actions delivery](github-actions-delivery.md) and [GitHub Actions release flow](github-actions-release-flow.md).
