# Release Plugin Package Ownership

> **Audience:** Contributors choosing or reviewing a Release Plugin package boundary.
>
> **Purpose:** Provide the concise canonical ownership and dependency-direction map for Release implementation packages.

## Authority

This is the concise current package and dependency view for the Release Plugin.
[current-state.md](current-state.md) remains authoritative for detailed runtime,
disk, I/O, and lifecycle contracts. Historical audits and implementation
sequences are preserved in the [Release documentation history](../history/README.md).

## Dependency direction

```text
plugin protocol and compatibility facades
                    |
                    v
focused command capabilities and release orchestration
                    |
                    v
consumer-owned ports and canonical facts
                    |
                    v
concrete filesystem, Git, process, and HTTP adapters
```

Composition occurs in `plugin/release/main.go`, public compatibility facades,
or command boundaries. Canonical facts do not depend upward on presentation,
Doctor diagnostics, journals, or HTTP. Extracted command capabilities do not
import the root Release lifecycle package.

## Responsibility map

| Package | Current responsibility |
| --- | --- |
| `plugin/release` | Plugin protocol, one explicit repository root, fresh V1 executor composition, and command routing |
| `internal/releasetool` and format subpackages | Shared identity plus format-specific configuration, invocation, artifact, and version facts |
| `internal/releaseworkflow` | Canonical workflow identity, dispatch inputs, repository target, and local consumer-operation facts |
| `internal/githubdispatch` | One bounded workflow-dispatch POST transport with response sanitization |
| `internal/releasesource` | Tolerant local read-only V1/V2 source classification |
| `internal/localaction` | Read-only repository-local composite action resolution into effective workflow steps |
| `internal/doctor` | Local inspection, optional bounded GET verification, diagnostics, and presentation |
| `internal/unitoverview` | Local read-only V2 unit inventory and presentation |
| `internal/pipelineinspection` | Immutable configured, runtime, and verification projection; no lifecycle implementation |
| `internal/contextvalidation` | Local read-only dispatched-context validation and presentation |
| `internal/workflowinit` | Canonical workflow preview and create-only persistence |
| `internal/legacyrequirements` | Retained V1 token/configuration requirements for compatibility and Validate |
| `pkg/config` | V1/V2 models, strict loading, validation, normalization, paths, atomic writes, and pair recovery |
| `pkg/release` | Authoritative V1/V2 planning and lifecycle, Git coordination, journals, handoff, resume, recovery, and public facades |
| `pkg/release/tool/*` | Behavior-preserving V1 process, filesystem, Git, and compatibility adapters |
| `pkg/init` | V2 pair initialization and unit addition |
| `pkg/migrate` | Separate V1-to-V2 migration lifecycle and recovery |
| `pkg/evidence` | Redacted evidence queries and guarded completed-evidence archival |
| `pkg/validate`, `pkg/history`, `pkg/contributors` | Focused read-only queries with consumer-owned reads |
| `pkg/pluginindex` | Query, deterministic byte construction, output policy, and optional atomic persistence |
| `pkg/workspace` | Explicit repository roots plus bounded cwd compatibility helpers |

## Authoritative lifecycle ownership

`pkg/release` retains only responsibilities that define or consume the release
identity and its safety protocol: source selection, V1 planning and execution,
V2 execution-context and plan construction, exact mutation ordering,
materialization and state restoration, Git commit/tag/push coordination,
execution and dispatch journal policy, accepted handoff classification, and
resume/recovery decisions. These responsibilities must stay together because
they share the authoritative lifecycle evidence and cannot be safely duplicated
in a generic pipeline or command capability.

Doctor, Units, Workflow Init, and Context Validation root files are thin
aliases, wrappers, or forwarders to their focused internal owners. Pipeline
root composition is the deliberate read-only exception: it reads authoritative
journal, local Git, recovery, and neutral Doctor facts, then passes immutable
data to the internal projector. It owns no writer, dispatch transport, retry,
or duplicate transition policy.

## Read-only and mutation boundaries

- Plan, Units, default Pipeline, Validate, History, Contributors, Evidence
  query, and Context Validation receive read capabilities only and remain
  offline and token-free.
- Doctor is offline and token-free by default. Explicit remote verification
  adds only its bounded GET reader and lazy typed token boundary.
- Explicit remote Pipeline verification delegates to the same Doctor-owned GET
  boundary and cannot change lifecycle status, resume eligibility, or retry
  safety.
- Workflow Init owns create, unchanged, or conflict behavior and cannot update
  an existing workflow.
- GitHub dispatch owns one POST; lifecycle code owns token resolution, journal
  transitions, request identity, result classification, and no-retry policy.
- Init and Migration reuse the config-owned pair persister. Plugin Index keeps
  query, byte construction, path policy, and persistence separate.

## Compatibility boundaries

The active compatibility inventory remains in
[compatibility-notes.md](compatibility-notes.md) and the detailed V1 decision
register remains in [v1-compatibility-policy.md](v1-compatibility-policy.md).
Changed-code cohesion and metric exceptions remain in
[maintainability-policy.md](maintainability-policy.md). These are living
controls, not archived completion records.

Current bounded debt includes V1 manual recovery for uncertain remote or
executor effects, evidence-driven refusal of corrupt or owner-ambiguous pair
and migration states, retained cwd and registry compatibility facades, and the
absence of durable workflow-run and publication-completion inspection. The
current decision boundary is recorded in
[architecture-decisions.md](architecture-decisions.md).
