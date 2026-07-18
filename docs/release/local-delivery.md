# Local Delivery

Local delivery is the delivery contract where Neko CLI runs the selected executor on the current machine.

## Delivery Values

| Delivery | Meaning | Local execution |
|----------|---------|-----------------|
| `local` | V1 local release delivery; invalid for executable V2 releases | V1 only |
| `github-actions` | Remote workflow delivery with validated workflow path | not local; non-dry-run uses the GitHub Actions release flow |

Unknown delivery values are rejected. For V2, `github-actions` is the only supported delivery mode. Existing V2 configs containing `delivery: "local"` are parsed only far enough to report a clear unsupported-delivery validation error.

## Context Roots

The local delivery context uses absolute paths:

```text
RepositoryRoot = absolute Git root
UnitRoot       = RepositoryRoot for V1
UnitRoot       = RepositoryRoot + workingDirectory for V2
```

V2 `workingDirectory` must be relative, stay inside the repository, and exist. Requirement files are checked under `UnitRoot`, not the process working directory. This validation still applies to GitHub Actions units because executor configuration is checked from the unit root.

## Current Boundary

V1 local release behavior is unchanged. V2 `patch`, `minor`, and `major` dry-runs build plans, check requirements, and show state, materialization, Git ownership, release commit, unit tag, known files, workflow dispatch facts, and push order without writing files.

V2 local executor execution is deliberately not configurable. Running an executor process on the current machine does not imply offline behavior or no remote side effects. The current supported executors do not expose a proven publish-only boundary that would let Neko CLI own version materialization, state, release commit, tag, push ordering, interruption evidence, and compensation without also making ambiguous remote publication claims.

GitHub Actions delivery requires `workflow` in canonical `.github/workflows/<file>.yml|yaml` form. V2 GitHub Actions non-dry-run releases are active, journaled, and dispatch the configured workflow after Neko CLI has committed, tagged, and pushed the unit release. See [GitHub Actions delivery](github-actions-delivery.md) and [GitHub Actions release flow](github-actions-release-flow.md).
